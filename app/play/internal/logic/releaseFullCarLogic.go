package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gacha_srv/pkg/utils/txutil"
	"time"

	"gacha_srv/app/dtm/client/dtmgrpc"
	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ReleaseFullCarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReleaseFullCarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReleaseFullCarLogic {
	return &ReleaseFullCarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ReleaseFullCar SAGA 03 补偿：barrier 事务内
//
//	① 报价 FOR UPDATE 先行——LOCKED ⇔ 本 gid 确实占座（需回滚）；OPEN ⇒ 正向
//	   已整体回滚（如 3022 座位已占），座位保持原样——绝不误释放他人座位；
//	② 座位释放 + 余票+1（仅 LOCKED 时；seats[i]==quote 用户才清，已回滚幂等跳过）。
//	   这是座位与余票的唯一回滚点（05 补偿只撤销记录/整车奖/库存，不碰余票——防双扣）；
//	③ 回退券（LOCKED 且 locked_by=本报价 → AVAILABLE）；
//	④ 报价 LOCKED/OPEN→RELEASED（幂等；USED 不回退）。
//
// 幂等：报价 RELEASED/USED/EXPIRED → 直接成功。
func (l *ReleaseFullCarLogic) ReleaseFullCar(in *play.ReleaseFullCarReq) (*play.ReleaseFullCarResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("ReleaseFullCar get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("ReleaseFullCar get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

		// ① 报价读（经营销 RPC）：USED（07 已捕获核销）不回退；RELEASED/EXPIRED 幂等无操作
		q, qerr := l.svcCtx.MarketingRpc.GetQuote(l.ctx, &marketingClient.GetQuoteReq{QuoteToken: in.QuoteToken})
		if qerr != nil {
			var ce *errorx.CodeError
			if errors.As(qerr, &ce) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
			}
			return qerr
		}
		if q.GetStatus() == "USED" || q.GetStatus() == "RELEASED" || q.GetStatus() == "EXPIRED" {
			l.Infof("ReleaseFullCar idempotent: quote status %s, quoteToken: %s", q.GetStatus(), in.QuoteToken)
			return nil
		}

		// ② 座位释放 + 余票+1：仅当报价 LOCKED（正向提交过占座）时执行
		if q.GetStatus() == "LOCKED" {
			var st gamemodelnc.FullCarState
			err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("config_id = ?", in.ConfigId).First(&st).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil {
				seats := fullCarParseSeats(st.SeatsJson)
				seat := in.SeatIndex
				if seat < 0 {
					// 自动选座：按 userId 反查座位释放
					for i, u := range seats {
						if u == q.GetUserId() {
							seat = int64(i)
							break
						}
					}
				}
				if seat >= 0 && int(seat) < len(seats) && seats[seat] == q.GetUserId() {
					seats[seat] = 0
					remaining := st.RemainingTickets + 1
					if remaining > int64(len(seats)) {
						remaining = int64(len(seats)) // 防御：余票不超座位数
					}
					if err := db.Model(&gamemodelnc.FullCarState{}).Where("id = ?", st.Id).
						Updates(map[string]interface{}{
							"seats_json":        fullCarMarshalSeats(seats),
							"remaining_tickets": remaining,
							"updated_at":        time.Now(),
						}).Error; err != nil {
						return err
					}
					l.Infof("ReleaseFullCar seat released, configId: %d, seat: %d", in.ConfigId, seat)
				}
			}
		}

		// ③④ 券回退+报价→RELEASED 经营销 ReleaseQuote sub-call（幂等；USED 不回退）
		if _, err := l.svcCtx.MarketingRpc.ReleaseQuote(l.ctx, &marketingClient.ReleaseQuoteReq{
			RequestId:  in.RequestId,
			QuoteToken: in.QuoteToken,
		}); err != nil {
			var ce *errorx.CodeError
			if errors.As(err, &ce) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
			}
			return err
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("ReleaseFullCar aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("ReleaseFullCar failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读报价如实返回（经营销 RPC；NotFound 保持 RELEASED 旧同款容错）
		statusOut := "RELEASED"
		q, qe := l.svcCtx.MarketingRpc.GetQuote(l.ctx, &marketingClient.GetQuoteReq{QuoteToken: in.QuoteToken})
		if qe == nil {
			statusOut = q.GetStatus()
		} else {
			var ce *errorx.CodeError
			if !errors.As(qe, &ce) || ce.Code != errorx.CodeQuoteNotFound {
				return nil, status.Error(codes.Internal, qe.Error())
			}
		}
		l.Infof("ReleaseFullCar success (readback), quoteToken: %s, status: %s", in.QuoteToken, statusOut)
		return &play.ReleaseFullCarResp{SeatIndex: in.SeatIndex, Status: statusOut}, nil
	}
	l.Infof("ReleaseFullCar success, quoteToken: %s", in.QuoteToken)
	return &play.ReleaseFullCarResp{SeatIndex: in.SeatIndex, Status: "RELEASED"}, nil
}

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

type ReleaseComboChainLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReleaseComboChainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReleaseComboChainLogic {
	return &ReleaseComboChainLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ReleaseComboChain SAGA 03 补偿：barrier 事务内
//
//	① 报价 FOR UPDATE 先行——LOCKED ⇔ 本 gid 确实锁定购票（需回滚）；OPEN ⇒ 正向
//	   已整体回滚（如 3029/3030），参与者保持原样——绝不误回滚他人购票；
//	② 按 userId 弹出当前车参与者 + 余票+1（仅 LOCKED 且移除成功时；
//	   05 补偿重建参与者后本用户已不在其中 → 幂等跳过）。
//	   这是参与者与余票的唯一回滚点（05 补偿只撤销记录/结算奖/连击/车次，不碰余票——防双扣）；
//	③ 回退券（LOCKED 且 locked_by=本报价 → AVAILABLE）；
//	④ 报价 LOCKED/OPEN→RELEASED（幂等；USED 不回退）。
//
// 幂等：报价 RELEASED/USED/EXPIRED → 直接成功。
func (l *ReleaseComboChainLogic) ReleaseComboChain(in *play.ReleaseComboChainReq) (*play.ReleaseComboChainResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("ReleaseComboChain get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("ReleaseComboChain get raw db failed, err: %v", err)
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
			l.Infof("ReleaseComboChain idempotent: quote status %s, quoteToken: %s", q.GetStatus(), in.QuoteToken)
			return nil
		}

		// ② 弹出当前车参与者 + 余票+1：仅当报价 LOCKED（正向提交过锁定）时执行
		if q.GetStatus() == "LOCKED" {
			var st gamemodelnc.ComboChainState
			err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("config_id = ?", in.ConfigId).First(&st).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil {
				state, err := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson)
				if err == nil {
					removed := false
					kept := make([]int64, 0, len(state.Participants))
					for _, pid := range state.Participants {
						if !removed && pid == q.GetUserId() {
							removed = true
							continue
						}
						kept = append(kept, pid)
					}
					if removed {
						if err := db.Model(&gamemodelnc.ComboChainState{}).Where("id = ?", st.Id).
							Updates(map[string]interface{}{
								"participants_json": comboChainParticipantsJSON(kept),
								"remaining_tickets": st.RemainingTickets + 1,
								"updated_at":        time.Now(),
							}).Error; err != nil {
							return err
						}
						l.Infof("ReleaseComboChain participant popped, requestId: %s, userId: %d", in.RequestId, q.GetUserId())
					}
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
			l.Errorf("ReleaseComboChain aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("ReleaseComboChain failed, quoteToken: %s, err: %v", in.QuoteToken, err)
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
		l.Infof("ReleaseComboChain success (readback), quoteToken: %s, status: %s", in.QuoteToken, statusOut)
		return &play.ReleaseComboChainResp{Status: statusOut}, nil
	}
	l.Infof("ReleaseComboChain success, quoteToken: %s", in.QuoteToken)
	return &play.ReleaseComboChainResp{Status: "RELEASED"}, nil
}

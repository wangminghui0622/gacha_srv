package gameimpl

import (
	"context"
	"errors"
	"fmt"
	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/play/internal/playkit"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"
	"math/bits"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func NewReleaseDigBoxLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReleaseDigBoxLogic {
	return &ReleaseDigBoxLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ReleaseDigBox SAGA 03 补偿：barrier 事务内
//
//	① 报价 FOR UPDATE 先行——状态决定会话是否回滚：正向 LockDigBox 在同一
//	   barrier 事务内提交「报价 LOCKED + 会话置位」，两者同生共死，故
//	   quote LOCKED ⇔ 本 gid 确实置位（需回滚）；quote OPEN ⇒ 正向已整体
//	   回滚（如 3016 格已挖），会话保持原样——绝不误回滚预置的已挖格；
//	② 会话回滚（仅 LOCKED 时）：mine = bitmap & mask(cells)——只回滚本 gid
//	   实际置位的格子（已回滚则 mine=0 幂等跳过）；
//	③ 计数/实付回滚（唯一回滚点，05 补偿只撤销记录不碰计数，杜绝双扣）；
//	④ 报价 LOCKED/OPEN→RELEASED + 券 LOCKED→AVAILABLE（幂等；USED 不回退）。
//
// 幂等：报价 RELEASED/USED/EXPIRED → 直接成功。
func (l *ReleaseDigBoxLogic) ReleaseDigBox(in *play.ReleaseDigBoxReq) (*play.ReleaseDigBoxResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.SessionNo == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "releaseDigBox", func(db *gorm.DB) error {

		q, qerr := l.svcCtx.MarketingRpc.GetQuote(l.ctx, &marketingClient.GetQuoteReq{QuoteToken: in.QuoteToken})
		if qerr != nil {
			var ce *errorx.CodeError
			if errors.As(qerr, &ce) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
			}
			return qerr
		}
		if q.GetStatus() == "USED" || q.GetStatus() == "RELEASED" || q.GetStatus() == "EXPIRED" {
			l.Infof("ReleaseDigBox idempotent: quote status %s, quoteToken: %s", q.GetStatus(), in.QuoteToken)
			return nil
		}

		if q.GetStatus() == "LOCKED" {
			var sess gamemodelnc.DigBoxSessions
			err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("session_no = ?", in.SessionNo).First(&sess).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil {

				mask := digCellMask(in.CellIndexes)
				mine := sess.OccupiedBitmap & mask
				if mine != 0 {
					newBitmap := sess.OccupiedBitmap &^ mine
					newPaid := sess.TotalCellsPaid - int64(bits.OnesCount64(uint64(mine)))
					if newPaid < 0 {
						newPaid = 0
					}
					newPrice := sess.TotalPriceFen - in.TotalPriceFen
					if newPrice < 0 {
						newPrice = 0
					}
					newStatus := sess.Status
					if newStatus == digSessionCompleted {
						newStatus = digSessionPlaying
					}
					if err := db.Model(&gamemodelnc.DigBoxSessions{}).Where("id = ?", sess.Id).
						Updates(map[string]interface{}{
							"occupied_bitmap":  newBitmap,
							"total_cells_paid": newPaid,
							"total_price_fen":  newPrice,
							"status":           newStatus,
							"updated_at":       time.Now(),
						}).Error; err != nil {
						return err
					}
					l.Infof("ReleaseDigBox session rolled back, sessionNo: %s, cells: %d", in.SessionNo, bits.OnesCount64(uint64(mine)))
				}
			}
		}

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
			l.Errorf("ReleaseDigBox aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("ReleaseDigBox failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {

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
		l.Infof("ReleaseDigBox success (readback), quoteToken: %s, status: %s", in.QuoteToken, statusOut)
		return &play.ReleaseDigBoxResp{SessionNo: in.SessionNo, Status: statusOut}, nil
	}
	l.Infof("ReleaseDigBox success, sessionNo: %s, quoteToken: %s", in.SessionNo, in.QuoteToken)
	return &play.ReleaseDigBoxResp{SessionNo: in.SessionNo, Status: "RELEASED"}, nil
}

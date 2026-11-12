package gameimpl

import (
	"context"
	"errors"
	"fmt"
	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/play/internal/core"
	"gacha_srv/app/play/internal/playkit"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func NewReleaseGroupBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReleaseGroupBuyLogic {
	return &ReleaseGroupBuyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ReleaseGroupBuy SAGA 03 补偿：barrier 事务内
//
//	① 报价 FOR UPDATE 先行——LOCKED ⇔ 本 gid 确实锁定下注（需回滚）；OPEN ⇒ 正向
//	   已整体回滚（如 3032/3033），下注保持原样——绝不误回滚他人下注；
//	② 按 draw_id（=requestId）弹出下注 + 余位+=count（仅 LOCKED 且弹出成功时；
//	   已弹出则幂等跳过）。这是下注记录与余位的唯一回滚点
//	   （05 补偿只撤销记录/结算奖/库存/幸运，不碰余位——防双扣）；
//	③ 回退券（LOCKED 且 locked_by=本报价 → AVAILABLE）；
//	④ 报价 LOCKED/OPEN→RELEASED（幂等；USED 不回退）。
//
// 幂等：报价 RELEASED/USED/EXPIRED → 直接成功。
func (l *ReleaseGroupBuyLogic) ReleaseGroupBuy(in *play.ReleaseGroupBuyReq) (*play.ReleaseGroupBuyResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	outCount := in.Count
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "releaseGroupBuy", func(db *gorm.DB) error {

		q, qerr := l.svcCtx.MarketingRpc.GetQuote(l.ctx, &marketingClient.GetQuoteReq{QuoteToken: in.QuoteToken})
		if qerr != nil {
			var ce *errorx.CodeError
			if errors.As(qerr, &ce) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
			}
			return qerr
		}
		if q.GetStatus() == "USED" || q.GetStatus() == "RELEASED" || q.GetStatus() == "EXPIRED" {
			l.Infof("ReleaseGroupBuy idempotent: quote status %s, quoteToken: %s", q.GetStatus(), in.QuoteToken)
			return nil
		}

		if q.GetStatus() == "LOCKED" {
			var st gamemodelnc.GroupBuyState
			err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("config_id = ?", in.ConfigId).First(&st).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil {
				state, err := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson)
				if err == nil {
					removed := int64(0)
					kept := make([]core.GroupBuyBet, 0, len(state.Bets))
					for _, b := range state.Bets {
						if b.DrawID == in.RequestId {
							removed = int64(b.Count)
							continue
						}
						kept = append(kept, b)
					}
					if removed > 0 {
						if err := db.Model(&gamemodelnc.GroupBuyState{}).Where("id = ?", st.Id).
							Updates(map[string]interface{}{
								"bets_json":      groupBuyBetsJSON(kept),
								"remaining_bets": st.RemainingBets + removed,
								"updated_at":     time.Now(),
							}).Error; err != nil {
							return err
						}
						l.Infof("ReleaseGroupBuy bet popped, requestId: %s, count: %d", in.RequestId, removed)
					}
					outCount = removed
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
			l.Errorf("ReleaseGroupBuy aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("ReleaseGroupBuy failed, quoteToken: %s, err: %v", in.QuoteToken, err)
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
		l.Infof("ReleaseGroupBuy success (readback), quoteToken: %s, status: %s", in.QuoteToken, statusOut)
		return &play.ReleaseGroupBuyResp{Count: outCount, Status: statusOut}, nil
	}
	l.Infof("ReleaseGroupBuy success, quoteToken: %s, count: %d", in.QuoteToken, outCount)
	return &play.ReleaseGroupBuyResp{Count: outCount, Status: "RELEASED"}, nil
}

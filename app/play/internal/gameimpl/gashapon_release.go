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

func NewReleaseGashaponLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReleaseGashaponLogic {
	return &ReleaseGashaponLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ReleaseGashapon SAGA 03 补偿：barrier 事务内
//
//	① 报价 FOR UPDATE 先行——LOCKED ⇔ 本 gid 确实锁定抽数（需回滚）；OPEN ⇒ 正向
//	   已整体回滚（如 3025 余票不足），参与记录保持原样——绝不误回滚他人抽数；
//	② 按 requestId 弹出记录 + participants-=count + 余票+=count（仅 LOCKED 时；
//	   记录已弹出则幂等跳过）。这是参与记录与余票的唯一回滚点
//	   （05 补偿只撤销记录/特别奖/sent/库存，不碰余票——防双扣）；
//	③ 回退券（LOCKED 且 locked_by=本报价 → AVAILABLE）；
//	④ 报价 LOCKED/OPEN→RELEASED（幂等；USED 不回退）。
//
// 幂等：报价 RELEASED/USED/EXPIRED → 直接成功。
func (l *ReleaseGashaponLogic) ReleaseGashapon(in *play.ReleaseGashaponReq) (*play.ReleaseGashaponResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "releaseGashapon", func(db *gorm.DB) error {

		q, qerr := l.svcCtx.MarketingRpc.GetQuote(l.ctx, &marketingClient.GetQuoteReq{QuoteToken: in.QuoteToken})
		if qerr != nil {
			var ce *errorx.CodeError
			if errors.As(qerr, &ce) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
			}
			return qerr
		}
		if q.GetStatus() == "USED" || q.GetStatus() == "RELEASED" || q.GetStatus() == "EXPIRED" {
			l.Infof("ReleaseGashapon idempotent: quote status %s, quoteToken: %s", q.GetStatus(), in.QuoteToken)
			return nil
		}

		if q.GetStatus() == "LOCKED" {
			var st gamemodelnc.GashaponState
			err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("config_id = ?", in.ConfigId).First(&st).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil {
				state, err := gashaponParseState(st.ParticipantsJson, st.RecordsJson, st.SentJson, st.AwardsJson)
				if err == nil {
					removed := int64(0)
					kept := make([]core.GashaponParticipation, 0, len(state.Records))
					for _, r := range state.Records {
						if r.RequestID == in.RequestId {
							removed = r.Count
							continue
						}
						kept = append(kept, r)
					}
					if removed > 0 {

						state.Participants[q.GetUserId()] -= removed
						if state.Participants[q.GetUserId()] <= 0 {
							delete(state.Participants, q.GetUserId())
						}
						if err := db.Model(&gamemodelnc.GashaponState{}).Where("id = ?", st.Id).
							Updates(map[string]interface{}{
								"participants_json": gashaponParticipantsJSON(state.Participants),
								"records_json":      gashaponRecordsJSON(kept),
								"remaining_tickets": st.RemainingTickets + removed,
								"updated_at":        time.Now(),
							}).Error; err != nil {
							return err
						}
						l.Infof("ReleaseGashapon record popped, requestId: %s, count: %d", in.RequestId, removed)
					}
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
			l.Errorf("ReleaseGashapon aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("ReleaseGashapon failed, quoteToken: %s, err: %v", in.QuoteToken, err)
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
		l.Infof("ReleaseGashapon success (readback), quoteToken: %s, status: %s", in.QuoteToken, statusOut)
		return &play.ReleaseGashaponResp{Count: in.Count, Status: statusOut}, nil
	}
	l.Infof("ReleaseGashapon success, quoteToken: %s", in.QuoteToken)
	return &play.ReleaseGashaponResp{Count: in.Count, Status: "RELEASED"}, nil
}

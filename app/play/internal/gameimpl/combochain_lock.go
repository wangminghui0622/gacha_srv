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
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func NewLockComboChainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LockComboChainLogic {
	return &LockComboChainLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LockComboChain SAGA 03 正向：barrier 事务内
//
//	① 报价只读校验（营销 GetQuote sub-call：userId/grossFen 取报价行）；
//	② state FOR UPDATE（uk config_id 串行化同机购买；种子缺失防御建行，
//	   档位库存=配置 COMBO_TIER quantity、当前车 1、余票=tickets_per_car）；
//	③ 状态校验（非 OPEN → 3029；当前车重复购买 → 3030；余票<1 → 3029）；
//	④ 价格一致（3018 复用：quote.gross == price_fen == 请求总额）；
//	⑤ 报价锁（营销 LockQuote sub-call，置于全部玩法校验之后）；
//	⑥ participants 追加、余票-1。
//
// 报价锁放在最后：玩法自身校验失败回滚时不触碰营销锁（营销事务已独立提交，
// 无法随本事务回滚），与旧仓「锁与业务同事务」的最终状态一致——自中止后报价保持 OPEN。
// userId 取报价行（网关 QuoteCheckout 落库），不信请求参数。
// 错误语义（SAGA 分支铁律）：业务失败 → codes.Aborted（触发补偿）；系统错误 → codes.Internal（DTM 重试）。
func (l *LockComboChainLogic) LockComboChain(in *play.LockComboChainReq) (*play.LockComboChainResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "LockComboChain", func(db *gorm.DB) error {

		quote, err := playkit.LoadQuote(l.ctx, l.svcCtx, in.QuoteToken)
		if err != nil {
			return err
		}
		userId := quote.UserID

		// 连击链配置（纯种子配置只读）
		var cfg gamemodel.ComboChainConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链配置不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		if cfg.Status != 1 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链配置不可用", errorx.CodeGameConfigInvalid))
		}

		// ② state FOR UPDATE（uk(config_id) 串行化同机并发购买）
		var st gamemodelnc.ComboChainState
		err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {

			shared, perr := comboChainParseShared(cfg.SharedPrizesJson)
			if perr != nil {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链共享奖配置无效", errorx.CodeGameConfigInvalid))
			}
			tiers := core.BuildComboTierStock(shared, nil)
			tierRemaining := make([]int32, len(tiers))
			for i := range tiers {
				tierRemaining[i] = tiers[i].Remaining
			}
			st = gamemodelnc.ComboChainState{
				ConfigId:          in.ConfigId,
				CarIndex:          1,
				ParticipantsJson:  "[]",
				AwardsJson:        "[]",
				TierRemainingJson: comboChainTierRemainingJSON(tierRemaining),
				StreaksBeforeJson: "{}",
				RemainingTickets:  cfg.TicketsPerCar,
				Status:            comboChainStateOpen,
				CreatedAt:         time.Now(),
				UpdatedAt:         time.Now(),
			}
			if err := db.Create(&st).Error; err != nil {
				if errors.Is(err, gorm.ErrDuplicatedKey) {

					if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
						Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
						return err
					}
				} else {
					return err
				}
			}
		} else if err != nil {
			return err
		}
		if st.Status != comboChainStateOpen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链已结算或状态不满足锁定", errorx.CodeComboChainNoTickets))
		}

		state, err := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链状态数据异常", errorx.CodeGameConfigInvalid))
		}
		for _, pid := range state.Participants {
			if pid == userId {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链当前车重复购买", errorx.CodeComboChainDupBuy))
			}
		}
		if st.RemainingTickets < 1 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链当前车已满或已结算", errorx.CodeComboChainNoTickets))
		}

		if quote.GrossFen != cfg.PriceFen || in.TotalPriceFen != cfg.PriceFen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链价格校验不一致", errorx.CodeDigPriceMismatch))
		}

		newLocked, err := playkit.LockQuote(l.ctx, l.svcCtx, in.RequestId, in.QuoteToken)
		if err != nil {
			return err
		}
		if !newLocked {
			l.Infof("LockComboChain idempotent: quote already locked, quoteToken: %s", in.QuoteToken)
			return nil
		}

		state.Participants = append(state.Participants, userId)
		if err := db.Model(&gamemodelnc.ComboChainState{}).Where("id = ?", st.Id).
			Updates(map[string]interface{}{
				"participants_json": comboChainParticipantsJSON(state.Participants),
				"remaining_tickets": st.RemainingTickets - 1,
				"status":            comboChainStateOpen,
				"updated_at":        time.Now(),
			}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("LockComboChain aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("LockComboChain failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {

		statusOut := "LOCKED"
		q, qe := l.svcCtx.MarketingRpc.GetQuote(l.ctx, &marketingClient.GetQuoteReq{QuoteToken: in.QuoteToken})
		if qe == nil {
			statusOut = q.GetStatus()
		} else {
			var ce *errorx.CodeError
			if !errors.As(qe, &ce) || ce.Code != errorx.CodeQuoteNotFound {
				return nil, status.Error(codes.Internal, qe.Error())
			}
		}
		l.Infof("LockComboChain success (readback), quoteToken: %s, status: %s", in.QuoteToken, statusOut)
		return &play.LockComboChainResp{Status: statusOut}, nil
	}
	l.Infof("LockComboChain success, quoteToken: %s", in.QuoteToken)
	return &play.LockComboChainResp{Status: "LOCKED"}, nil
}

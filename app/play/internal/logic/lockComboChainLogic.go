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
	"gacha_srv/app/play/internal/core"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type LockComboChainLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

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

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("LockComboChain get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("LockComboChain get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

		// ① 报价只读校验（营销 GetQuote sub-call；quote 不存在 → Aborted）
		quoteResp, err := l.svcCtx.MarketingRpc.GetQuote(l.ctx, &marketingClient.GetQuoteReq{
			QuoteToken: in.QuoteToken,
		})
		if err != nil {
			var ce *errorx.CodeError
			if errors.As(err, &ce) {
				// 营销业务失败（报价不存在）→ Aborted 触发补偿
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
			}
			return err
		}
		userId := quoteResp.GetUserId()

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
			// 防御：种子缺失时建行（档位库存=配置 quantity、当前车 1、余票=tickets_per_car）
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
					// 并发首建竞争：重读（行锁串行化后可见对方状态）
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

		// ③ 当前车校验：每用户每车限 1 票（3030）；余票 ≥1（3029）
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

		// ④ 价格一致（锁内重算）：单价 == 报价 gross == 请求总额
		if quoteResp.GetGrossFen() != cfg.PriceFen || in.TotalPriceFen != cfg.PriceFen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链价格校验不一致", errorx.CodeDigPriceMismatch))
		}

		// ⑤ 报价锁（营销 LockQuote sub-call，置于玩法校验之后；幂等：newLocked=false=已锁重发 → 业务已成功，直接返回）
		lockResp, err := l.svcCtx.MarketingRpc.LockQuote(l.ctx, &marketingClient.LockQuoteReq{
			RequestId:  in.RequestId,
			QuoteToken: in.QuoteToken,
		})
		if err != nil {
			var ce *errorx.CodeError
			if errors.As(err, &ce) {
				// 营销业务失败（过期/状态非法/券被占）→ Aborted 触发补偿
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
			}
			return err
		}
		if !lockResp.GetNewLocked() {
			l.Infof("LockComboChain idempotent: quote already locked, quoteToken: %s", in.QuoteToken)
			return nil
		}

		// ⑥ participants 追加、余票-1
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
		// 屏障级跳过（正向已成功）：读报价如实返回（经营销 RPC；NotFound 保持 LOCKED 旧同款容错）
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

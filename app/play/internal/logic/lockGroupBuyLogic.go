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

type LockGroupBuyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLockGroupBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LockGroupBuyLogic {
	return &LockGroupBuyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LockGroupBuy SAGA 03 正向：barrier 事务内
//
//	① 报价只读校验（营销 GetQuote sub-call：userId/grossFen 取报价行）；
//	② state FOR UPDATE（uk config_id 串行化同机下注；种子缺失防御建行，
//	   幸运已发=全 0、余位=target_bet_count）；
//	③ 下注校验（count≤0 → 3033；状态非 OPEN / 余位<count → 3032）；
//	④ 价格一致（3018 复用：quote.gross == 单价×count == 请求总额）；
//	⑤ 报价锁（营销 LockQuote sub-call，置于全部玩法校验之后）；
//	⑥ bets 追加 {user_id, draw_id=requestId, count}、余位-=count。
//
// 报价锁放在最后：玩法自身校验失败回滚时不触碰营销锁（营销事务已独立提交，
// 无法随本事务回滚），与旧仓「锁与业务同事务」的最终状态一致——自中止后报价保持 OPEN。
// userId 取报价行（网关 QuoteCheckout 落库），不信请求参数。
// 错误语义（SAGA 分支铁律）：业务失败 → codes.Aborted（触发补偿）；系统错误 → codes.Internal（DTM 重试）。
func (l *LockGroupBuyLogic) LockGroupBuy(in *play.LockGroupBuyReq) (*play.LockGroupBuyResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("LockGroupBuy get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("LockGroupBuy get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	outCount := in.Count
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
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
			}
			return err
		}
		userId := quoteResp.GetUserId()

		// 拼团配置（纯种子配置只读）
		var cfg gamemodel.GroupBuyConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团配置不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		if cfg.Status != 1 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团配置不可用", errorx.CodeGameConfigInvalid))
		}

		// ② state FOR UPDATE（uk(config_id) 串行化同机并发下注）
		var st gamemodelnc.GroupBuyState
		err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 防御：种子缺失时建行（幸运已发=全 0、余位=target_bet_count）
			lucky, perr := groupBuyParseLucky(cfg.LuckyPrizesJson)
			if perr != nil {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团幸运奖配置无效", errorx.CodeGameConfigInvalid))
			}
			sent := make([]int32, len(lucky))
			st = gamemodelnc.GroupBuyState{
				ConfigId:      in.ConfigId,
				BetsJson:      "[]",
				AwardsJson:    "[]",
				LuckySentJson: groupBuyLuckySentJSON(sent),
				RemainingBets: cfg.TargetBetCount,
				Status:        groupBuyStateOpen,
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
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
		if st.Status != groupBuyStateOpen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团已结算或状态不满足锁定", errorx.CodeGroupBuyNoSlots))
		}

		// ③ 下注校验：count>0（3033）；余位 ≥ count（3032）
		if in.Count <= 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团下注数非法", errorx.CodeGroupBuyCountInvalid))
		}
		if st.RemainingBets < in.Count {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团余位不足", errorx.CodeGroupBuyNoSlots))
		}

		// ④ 价格一致（锁内重算）：单价×count == 报价 gross == 请求总额
		if quoteResp.GetGrossFen() != cfg.PriceFen*in.Count || in.TotalPriceFen != cfg.PriceFen*in.Count {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团价格校验不一致", errorx.CodeDigPriceMismatch))
		}

		// ⑤ 报价锁（营销 LockQuote sub-call，置于玩法校验之后；幂等：newLocked=false=已锁重发 → 业务已成功，读下注记录如实返回）
		lockResp, err := l.svcCtx.MarketingRpc.LockQuote(l.ctx, &marketingClient.LockQuoteReq{
			RequestId:  in.RequestId,
			QuoteToken: in.QuoteToken,
		})
		if err != nil {
			var ce *errorx.CodeError
			if errors.As(err, &ce) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
			}
			return err
		}
		if !lockResp.GetNewLocked() {
			l.Infof("LockGroupBuy idempotent: quote already locked, quoteToken: %s", in.QuoteToken)
			outCount = l.countOfRequest(db, in.ConfigId, in.RequestId, in.Count)
			return nil
		}

		// ⑥ bets 追加、余位-=count
		state, err := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团状态数据异常", errorx.CodeGameConfigInvalid))
		}
		state.Bets = append(state.Bets, core.GroupBuyBet{UserID: userId, DrawID: in.RequestId, Count: int32(in.Count)})
		if err := db.Model(&gamemodelnc.GroupBuyState{}).Where("id = ?", st.Id).
			Updates(map[string]interface{}{
				"bets_json":      groupBuyBetsJSON(state.Bets),
				"remaining_bets": st.RemainingBets - in.Count,
				"status":         groupBuyStateOpen,
				"updated_at":     time.Now(),
			}).Error; err != nil {
			return err
		}
		outCount = in.Count
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("LockGroupBuy aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("LockGroupBuy failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（正向已成功）：读库如实返回
		outCount = l.countOfRequest(l.svcCtx.DB, in.ConfigId, in.RequestId, in.Count)
	}
	l.Infof("LockGroupBuy success, quoteToken: %s, count: %d", in.QuoteToken, outCount)
	return &play.LockGroupBuyResp{Count: outCount, Status: "LOCKED"}, nil
}

// countOfRequest 幂等/屏障跳过路径：按 requestId 读下注记录如实返回；未找到 → 回退请求值
func (l *LockGroupBuyLogic) countOfRequest(db *gorm.DB, configId int64, requestId string, fallback int64) int64 {
	var st gamemodelnc.GroupBuyState
	if err := db.Where("config_id = ?", configId).First(&st).Error; err != nil {
		return fallback
	}
	state, err := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson)
	if err != nil {
		return fallback
	}
	for _, b := range state.Bets {
		if b.DrawID == requestId {
			return int64(b.Count)
		}
	}
	return fallback
}

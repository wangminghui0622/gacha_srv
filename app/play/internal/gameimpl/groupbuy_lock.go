package gameimpl

import (
	"context"
	"errors"
	"fmt"
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

	outCount := in.Count
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "LockGroupBuy", func(db *gorm.DB) error {

		quote, err := playkit.LoadQuote(l.ctx, l.svcCtx, in.QuoteToken)
		if err != nil {
			return err
		}
		userId := quote.UserID

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

		if in.Count <= 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团下注数非法", errorx.CodeGroupBuyCountInvalid))
		}
		if st.RemainingBets < in.Count {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团余位不足", errorx.CodeGroupBuyNoSlots))
		}

		if quote.GrossFen != cfg.PriceFen*in.Count || in.TotalPriceFen != cfg.PriceFen*in.Count {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团价格校验不一致", errorx.CodeDigPriceMismatch))
		}

		newLocked, err := playkit.LockQuote(l.ctx, l.svcCtx, in.RequestId, in.QuoteToken)
		if err != nil {
			return err
		}
		if !newLocked {
			l.Infof("LockGroupBuy idempotent: quote already locked, quoteToken: %s", in.QuoteToken)
			outCount = l.countOfRequest(db, in.ConfigId, in.RequestId, in.Count)
			return nil
		}

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

		outCount = l.countOfRequest(l.svcCtx.DB, in.ConfigId, in.RequestId, in.Count)
	}
	l.Infof("LockGroupBuy success, quoteToken: %s, count: %d", in.QuoteToken, outCount)
	return &play.LockGroupBuyResp{Count: outCount, Status: "LOCKED"}, nil
}

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

func NewLockGashaponLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LockGashaponLogic {
	return &LockGashaponLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LockGashapon SAGA 03 正向：barrier 事务内
//
//	① 报价只读校验（营销 GetQuote sub-call：userId/grossFen 取报价行）；
//	② state FOR UPDATE（uk config_id 串行化同机购买；种子缺失防御建行，总票数=Σ entries quantity）；
//	③ 余票校验（count≤0 → 3028；status 非 OPEN / remaining<count → 3025）；
//	④ 价格一致（3018 复用：quote.gross == 单价×count == 请求总额）；
//	⑤ 报价锁（营销 LockQuote sub-call，置于全部玩法校验之后）；
//	⑥ participants+=count、records 追加、余票-=count。
//
// 报价锁放在最后：玩法自身校验失败回滚时不触碰营销锁（营销事务已独立提交，
// 无法随本事务回滚），与旧仓「锁与业务同事务」的最终状态一致——自中止后报价保持 OPEN。
// userId 取报价行（网关 QuoteCheckout 落库），不信请求参数。
// 错误语义（SAGA 分支铁律）：业务失败 → codes.Aborted（触发补偿）；系统错误 → codes.Internal（DTM 重试）。
func (l *LockGashaponLogic) LockGashapon(in *play.LockGashaponReq) (*play.LockGashaponResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	outCount := in.Count
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "LockGashapon", func(db *gorm.DB) error {

		quote, err := playkit.LoadQuote(l.ctx, l.svcCtx, in.QuoteToken)
		if err != nil {
			return err
		}
		userId := quote.UserID

		// 扭蛋配置（纯种子配置只读，不走缓存锁）
		var cfg gamemodel.GashaponConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋配置不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		if cfg.Status != 1 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋配置不可用", errorx.CodeGameConfigInvalid))
		}

		prizes, err := gashaponParsePrizes(cfg.SpecialPrizesJson)
		if err != nil || len(prizes) == 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋特别奖配置无效", errorx.CodeGameConfigInvalid))
		}

		// ② state FOR UPDATE（uk(config_id) 串行化同机并发购买）
		var st gamemodelnc.GashaponState
		err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 防御：种子缺失时建行（总票数=Σ entries quantity；全部有限量，-1 非法）
			var entries []gamemodel.GashaponEntries
			if err := db.Where("config_id = ?", in.ConfigId).Find(&entries).Error; err != nil {
				return err
			}
			var total int64
			for _, e := range entries {
				if e.Quantity < 0 {
					return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋条目必须有限量", errorx.CodeGameConfigInvalid))
				}
				total += e.Quantity
			}
			if total <= 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋总票数非法", errorx.CodeGameConfigInvalid))
			}
			sent := make([]int32, len(prizes))
			st = gamemodelnc.GashaponState{
				ConfigId:         in.ConfigId,
				ParticipantsJson: "{}",
				RecordsJson:      "[]",
				SentJson:         gashaponSentJSON(sent),
				AwardsJson:       "[]",
				RemainingTickets: total,
				Status:           gashaponStateOpen,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
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
		if st.Status != gashaponStateOpen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋已结算或状态不满足锁定", errorx.CodeGashaponNoTickets))
		}

		if in.Count <= 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=抽数非法", errorx.CodeGashaponCountInvalid))
		}
		if st.RemainingTickets < in.Count {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋余票不足", errorx.CodeGashaponNoTickets))
		}

		if quote.GrossFen != cfg.PriceFen*in.Count || in.TotalPriceFen != cfg.PriceFen*in.Count {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋价格校验不一致", errorx.CodeDigPriceMismatch))
		}

		newLocked, err := playkit.LockQuote(l.ctx, l.svcCtx, in.RequestId, in.QuoteToken)
		if err != nil {
			return err
		}
		if !newLocked {
			l.Infof("LockGashapon idempotent: quote already locked, quoteToken: %s", in.QuoteToken)
			outCount = l.countOfRequest(db, in.ConfigId, in.RequestId, in.Count)
			return nil
		}

		state, err := gashaponParseState(st.ParticipantsJson, st.RecordsJson, st.SentJson, st.AwardsJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋状态数据异常", errorx.CodeGameConfigInvalid))
		}
		state.Participants[userId] += in.Count
		state.Records = append(state.Records, core.GashaponParticipation{
			UserID:    userId,
			RequestID: in.RequestId,
			Count:     in.Count,
		})
		if err := db.Model(&gamemodelnc.GashaponState{}).Where("id = ?", st.Id).
			Updates(map[string]interface{}{
				"participants_json": gashaponParticipantsJSON(state.Participants),
				"records_json":      gashaponRecordsJSON(state.Records),
				"remaining_tickets": st.RemainingTickets - in.Count,
				"status":            gashaponStateOpen,
				"updated_at":        time.Now(),
			}).Error; err != nil {
			return err
		}
		outCount = in.Count
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("LockGashapon aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("LockGashapon failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {

		outCount = l.countOfRequest(l.svcCtx.DB, in.ConfigId, in.RequestId, in.Count)
	}
	l.Infof("LockGashapon success, quoteToken: %s, count: %d", in.QuoteToken, outCount)
	return &play.LockGashaponResp{Count: outCount, Status: "LOCKED"}, nil
}

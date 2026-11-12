package gameimpl

import (
	"context"
	"errors"
	"fmt"
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

func NewLockDigBoxLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LockDigBoxLogic {
	return &LockDigBoxLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LockDigBox SAGA 03 正向：barrier 事务内
//
//	① 报价只读校验（营销 GetQuote sub-call：userId/grossFen/boxId 取报价行）；
//	② 会话 FOR UPDATE（COMPLETED/CANCELED → 回收清零位图与计数）；
//	③ 校验格子未占用（3016）与价格一致（3018：quote.gross == 单价×格数）；
//	④ 报价锁（营销 LockQuote sub-call，置于全部玩法校验之后）；
//	⑤ 位图置位 + 计数累加。
//
// 报价锁放在最后：玩法自身校验失败回滚时不触碰营销锁（营销事务已独立提交，
// 无法随本事务回滚），与旧仓「锁与业务同事务」的最终状态一致——自中止后报价保持 OPEN。
// userId 取报价行（网关 QuoteCheckout 落库），不信请求参数。
// 错误语义（SAGA 分支铁律）：业务失败 → codes.Aborted（触发补偿）；系统错误 → codes.Internal（DTM 重试）。
func (l *LockDigBoxLogic) LockDigBox(in *play.LockDigBoxReq) (*play.LockDigBoxResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.SessionNo == "" || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "LockDigBox", func(db *gorm.DB) error {

		quote, err := playkit.LoadQuote(l.ctx, l.svcCtx, in.QuoteToken)
		if err != nil {
			return err
		}
		userId := quote.UserID

		// 挖宝箱配置（纯种子配置只读，不走缓存锁）
		var cfg gamemodel.DigBoxConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝箱不存在", errorx.CodeDigBoxNotFound))
			}
			return err
		}
		if cfg.Status != 1 || cfg.CellCount <= 0 || cfg.CellCount > 62 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝箱不可用", errorx.CodeDigBoxNotFound))
		}

		// ② 会话 FOR UPDATE（uk(user_id,config_id) 串行化同箱并发购买）
		var sess gamemodelnc.DigBoxSessions
		err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND config_id = ?", userId, in.ConfigId).First(&sess).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {

			sess = gamemodelnc.DigBoxSessions{
				SessionNo:      in.SessionNo,
				UserId:         userId,
				ConfigId:       in.ConfigId,
				OccupiedBitmap: 0,
				TotalCellsPaid: 0,
				TotalPriceFen:  0,
				Status:         digSessionPlaying,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}
			if err := db.Create(&sess).Error; err != nil {
				if errors.Is(err, gorm.ErrDuplicatedKey) {

					if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
						Where("user_id = ? AND config_id = ?", userId, in.ConfigId).First(&sess).Error; err != nil {
						return err
					}
				} else {
					return err
				}
			}
		} else if err != nil {
			return err
		}

		if sess.Status != digSessionPlaying {
			if err := db.Model(&gamemodelnc.DigBoxSessions{}).Where("id = ?", sess.Id).
				Updates(map[string]interface{}{
					"occupied_bitmap":  0,
					"total_cells_paid": 0,
					"total_price_fen":  0,
					"status":           digSessionPlaying,
				}).Error; err != nil {
				return err
			}
			sess.OccupiedBitmap = 0
			sess.TotalCellsPaid = 0
			sess.TotalPriceFen = 0
			sess.Status = digSessionPlaying
		}

		// ③ 格子解析与校验（takeAll=锁内解析全部空格；网关侧已解析为显式格子）
		var cells []int64
		if in.TakeAll {
			for i := int64(0); i < cfg.CellCount; i++ {
				if sess.OccupiedBitmap&(1<<i) == 0 {
					cells = append(cells, i)
				}
			}
		} else {
			cells = in.CellIndexes
		}
		if !digCellsValid(cells, cfg.CellCount) {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=所选格子无效", errorx.CodeDigCellOccupied))
		}
		mask := digCellMask(cells)
		if sess.OccupiedBitmap&mask != 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=所选格子已被挖开", errorx.CodeDigCellOccupied))
		}

		expected := int64(len(cells)) * cfg.PriceFen
		if quote.GrossFen != expected || in.TotalPriceFen != expected {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝价格校验不一致", errorx.CodeDigPriceMismatch))
		}
		if quote.BoxID != 0 && quote.BoxID != in.ConfigId {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝价格校验不一致", errorx.CodeDigPriceMismatch))
		}

		newLocked, err := playkit.LockQuote(l.ctx, l.svcCtx, in.RequestId, in.QuoteToken)
		if err != nil {
			return err
		}
		if !newLocked {
			l.Infof("LockDigBox idempotent: quote already locked, quoteToken: %s", in.QuoteToken)
			return nil
		}

		newBitmap := sess.OccupiedBitmap | mask
		if err := db.Model(&gamemodelnc.DigBoxSessions{}).Where("id = ?", sess.Id).
			Updates(map[string]interface{}{
				"occupied_bitmap":  newBitmap,
				"total_cells_paid": sess.TotalCellsPaid + int64(len(cells)),
				"total_price_fen":  sess.TotalPriceFen + expected,
				"status":           digSessionPlaying,
				"updated_at":       time.Now(),
			}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("LockDigBox aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("LockDigBox failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {

		return l.readBackLock(in.SessionNo)
	}
	l.Infof("LockDigBox success, sessionNo: %s, cells: %d", in.SessionNo, len(in.CellIndexes))
	return &play.LockDigBoxResp{SessionNo: in.SessionNo, Status: "LOCKED"}, nil
}

// readBackLock 屏障跳过时读会话如实返回
func (l *LockDigBoxLogic) readBackLock(sessionNo string) (*play.LockDigBoxResp, error) {
	var sess gamemodelnc.DigBoxSessions
	if err := l.svcCtx.DB.Where("session_no = ?", sessionNo).First(&sess).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.Internal, "挖宝会话不存在")
		}
		l.Errorf("readBackLock failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	l.Infof("readBackLock: sessionNo: %s, paid: %d, bitmap: %d",
		sessionNo, sess.TotalCellsPaid, sess.OccupiedBitmap)
	return &play.LockDigBoxResp{SessionNo: sessionNo, Status: "LOCKED"}, nil
}

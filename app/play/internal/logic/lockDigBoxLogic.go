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
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type LockDigBoxLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

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

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("LockDigBox get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("LockDigBox get raw db failed, err: %v", err)
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
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
			}
			return err
		}
		userId := quoteResp.GetUserId()

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
			// 首次挖该箱：建会话（bitmap=0）
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
					// 并发首建竞争：重读（行锁串行化后可见对方会话）
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
		// 满格/取消会话回收：位图计数清零重开（D5）
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

		// ④ 价格一致（锁内重算）：count×单价 == 报价 gross == 请求总额
		expected := int64(len(cells)) * cfg.PriceFen
		if quoteResp.GetGrossFen() != expected || in.TotalPriceFen != expected {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝价格校验不一致", errorx.CodeDigPriceMismatch))
		}
		if quoteResp.GetBoxId() != 0 && quoteResp.GetBoxId() != in.ConfigId {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝价格校验不一致", errorx.CodeDigPriceMismatch))
		}

		// ⑤ 报价锁（营销 LockQuote sub-call，置于玩法校验之后；幂等：newLocked=false=已锁重发 → 业务已成功，直接返回）
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
			l.Infof("LockDigBox idempotent: quote already locked, quoteToken: %s", in.QuoteToken)
			return nil
		}

		// ⑥ 位图置位 + 计数累加
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
		// 屏障级跳过（正向已成功）：读库如实返回
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

package logic

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"gacha_srv/app/dtm/client/dtmgrpc"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/txutil"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SettleMemoryMatchRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleMemoryMatchRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleMemoryMatchRevertLogic {
	return &SettleMemoryMatchRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleMemoryMatchRevert SAGA 05 补偿：会话 AWAITING_DESTINY → PENDING（清引擎种子、回扣擂主分成）。
// 已进入游玩（AWAITING_SECOND/PLAYING/COMPLETED/EXPIRED）→ 跳过不打扰（盲撤销幂等）。
//
// 擂主分成回扣竞态守卫：仅当现任擂主仍是结算时记账的 champion_credit_user_id 才扣回
// （max(0, pending-share) 兜底）；擂主已易主则接受泄漏，绝不从新擂主处偷扣。
func (l *SettleMemoryMatchRevertLogic) SettleMemoryMatchRevert(in *play.SettleMemoryMatchReq) (*play.SettleMemoryMatchResp, error) {
	if in.RequestId == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("SettleMemoryMatchRevert get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("SettleMemoryMatchRevert get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	businessRan := false
	var outStatus int64
	var outSessionID int64
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

		var sess gamemodelnc.MemoryMatchSessions
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&sess).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil // 05 正向从未成功：无可回滚
			}
			return err
		}
		outSessionID = sess.Id
		outStatus = sess.Status
		if sess.Status != mmSessionAwaitingDestiny {
			return nil // 非待命运状态：跳过不打扰
		}

		// ① 回扣擂主分成（竞态守卫：现任擂主仍是记账用户才扣，max(0,...) 兜底）
		if sess.ChampionCreditUserId > 0 && sess.ChampionShareMinor > 0 {
			var ch gamemodelnc.MemoryMatchChampion
			if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("config_id = ?", sess.ConfigId).First(&ch).Error; err == nil {
				if ch.UserId == sess.ChampionCreditUserId {
					if err := db.Model(&gamemodelnc.MemoryMatchChampion{}).Where("id = ?", ch.Id).
						Update("pending_income_minor",
							gorm.Expr("GREATEST(0, pending_income_minor - ?)", sess.ChampionShareMinor)).Error; err != nil {
						return err
					}
				}
				// 擂主已易主：接受泄漏，不偷扣新擂主
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		// ② AWAITING_DESTINY → PENDING + 清种子 + 清记账标记（盲更新幂等）
		if err := db.Model(&gamemodelnc.MemoryMatchSessions{}).
			Where("id = ? AND status = ?", sess.Id, mmSessionAwaitingDestiny).
			Updates(map[string]interface{}{
				"engine_seed":             gorm.Expr("NULL"),
				"champion_credit_user_id": 0,
				"champion_share_minor":    0,
				"status":                  mmSessionPending,
				"version":                 gorm.Expr("version + 1"),
				"updated_at":              time.Now(),
			}).Error; err != nil {
			return err
		}
		outStatus = mmSessionPending
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleMemoryMatchRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleMemoryMatchRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（分支已成功/已补偿）：读库如实返回
		var sess gamemodelnc.MemoryMatchSessions
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&sess).Error; err == nil {
			outStatus = sess.Status
			outSessionID = sess.Id
		}
	}
	l.Infof("SettleMemoryMatchRevert success, requestId: %s, sessionId: %d, status: %d", in.RequestId, outSessionID, outStatus)
	return &play.SettleMemoryMatchResp{
		SessionId: outSessionID,
		State:     play.MemoryMatchSessionState(outStatus),
	}, nil
}

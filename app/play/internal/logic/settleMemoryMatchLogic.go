package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gacha_srv/app/dtm/client/dtmgrpc"
	"gacha_srv/app/play/internal/core/memorymatch"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"
	"gacha_srv/pkg/utils/txutil"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SettleMemoryMatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleMemoryMatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleMemoryMatchLogic {
	return &SettleMemoryMatchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleMemoryMatch SAGA 05 正向：barrier 事务内
//
//	① 会话 FOR UPDATE（uk request_id 幂等重放：非 PENDING → 读回当前状态成功返回）；
//	② PENDING → AWAITING_DESTINY：engine_seed crypto/rand 落库；
//	③ 擂主分成：现任擂主 pending += 购买价×bps/10000（无擂主/未启用 → 跳过；
//	   记账到会话 champion_credit_user_id/champion_share_minor 供 Revert 等额回扣）。
//
// 错误语义：业务失败 → codes.Aborted；系统错误 → codes.Internal。
func (l *SettleMemoryMatchLogic) SettleMemoryMatch(in *play.SettleMemoryMatchReq) (*play.SettleMemoryMatchResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("SettleMemoryMatch get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("SettleMemoryMatch get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	businessRan := false
	var outStatus int64
	var outSessionID int64
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

		// ① 会话 FOR UPDATE（request_id 唯一键幂等）
		var sess gamemodelnc.MemoryMatchSessions
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&sess).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=会话不存在", errorx.CodeMemoryMatchConflict))
			}
			return err
		}
		if in.SessionId > 0 && sess.Id != in.SessionId || sess.UserId != in.UserId || sess.ConfigId != in.ConfigId {
			return status.Error(codes.Aborted, "code=400 msg=会话参数不一致")
		}
		outStatus = sess.Status
		outSessionID = sess.Id
		if sess.Status != mmSessionPending {
			// 幂等重放（已结算/已游玩）：读回当前状态成功返回
			l.Infof("SettleMemoryMatch idempotent: session %d already at status %d", sess.Id, sess.Status)
			return nil
		}

		rules, err := mmSessionRules(&sess)
		if err != nil {
			return mmAborted(err)
		}

		// ② PENDING → AWAITING_DESTINY + 引擎种子
		if err := db.Model(&gamemodelnc.MemoryMatchSessions{}).Where("id = ? AND status = ?", sess.Id, mmSessionPending).
			Updates(map[string]interface{}{
				"engine_seed": mmEngineSeedHex(),
				"status":      mmSessionAwaitingDestiny,
				"version":     gorm.Expr("version + 1"),
				"updated_at":  time.Now(),
			}).Error; err != nil {
			return err
		}

		// ③ 擂主分成（现任擂主存在且玩法启用擂主时；记账供 Revert 回扣）
		if rules.ChampionMinTotalPairs > 0 && rules.ChampionShareBps > 0 {
			share := memorymatch.ChampionCreditMinor(sess.PurchasePriceMinor, rules.ChampionShareBps)
			if share > 0 {
				var ch gamemodelnc.MemoryMatchChampion
				err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("config_id = ?", sess.ConfigId).First(&ch).Error
				if err == nil {
					if err := db.Model(&gamemodelnc.MemoryMatchChampion{}).Where("id = ?", ch.Id).
						Update("pending_income_minor", gorm.Expr("pending_income_minor + ?", share)).Error; err != nil {
						return err
					}
					if err := db.Model(&gamemodelnc.MemoryMatchSessions{}).Where("id = ?", sess.Id).
						Updates(map[string]interface{}{
							"champion_credit_user_id": ch.UserId,
							"champion_share_minor":    share,
						}).Error; err != nil {
						return err
					}
				} else if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				// 无擂主行：无现任可分成，跳过
			}
		}
		outStatus = mmSessionAwaitingDestiny
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleMemoryMatch aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleMemoryMatch failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（分支已成功）：读库如实返回
		var sess gamemodelnc.MemoryMatchSessions
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&sess).Error; err == nil {
			outStatus = sess.Status
			outSessionID = sess.Id
		}
	}
	l.Infof("SettleMemoryMatch success, requestId: %s, sessionId: %d, status: %d", in.RequestId, outSessionID, outStatus)
	return &play.SettleMemoryMatchResp{
		SessionId: outSessionID,
		State:     play.MemoryMatchSessionState(outStatus),
	}, nil
}

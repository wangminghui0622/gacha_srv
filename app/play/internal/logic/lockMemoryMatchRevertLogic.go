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

type LockMemoryMatchRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLockMemoryMatchRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LockMemoryMatchRevertLogic {
	return &LockMemoryMatchRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LockMemoryMatchRevert SAGA 03 补偿：会话 PENDING/AWAITING_DESTINY → EXPIRED（盲撤销幂等）；
// 已开始游玩（AWAITING_SECOND/PLAYING/COMPLETED/EXPIRED）→ 跳过不打扰。报价释放由 payment 04 补偿负责。
func (l *LockMemoryMatchRevertLogic) LockMemoryMatchRevert(in *play.LockMemoryMatchReq) (*play.LockMemoryMatchResp, error) {
	if in.RequestId == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("LockMemoryMatchRevert get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("LockMemoryMatchRevert get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	businessRan := false
	var outSessionID int64
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

		var sess gamemodelnc.MemoryMatchSessions
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&sess).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // 03 正向从未成功：无会话可撤销
		}
		if err != nil {
			return err
		}
		outSessionID = sess.Id
		if sess.Status == mmSessionPending || sess.Status == mmSessionAwaitingDestiny {
			if err := db.Model(&gamemodelnc.MemoryMatchSessions{}).
				Where("id = ? AND status = ?", sess.Id, sess.Status).
				Updates(map[string]interface{}{
					"status":     mmSessionExpired,
					"version":    gorm.Expr("version + 1"),
					"updated_at": time.Now(),
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("LockMemoryMatchRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("LockMemoryMatchRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan || outSessionID == 0 {
		var sess gamemodelnc.MemoryMatchSessions
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&sess).Error; err == nil {
			outSessionID = sess.Id
		}
	}
	l.Infof("LockMemoryMatchRevert success, requestId: %s, sessionId: %d", in.RequestId, outSessionID)
	return &play.LockMemoryMatchResp{SessionId: outSessionID, Status: "RELEASED"}, nil
}

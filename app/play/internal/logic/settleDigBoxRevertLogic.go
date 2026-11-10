package logic

import (
	"context"
	"database/sql"
	"errors"
	"gacha_srv/pkg/utils/txutil"
	"time"

	"gacha_srv/app/dtm/client/dtmgrpc"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SettleDigBoxRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleDigBoxRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleDigBoxRevertLogic {
	return &SettleDigBoxRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleDigBoxRevert SAGA 05 补偿：记录置撤销 + 满格状态回退。
// 位图清位与计数回滚由 03 补偿（ReleaseDigBox）唯一负责——避免双扣；
// 本分支只撤销自己造成的影响（记录 2 已撤销、会话 COMPLETED→PLAYING），幂等。
func (l *SettleDigBoxRevertLogic) SettleDigBoxRevert(in *play.SettleDigBoxReq) (*play.SettleDigBoxResp, error) {
	if in.RequestId == "" || in.SessionNo == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("SettleDigBoxRevert get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("SettleDigBoxRevert get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var outRecordId int64
	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

		// 记录 FOR UPDATE → 2 已撤销（盲更新，幂等；无记录=05 正向从未成功，跳过）
		var rec gamemodelnc.DigBoxRecords
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&rec).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			outRecordId = rec.Id
			if rec.Status == digRecordNormal {
				if err := db.Model(&gamemodelnc.DigBoxRecords{}).
					Where("id = ? AND status = ?", rec.Id, digRecordNormal).
					Update("status", digRecordCanceled).Error; err != nil {
					return err
				}
			}
		}

		// 会话满格状态回退（满格必然由本 gid 造成：此前 COMPLETED 会话在 03 已回收）
		if err := db.Model(&gamemodelnc.DigBoxSessions{}).
			Where("session_no = ? AND status = ?", in.SessionNo, digSessionCompleted).
			Updates(map[string]interface{}{"status": digSessionPlaying, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleDigBoxRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleDigBoxRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读记录如实返回
		var rec gamemodelnc.DigBoxRecords
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&rec).Error; err == nil {
			outRecordId = rec.Id
		}
	}
	l.Infof("SettleDigBoxRevert success, requestId: %s, recordId: %d", in.RequestId, outRecordId)
	return &play.SettleDigBoxResp{RecordId: outRecordId, Status: "REVERTED"}, nil
}

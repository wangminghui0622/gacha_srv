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
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SettleFullCarRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleFullCarRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleFullCarRevertLogic {
	return &SettleFullCarRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleFullCarRevert SAGA 05 补偿：只撤销本分支造成的影响（dig 先例——唯一回滚点防双扣）：
//
//	① 记录 → 2 已撤销（无记录=05 正向从未成功，跳过）；
//	② regular 奖库存回补（按记录的 entry_name 定位，quantity=-1 不限量跳过）；
//	③ awards_json 按 requestId 删除本 gid 触发的整车奖；DEPARTED→OPEN。
//
// 座位释放与余票+1 由 03 补偿（ReleaseFullCar）唯一负责，本分支不碰余票。
func (l *SettleFullCarRevertLogic) SettleFullCarRevert(in *play.SettleFullCarReq) (*play.SettleFullCarResp, error) {
	if in.RequestId == "" || in.UserId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("SettleFullCarRevert get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("SettleFullCarRevert get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var outRecordId int64
	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

		// ① 记录 FOR UPDATE → 2 已撤销（盲更新，幂等；无记录=正向从未成功，跳过）
		var rec gamemodelnc.FullCarRecords
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&rec).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			outRecordId = rec.Id
			if rec.Status == fullCarRecordNormal {
				if err := db.Model(&gamemodelnc.FullCarRecords{}).
					Where("id = ? AND status = ?", rec.Id, fullCarRecordNormal).
					Update("status", fullCarRecordCanceled).Error; err != nil {
					return err
				}
			}
			// ② regular 奖库存回补（按记录条目名定位；-1 不限量不回补）
			if err := db.Model(&gamemodel.FullCarEntries{}).
				Where("config_id = ? AND name = ? AND quantity != -1", rec.ConfigId, rec.EntryName).
				Update("remaining", gorm.Expr("remaining + 1")).Error; err != nil {
				return err
			}
		}

		// ③ awards 按 requestId 删除 + DEPARTED→OPEN
		var st gamemodelnc.FullCarState
		err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			all := fullCarParseAwards(st.AwardsJson)
			kept := make([]fullCarStoredAward, 0, len(all))
			removed := 0
			for _, a := range all {
				if a.RequestId == in.RequestId {
					removed++
					continue
				}
				kept = append(kept, a)
			}
			updates := map[string]interface{}{"updated_at": time.Now()}
			if removed > 0 {
				updates["awards_json"] = fullCarAwardsJSON(kept)
			}
			if st.Status == fullCarStateDeparted {
				updates["status"] = fullCarStateOpen
			}
			if len(updates) > 1 {
				if err := db.Model(&gamemodelnc.FullCarState{}).Where("id = ?", st.Id).
					Updates(updates).Error; err != nil {
					return err
				}
			}
			if removed > 0 {
				l.Infof("SettleFullCarRevert awards removed, requestId: %s, removed: %d", in.RequestId, removed)
			}
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleFullCarRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleFullCarRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读记录如实返回
		var rec gamemodelnc.FullCarRecords
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&rec).Error; err == nil {
			outRecordId = rec.Id
		}
	}
	l.Infof("SettleFullCarRevert success, requestId: %s, recordId: %d", in.RequestId, outRecordId)
	return &play.SettleFullCarResp{RecordId: outRecordId, Status: "REVERTED"}, nil
}

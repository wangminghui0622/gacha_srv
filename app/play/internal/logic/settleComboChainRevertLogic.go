package logic

import (
	"context"
	"database/sql"
	"errors"
	"gacha_srv/pkg/utils/txutil"
	"time"

	"gacha_srv/app/dtm/client/dtmgrpc"
	"gacha_srv/app/play/internal/core"
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

type SettleComboChainRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleComboChainRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleComboChainRevertLogic {
	return &SettleComboChainRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleComboChainRevert SAGA 05 补偿：只撤销本分支造成的影响（dig/fullcar/gashapon 先例——唯一回滚点防双扣）：
//
//	① 记录 → 2 已撤销（无记录=05 正向从未成功，跳过）；
//	② 若本 gid 触发过整车结算（awards_json 有本 requestId 的结算奖）：
//	   按 requestId 删结算奖；COMBO_TIER 档位库存回补（combo_index++）；
//	   连击计数按 streaks_before_json 快照回滚；车次回退——car_index=记录车号、
//	   参与者由该车 status=1 记录重建（本 gid 记录已置 2 故自动排除）、
//	   余票=tickets_per_car-重建人数、状态 OPEN。
//
// 参与者弹出与余票回补由 03 补偿（ReleaseComboChain）唯一负责，本分支不碰余票
// 的普通回补（重建路径例外：整车结算回滚必须复原整辆车）。
func (l *SettleComboChainRevertLogic) SettleComboChainRevert(in *play.SettleComboChainReq) (*play.SettleComboChainResp, error) {
	if in.RequestId == "" || in.UserId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("SettleComboChainRevert get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("SettleComboChainRevert get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var outRecordId int64
	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

		// ① 记录 FOR UPDATE（盲撤销幂等；无记录=正向从未成功，跳过）
		var rec gamemodelnc.ComboChainRecords
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&rec).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			outRecordId = rec.Id
			if rec.Status == comboChainRecordNormal {
				if err := db.Model(&gamemodelnc.ComboChainRecords{}).
					Where("id = ? AND status = ?", rec.Id, comboChainRecordNormal).
					Update("status", comboChainRecordCanceled).Error; err != nil {
					return err
				}
				// ② 整车结算回滚（本 gid 触发过结算才有 awards；仅状态撤销幂等跳过）
				if err := l.revertSettlement(db, &rec); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleComboChainRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleComboChainRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读记录如实返回
		var rec gamemodelnc.ComboChainRecords
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&rec).Error; err == nil {
			outRecordId = rec.Id
		}
	}
	l.Infof("SettleComboChainRevert success, requestId: %s, recordId: %d", in.RequestId, outRecordId)
	return &play.SettleComboChainResp{RecordId: outRecordId, Status: "REVERTED"}, nil
}

// revertSettlement 整车结算回滚：删结算奖、档位回补、连击快照回滚、车次回退
func (l *SettleComboChainRevertLogic) revertSettlement(db *gorm.DB, rec *gamemodelnc.ComboChainRecords) error {
	var st gamemodelnc.ComboChainState
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("config_id = ?", rec.ConfigId).First(&st).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err != nil {
		return nil
	}
	state, perr := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson)
	if perr != nil {
		return nil
	}
	// 按 requestId 删结算奖 + 档位库存回补
	kept := make([]core.ComboChainAward, 0, len(state.Awards))
	removed := 0
	for _, a := range state.Awards {
		if a.RequestID != rec.RequestId {
			kept = append(kept, a)
			continue
		}
		removed++
		if a.Kind == core.ComboAwardKindCombo {
			if idx := int(a.ComboIndex); idx >= 0 && idx < len(state.TierRemaining) {
				state.TierRemaining[idx]++
			}
		}
	}
	if removed == 0 {
		// 非结算票：无结算影响，只撤销记录（03 补偿负责弹出参与者与余票回补）
		return nil
	}
	// 连击计数按结算前快照回滚
	for userId, s := range state.StreaksBefore {
		if err := upsertComboChainStreak(db, rec.ConfigId, userId, s); err != nil {
			return err
		}
	}
	// 车次回退：参与者由该车 status=1 记录重建（本 gid 已置 2 自动排除）
	var recs []gamemodelnc.ComboChainRecords
	if err := db.Where("config_id = ? AND car_index = ? AND status = ?", rec.ConfigId, rec.CarIndex, comboChainRecordNormal).
		Order("id ASC").Find(&recs).Error; err != nil {
		return err
	}
	participants := make([]int64, 0, len(recs))
	for _, r := range recs {
		participants = append(participants, r.UserId)
	}
	updates := map[string]interface{}{
		"awards_json":         comboChainAwardsJSON(kept),
		"tier_remaining_json": comboChainTierRemainingJSON(state.TierRemaining),
		"car_index":           rec.CarIndex,
		"participants_json":   comboChainParticipantsJSON(participants),
		"status":              comboChainStateOpen,
		"updated_at":          time.Now(),
	}
	// 余票复原 = tickets_per_car - 重建人数（配置缺失时保持原值防御）
	var cfg gamemodel.ComboChainConfigs
	if err := db.Where("id = ?", rec.ConfigId).First(&cfg).Error; err == nil {
		updates["remaining_tickets"] = cfg.TicketsPerCar - int64(len(participants))
	}
	if err := db.Model(&gamemodelnc.ComboChainState{}).Where("id = ?", st.Id).
		Updates(updates).Error; err != nil {
		return err
	}
	l.Infof("revertSettlement: requestId: %s, removed: %d, participants rebuilt: %d",
		rec.RequestId, removed, len(participants))
	return nil
}

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

type SettleGroupBuyRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleGroupBuyRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleGroupBuyRevertLogic {
	return &SettleGroupBuyRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleGroupBuyRevert SAGA 05 补偿：只撤销本分支造成的影响（dig/fullcar/gashapon 先例——唯一回滚点防双扣）：
//
//	① 记录 → 2 已撤销（无记录=05 正向从未成功，跳过）；
//	② 若本 gid 触发过整批结算（awards_json 有本 requestId 的结算奖）：
//	   按 requestId 删结算奖；条目库存回补（normal 按 entry_id 计数、
//	   条目子奖按 entry_id 计数；幸运子奖无库存不回补）；lucky_sent 逐幸运奖回退；
//	   SETTLED→OPEN。
//
// 下注记录弹出与余位回补由 03 补偿（ReleaseGroupBuy）唯一负责，本分支不碰余位。
func (l *SettleGroupBuyRevertLogic) SettleGroupBuyRevert(in *play.SettleGroupBuyReq) (*play.SettleGroupBuyResp, error) {
	if in.RequestId == "" || in.UserId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("SettleGroupBuyRevert get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("SettleGroupBuyRevert get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var outRecordId int64
	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

		// ① 记录 FOR UPDATE（盲撤销幂等；无记录=正向从未成功，跳过）
		var rec gamemodelnc.GroupBuyRecords
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&rec).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			outRecordId = rec.Id
			if rec.Status == groupBuyRecordNormal {
				if err := db.Model(&gamemodelnc.GroupBuyRecords{}).
					Where("id = ? AND status = ?", rec.Id, groupBuyRecordNormal).
					Update("status", groupBuyRecordCanceled).Error; err != nil {
					return err
				}
				// ② 整批结算回滚（本 gid 触发过结算才有 awards；仅状态撤销幂等跳过）
				if err := l.revertSettlement(db, &rec); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleGroupBuyRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleGroupBuyRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读记录如实返回
		var rec gamemodelnc.GroupBuyRecords
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&rec).Error; err == nil {
			outRecordId = rec.Id
		}
	}
	l.Infof("SettleGroupBuyRevert success, requestId: %s, recordId: %d", in.RequestId, outRecordId)
	return &play.SettleGroupBuyResp{RecordId: outRecordId, Status: "REVERTED"}, nil
}

// revertSettlement 整批结算回滚：删结算奖、条目库存回补、lucky_sent 回退、SETTLED→OPEN
func (l *SettleGroupBuyRevertLogic) revertSettlement(db *gorm.DB, rec *gamemodelnc.GroupBuyRecords) error {
	var st gamemodelnc.GroupBuyState
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("config_id = ?", rec.ConfigId).First(&st).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err != nil {
		return nil
	}
	state, perr := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson)
	if perr != nil {
		return nil
	}
	// 按 requestId 删结算奖 + 库存计数
	kept := make([]core.GroupBuyAward, 0, len(state.Awards))
	removed := 0
	entryDelta := map[int64]int64{}
	luckyDelta := map[int32]int32{}
	for _, a := range state.Awards {
		if a.RequestID != rec.RequestId {
			kept = append(kept, a)
			continue
		}
		removed++
		switch a.Kind {
		case core.GroupBuyAwardKindNormal:
			entryDelta[a.EntryID]++
		case core.GroupBuyAwardKindChild:
			if a.EntryID > 0 {
				entryDelta[a.EntryID]++
			}
		case core.GroupBuyAwardKindLucky:
			luckyDelta[a.LuckyIndex]++
		}
	}
	if removed == 0 {
		// 非成团注：无结算影响，只撤销记录（03 补偿负责弹出下注与余位回补）
		return nil
	}
	// 条目库存回补（normal/条目子奖按 entry_id 计数；幸运子奖无库存跳过）
	for entryId, n := range entryDelta {
		if err := db.Model(&gamemodel.GroupBuyEntries{}).
			Where("id = ?", entryId).
			Update("remaining", gorm.Expr("remaining + ?", n)).Error; err != nil {
			return err
		}
	}
	// lucky_sent 回退（防御下限 0）
	for index, n := range luckyDelta {
		if int(index) >= 0 && int(index) < len(state.LuckySent) {
			state.LuckySent[index] -= n
			if state.LuckySent[index] < 0 {
				state.LuckySent[index] = 0
			}
		}
	}
	// 结算奖删除 + lucky_sent 落库 + SETTLED→OPEN（余位不碰——03 补偿负责）
	updates := map[string]interface{}{
		"awards_json":     groupBuyAwardsJSON(kept),
		"lucky_sent_json": groupBuyLuckySentJSON(state.LuckySent),
		"status":          groupBuyStateOpen,
		"updated_at":      time.Now(),
	}
	if err := db.Model(&gamemodelnc.GroupBuyState{}).Where("id = ?", st.Id).
		Updates(updates).Error; err != nil {
		return err
	}
	l.Infof("revertSettlement: requestId: %s, removed: %d", rec.RequestId, removed)
	return nil
}

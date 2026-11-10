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

type SettleGashaponRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleGashaponRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleGashaponRevertLogic {
	return &SettleGashaponRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleGashaponRevert SAGA 05 补偿：只撤销本分支造成的影响（dig/fullcar 先例——唯一回滚点防双扣）：
//
//	① 记录 → 2 已撤销（无记录=05 正向从未成功，跳过）；
//	② regular 奖库存回补（按记录 entries_json 逐条回补；同名条目多行时
//	   每单位回补一行 LIMIT 1——总库存按名复原，与抽中行号无关）；
//	③ awards_json 按 requestId 删本 gid 触发的特别奖、sent 逐奖回退；SETTLED→OPEN。
//
// 参与记录弹出与余票回补由 03 补偿（ReleaseGashapon）唯一负责，本分支不碰余票。
func (l *SettleGashaponRevertLogic) SettleGashaponRevert(in *play.SettleGashaponReq) (*play.SettleGashaponResp, error) {
	if in.RequestId == "" || in.UserId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("SettleGashaponRevert get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("SettleGashaponRevert get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var outRecordId int64
	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)

		// ① 记录 FOR UPDATE → 2 已撤销（盲更新，幂等；无记录=正向从未成功，跳过）
		var rec gamemodelnc.GashaponRecords
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&rec).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			outRecordId = rec.Id
			if rec.Status == gashaponRecordNormal {
				if err := db.Model(&gamemodelnc.GashaponRecords{}).
					Where("id = ? AND status = ?", rec.Id, gashaponRecordNormal).
					Update("status", gashaponRecordCanceled).Error; err != nil {
					return err
				}
			}
			// ② regular 奖库存回补：优先按条目主键精确定位；旧记录无主键回退按名回补
			//   一行（同名多行时总库存按名复原）
			for _, o := range gashaponParseOutcomes(rec.EntriesJson) {
				if o.EntryId > 0 {
					if err := db.Model(&gamemodel.GashaponEntries{}).
						Where("id = ?", o.EntryId).
						Update("remaining", gorm.Expr("remaining + 1")).Error; err != nil {
						return err
					}
					continue
				}
				if err := db.Model(&gamemodel.GashaponEntries{}).
					Where("config_id = ? AND name = ?", rec.ConfigId, o.Name).
					Order("id ASC").Limit(1).
					Update("remaining", gorm.Expr("remaining + 1")).Error; err != nil {
					return err
				}
			}
		}

		// ③ awards 按 requestId 删除 + sent 回退 + SETTLED→OPEN
		var st gamemodelnc.GashaponState
		err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			state, perr := gashaponParseState(st.ParticipantsJson, st.RecordsJson, st.SentJson, st.AwardsJson)
			updates := map[string]interface{}{"updated_at": time.Now()}
			if perr == nil {
				kept := make([]core.GashaponAward, 0, len(state.Awards))
				removed := 0
				for _, a := range state.Awards {
					if a.RequestID == in.RequestId {
						removed++
						// sent 逐奖回退（防御下限 0）
						if idx := int(a.PrizeIndex); idx >= 0 && idx < len(state.Sent) && state.Sent[idx] > 0 {
							state.Sent[idx]--
						}
						continue
					}
					kept = append(kept, a)
				}
				if removed > 0 {
					updates["awards_json"] = gashaponAwardsJSON(kept)
					updates["sent_json"] = gashaponSentJSON(state.Sent)
					l.Infof("SettleGashaponRevert awards removed, requestId: %s, removed: %d", in.RequestId, removed)
				}
			}
			if st.Status == gashaponStateSettled {
				updates["status"] = gashaponStateOpen
			}
			if len(updates) > 1 {
				if err := db.Model(&gamemodelnc.GashaponState{}).Where("id = ?", st.Id).
					Updates(updates).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleGashaponRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleGashaponRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读记录如实返回
		var rec gamemodelnc.GashaponRecords
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&rec).Error; err == nil {
			outRecordId = rec.Id
		}
	}
	l.Infof("SettleGashaponRevert success, requestId: %s, recordId: %d", in.RequestId, outRecordId)
	return &play.SettleGashaponResp{RecordId: outRecordId, Status: "REVERTED"}, nil
}

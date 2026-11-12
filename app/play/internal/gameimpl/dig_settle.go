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
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func NewSettleDigBoxLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleDigBoxLogic {
	return &SettleDigBoxLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleDigBox SAGA 05 正向：barrier 事务内
//
//	① 会话 FOR UPDATE（必须 PLAYING）；② 校验格子已由 03 置位（3017）；
//	③ 揭晓奖品（dig_box_cells 固定奖品，空格跳过）落 prizes_json；
//	④ 插挖宝记录（uk request_id 撞键=已结算 → 读回幂等成功）；
//	⑤ 满格（位图全 1）→ 会话 COMPLETED（下次购买回收）。
//
// 错误语义：业务失败 → codes.Aborted；系统错误 → codes.Internal。
func (l *SettleDigBoxLogic) SettleDigBox(in *play.SettleDigBoxReq) (*play.SettleDigBoxResp, error) {
	if in.RequestId == "" || in.SessionNo == "" || in.UserId <= 0 || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	var outRecordId int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleDigBox", func(db *gorm.DB) error {

		// ① 会话 FOR UPDATE
		var sess gamemodelnc.DigBoxSessions
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("session_no = ? AND user_id = ?", in.SessionNo, in.UserId).First(&sess).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝会话不存在", errorx.CodeDigSessionInvalid))
			}
			return err
		}
		if sess.Status != digSessionPlaying {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝会话状态不满足结算", errorx.CodeDigSessionInvalid))
		}

		// ② 格子必须已由 03 置位（防绕过锁定直接结算）；格数上限取配置
		var cfg gamemodel.DigBoxConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝箱不存在", errorx.CodeDigBoxNotFound))
			}
			return err
		}
		if !digCellsValid(in.CellIndexes, cfg.CellCount) {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=所选格子无效", errorx.CodeDigCellOccupied))
		}
		mask := digCellMask(in.CellIndexes)
		if sess.OccupiedBitmap&mask != mask {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=会话位图与所选格子不一致", errorx.CodeDigSessionInvalid))
		}

		// ③ 揭晓奖品（格行按 cell_index 排序，与 grants #idx 对齐）
		var cells []*gamemodel.DigBoxCells
		if err := db.Where("config_id = ? AND cell_index IN ?", in.ConfigId, in.CellIndexes).
			Order("cell_index ASC").Find(&cells).Error; err != nil {
			return err
		}
		prizesJSON, _ := digPrizesOfCells(l.ctx, l.svcCtx, cells)

		priceFen := int64(0)
		if in.Price != "" {
			f, err := utils.YuanToFen(in.Price)
			if err != nil || f < 0 {
				return status.Error(codes.Aborted, "code=400 msg=实付价格无效")
			}
			priceFen = f
		}
		rec := &gamemodelnc.DigBoxRecords{
			RequestId:     in.RequestId,
			UserId:        in.UserId,
			ConfigId:      in.ConfigId,
			SessionNo:     in.SessionNo,
			CellsChosen:   digJoinCells(in.CellIndexes),
			PrizesJson:    prizesJSON,
			TotalPriceFen: priceFen,
			Status:        digRecordNormal,
			CreatedAt:     time.Now(),
		}
		if err := db.Create(rec).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errDigRecordDup
			}
			return err
		}
		outRecordId = rec.Id

		if sess.OccupiedBitmap == (int64(1)<<cfg.CellCount)-1 {
			if err := db.Model(&gamemodelnc.DigBoxSessions{}).Where("id = ?", sess.Id).
				Updates(map[string]interface{}{"status": digSessionCompleted, "updated_at": time.Now()}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, errDigRecordDup) {

		return l.readBackRecord(in.RequestId, in.UserId)
	}
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleDigBox aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleDigBox failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {

		return l.readBackRecord(in.RequestId, in.UserId)
	}
	l.Infof("SettleDigBox success, requestId: %s, recordId: %d", in.RequestId, outRecordId)
	return &play.SettleDigBoxResp{RecordId: outRecordId, Status: "SETTLED"}, nil
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

	var outRecordId int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleDigBoxRevert", func(db *gorm.DB) error {

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

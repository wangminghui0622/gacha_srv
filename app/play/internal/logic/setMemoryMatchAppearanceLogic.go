package logic

import (
	"context"
	"errors"
	"time"

	"gacha_srv/app/play/internal/core/memorymatch"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SetMemoryMatchAppearanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetMemoryMatchAppearanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetMemoryMatchAppearanceLogic {
	return &SetMemoryMatchAppearanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SetMemoryMatchAppearance 存外观选择（UPSERT）：
// face：0=默认；非默认须存在于配置且（默认或已拥有权益）否则 3044；
// back：0=默认；非默认须存在于配置（背面预设不校验权益，同老项目 face 权益语义）。
func (l *SetMemoryMatchAppearanceLogic) SetMemoryMatchAppearance(in *play.SetMemoryMatchAppearanceReq) (*play.SetMemoryMatchAppearanceResp, error) {
	if in.ConfigId <= 0 || in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	cfg, _, err := mmLoadConfig(l.svcCtx.DB, in.ConfigId)
	if err != nil {
		return nil, err
	}
	faces, err := mmFacePresets(cfg)
	if err != nil {
		return nil, err
	}
	backs, err := mmBackPresets(cfg)
	if err != nil {
		return nil, err
	}

	faceID := in.FacePresetId
	if faceID == 0 {
		faceID = cfg.DefaultFacePresetId
	}
	if !memorymatch.PresetExists(faces, int(faceID)) {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "正面预设不存在")
	}
	if faceID != cfg.DefaultFacePresetId {
		var ent gamemodelnc.MemoryMatchAppearanceEntitlements
		err := l.svcCtx.DB.Where("config_id = ? AND user_id = ? AND preset_id = ?",
			in.ConfigId, in.UserId, faceID).First(&ent).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeMemoryMatchAppearanceNotOwned, "外观未拥有")
		}
		if err != nil {
			return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
		}
	}
	backID := in.BackPresetId
	if backID == 0 {
		backID = cfg.DefaultBackPresetId
	}
	if !memorymatch.BackPresetExists(backs, int(backID)) {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "背面预设不存在")
	}

	// UPSERT（uk(config_id,user_id)）
	row := &gamemodelnc.MemoryMatchAppearanceSelections{
		ConfigId: in.ConfigId, UserId: in.UserId,
		FacePresetId: faceID, BackPresetId: backID,
		Version: 1, UpdatedAt: time.Now(),
	}
	err = l.svcCtx.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "config_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"face_preset_id": faceID,
			"back_preset_id": backID,
			"updated_at":     time.Now(),
		}),
	}).Create(row).Error
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	view, err := mmAppearanceView(l.svcCtx.DB, cfg, in.UserId)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "数据库错误")
	}
	return &play.SetMemoryMatchAppearanceResp{Appearance: view}, nil
}

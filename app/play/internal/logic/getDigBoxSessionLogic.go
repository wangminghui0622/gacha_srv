package logic

import (
	"context"
	"errors"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type GetDigBoxSessionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDigBoxSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDigBoxSessionLogic {
	return &GetDigBoxSessionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetDigBoxSession 挖宝会话（uk(user_id,config_id)；无会话返回 hasSession=false + 配置格数。
// 非 SAGA RPC：业务错误 CodeError）
func (l *GetDigBoxSessionLogic) GetDigBoxSession(in *play.GetDigBoxSessionReq) (*play.GetDigBoxSessionResp, error) {
	if in.UserId <= 0 || in.BoxId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	var cfg gamemodel.DigBoxConfigs
	if err := l.svcCtx.DB.Where("id = ?", in.BoxId).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeDigBoxNotFound, "挖宝箱不存在")
		}
		l.Errorf("GetDigBoxSession query config failed, err: %v", err)
		return nil, err
	}

	var sess gamemodelnc.DigBoxSessions
	err := l.svcCtx.DB.Where("user_id = ? AND config_id = ?", in.UserId, in.BoxId).First(&sess).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &play.GetDigBoxSessionResp{
			HasSession: false,
			CellCount:  cfg.CellCount,
		}, nil
	}
	if err != nil {
		l.Errorf("GetDigBoxSession query failed, err: %v", err)
		return nil, err
	}
	return &play.GetDigBoxSessionResp{
		HasSession:     true,
		SessionNo:      sess.SessionNo,
		OccupiedBitmap: sess.OccupiedBitmap,
		TotalCellsPaid: sess.TotalCellsPaid,
		Status:         sess.Status,
		CellCount:      cfg.CellCount,
	}, nil
}

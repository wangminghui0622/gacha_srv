package gameimpl

import (
	"context"
	"errors"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

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

func NewListDigBoxesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDigBoxesLogic {
	return &ListDigBoxesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListDigBoxes 挖宝箱列表（status=1 上架；非 SAGA RPC：业务错误 CodeError，系统错误 Internal）
func (l *ListDigBoxesLogic) ListDigBoxes(in *play.ListDigBoxesReq) (*play.ListDigBoxesResp, error) {
	var cfgs []gamemodel.DigBoxConfigs
	if err := l.svcCtx.DB.Where("status = ?", 1).Order("id ASC").Find(&cfgs).Error; err != nil {
		l.Errorf("ListDigBoxes query failed, err: %v", err)
		return nil, err
	}
	items := make([]*play.DigBoxItem, 0, len(cfgs))
	for i := range cfgs {
		items = append(items, &play.DigBoxItem{
			BoxId:     cfgs[i].Id,
			Name:      cfgs[i].Name,
			Price:     utils.FenToYuan(cfgs[i].PriceFen),
			CellCount: cfgs[i].CellCount,
		})
	}
	return &play.ListDigBoxesResp{Items: items}, nil
}

func NewListDigRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDigRecordsLogic {
	return &ListDigRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListDigRecords 挖宝记录：requestId 非空=精确查一条（购买轮询），空=分页列表。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *ListDigRecordsLogic) ListDigRecords(in *play.ListDigRecordsReq) (*play.ListDigRecordsResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}

	if in.RequestId != "" {
		var rec gamemodelnc.DigBoxRecords
		if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", in.RequestId, in.UserId).
			First(&rec).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {

				return &play.ListDigRecordsResp{Items: []*play.DigRecordItem{}, Total: 0}, nil
			}
			l.Errorf("ListDigRecords query failed, err: %v", err)
			return nil, err
		}
		item, err := l.buildItem(&rec)
		if err != nil {
			return nil, err
		}
		return &play.ListDigRecordsResp{Items: []*play.DigRecordItem{item}, Total: 1}, nil
	}

	page, pageSize := in.Page, in.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	if err := l.svcCtx.DB.Model(&gamemodelnc.DigBoxRecords{}).
		Where("user_id = ?", in.UserId).Count(&total).Error; err != nil {
		l.Errorf("ListDigRecords count failed, err: %v", err)
		return nil, err
	}
	var recs []gamemodelnc.DigBoxRecords
	if err := l.svcCtx.DB.Where("user_id = ?", in.UserId).
		Order("id DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).
		Find(&recs).Error; err != nil {
		l.Errorf("ListDigRecords page failed, err: %v", err)
		return nil, err
	}
	items := make([]*play.DigRecordItem, 0, len(recs))
	for i := range recs {
		item, err := l.buildItem(&recs[i])
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return &play.ListDigRecordsResp{Items: items, Total: total}, nil
}

package gameimpl

import (
	"context"
	"encoding/json"
	"strings"

	"gacha_srv/app/play/internal/repo"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type collectionConfig struct {
	Rewards []struct {
		Name  string `json:"name"`
		Stock int64  `json:"stock"`
	} `json:"rewards"`
}

func collectionSetView(row *repo.CollectionSetRow) *play.CollectionSet {
	if row == nil {
		return nil
	}
	return &play.CollectionSet{
		Id:             row.ID,
		Code:           row.Code,
		Name:           row.Name,
		Version:        row.Version,
		Status:         row.Status,
		AccumulateType: row.AccumulateType,
		CycleType:      row.CycleType,
		Sort:           row.Sort,
		ConfigJson:     row.ConfigJSON,
		StartAt:        row.StartAt,
		EndAt:          row.EndAt,
		CreatedAt:      row.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:      row.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func validateCollectionSet(code, name string, accumulateType, cycleType, status int64, configJSON string) error {
	if strings.TrimSpace(code) == "" {
		return errorx.NewCodeError(errorx.CodeParamError, "集合编码必填")
	}
	if strings.TrimSpace(name) == "" {
		return errorx.NewCodeError(errorx.CodeParamError, "集合名称必填")
	}
	if accumulateType != 1 && accumulateType != 2 {
		return errorx.NewCodeError(errorx.CodeParamError, "累计类型只能是 1 或 2")
	}
	if cycleType < -1 {
		return errorx.NewCodeError(errorx.CodeParamError, "循环轮数非法")
	}
	if status != 1 && status != 2 && status != 3 {
		return errorx.NewCodeError(errorx.CodeParamError, "状态只能是 1/2/3")
	}
	if strings.TrimSpace(configJSON) != "" && !json.Valid([]byte(configJSON)) {
		return errorx.NewCodeError(errorx.CodeParamError, "配置不是合法 JSON")
	}
	return nil
}

type ListCollectionSetsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCollectionSetsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCollectionSetsLogic {
	return &ListCollectionSetsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListCollectionSetsLogic) ListCollectionSets(in *play.ListCollectionSetsReq) (*play.ListCollectionSetsResp, error) {
	page, pageSize := in.Page, in.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	rows, total, err := repo.NewCollectionAdminStore(l.svcCtx.DB).ListCollectionSets(strings.TrimSpace(in.Keyword), in.Status, page, pageSize)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "读取图鉴集合失败")
	}
	resp := &play.ListCollectionSetsResp{Total: total, Page: page, PageSize: pageSize}
	for i := range rows {
		resp.Items = append(resp.Items, collectionSetView(&rows[i]))
	}
	return resp, nil
}

type CreateCollectionSetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateCollectionSetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCollectionSetLogic {
	return &CreateCollectionSetLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *CreateCollectionSetLogic) CreateCollectionSet(in *play.CreateCollectionSetReq) (*play.CollectionSet, error) {
	code := strings.TrimSpace(in.Code)
	name := strings.TrimSpace(in.Name)
	status := in.Status
	if status == 0 {
		status = 1
	}
	if err := validateCollectionSet(code, name, in.AccumulateType, in.CycleType, status, in.ConfigJson); err != nil {
		return nil, err
	}
	var count int64
	if err := l.svcCtx.DB.Model(&repo.CollectionSetRow{}).Where("code = ?", code).Count(&count).Error; err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "读取图鉴集合失败")
	}
	if count > 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "集合编码已存在")
	}
	row := &repo.CollectionSetRow{
		Code:           code,
		Name:           name,
		Version:        1,
		Status:         status,
		AccumulateType: in.AccumulateType,
		CycleType:      in.CycleType,
		Sort:           in.Sort,
		ConfigJSON:     in.ConfigJson,
		StartAt:        in.StartAt,
		EndAt:          in.EndAt,
	}
	if err := l.svcCtx.DB.Create(row).Error; err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "创建图鉴集合失败")
	}
	return collectionSetView(row), nil
}

type UpdateCollectionSetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateCollectionSetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateCollectionSetLogic {
	return &UpdateCollectionSetLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdateCollectionSetLogic) UpdateCollectionSet(in *play.UpdateCollectionSetReq) (*play.CollectionSet, error) {
	var row repo.CollectionSetRow
	if err := l.svcCtx.DB.Where("id = ?", in.Id).First(&row).Error; err != nil {
		return nil, errorx.NewCodeError(errorx.CodeNotFound, "图鉴集合不存在")
	}
	name := strings.TrimSpace(in.Name)
	if err := validateCollectionSet(row.Code, name, in.AccumulateType, in.CycleType, in.Status, in.ConfigJson); err != nil {
		return nil, err
	}
	updates := map[string]any{
		"name":            name,
		"accumulate_type": in.AccumulateType,
		"cycle_type":      in.CycleType,
		"sort":            in.Sort,
		"status":          in.Status,
		"config_json":     in.ConfigJson,
		"start_at":        in.StartAt,
		"end_at":          in.EndAt,
	}
	if in.ConfigJson != "" && in.ConfigJson != row.ConfigJSON {
		updates["version"] = row.Version + 1
	}
	if err := l.svcCtx.DB.Model(&repo.CollectionSetRow{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "更新图鉴集合失败")
	}
	if err := l.svcCtx.DB.Where("id = ?", row.ID).First(&row).Error; err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "读取图鉴集合失败")
	}
	return collectionSetView(&row), nil
}

type SetCollectionSetStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetCollectionSetStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetCollectionSetStatusLogic {
	return &SetCollectionSetStatusLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *SetCollectionSetStatusLogic) SetCollectionSetStatus(in *play.SetCollectionSetStatusReq) (*play.CollectionSet, error) {
	if in.Status != 1 && in.Status != 2 && in.Status != 3 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "状态只能是 1/2/3")
	}
	var row repo.CollectionSetRow
	if err := l.svcCtx.DB.Where("id = ?", in.Id).First(&row).Error; err != nil {
		return nil, errorx.NewCodeError(errorx.CodeNotFound, "图鉴集合不存在")
	}
	if err := l.svcCtx.DB.Model(&repo.CollectionSetRow{}).Where("id = ?", row.ID).Update("status", in.Status).Error; err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "更新状态失败")
	}
	row.Status = in.Status
	return collectionSetView(&row), nil
}

type GetCollectionSetDataLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCollectionSetDataLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCollectionSetDataLogic {
	return &GetCollectionSetDataLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetCollectionSetDataLogic) GetCollectionSetData(in *play.GetCollectionSetDataReq) (*play.GetCollectionSetDataResp, error) {
	var row repo.CollectionSetRow
	if err := l.svcCtx.DB.Where("id = ?", in.Id).First(&row).Error; err != nil {
		return nil, errorx.NewCodeError(errorx.CodeNotFound, "图鉴集合不存在")
	}
	resp := &play.GetCollectionSetDataResp{}
	var cfg collectionConfig
	if json.Unmarshal([]byte(row.ConfigJSON), &cfg) == nil {
		for _, reward := range cfg.Rewards {
			resp.Rewards = append(resp.Rewards, &play.CollectionRewardStat{Name: reward.Name, Stock: reward.Stock})
		}
	}
	return resp, nil
}

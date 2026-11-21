package logic

import (
	"context"
	"errors"
	"time"

	"gacha_srv/app/play/internal/repo"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type SaveMemoryMatchAppearanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSaveMemoryMatchAppearanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveMemoryMatchAppearanceLogic {
	return &SaveMemoryMatchAppearanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SaveMemoryMatchAppearanceLogic) SaveMemoryMatchAppearance(in *play.SaveMemoryMatchAppearanceReq) (*play.SaveMemoryMatchAppearanceResp, error) {
	if in == nil || in.Item == nil {
		return nil, mmValidateAppearance(nil)
	}
	if err := mmValidateAppearance(in.Item); err != nil {
		return nil, err
	}
	now := time.Now()
	row := &repo.MemoryMatchAppearanceRow{
		ID:             in.Item.Id,
		AppearanceType: in.Item.Type,
		Name:           in.Item.Name,
		ImageURL:       in.Item.ImageUrl,
		BackImageURL:   in.Item.BackImageUrl,
		Status:         in.Item.Status,
		Free:           in.Item.Free,
		ExchangePoints: in.Item.ExchangePoints,
		FunctionType:   in.Item.FunctionType,
		FunctionConfig: in.Item.FunctionConfig,
		Weight:         in.Item.Weight,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	store := repo.NewMemoryMatchAdminStore(l.svcCtx.DB)
	if row.ID != 0 {
		var old repo.MemoryMatchAppearanceRow
		if err := l.svcCtx.DB.WithContext(l.ctx).First(&old, row.ID).Error; err == nil {
			row.CreatedAt = old.CreatedAt
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, mmDBError(err)
		}
	}
	if err := store.SaveAppearance(l.ctx, row); err != nil {
		return nil, mmDBError(err)
	}
	return &play.SaveMemoryMatchAppearanceResp{Item: mmAppearanceToProto(row)}, nil
}

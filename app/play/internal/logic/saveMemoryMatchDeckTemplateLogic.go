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

type SaveMemoryMatchDeckTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSaveMemoryMatchDeckTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveMemoryMatchDeckTemplateLogic {
	return &SaveMemoryMatchDeckTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SaveMemoryMatchDeckTemplateLogic) SaveMemoryMatchDeckTemplate(in *play.SaveMemoryMatchDeckTemplateReq) (*play.SaveMemoryMatchDeckTemplateResp, error) {
	if in == nil || in.Template == nil {
		return nil, mmValidateDeckTemplate(nil)
	}
	if err := mmValidateDeckTemplate(in.Template); err != nil {
		return nil, err
	}
	now := time.Now()
	row := &repo.MemoryMatchDeckTemplateRow{
		ID:         in.Template.Id,
		Name:       in.Template.Name,
		Status:     in.Template.Status,
		TotalCards: in.Template.TotalCards,
		Weight:     in.Template.Weight,
		ConfigJSON: in.Template.ConfigJson,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	store := repo.NewMemoryMatchAdminStore(l.svcCtx.DB)
	if row.ID != 0 {
		var old repo.MemoryMatchDeckTemplateRow
		if err := l.svcCtx.DB.WithContext(l.ctx).First(&old, row.ID).Error; err == nil {
			row.CreatedAt = old.CreatedAt
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, mmDBError(err)
		}
	}
	if err := store.SaveDeckTemplate(l.ctx, row); err != nil {
		return nil, mmDBError(err)
	}
	return &play.SaveMemoryMatchDeckTemplateResp{Template: mmDeckTemplateToProto(row)}, nil
}

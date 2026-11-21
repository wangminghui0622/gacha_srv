package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListCollectionSetsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCollectionSetsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCollectionSetsLogic {
	return &ListCollectionSetsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListCollectionSetsLogic) ListCollectionSets(in *play.ListCollectionSetsReq) (*play.ListCollectionSetsResp, error) {
	return gameimpl.NewListCollectionSetsLogic(l.ctx, l.svcCtx).ListCollectionSets(in)
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
	return gameimpl.NewCreateCollectionSetLogic(l.ctx, l.svcCtx).CreateCollectionSet(in)
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
	return gameimpl.NewUpdateCollectionSetLogic(l.ctx, l.svcCtx).UpdateCollectionSet(in)
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
	return gameimpl.NewSetCollectionSetStatusLogic(l.ctx, l.svcCtx).SetCollectionSetStatus(in)
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
	return gameimpl.NewGetCollectionSetDataLogic(l.ctx, l.svcCtx).GetCollectionSetData(in)
}

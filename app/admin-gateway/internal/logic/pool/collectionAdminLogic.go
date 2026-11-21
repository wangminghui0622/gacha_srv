package pool

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListCollectionSetsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListCollectionSetsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCollectionSetsLogic {
	return &ListCollectionSetsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListCollectionSetsLogic) ListCollectionSets(req *play.ListCollectionSetsReq) (*play.ListCollectionSetsResp, error) {
	return l.svcCtx.PlayRpc.ListCollectionSets(l.ctx, req)
}

type CreateCollectionSetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateCollectionSetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCollectionSetLogic {
	return &CreateCollectionSetLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateCollectionSetLogic) CreateCollectionSet(req *play.CreateCollectionSetReq) (*play.CollectionSet, error) {
	return l.svcCtx.PlayRpc.CreateCollectionSet(l.ctx, req)
}

type UpdateCollectionSetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateCollectionSetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateCollectionSetLogic {
	return &UpdateCollectionSetLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateCollectionSetLogic) UpdateCollectionSet(req *play.UpdateCollectionSetReq) (*play.CollectionSet, error) {
	return l.svcCtx.PlayRpc.UpdateCollectionSet(l.ctx, req)
}

type SetCollectionSetStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSetCollectionSetStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetCollectionSetStatusLogic {
	return &SetCollectionSetStatusLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *SetCollectionSetStatusLogic) SetCollectionSetStatus(req *play.SetCollectionSetStatusReq) (*play.CollectionSet, error) {
	return l.svcCtx.PlayRpc.SetCollectionSetStatus(l.ctx, req)
}

type GetCollectionSetDataLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetCollectionSetDataLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCollectionSetDataLogic {
	return &GetCollectionSetDataLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetCollectionSetDataLogic) GetCollectionSetData(req *play.GetCollectionSetDataReq) (*play.GetCollectionSetDataResp, error) {
	return l.svcCtx.PlayRpc.GetCollectionSetData(l.ctx, req)
}

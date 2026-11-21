package match

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/play/play"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMemoryMatchGlobalRulesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetMemoryMatchGlobalRulesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMemoryMatchGlobalRulesLogic {
	return &GetMemoryMatchGlobalRulesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetMemoryMatchGlobalRulesLogic) GetMemoryMatchGlobalRules(req *play.GetMemoryMatchGlobalRulesReq) (*play.GetMemoryMatchGlobalRulesResp, error) {
	return l.svcCtx.PlayRpc.GetMemoryMatchGlobalRules(l.ctx, req)
}

type SaveMemoryMatchGlobalRulesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSaveMemoryMatchGlobalRulesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveMemoryMatchGlobalRulesLogic {
	return &SaveMemoryMatchGlobalRulesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *SaveMemoryMatchGlobalRulesLogic) SaveMemoryMatchGlobalRules(req *play.SaveMemoryMatchGlobalRulesReq) (*play.SaveMemoryMatchGlobalRulesResp, error) {
	return l.svcCtx.PlayRpc.SaveMemoryMatchGlobalRules(l.ctx, req)
}

type ListMemoryMatchDeckTemplatesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListMemoryMatchDeckTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMemoryMatchDeckTemplatesLogic {
	return &ListMemoryMatchDeckTemplatesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListMemoryMatchDeckTemplatesLogic) ListMemoryMatchDeckTemplates(req *play.ListMemoryMatchDeckTemplatesReq) (*play.ListMemoryMatchDeckTemplatesResp, error) {
	return l.svcCtx.PlayRpc.ListMemoryMatchDeckTemplates(l.ctx, req)
}

type SaveMemoryMatchDeckTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSaveMemoryMatchDeckTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveMemoryMatchDeckTemplateLogic {
	return &SaveMemoryMatchDeckTemplateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *SaveMemoryMatchDeckTemplateLogic) SaveMemoryMatchDeckTemplate(req *play.SaveMemoryMatchDeckTemplateReq) (*play.SaveMemoryMatchDeckTemplateResp, error) {
	return l.svcCtx.PlayRpc.SaveMemoryMatchDeckTemplate(l.ctx, req)
}

type DeleteMemoryMatchDeckTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteMemoryMatchDeckTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteMemoryMatchDeckTemplateLogic {
	return &DeleteMemoryMatchDeckTemplateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DeleteMemoryMatchDeckTemplateLogic) DeleteMemoryMatchDeckTemplate(req *play.DeleteMemoryMatchDeckTemplateReq) (*play.DeleteMemoryMatchDeckTemplateResp, error) {
	return l.svcCtx.PlayRpc.DeleteMemoryMatchDeckTemplate(l.ctx, req)
}

type ListMemoryMatchAppearancesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListMemoryMatchAppearancesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMemoryMatchAppearancesLogic {
	return &ListMemoryMatchAppearancesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListMemoryMatchAppearancesLogic) ListMemoryMatchAppearances(req *play.ListMemoryMatchAppearancesReq) (*play.ListMemoryMatchAppearancesResp, error) {
	return l.svcCtx.PlayRpc.ListMemoryMatchAppearances(l.ctx, req)
}

type SaveMemoryMatchAppearanceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSaveMemoryMatchAppearanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveMemoryMatchAppearanceLogic {
	return &SaveMemoryMatchAppearanceLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *SaveMemoryMatchAppearanceLogic) SaveMemoryMatchAppearance(req *play.SaveMemoryMatchAppearanceReq) (*play.SaveMemoryMatchAppearanceResp, error) {
	return l.svcCtx.PlayRpc.SaveMemoryMatchAppearance(l.ctx, req)
}

type DeleteMemoryMatchAppearanceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteMemoryMatchAppearanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteMemoryMatchAppearanceLogic {
	return &DeleteMemoryMatchAppearanceLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DeleteMemoryMatchAppearanceLogic) DeleteMemoryMatchAppearance(req *play.DeleteMemoryMatchAppearanceReq) (*play.DeleteMemoryMatchAppearanceResp, error) {
	return l.svcCtx.PlayRpc.DeleteMemoryMatchAppearance(l.ctx, req)
}

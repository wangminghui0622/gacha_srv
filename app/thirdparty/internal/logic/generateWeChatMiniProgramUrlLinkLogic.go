package logic

import (
	"context"

	"gacha_srv/app/thirdparty/internal/svc"
	"gacha_srv/app/thirdparty/thirdparty"

	"github.com/zeromicro/go-zero/core/logx"
)

type GenerateWeChatMiniProgramUrlLinkLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGenerateWeChatMiniProgramUrlLinkLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GenerateWeChatMiniProgramUrlLinkLogic {
	return &GenerateWeChatMiniProgramUrlLinkLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GenerateWeChatMiniProgramUrlLinkLogic) GenerateWeChatMiniProgramUrlLink(in *thirdparty.GenerateWeChatMiniProgramUrlLinkReq) (*thirdparty.GenerateWeChatMiniProgramUrlLinkResp, error) {
	// server_v2 GenerateWeChatMiniProgramUrlLink 同款（path 必填回 url_link）
	link, err := l.svcCtx.Wechat.GenerateMiniProgramUrlLink(l.ctx, in.GetPath(), in.GetQuery())
	if err != nil {
		return nil, mapWeChatError(err)
	}
	return &thirdparty.GenerateWeChatMiniProgramUrlLinkResp{UrlLink: link}, nil
}

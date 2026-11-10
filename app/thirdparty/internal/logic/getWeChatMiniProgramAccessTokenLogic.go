package logic

import (
	"context"

	"gacha_srv/app/thirdparty/internal/svc"
	"gacha_srv/app/thirdparty/thirdparty"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetWeChatMiniProgramAccessTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWeChatMiniProgramAccessTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWeChatMiniProgramAccessTokenLogic {
	return &GetWeChatMiniProgramAccessTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetWeChatMiniProgramAccessTokenLogic) GetWeChatMiniProgramAccessToken(in *thirdparty.GetWeChatMiniProgramAccessTokenReq) (*thirdparty.GetWeChatMiniProgramAccessTokenResp, error) {
	// server_v2 GetWeChatMiniProgramAccessToken 同款：本地 2h 缓存 → Redis → /cgi-bin/token
	token, err := l.svcCtx.Wechat.MiniProgramAccessToken(l.ctx)
	if err != nil {
		return nil, mapWeChatError(err)
	}
	return &thirdparty.GetWeChatMiniProgramAccessTokenResp{AccessToken: token}, nil
}

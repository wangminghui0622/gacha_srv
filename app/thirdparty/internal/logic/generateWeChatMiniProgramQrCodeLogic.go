package logic

import (
	"context"

	"gacha_srv/app/thirdparty/internal/svc"
	"gacha_srv/app/thirdparty/thirdparty"

	"github.com/zeromicro/go-zero/core/logx"
)

type GenerateWeChatMiniProgramQrCodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGenerateWeChatMiniProgramQrCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GenerateWeChatMiniProgramQrCodeLogic {
	return &GenerateWeChatMiniProgramQrCodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GenerateWeChatMiniProgramQrCodeLogic) GenerateWeChatMiniProgramQrCode(in *thirdparty.GenerateWeChatMiniProgramQrCodeReq) (*thirdparty.GenerateWeChatMiniProgramQrCodeResp, error) {
	// server_v2 GenerateWeChatMiniProgramQrCode 同款（scene 必填/page 空用 DefaultPage/width 0→430）
	img, err := l.svcCtx.Wechat.GenerateMiniProgramQrCode(l.ctx, in.GetScene(), in.GetPage(), in.GetWidth(), in.GetCheckPath())
	if err != nil {
		return nil, mapWeChatError(err)
	}
	return &thirdparty.GenerateWeChatMiniProgramQrCodeResp{ImagePng: img}, nil
}

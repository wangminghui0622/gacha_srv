package logic

import (
	"context"

	"gacha_srv/app/thirdparty/internal/svc"
	"gacha_srv/app/thirdparty/thirdparty"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetWeChatMiniProgramPhoneNumberLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWeChatMiniProgramPhoneNumberLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWeChatMiniProgramPhoneNumberLogic {
	return &GetWeChatMiniProgramPhoneNumberLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetWeChatMiniProgramPhoneNumberLogic) GetWeChatMiniProgramPhoneNumber(in *thirdparty.GetWeChatMiniProgramPhoneNumberReq) (*thirdparty.GetWeChatMiniProgramPhoneNumberResp, error) {
	// server_v2 GetWeChatMiniProgramPhoneNumber 同款（errcode 40001/42001 清 token 重试在 gateway 内）
	phone, country, err := l.svcCtx.Wechat.MiniProgramPhoneNumber(l.ctx, in.GetPhoneCode())
	if err != nil {
		return nil, mapWeChatError(err)
	}
	return &thirdparty.GetWeChatMiniProgramPhoneNumberResp{PhoneNumber: phone, CountryCode: country}, nil
}

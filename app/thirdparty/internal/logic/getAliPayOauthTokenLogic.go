package logic

import (
	"context"

	"gacha_srv/app/thirdparty/internal/alipay"
	"gacha_srv/app/thirdparty/internal/svc"
	"gacha_srv/app/thirdparty/thirdparty"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAliPayOauthTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAliPayOauthTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAliPayOauthTokenLogic {
	return &GetAliPayOauthTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetAliPayOauthTokenLogic) GetAliPayOauthToken(in *thirdparty.GetAliPayOauthTokenReq) (*thirdparty.GetAliPayOauthTokenResp, error) {
	// server_v2 GetAliPayOauthToken 同款：未配置 6007，其余错误 6008
	if l.svcCtx.Alipay == nil {
		return nil, errorx.NewCodeError(errorx.CodeAlipayNotConfigured, alipay.ErrNotConfigured.Error())
	}
	reply, err := l.svcCtx.Alipay.ExchangeToken(l.ctx, in)
	if err != nil {
		return nil, mapAlipayError(err)
	}
	return reply, nil
}

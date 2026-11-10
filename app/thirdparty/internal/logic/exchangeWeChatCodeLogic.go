package logic

import (
	"context"

	"gacha_srv/app/thirdparty/internal/svc"
	"gacha_srv/app/thirdparty/thirdparty"

	"github.com/zeromicro/go-zero/core/logx"
)

type ExchangeWeChatCodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewExchangeWeChatCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExchangeWeChatCodeLogic {
	return &ExchangeWeChatCodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ExchangeWeChatCodeLogic) ExchangeWeChatCode(in *thirdparty.ExchangeWeChatCodeReq) (*thirdparty.ExchangeWeChatCodeResp, error) {
	openID, unionID, sessionKey, err := l.svcCtx.Wechat.ExchangeCode(l.ctx, in.Scene, in.Code)
	if err != nil {
		return nil, mapWeChatError(err)
	}
	return &thirdparty.ExchangeWeChatCodeResp{OpenId: openID, UnionId: unionID, SessionKey: sessionKey}, nil
}

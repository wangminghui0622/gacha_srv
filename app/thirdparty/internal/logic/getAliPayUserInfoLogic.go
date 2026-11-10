package logic

import (
	"context"

	"gacha_srv/app/thirdparty/internal/svc"
	"gacha_srv/app/thirdparty/thirdparty"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAliPayUserInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAliPayUserInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAliPayUserInfoLogic {
	return &GetAliPayUserInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetAliPayUserInfoLogic) GetAliPayUserInfo(in *thirdparty.GetAliPayUserInfoReq) (*thirdparty.GetAliPayUserInfoResp, error) {
	// server_v2 同款空实现：老项目该 RPC 是空 stub（无任何调用方），接口保持返回空响应
	return &thirdparty.GetAliPayUserInfoResp{}, nil
}

package logic

import (
	"context"

	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConsoleLogoutLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConsoleLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConsoleLogoutLogic {
	return &ConsoleLogoutLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ConsoleLogout 吊销会话（session_id = 网关从 JWT jti 解析传入；同时写 Redis 吊销键）。
func (l *ConsoleLogoutLogic) ConsoleLogout(in *ops.ConsoleLogoutRequest) (*ops.ConsoleLogoutReply, error) {
	if in.SessionId == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "会话 ID 必填")
	}
	if err := l.svcCtx.SessionMgr.Logout(l.ctx, in.SessionId); err != nil {
		return nil, err
	}
	return &ops.ConsoleLogoutReply{Revoked: true}, nil
}

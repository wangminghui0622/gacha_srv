package logic

import (
	"context"

	"gacha_srv/app/ops/internal/core"
	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConsoleRefreshLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConsoleRefreshLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConsoleRefreshLogic {
	return &ConsoleRefreshLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ConsoleRefresh 旋转 refresh 令牌（指纹校验 + 家族吊销 + 滑动续期）。
func (l *ConsoleRefreshLogic) ConsoleRefresh(in *ops.ConsoleRefreshRequest) (*ops.ConsoleSessionTokens, error) {
	if in.RefreshToken == "" || in.ClientFingerprint == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "refresh 令牌与客户端指纹必填")
	}
	pair, err := l.svcCtx.SessionMgr.Refresh(l.ctx, in.RefreshToken, in.ClientFingerprint)
	if err == core.ErrInvalidRefreshToken {
		return nil, errorx.NewCodeError(errorx.CodeConsoleRefreshInvalid, "refresh 令牌无效")
	}
	if err != nil {
		return nil, err
	}
	return toTokens(pair), nil
}

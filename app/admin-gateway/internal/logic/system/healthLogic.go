// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package system

import (
	"context"

	"gacha_srv/app/admin-gateway/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type HealthLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 健康检查（公开，无中间件）
func NewHealthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HealthLogic {
	return &HealthLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *HealthLogic) Health() (any, error) {
	return map[string]string{"status": "ok"}, nil
}

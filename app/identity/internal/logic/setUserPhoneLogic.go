package logic

import (
	"context"

	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/app/identity/identity"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetUserPhoneLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetUserPhoneLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetUserPhoneLogic {
	return &SetUserPhoneLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ── M15a 手机号绑定（内部 RPC，无网关路由——绑定入口照抄 server_v2）──
// SetUserPhone 绑定/改绑手机号（盲索引写入；跨用户冲突 → 1012）
func (l *SetUserPhoneLogic) SetUserPhone(in *identity.SetUserPhoneReq) (*identity.SetUserPhoneResp, error) {
	if in.TenantId == 0 || in.UserId == 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "tenant and user required")
	}
	if err := l.svcCtx.PhoneRepo.Set(l.ctx, in.TenantId, in.UserId, in.Phone); err != nil {
		return nil, mapPhoneRepoError(err)
	}
	return &identity.SetUserPhoneResp{UserId: in.UserId}, nil
}

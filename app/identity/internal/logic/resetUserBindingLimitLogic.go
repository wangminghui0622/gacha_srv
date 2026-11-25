package logic

import (
	"context"
	"strings"
	"time"

	"gacha_srv/app/identity/identity"
	"gacha_srv/app/identity/internal/repo"
	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResetUserBindingLimitLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResetUserBindingLimitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetUserBindingLimitLogic {
	return &ResetUserBindingLimitLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ResetUserBindingLimitLogic) ResetUserBindingLimit(in *identity.ResetUserBindingLimitReq) (*identity.ResetUserBindingLimitResp, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "tenantId 须为 1（单租户）")
	}
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户ID不能为空")
	}
	kind := strings.TrimSpace(in.Kind)
	if kind != "sms" && kind != "realname" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "重置类型无效")
	}
	if _, err := l.svcCtx.UserModel.FindOne(l.ctx, in.UserId); err != nil {
		return nil, errorx.NewCodeError(errorx.CodeUserNotFound, "用户不存在")
	}
	settings, err := repo.GetUserFinanceSettings(l.svcCtx.DB.WithContext(l.ctx), in.UserId)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if settings == nil {
		settings = &repo.UserFinanceSettings{
			UserID:    in.UserId,
			CreatedAt: now,
		}
	}
	if kind == "sms" {
		settings.SMSLimitResetAt = &now
	} else {
		settings.RealnameLimitResetAt = &now
	}
	settings.UpdatedAt = now
	if err := repo.SaveUserFinanceSettings(l.svcCtx.DB.WithContext(l.ctx), settings); err != nil {
		return nil, err
	}
	return &identity.ResetUserBindingLimitResp{}, nil
}

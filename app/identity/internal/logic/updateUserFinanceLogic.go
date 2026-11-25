package logic

import (
	"context"
	"strconv"
	"time"

	"gacha_srv/app/identity/identity"
	"gacha_srv/app/identity/internal/repo"
	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateUserFinanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateUserFinanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserFinanceLogic {
	return &UpdateUserFinanceLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdateUserFinanceLogic) UpdateUserFinance(in *identity.UpdateUserFinanceReq) (*identity.UserFinance, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "tenantId 须为 1（单租户）")
	}
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户ID不能为空")
	}
	if _, err := l.svcCtx.UserModel.FindOne(l.ctx, in.UserId); err != nil {
		return nil, errorx.NewCodeError(errorx.CodeUserNotFound, "用户不存在")
	}
	if in.RefundLimitFen < 0 || in.RefundLimitCount < 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "退款上限不能为负数")
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
	walletSettings := repo.UnmarshalWalletSettings(settings.WalletSettingsJSON)
	for _, wallet := range in.Wallets {
		if wallet == nil || wallet.Kind < 1 || wallet.Kind > 5 {
			continue
		}
		walletSettings[strconv.Itoa(int(wallet.Kind))] = wallet.Enabled
	}
	settings.WalletSettingsJSON = repo.MarshalWalletSettings(walletSettings)
	settings.RefundLimitFen = in.RefundLimitFen
	settings.RefundLimitCount = in.RefundLimitCount
	settings.UpdatedAt = now
	if err := repo.SaveUserFinanceSettings(l.svcCtx.DB.WithContext(l.ctx), settings); err != nil {
		return nil, err
	}
	saved, err := repo.GetUserFinanceSettings(l.svcCtx.DB.WithContext(l.ctx), in.UserId)
	if err != nil {
		return nil, err
	}
	return buildUserFinance(in.UserId, saved), nil
}

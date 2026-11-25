package logic

import (
	"context"

	"gacha_srv/app/identity/identity"
	"gacha_srv/app/identity/internal/repo"
	"gacha_srv/app/identity/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	defaultRefundLimitFen   int64 = 500000 // 5000 元
	defaultRefundLimitCount int32 = 10
)

type GetUserFinanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserFinanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserFinanceLogic {
	return &GetUserFinanceLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetUserFinanceLogic) GetUserFinance(in *identity.GetUserFinanceReq) (*identity.UserFinance, error) {
	if in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "tenantId 须为 1（单租户）")
	}
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户ID不能为空")
	}
	if _, err := l.svcCtx.UserModel.FindOne(l.ctx, in.UserId); err != nil {
		return nil, errorx.NewCodeError(errorx.CodeUserNotFound, "用户不存在")
	}
	settings, err := repo.GetUserFinanceSettings(l.svcCtx.DB.WithContext(l.ctx), in.UserId)
	if err != nil {
		return nil, err
	}
	return buildUserFinance(in.UserId, settings), nil
}

func buildUserFinance(userID int64, settings *repo.UserFinanceSettings) *identity.UserFinance {
	enabled := map[string]bool{
		"1": true, "2": true, "3": true, "4": true, "5": true,
	}
	refundLimitFen := int64(0)
	refundLimitCount := int32(0)
	if settings != nil {
		for key, value := range repo.UnmarshalWalletSettings(settings.WalletSettingsJSON) {
			enabled[key] = value
		}
		refundLimitFen = settings.RefundLimitFen
		refundLimitCount = settings.RefundLimitCount
	}
	wallets := make([]*identity.WalletSetting, 0, 5)
	for kind := 1; kind <= 5; kind++ {
		key := string(rune('0' + kind))
		wallets = append(wallets, &identity.WalletSetting{
			Kind:    int32(kind),
			Enabled: enabled[key],
		})
	}
	effectiveLimitFen := refundLimitFen
	if effectiveLimitFen <= 0 {
		effectiveLimitFen = defaultRefundLimitFen
	}
	effectiveLimitCount := refundLimitCount
	if effectiveLimitCount <= 0 {
		effectiveLimitCount = defaultRefundLimitCount
	}
	return &identity.UserFinance{
		UserId:              userID,
		Wallets:             wallets,
		RefundLimitFen:      refundLimitFen,
		RefundLimitCount:    refundLimitCount,
		EffectiveLimitFen:   effectiveLimitFen,
		EffectiveLimitCount: effectiveLimitCount,
	}
}

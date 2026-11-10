package logic

import (
	"context"
	"errors"

	"gacha_srv/app/base/base"
	"gacha_srv/app/base/internal/repo"
	"gacha_srv/app/base/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type GetPolicyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPolicyLogic {
	return &GetPolicyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetPolicy 读政策（D3）：version=0 读 latest，非 0 读指定版本。
func (l *GetPolicyLogic) GetPolicy(in *base.GetPolicyRequest) (*base.EligibilityPolicyRecord, error) {
	if (in.TenantId != 0 && in.TenantId != 1) || in.PolicyId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodePolicyInvalid, "tenantId 须为 1 且 policyId 必须大于 0")
	}
	db := l.svcCtx.DB.WithContext(l.ctx)
	head, err := repo.GetEligibilityHead(db, int64(in.PolicyId))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errorx.NewCodeError(errorx.CodePolicyNotFound, "政策不存在")
	}
	if err != nil {
		l.Errorf("GetPolicy find head failed, policyId: %d, err: %v", in.PolicyId, err)
		return nil, errorx.FromMySQLError(err)
	}
	var versionRow *repo.EligibilityVersionRow
	if in.Version == 0 {
		versionRow, err = repo.GetLatestEligibilityVersion(db, head.ID)
	} else {
		versionRow, err = repo.GetEligibilityVersion(db, head.ID, int64(in.Version))
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errorx.NewCodeError(errorx.CodePolicyNotFound, "政策版本不存在")
	}
	if err != nil {
		l.Errorf("GetPolicy find version failed, policyId: %d, version: %d, err: %v",
			in.PolicyId, in.Version, err)
		return nil, errorx.FromMySQLError(err)
	}
	rules, err := repo.ListEligibilityRules(db, head.ID, versionRow.Version)
	if err != nil {
		l.Errorf("GetPolicy list rules failed, policyId: %d, err: %v", in.PolicyId, err)
		return nil, errorx.FromMySQLError(err)
	}
	return policyRecordOf(head, versionRow, rules), nil
}

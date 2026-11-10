package logic

import (
	"context"

	"gacha_srv/app/base/base"
	"gacha_srv/app/base/internal/repo"
	"gacha_srv/app/base/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPoliciesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPoliciesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPoliciesLogic {
	return &ListPoliciesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListPolicies 政策列表（D3）：游标 after_policy_id 正序分页，回 head + latest 版本。
func (l *ListPoliciesLogic) ListPolicies(in *base.ListPoliciesRequest) (*base.ListPoliciesReply, error) {
	if in.TenantId != 0 && in.TenantId != 1 {
		return nil, errorx.NewCodeError(errorx.CodePolicyInvalid, "tenantId 须为 1")
	}
	pageSize := int(in.PageSize)
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	db := l.svcCtx.DB.WithContext(l.ctx)

	var heads []repo.EligibilityHead
	if err := db.Where("id > ?", in.AfterPolicyId).Order("id").
		Limit(pageSize + 1).Find(&heads).Error; err != nil {
		l.Errorf("ListPolicies query failed, err: %v", err)
		return nil, errorx.FromMySQLError(err)
	}
	reply := &base.ListPoliciesReply{}
	hasMore := len(heads) > pageSize
	if hasMore {
		heads = heads[:pageSize]
	}
	for i := range heads {
		h := &heads[i]
		v, err := repo.GetLatestEligibilityVersion(db, h.ID)
		if err != nil {
			l.Errorf("ListPolicies get version failed, policyId: %d, err: %v", h.ID, err)
			return nil, errorx.FromMySQLError(err)
		}
		rules, err := repo.ListEligibilityRules(db, h.ID, v.Version)
		if err != nil {
			l.Errorf("ListPolicies list rules failed, policyId: %d, err: %v", h.ID, err)
			return nil, errorx.FromMySQLError(err)
		}
		reply.Policies = append(reply.Policies, policyRecordOf(h, v, rules))
	}
	if hasMore {
		reply.NextAfterPolicyId = uint64(heads[len(heads)-1].ID)
	}
	return reply, nil
}

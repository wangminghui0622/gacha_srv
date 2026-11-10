package logic

import (
	"bytes"
	"context"
	"errors"
	"time"

	"gacha_srv/app/base/base"
	"gacha_srv/app/base/internal/repo"
	"gacha_srv/app/base/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type PublishPolicyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublishPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishPolicyLogic {
	return &PublishPolicyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PublishPolicy 发布版本（D3）：锁头 + 锁版本行 → 状态必须 DRAFT 且 config_hash 与
// expectedConfigHash 一致（CAS）→ 旧 PUBLISHED 全部 RETIRE → 本行 PUBLISHED+published_at
// → head.current_version=v。config_hash 不符/非 DRAFT → 409 POLICY_CONFLICT。
func (l *PublishPolicyLogic) PublishPolicy(in *base.PublishPolicyRequest) (*base.EligibilityPolicyRecord, error) {
	if (in.TenantId != 0 && in.TenantId != 1) || in.PolicyId <= 0 || in.Version <= 0 {
		return nil, errorx.NewCodeError(errorx.CodePolicyInvalid, "tenantId 须为 1 且 policyId/version 必须大于 0")
	}
	if len(in.ExpectedConfigHash) != 32 {
		return nil, errorx.NewCodeError(errorx.CodePolicyInvalid, "expectedConfigHash 须为 32 字节")
	}

	var (
		head       *repo.EligibilityHead
		versionRow *repo.EligibilityVersionRow
	)
	err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		h, err := repo.LockEligibilityHead(tx, int64(in.PolicyId))
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.NewCodeError(errorx.CodePolicyNotFound, "政策不存在")
		}
		if err != nil {
			return err
		}
		v, err := repo.LockEligibilityVersion(tx, h.ID, int64(in.Version))
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.NewCodeError(errorx.CodePolicyNotFound, "政策版本不存在")
		}
		if err != nil {
			return err
		}
		if v.Status != repo.PolicyStatusDraft {
			return errorx.NewCodeError(errorx.CodePolicyConflict, "版本非 DRAFT 状态不可发布")
		}
		if !bytes.Equal(v.ConfigHash, in.ExpectedConfigHash) {
			return errorx.NewCodeError(errorx.CodePolicyConflict, "版本内容已变更（configHash 不符）")
		}
		if err := repo.PublishEligibilityVersion(tx, v, time.Now()); err != nil {
			return err
		}
		h.CurrentVersion = v.Version
		if err := repo.UpdateEligibilityHead(tx, h); err != nil {
			return err
		}
		// 发布后读回最新状态用于回显
		v.Status = repo.PolicyStatusPublished
		head, versionRow = h, v
		return nil
	})
	if err != nil {
		if errorx.IsCodeError(err) {
			return nil, err
		}
		l.Errorf("PublishPolicy failed, policyId: %d, version: %d, err: %v", in.PolicyId, in.Version, err)
		return nil, errorx.FromMySQLError(err)
	}
	rules, err := repo.ListEligibilityRules(l.svcCtx.DB.WithContext(l.ctx), head.ID, versionRow.Version)
	if err != nil {
		l.Errorf("PublishPolicy list rules failed, policyId: %d, err: %v", in.PolicyId, err)
		return nil, errorx.FromMySQLError(err)
	}
	l.Infof("PublishPolicy success, policyId: %d, version: %d", in.PolicyId, in.Version)
	return policyRecordOf(head, versionRow, rules), nil
}

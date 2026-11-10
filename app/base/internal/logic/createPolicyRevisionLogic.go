package logic

import (
	"context"
	"encoding/json"
	"errors"

	"gacha_srv/app/base/base"
	"gacha_srv/app/base/internal/core"
	"gacha_srv/app/base/internal/repo"
	"gacha_srv/app/base/internal/svc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type CreatePolicyRevisionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreatePolicyRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePolicyRevisionLogic {
	return &CreatePolicyRevisionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreatePolicyRevision 政策修订（D3）：锁头 → latest_version==expected CAS → v=latest+1
// DRAFT → 同步 head.name。同内容版本命中 uk(policy_id,config_hash) → 409 POLICY_CONFLICT。
func (l *CreatePolicyRevisionLogic) CreatePolicyRevision(in *base.CreatePolicyRevisionRequest) (*base.EligibilityPolicyRecord, error) {
	if in.PolicyId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodePolicyInvalid, "policyId 必须大于 0")
	}
	if err := core.ValidatePolicyDraft(in.TenantId, "", in.Name, in.Scene, in.Combination,
		in.Rules, in.RequiredFacts, false); err != nil {
		return nil, errorx.NewCodeError(errorx.CodePolicyInvalid, err.Error())
	}
	requiredFacts, err := core.ValidateRequiredFacts(in.RequiredFacts)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodePolicyInvalid, err.Error())
	}
	factsJSON, err := json.Marshal(requiredFacts)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodePolicyInvalid, "requiredFacts 序列化失败")
	}
	configHash := core.ConfigHashOf(&base.PolicyVersion{
		Scene:         in.Scene,
		Combination:   in.Combination,
		Rules:         in.Rules,
		RequiredFacts: requiredFacts,
	})

	var (
		head       *repo.EligibilityHead
		versionRow *repo.EligibilityVersionRow
	)
	err = l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		h, err := repo.LockEligibilityHead(tx, int64(in.PolicyId))
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.NewCodeError(errorx.CodePolicyNotFound, "政策不存在")
		}
		if err != nil {
			return err
		}
		if h.LatestVersion != int64(in.ExpectedLatestVersion) {
			return errorx.NewCodeError(errorx.CodePolicyConflict, "最新版本号已变更（expectedLatestVersion 不符）")
		}
		v := h.LatestVersion + 1
		row := &repo.EligibilityVersionRow{
			PolicyID:      h.ID,
			Version:       v,
			Scene:         int32(in.Scene),
			Status:        repo.PolicyStatusDraft,
			Combination:   int32(in.Combination),
			RequiredFacts: string(factsJSON),
			ConfigHash:    configHash,
		}
		if err := repo.InsertEligibilityVersion(tx, row); err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errorx.NewCodeError(errorx.CodePolicyConflict, "同内容版本已存在")
			}
			return err
		}
		if err := repo.InsertEligibilityRules(tx, rulesOf(h.ID, v, in.Rules)); err != nil {
			return err
		}
		h.Name = in.Name
		h.LatestVersion = v
		if err := repo.UpdateEligibilityHead(tx, h); err != nil {
			return err
		}
		head, versionRow = h, row
		return nil
	})
	if err != nil {
		if errorx.IsCodeError(err) {
			return nil, err
		}
		l.Errorf("CreatePolicyRevision failed, policyId: %d, err: %v", in.PolicyId, err)
		return nil, errorx.FromMySQLError(err)
	}
	l.Infof("CreatePolicyRevision success, policyId: %d, version: %d", head.ID, versionRow.Version)
	return policyRecordOf(head, versionRow, rulesOf(head.ID, versionRow.Version, in.Rules)), nil
}

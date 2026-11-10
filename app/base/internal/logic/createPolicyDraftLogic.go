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

type CreatePolicyDraftLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreatePolicyDraftLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePolicyDraftLogic {
	return &CreatePolicyDraftLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreatePolicyDraft 新建政策草稿（D3）：头 + v1 DRAFT + 规则，单事务。
// code 已存在 → 409 POLICY_CONFLICT。
func (l *CreatePolicyDraftLogic) CreatePolicyDraft(in *base.CreatePolicyDraftRequest) (*base.EligibilityPolicyRecord, error) {
	if err := core.ValidatePolicyDraft(in.TenantId, in.Code, in.Name, in.Scene, in.Combination,
		in.Rules, in.RequiredFacts, true); err != nil {
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
	// 先算 config_hash（存储值与发布 CAS 同算法）
	configHash := core.ConfigHashOf(&base.PolicyVersion{
		Scene:         in.Scene,
		Combination:   in.Combination,
		Rules:         in.Rules,
		RequiredFacts: requiredFacts,
	})

	head := &repo.EligibilityHead{
		PolicyCode:     in.Code,
		Name:           in.Name,
		CurrentVersion: 0,
		LatestVersion:  1,
	}
	versionRow := &repo.EligibilityVersionRow{
		Version:       1,
		Scene:         int32(in.Scene),
		Status:        repo.PolicyStatusDraft,
		Combination:   int32(in.Combination),
		RequiredFacts: string(factsJSON),
		ConfigHash:    configHash,
	}
	err = l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		_, err := repo.GetEligibilityHeadByCode(tx, in.Code)
		if err == nil {
			return errorx.NewCodeError(errorx.CodePolicyConflict, "政策编码已存在")
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := repo.InsertEligibilityHead(tx, head); err != nil {
			return err
		}
		versionRow.PolicyID = head.ID
		if err := repo.InsertEligibilityVersion(tx, versionRow); err != nil {
			return err
		}
		return repo.InsertEligibilityRules(tx, rulesOf(head.ID, 1, in.Rules))
	})
	if err != nil {
		if errorx.IsCodeError(err) {
			return nil, err
		}
		l.Errorf("CreatePolicyDraft failed, code: %s, err: %v", in.Code, err)
		return nil, errorx.FromMySQLError(err)
	}
	l.Infof("CreatePolicyDraft success, code: %s, policyId: %d", in.Code, head.ID)
	return policyRecordOf(head, versionRow, rulesOf(head.ID, 1, in.Rules)), nil
}

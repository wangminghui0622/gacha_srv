package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gacha_srv/app/base/base"
	"gacha_srv/app/base/internal/core"
	"gacha_srv/app/base/internal/repo"
	"gacha_srv/app/base/internal/svc"
	basemodelnc "gacha_srv/pkg/model_nocache/gacha_base"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// missingRequiredFactReason 必需事实缺失的拒绝原因（server_v2 常量）
const missingRequiredFactReason = "MISSING_REQUIRED_FACT"

type EvaluatePolicyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEvaluatePolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EvaluatePolicyLogic {
	return &EvaluatePolicyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// EvaluatePolicy 资格评估（同步预检，非 SAGA 分支）。
//
// 错误语义：
//   - 业务失败（策略拒绝）→ CodeError(4001)，网关当场返回给前端
//   - 系统错误 → codes.Internal，折叠为 500
//
// 关键设计：
//   - 评估纯读（不占事务连接）；决策日志幂等落库（uk decision_no="d_"+requestId），
//     拒绝时先落库再返回——决策日志只追加，不撤销。
//   - 生效策略解析（M14d CEL 化）：head.current_version 指向 PUBLISHED 且 scene 匹配的
//     版本中 published_at 最新者（同刻取 head id 大者）；无生效策略=默认放行。
//   - 规则语义（CEL）：required_facts 缺失→拒绝 MISSING_REQUIRED_FACT；ALL=全部通过、
//     ANY=至少一条通过；空规则列表=放行（种子策略）；失败规则的 failure_code 进 reason。
func (l *EvaluatePolicyLogic) EvaluatePolicy(in *base.EvaluatePolicyReq) (*base.EvaluatePolicyResp, error) {
	if in.RequestId == "" || in.Scene == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "requestId与scene不能为空")
	}

	// ① 纯读评估
	policyNo, denied, matchedRuleId, reason, err := l.evaluate(in)
	if err != nil {
		return nil, err
	}

	// ② 落决策日志（uk decision_no 幂等；撞键读回已有结果）
	decisionNo := "d_" + in.RequestId
	result := int64(1)
	if denied {
		result = 0
	}
	entry := &basemodelnc.EligibilityDecisionLogs{
		DecisionNo:    decisionNo,
		Scene:         in.Scene,
		PolicyNo:      policyNo,
		UserId:        in.UserId,
		BoxId:         in.BoxId,
		Result:        result,
		MatchedRuleId: matchedRuleId,
		Reason:        reason,
		CreatedAt:     time.Now(),
	}
	if err := l.svcCtx.DB.WithContext(l.ctx).Create(entry).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			// 同 decision_no 已落库：读回已有结果，幂等返回
			return l.readBackDecision(decisionNo, in)
		}
		l.Errorf("EvaluatePolicy failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	// ③ 拒绝：日志已落库，返回业务码
	if denied {
		msg := reason
		if msg == "" {
			msg = "资格校验未通过"
		}
		l.Errorf("EvaluatePolicy denied, requestId: %s, scene: %s, policy: %s, ruleId: %d, reason: %s", in.RequestId, in.Scene, policyNo, matchedRuleId, reason)
		return nil, errorx.NewCodeError(errorx.CodePolicyDenied, msg)
	}
	l.Infof("EvaluatePolicy allowed, requestId: %s, scene: %s, policy: %s", in.RequestId, in.Scene, policyNo)
	return &base.EvaluatePolicyResp{Allowed: true, PolicyNo: policyNo}, nil
}

// resolvedPolicy 场景解析结果：生效版本行 + 头
type resolvedPolicy struct {
	head          *repo.EligibilityHead
	version       *repo.EligibilityVersionRow
	rules         []repo.EligibilityRuleRow
	requiredFacts []string
}

// evaluate 纯读评估：返回 policyNo 与拒绝信息（拒绝时 matchedRuleId/reason 非空）
func (l *EvaluatePolicyLogic) evaluate(in *base.EvaluatePolicyReq) (policyNo string, denied bool, matchedRuleId int64, reason string, err error) {
	sceneInt, ok := core.SceneIntOf(in.Scene)
	if !ok {
		// 未知场景无政策定义 = 默认放行
		return "", false, 0, "", nil
	}
	p, err := l.resolvePolicy(sceneInt)
	if err != nil {
		return "", false, 0, "", status.Error(codes.Internal, err.Error())
	}
	if p == nil {
		// 无生效策略 = 默认放行（决策日志 policy_no 置空仍落库）
		return "", false, 0, "", nil
	}

	// 空规则列表 = 放行（种子策略）；MISSING_REQUIRED_FACT 优先于规则评估
	if len(p.rules) == 0 {
		return p.head.PolicyCode, false, 0, "", nil
	}
	facts := map[string]any{"user_id": in.UserId, "box_id": in.BoxId}
	for _, key := range p.requiredFacts {
		if _, exists := facts[key]; !exists {
			return p.head.PolicyCode, true, 0, missingRequiredFactReason, nil
		}
	}

	passed := 0
	var failedRule *repo.EligibilityRuleRow
	for i := range p.rules {
		r := &p.rules[i]
		holds, eerr := core.EvaluateRuleExpression(r.Expression, facts)
		if eerr != nil {
			// 表达式写路径已校验；运行时异常（除零等）保守拒绝（fail-closed）
			l.Errorf("evaluate rule failed, policy: %s, ruleId: %s, err: %v",
				p.head.PolicyCode, r.RuleID, eerr)
			return p.head.PolicyCode, true, r.ID, r.FailureCode, nil
		}
		if holds {
			passed++
		} else if failedRule == nil {
			failedRule = r
		}
	}

	eligible := passed == len(p.rules) // ALL
	if p.version.Combination == 2 {    // ANY
		eligible = passed > 0
	}
	if !eligible && failedRule != nil {
		return p.head.PolicyCode, true, failedRule.ID, failedRule.FailureCode, nil
	}
	if !eligible {
		// ANY 全失败但 failedRule 不应为空；防御
		return p.head.PolicyCode, true, 0, "POLICY_RULE_FAILED", nil
	}
	return p.head.PolicyCode, false, 0, "", nil
}

// resolvePolicy 场景解析：head.current_version 指向 PUBLISHED 且 scene 匹配的版本中
// published_at 最新者（同刻取 head id 大者）。无 → nil。
func (l *EvaluatePolicyLogic) resolvePolicy(sceneInt int32) (*resolvedPolicy, error) {
	var row struct {
		HeadID        int64
		PolicyCode    string
		Version       int64
		Combination   int32
		RequiredFacts string
	}
	err := l.svcCtx.DB.WithContext(l.ctx).Table("eligibility_policies AS h").Select("h.id AS head_id, h.policy_code, v.version, v.combination, v.required_facts").
		Joins("JOIN eligibility_policy_versions AS v ON v.policy_id = h.id AND v.version = h.current_version").Where("h.current_version > 0 AND v.status = ? AND v.scene = ?", repo.PolicyStatusPublished, sceneInt).
		Order("v.published_at DESC, h.id DESC").Limit(1).Scan(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.HeadID == 0 {
		return nil, nil
	}
	rules, err := repo.ListEligibilityRules(l.svcCtx.DB, row.HeadID, row.Version)
	if err != nil {
		return nil, err
	}
	var requiredFacts []string
	if row.RequiredFacts != "" {
		if err := json.Unmarshal([]byte(row.RequiredFacts), &requiredFacts); err != nil {
			return nil, fmt.Errorf("policy required_facts invalid: %w", err)
		}
	}
	return &resolvedPolicy{
		head:          &repo.EligibilityHead{ID: row.HeadID, PolicyCode: row.PolicyCode},
		version:       &repo.EligibilityVersionRow{Version: row.Version, Combination: row.Combination},
		rules:         rules,
		requiredFacts: requiredFacts,
	}, nil
}

// readBackDecision 幂等/屏障跳过路径：读已有决策日志按其结果如实返回
func (l *EvaluatePolicyLogic) readBackDecision(decisionNo string, in *base.EvaluatePolicyReq) (*base.EvaluatePolicyResp, error) {
	entry, err := l.svcCtx.DecisionModel.FindOneByDecisionNo(l.ctx, decisionNo)
	if err != nil {
		if errors.Is(err, basemodelnc.ErrNotFound) {
			l.Errorf("readBackDecision: log not found, decisionNo: %s", decisionNo)
			return nil, status.Error(codes.Internal, "决策日志不存在")
		}
		l.Errorf("readBackDecision failed, decisionNo: %s, err: %v", decisionNo, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if entry.Result == 0 {
		msg := entry.Reason
		if msg == "" {
			msg = "资格校验未通过"
		}
		l.Infof("readBackDecision: denied, decisionNo: %s, policy: %s", decisionNo, entry.PolicyNo)
		return nil, errorx.NewCodeError(errorx.CodePolicyDenied, msg)
	}
	l.Infof("readBackDecision: allowed, decisionNo: %s, policy: %s", decisionNo, entry.PolicyNo)
	return &base.EvaluatePolicyResp{Allowed: true, PolicyNo: entry.PolicyNo}, nil
}

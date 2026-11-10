package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gacha_srv/app/base/base"
	"gacha_srv/app/base/internal/core"
	"gacha_srv/app/base/internal/repo"
	"gacha_srv/app/base/internal/svc"
	"gacha_srv/app/dtm/client/dtmgrpc"
	basemodelnc "gacha_srv/pkg/model_nocache/gacha_base"
	"gacha_srv/pkg/utils/errorx"
	"gacha_srv/pkg/utils/txutil"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// errDecisionDup 事务内哨兵：同 decision_no 决策日志已存在（历史分支已评估）
var errDecisionDup = errors.New("decision log duplicated")

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

// EvaluatePolicy SAGA 分支 01 正向：资格评估。
//
// 错误语义（SAGA 分支铁律）：
//   - 业务失败（策略拒绝）→ codes.Aborted(5001)，DTM 触发补偿
//   - 系统错误 → codes.Internal，DTM 退避重试
//
// 关键设计：
//   - 评估在屏障事务外（纯读，不占事务连接）；决策日志在屏障事务内落库
//     （uk decision_no="d_"+requestId 幂等），拒绝时先落库再返回 Aborted——
//     决策日志只追加，不随补偿撤销。
//   - 生效策略解析（M14d CEL 化）：head.current_version 指向 PUBLISHED 且 scene 匹配的
//     版本中 published_at 最新者（同刻取 head id 大者）；无生效策略=默认放行。
//   - 规则语义（CEL）：required_facts 缺失→拒绝 MISSING_REQUIRED_FACT；ALL=全部通过、
//     ANY=至少一条通过；空规则列表=放行（种子策略）；失败规则的 failure_code 进 reason。
func (l *EvaluatePolicyLogic) EvaluatePolicy(in *base.EvaluatePolicyReq) (*base.EvaluatePolicyResp, error) {
	if in.RequestId == "" || in.Scene == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=requestId与scene不能为空")
	}

	// ① 屏障事务外评估（纯读）
	policyNo, denied, matchedRuleId, reason, err := l.evaluate(in)
	if err != nil {
		return nil, err
	}

	// ② 屏障事务内落决策日志（uk 幂等；撞键/屏障跳过=读回已有结果）
	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("EvaluatePolicy get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("EvaluatePolicy get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	decisionNo := "d_" + in.RequestId
	result := int64(1)
	if denied {
		result = 0
	}
	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)
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
		if err := db.Create(entry).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errDecisionDup
			}
			return err
		}
		return nil
	})
	if errors.Is(err, errDecisionDup) {
		// 同 decision_no 已落库：读回已有结果，幂等返回
		return l.readBackDecision(decisionNo, in)
	}
	if err != nil {
		l.Errorf("EvaluatePolicy failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（分支已成功）：读库如实返回已有决策
		return l.readBackDecision(decisionNo, in)
	}

	// ③ 拒绝：日志已落库，返回 Aborted 触发补偿
	if denied {
		l.Errorf("EvaluatePolicy denied, requestId: %s, scene: %s, policy: %s, ruleId: %d, reason: %s",
			in.RequestId, in.Scene, policyNo, matchedRuleId, reason)
		return nil, status.Error(codes.Aborted,
			fmt.Sprintf("code=%d msg=%s", errorx.CodePolicyDenied, reason))
	}
	l.Infof("EvaluatePolicy allowed, requestId: %s, scene: %s, policy: %s", in.RequestId, in.Scene, policyNo)
	return &base.EvaluatePolicyResp{Allowed: true, PolicyNo: policyNo}, nil
}

// resolvedPolicy 场景解析结果：生效版本行 + 头
type resolvedPolicy struct {
	head      *repo.EligibilityHead
	version   *repo.EligibilityVersionRow
	rules     []repo.EligibilityRuleRow
	requiredFacts []string
}

// evaluate 屏障事务外纯读评估：返回 policyNo 与拒绝信息（拒绝时 matchedRuleId/reason 非空）
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
		HeadID          int64
		PolicyCode      string
		Version         int64
		Combination     int32
		RequiredFacts   string
	}
	err := l.svcCtx.DB.WithContext(l.ctx).Table("eligibility_policies AS h").
		Select("h.id AS head_id, h.policy_code, v.version, v.combination, v.required_facts").
		Joins("JOIN eligibility_policy_versions AS v ON v.policy_id = h.id AND v.version = h.current_version").
		Where("h.current_version > 0 AND v.status = ? AND v.scene = ?",
			repo.PolicyStatusPublished, sceneInt).
		Order("v.published_at DESC, h.id DESC").
		Limit(1).Scan(&row).Error
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
		head:      &repo.EligibilityHead{ID: row.HeadID, PolicyCode: row.PolicyCode},
		version:   &repo.EligibilityVersionRow{Version: row.Version, Combination: row.Combination},
		rules:     rules,
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
		return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", errorx.CodePolicyDenied, msg))
	}
	l.Infof("readBackDecision: allowed, decisionNo: %s, policy: %s", decisionNo, entry.PolicyNo)
	return &base.EvaluatePolicyResp{Allowed: true, PolicyNo: entry.PolicyNo}, nil
}

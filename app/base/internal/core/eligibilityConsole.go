// Package core M14d 控制台纯函数（base 域）：条件政策校验 + config_hash 计算 + CEL 表达式
// 校验/评估（game/rpc/internal/core 先例，全部无 IO 可单测）。
package core

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gacha_srv/app/base/base"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"google.golang.org/protobuf/proto"
)

// policyCodeRe 政策编码（D3 照抄 campaign key 规则）
var policyCodeRe = regexp.MustCompile(`^[a-z][a-z0-9_.-]{1,63}$`)

// celEnv 评估环境：facts map(string,dyn)（user_id/box_id），表达式必须输出 bool
var celEnv = mustNewCelEnv()

func mustNewCelEnv() *cel.Env {
	env, err := cel.NewEnv(cel.Variable("facts", cel.MapType(cel.StringType, cel.DynType)))
	if err != nil {
		panic(fmt.Sprintf("build cel env failed: %v", err))
	}
	return env
}

// SceneIntOf 场景字符串 → 数值（policy 版本表 scene TINYINT 1-4）；未知场景=false
func SceneIntOf(scene string) (int32, bool) {
	switch strings.ToUpper(scene) {
	case "DISCOVER":
		return 1, true
	case "VIEW":
		return 2, true
	case "DRAW":
		return 3, true
	case "BOOK":
		return 4, true
	default:
		return 0, false
	}
}

// ValidatePolicyRules 校验规则列表：ruleId/expression/failureCode 非空且 expression 为合法
// bool 输出 CEL；规则 ID 版本内唯一。
func ValidatePolicyRules(rules []*base.EligibilityRule) error {
	seen := make(map[string]bool, len(rules))
	for i, r := range rules {
		if r == nil || r.RuleId == "" || len(r.RuleId) > 64 {
			return fmt.Errorf("第 %d 条规则 ruleId 为空或超长", i+1)
		}
		if seen[r.RuleId] {
			return fmt.Errorf("规则 ID 重复: %s", r.RuleId)
		}
		seen[r.RuleId] = true
		if strings.TrimSpace(r.Expression) == "" {
			return fmt.Errorf("规则 %s 表达式为空", r.RuleId)
		}
		if err := ValidateRuleExpression(r.Expression); err != nil {
			return fmt.Errorf("规则 %s 表达式非法: %v", r.RuleId, err)
		}
		if r.FailureCode == "" || len(r.FailureCode) > 64 {
			return fmt.Errorf("规则 %s failureCode 为空或超长", r.RuleId)
		}
	}
	return nil
}

// ValidateRequiredFacts 必需事实键非空且去重
func ValidateRequiredFacts(facts []string) ([]string, error) {
	out := make([]string, 0, len(facts))
	seen := make(map[string]bool, len(facts))
	for _, f := range facts {
		if f == "" {
			return nil, fmt.Errorf("requiredFacts 含空键")
		}
		if seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out, nil
}

// ValidateRuleExpression 表达式必须可编译且输出 bool（写路径校验，读路径兜底拒绝）
func ValidateRuleExpression(expr string) error {
	ast, iss := celEnv.Compile(expr)
	if iss.Err() != nil {
		return iss.Err()
	}
	if ast.OutputType() != types.BoolType {
		return fmt.Errorf("表达式输出非 bool: %s", ast.OutputType())
	}
	return nil
}

// EvaluateRuleExpression 评估表达式：facts 缺键/求值异常/非 bool 输出均返回 error（调用方保守拒绝）
func EvaluateRuleExpression(expr string, facts map[string]any) (bool, error) {
	ast, iss := celEnv.Compile(expr)
	if iss.Err() != nil {
		return false, iss.Err()
	}
	prg, err := celEnv.Program(ast)
	if err != nil {
		return false, err
	}
	out, _, err := prg.Eval(map[string]any{"facts": facts})
	if err != nil {
		return false, err
	}
	if out.Type() != types.BoolType {
		return false, fmt.Errorf("表达式输出非 bool: %s", out.Type())
	}
	return out.Value().(bool), nil
}

// ConfigHashOf 内容哈希（照抄 prepareVersion）：克隆后 PolicyId/Version/Status/ConfigHash
// 置零、required_facts 排序，Deterministic proto marshal → SHA256。
// 发布 CAS 依据；同内容版本命中 uk(policy_id,config_hash) 即冲突。
func ConfigHashOf(v *base.PolicyVersion) []byte {
	clone := proto.Clone(v).(*base.PolicyVersion)
	clone.PolicyId = 0
	clone.Version = 0
	clone.Status = base.PolicyStatus_POLICY_STATUS_UNSPECIFIED
	clone.ConfigHash = nil
	sorted := append([]string(nil), clone.RequiredFacts...)
	sort.Strings(sorted)
	clone.RequiredFacts = sorted
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(clone)
	if err != nil {
		// proto 自产消息 marshal 不会失败；防御性返回空哈希（调用方按空处理报校验错）
		return nil
	}
	sum := sha256.Sum256(raw)
	return sum[:]
}

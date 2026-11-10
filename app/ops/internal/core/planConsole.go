// 任务计划核心（M14e）：计划校验 + config_hash 纯函数 + structured reward 判定。
// 语义照抄 server_v2 task internal/biz/plan.go + reward.go + task.go/pass.go/member.go：
//   - ValidateTaskPlan = SnapshotFromTaskPlan 校验（tasks/passes/member_tiers 至少一个；
//     任务 id 唯一、奖励二选一 legacy/structured；pass/member_tier 同款逐条校验）
//   - TaskPlanConfigHash = SHA256(Deterministic marshal(清 version/status/hash/时间戳 的
//     canonical clone))——与 server_v2 ValidateTaskPlan 逐字一致，SQL 种子与运行时同源
//   - HasStructuredRewards：任一 task/pass/tier 带 structured reward → 发布拒绝（
//     server_v2 半成品语义：原子发奖持久化未就绪）
package core

import (
	"crypto/sha256"
	"fmt"

	"gacha_srv/app/ops/ops"

	"google.golang.org/protobuf/proto"
)

// ValidateTaskPlan 校验计划（语义 = server_v2 SnapshotFromTaskPlan + NewBoard/AddPass/SetMemberPlan）。
// 不修改入参（config_hash 对提交顺序做确定性序列化，与校验排序无关）。
func ValidateTaskPlan(plan *ops.TaskPlan) error {
	if plan == nil || len(plan.GetTasks())+len(plan.GetPasses())+len(plan.GetMemberTiers()) == 0 {
		return fmt.Errorf("计划须至少包含一个任务/通行证赛季/会员层级")
	}
	seen := make(map[uint64]struct{}, len(plan.GetTasks()))
	for _, task := range plan.GetTasks() {
		if task.GetTaskId() == 0 || task.GetVersion() == 0 || task.GetTarget() <= 0 ||
			task.GetMetric() == ops.TaskMetric_TASK_METRIC_UNSPECIFIED ||
			task.GetPeriod() == ops.TaskPeriod_TASK_PERIOD_UNSPECIFIED ||
			!ValidTaskReward(task.GetReward(), task.GetRewardTemplateId(), task.GetRewardCount()) {
			return fmt.Errorf("任务 %d 非法（id/version/target 须 >0，period/metric 非空，奖励二选一）", task.GetTaskId())
		}
		if _, ok := seen[task.GetTaskId()]; ok {
			return fmt.Errorf("任务 id 重复: %d", task.GetTaskId())
		}
		seen[task.GetTaskId()] = struct{}{}
	}
	seenPass := make(map[uint64]struct{}, len(plan.GetPasses()))
	for _, pass := range plan.GetPasses() {
		if pass.GetPassId() == 0 || pass.GetVersion() == 0 || pass.GetXpPerLevel() <= 0 ||
			pass.GetXpPerSpendMinor() <= 0 || pass.GetMaxLevel() == 0 || pass.GetMaxLevel() > 200 ||
			len(pass.GetRewards()) == 0 {
			return fmt.Errorf("通行证赛季 %d 非法（id/version/xp 须 >0，1≤max_level≤200，奖励非空）", pass.GetPassId())
		}
		if _, ok := seenPass[pass.GetPassId()]; ok {
			return fmt.Errorf("通行证赛季 id 重复: %d", pass.GetPassId())
		}
		seenPass[pass.GetPassId()] = struct{}{}
		premium := false
		seenReward := make(map[string]struct{}, len(pass.GetRewards()))
		for _, reward := range pass.GetRewards() {
			if reward.GetLevel() == 0 || reward.GetLevel() > pass.GetMaxLevel() ||
				!ValidTaskReward(reward.GetReward(), reward.GetRewardTemplateId(), reward.GetRewardCount()) ||
				(reward.GetTrack() != ops.PassTrack_PASS_TRACK_FREE && reward.GetTrack() != ops.PassTrack_PASS_TRACK_PREMIUM) {
				return fmt.Errorf("通行证 %d 奖励非法（1≤level≤max，track 须 FREE/PREMIUM，奖励二选一）", pass.GetPassId())
			}
			if reward.GetTrack() == ops.PassTrack_PASS_TRACK_PREMIUM {
				premium = true
			}
			key := fmt.Sprintf("%d:%d", reward.GetLevel(), reward.GetTrack())
			if _, ok := seenReward[key]; ok {
				return fmt.Errorf("通行证 %d 奖励重复（level:track）: %s", pass.GetPassId(), key)
			}
			seenReward[key] = struct{}{}
		}
		switch pass.GetPremiumLine() {
		case ops.PassPremiumLine_PASS_PREMIUM_LINE_UNSPECIFIED:
			if premium || pass.GetPremiumMemberLevel() != 0 {
				return fmt.Errorf("通行证 %d premium_line 非法（UNSPECIFIED 不允许 premium 奖励/会员等级）", pass.GetPassId())
			}
		case ops.PassPremiumLine_PASS_PREMIUM_LINE_PAYMENT:
			if !premium || pass.GetPremiumMemberLevel() != 0 {
				return fmt.Errorf("通行证 %d premium_line 非法（PAYMENT 须含 premium 奖励且无会员等级）", pass.GetPassId())
			}
		case ops.PassPremiumLine_PASS_PREMIUM_LINE_MEMBER:
			if !premium || pass.GetPremiumMemberLevel() == 0 {
				return fmt.Errorf("通行证 %d premium_line 非法（MEMBER 须含 premium 奖励且指定会员等级）", pass.GetPassId())
			}
		default:
			return fmt.Errorf("通行证 %d premium_line 未知: %s", pass.GetPassId(), pass.GetPremiumLine())
		}
	}
	if len(plan.GetMemberTiers()) > 0 {
		if len(plan.GetMemberTiers()) > 50 {
			return fmt.Errorf("会员层级最多 50 个")
		}
		var previous int64
		seenTier := make(map[uint32]struct{}, len(plan.GetMemberTiers()))
		for _, tier := range plan.GetMemberTiers() {
			if tier.GetLevel() == 0 || tier.GetSpendMinor() <= previous ||
				!ValidTaskReward(tier.GetReward(), tier.GetRewardTemplateId(), tier.GetRewardCount()) {
				return fmt.Errorf("会员层级非法（level>0，spend 严格递增，奖励二选一）")
			}
			if _, ok := seenTier[tier.GetLevel()]; ok {
				return fmt.Errorf("会员层级重复: %d", tier.GetLevel())
			}
			seenTier[tier.GetLevel()] = struct{}{}
			previous = tier.GetSpendMinor()
		}
	}
	return nil
}

// ValidTaskReward 奖励二选一校验（server_v2 validReward 同款）：
// legacy = reward_template_id 非空 + count 1..1000 且无 structured reward；
// structured = count 1..1000 + coupon/inventory 引用完整且 legacy 字段为空。
func ValidTaskReward(reward *ops.TaskRewardDefinition, legacyID string, legacyCount uint32) bool {
	if reward == nil {
		return legacyID != "" && legacyCount > 0 && legacyCount <= 1000
	}
	if legacyID != "" || legacyCount != 0 || reward.GetCount() == 0 || reward.GetCount() > 1000 {
		return false
	}
	switch target := reward.GetTarget().(type) {
	case *ops.TaskRewardDefinition_Coupon:
		return target.Coupon.GetTemplateId() > 0 && target.Coupon.GetTemplateVersion() > 0
	case *ops.TaskRewardDefinition_Inventory:
		return target.Inventory.GetProductId() > 0 && target.Inventory.GetSkuId() > 0 && target.Inventory.GetProductVersion() > 0
	default:
		return false
	}
}

// TaskPlanConfigHash 校验并计算 canonical config_hash（SHA256 确定性序列化）。
// 与 SQL 种子（20_gacha_ops_m14e.sql 的 UNHEX 哈希）逐字节同源。
func TaskPlanConfigHash(plan *ops.TaskPlan) ([]byte, error) {
	hash, _, err := TaskPlanPayload(plan)
	return hash, err
}

// TaskPlanPayload 校验 + 产出 (config_hash, canonical blob)（server_v2 taskPlanPayload 同款）。
// blob = 清 version/status/hash/时间戳 的确定性序列化——DB 行只存 canonical 字节，
// 读取时从行列重挂 version/status/时间戳（decodeTaskPlan 语义）。
func TaskPlanPayload(plan *ops.TaskPlan) ([]byte, []byte, error) {
	if err := ValidateTaskPlan(plan); err != nil {
		return nil, nil, err
	}
	canonical := proto.Clone(plan).(*ops.TaskPlan)
	canonical.Version = 0
	canonical.Status = ops.TaskPlanStatus_TASK_PLAN_STATUS_UNSPECIFIED
	canonical.ConfigHash = nil
	canonical.CreatedAtUnixMillis = 0
	canonical.PublishedAtUnixMillis = 0
	blob, err := proto.MarshalOptions{Deterministic: true}.Marshal(canonical)
	if err != nil {
		return nil, nil, err
	}
	hash := sha256.Sum256(blob)
	return hash[:], blob, nil
}

// HasStructuredRewards 任一任务/通行证奖励/会员层级带 structured reward（发布拒绝依据）。
func HasStructuredRewards(plan *ops.TaskPlan) bool {
	for _, task := range plan.GetTasks() {
		if task.GetReward() != nil {
			return true
		}
	}
	for _, pass := range plan.GetPasses() {
		for _, reward := range pass.GetRewards() {
			if reward.GetReward() != nil {
				return true
			}
		}
	}
	for _, tier := range plan.GetMemberTiers() {
		if tier.GetReward() != nil {
			return true
		}
	}
	return false
}

// BootstrapTaskPlan M14e 启动种子计划 v1（1 个 legacy 任务「每日登录」）。
// 与 20_gacha_ops_m14e.sql 种子 INSERT 同构——EnsureBootstrapPlan 重算哈希须一致。
func BootstrapTaskPlan() *ops.TaskPlan {
	return &ops.TaskPlan{
		Tasks: []*ops.TaskDefinition{{
			TaskId: 1, Version: 1, Name: "每日登录",
			Period: ops.TaskPeriod_TASK_PERIOD_DAILY, Metric: ops.TaskMetric_TASK_METRIC_LOGIN,
			Target: 1, CycleLimit: 0, RewardTemplateId: "T-BOOT", RewardCount: 1,
		}},
	}
}

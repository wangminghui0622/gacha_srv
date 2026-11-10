package task

import (
	"gacha_srv/app/ops/ops"
	"google.golang.org/protobuf/proto"
)

func cloneReward(reward *ops.TaskRewardDefinition) *ops.TaskRewardDefinition {
	if reward == nil {
		return nil
	}
	return proto.Clone(reward).(*ops.TaskRewardDefinition)
}

// A definition selects one representation. Legacy strings are never parsed into IDs.
func validReward(reward *ops.TaskRewardDefinition, legacyID string, legacyCount uint32) bool {
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

// HasStructuredRewards 计划是否含结构化奖励（老项目同款；D3 不做奖励发放管线，
// 仅用于快照校验与 ops 侧一致性）
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

// RewardCount 领取数量（老项目领取 SQL 同款）：结构化取定义 count，legacy 取定义字段
func RewardCount(reward *ops.TaskRewardDefinition, legacyCount uint32) uint32 {
	if reward != nil {
		return reward.GetCount()
	}
	return legacyCount
}

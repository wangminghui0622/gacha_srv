package task

import (
	"crypto/sha256"

	"gacha_srv/app/ops/ops"
	taskpb "gacha_srv/app/task/task"
	"google.golang.org/protobuf/proto"
)

// Snapshot 已发布计划快照（老项目 PlanSnapshot；game-rpc 只读用途，
// 不含 store，LoadPublishedPlan 走 repo 层跨库直读 gacha_ops）
type Snapshot struct {
	Version     uint64
	Tasks       []Definition
	Passes      map[uint64]PassSeason
	MemberTiers []MemberTier
}

// SnapshotFromOpsPlan 校验并快照化 ops.TaskPlan（老项目 SnapshotFromTaskPlan）。
// 计划 payload 与老 taskv2.TaskPlan 线级一致，复用 ops 生成类型解码（D2）。
func SnapshotFromOpsPlan(plan *ops.TaskPlan) (Snapshot, error) {
	if plan == nil || len(plan.GetTasks())+len(plan.GetPasses())+len(plan.GetMemberTiers()) == 0 {
		return Snapshot{}, ErrInvalidTask
	}
	tasks := make([]Definition, 0, len(plan.GetTasks()))
	for _, task := range plan.GetTasks() {
		tasks = append(tasks, Definition{ID: task.GetTaskId(), Version: task.GetVersion(), Name: task.GetName(), Period: taskpb.TaskPeriod(task.GetPeriod()), Metric: taskpb.TaskMetric(task.GetMetric()), Target: task.GetTarget(), CycleLimit: task.GetCycleLimit(), RewardTemplateID: task.GetRewardTemplateId(), RewardCount: task.GetRewardCount(), Reward: cloneReward(task.GetReward())})
	}
	board, err := NewBoard(tasks, nil)
	if err != nil {
		return Snapshot{}, err
	}
	for _, pass := range plan.GetPasses() {
		rewards := make([]PassReward, 0, len(pass.GetRewards()))
		for _, reward := range pass.GetRewards() {
			rewards = append(rewards, PassReward{Level: reward.GetLevel(), Track: taskpb.PassTrack(reward.GetTrack()), RewardTemplateID: reward.GetRewardTemplateId(), RewardCount: reward.GetRewardCount(), Reward: cloneReward(reward.GetReward())})
		}
		if err = board.AddPass(PassSeason{ID: pass.GetPassId(), Version: pass.GetVersion(), Name: pass.GetName(), XPPerLevel: pass.GetXpPerLevel(), MaxLevel: pass.GetMaxLevel(), Rewards: rewards, XPPerSpendMinor: pass.GetXpPerSpendMinor(), PremiumLine: taskpb.PassPremiumLine(pass.GetPremiumLine()), PremiumMemberLevel: pass.GetPremiumMemberLevel()}); err != nil {
			return Snapshot{}, err
		}
	}
	if len(plan.GetMemberTiers()) > 0 {
		tiers := make([]MemberTier, 0, len(plan.GetMemberTiers()))
		for _, tier := range plan.GetMemberTiers() {
			tiers = append(tiers, MemberTier{Level: tier.GetLevel(), SpendMinor: tier.GetSpendMinor(), RewardTemplateID: tier.GetRewardTemplateId(), RewardCount: tier.GetRewardCount(), Reward: cloneReward(tier.GetReward())})
		}
		if err = board.SetMemberPlan(tiers); err != nil {
			return Snapshot{}, err
		}
	}
	return Snapshot{Version: plan.GetVersion(), Tasks: append([]Definition(nil), board.tasks...), Passes: clonePasses(board.passes), MemberTiers: append([]MemberTier(nil), board.memberTiers...)}, nil
}

func clonePasses(source map[uint64]PassSeason) map[uint64]PassSeason {
	result := make(map[uint64]PassSeason, len(source))
	for id, season := range source {
		season.Rewards = append([]PassReward(nil), season.Rewards...)
		result[id] = season
	}
	return result
}

// PlanHash 计划配置哈希（老项目 ValidateTaskPlan 的 canonical SHA256）：
// 校验计划 → 清 version/status/config_hash/时间戳 → Deterministic marshal → SHA256。
// 与 ops-rpc M14e TaskPlanPayload 逐字同源（game-rpc 不可导入 ops internal 包，自实现）。
func PlanHash(plan *ops.TaskPlan) ([]byte, error) {
	if _, err := SnapshotFromOpsPlan(plan); err != nil {
		return nil, err
	}
	canonical := proto.Clone(plan).(*ops.TaskPlan)
	canonical.Version = 0
	canonical.Status = ops.TaskPlanStatus_TASK_PLAN_STATUS_UNSPECIFIED
	canonical.ConfigHash = nil
	canonical.CreatedAtUnixMillis = 0
	canonical.PublishedAtUnixMillis = 0
	blob, err := proto.MarshalOptions{Deterministic: true}.Marshal(canonical)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(blob)
	return hash[:], nil
}

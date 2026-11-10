package task

import (
	taskClient "gacha_srv/app/task/taskClient"
	"gacha_srv/app/gateway/internal/types"
)

// tasksToResp game ListTasksResp → 网关视图（任务/通行证/会员）
func tasksToResp(r *taskClient.ListTasksResp) *types.ListTasksResp {
	resp := &types.ListTasksResp{NextPageToken: r.NextPageToken}
	for _, view := range r.Tasks {
		def := view.Definition
		resp.Tasks = append(resp.Tasks, types.TaskResp{
			TaskId:           int64(def.TaskId), Version: int64(def.Version), Name: def.Name,
			Period: int64(def.Period), Metric: int64(def.Metric), Target: def.Target,
			CycleLimit:       int64(def.CycleLimit),
			RewardTemplateId: def.RewardTemplateId, RewardCount: int64(def.RewardCount),
			Progress: view.Progress, Claimed: int64(view.Claimed), State: int64(view.State),
		})
	}
	for _, pass := range r.Passes {
		view := types.PassResp{PassId: pass.PassId, Version: pass.Version, Name: pass.Name,
			XpPerLevel: pass.XpPerLevel, MaxLevel: int64(pass.MaxLevel), XpPerSpendMinor: pass.XpPerSpendMinor,
			PremiumLine: int64(pass.PremiumLine), PremiumMemberLevel: int64(pass.PremiumMemberLevel),
			Xp: pass.Xp, Level: int64(pass.Level), Premium: pass.Premium}
		for _, reward := range pass.Rewards {
			view.Rewards = append(view.Rewards, types.PassRewardResp{Level: int64(reward.Level), Track: int64(reward.Track), RewardTemplateId: reward.RewardTemplateId, RewardCount: int64(reward.RewardCount)})
		}
		resp.Passes = append(resp.Passes, view)
	}
	if member := r.Member; member != nil {
		view := &types.MemberResp{SpendMinor: member.SpendMinor, Level: int64(member.Level)}
		for _, tier := range member.Tiers {
			view.Tiers = append(view.Tiers, types.MemberTierResp{Level: int64(tier.Level), SpendMinor: tier.SpendMinor, RewardTemplateId: tier.RewardTemplateId, RewardCount: int64(tier.RewardCount)})
		}
		resp.Member = view
	}
	return resp
}

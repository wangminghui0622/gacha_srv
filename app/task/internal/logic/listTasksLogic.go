package logic

import (
	"context"
	"sort"

	"gacha_srv/app/task/internal/core/task"
	"gacha_srv/app/task/internal/repo"
	"gacha_srv/app/task/internal/svc"
	taskpb "gacha_srv/app/task/task"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListTasksLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListTasksLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTasksLogic {
	return &ListTasksLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListTasks 任务分页列表 + 通行证/会员视图（老项目 ListTasks + 本仓前端展示扩展 D2b）。
// userID==0 → 3048；pageSize≤0/>50 → 20（core 层同款）；分页 token 语义照抄老项目。
func (l *ListTasksLogic) ListTasks(in *taskpb.ListTasksReq) (*taskpb.ListTasksResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeTaskInvalid, "任务参数无效")
	}
	if in.Period < taskpb.TaskPeriod_TASK_PERIOD_UNSPECIFIED || in.Period > taskpb.TaskPeriod_TASK_PERIOD_WEEKLY {
		return nil, errorx.NewCodeError(errorx.CodeTaskInvalid, "任务参数无效")
	}
	snapshot, err := loadTaskSnapshot(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	userID := uint64(in.UserId)
	when := taskNow()
	state, err := repo.LoadUserState(l.ctx, l.svcCtx.DB, userID)
	if err != nil {
		return nil, err
	}
	views, next, err := task.ListFromState(snapshot.Tasks, userID, when, in.Period, int(in.PageSize), in.PageToken, state)
	if err != nil {
		return nil, mapApplyErr(err)
	}
	resp := &taskpb.ListTasksResp{NextPageToken: next, Tasks: make([]*taskpb.TaskView, 0, len(views))}
	for _, view := range views {
		resp.Tasks = append(resp.Tasks, &taskpb.TaskView{
			Definition: &taskpb.TaskDefinition{TaskId: view.Definition.ID, Version: view.Definition.Version, Name: view.Definition.Name,
				Period: view.Definition.Period, Metric: view.Definition.Metric, Target: view.Definition.Target,
				CycleLimit: view.Definition.CycleLimit, RewardTemplateId: view.Definition.RewardTemplateID, RewardCount: view.Definition.RewardCount},
			Progress: view.Progress, Claimed: view.Claimed, State: view.State,
		})
	}
	// 通行证视图：赛季按 ID 升序（快照 map 需确定序）；用户进度行映射
	passProgress, err := repo.LoadUserPassProgress(l.ctx, l.svcCtx.DB, userID)
	if err != nil {
		return nil, err
	}
	xpByPass := make(map[uint64]repo.TaskPassProgressRow, len(passProgress))
	for _, row := range passProgress {
		xpByPass[row.PassID] = row
	}
	seasonIDs := make([]uint64, 0, len(snapshot.Passes))
	for id := range snapshot.Passes {
		seasonIDs = append(seasonIDs, id)
	}
	sort.Slice(seasonIDs, func(i, j int) bool { return seasonIDs[i] < seasonIDs[j] })
	for _, id := range seasonIDs {
		season := snapshot.Passes[id]
		view := &taskpb.PassView{PassId: int64(season.ID), Version: int64(season.Version), Name: season.Name,
			XpPerLevel: season.XPPerLevel, MaxLevel: season.MaxLevel, XpPerSpendMinor: season.XPPerSpendMinor,
			PremiumLine: season.PremiumLine, PremiumMemberLevel: season.PremiumMemberLevel}
		for _, reward := range season.Rewards {
			view.Rewards = append(view.Rewards, &taskpb.PassRewardView{Level: reward.Level, Track: reward.Track, RewardTemplateId: reward.RewardTemplateID, RewardCount: reward.RewardCount})
		}
		if row, ok := xpByPass[id]; ok {
			view.Xp = row.XP
			view.Level = task.PassLevel(season, row.XP)
			view.Premium = row.Premium == 1
		}
		resp.Passes = append(resp.Passes, view)
	}
	// 会员视图：档位来自快照；用户行无行即 0（未达最低档）
	memberRow, err := repo.LoadUserMember(l.ctx, l.svcCtx.DB, userID)
	if err != nil {
		return nil, err
	}
	memberView := &taskpb.MemberView{SpendMinor: memberRow.SpendMinor, Level: memberRow.Level, Tiers: make([]*taskpb.MemberTierView, 0, len(snapshot.MemberTiers))}
	for _, tier := range snapshot.MemberTiers {
		memberView.Tiers = append(memberView.Tiers, &taskpb.MemberTierView{Level: tier.Level, SpendMinor: tier.SpendMinor, RewardTemplateId: tier.RewardTemplateID, RewardCount: tier.RewardCount})
	}
	resp.Member = memberView
	return resp, nil
}

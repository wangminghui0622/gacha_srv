package logic

import (
	"context"

	"gacha_srv/app/task/internal/core/task"
	"gacha_srv/app/task/internal/repo"
	"gacha_srv/app/task/internal/svc"
	taskpb "gacha_srv/app/task/task"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type ClaimTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClaimTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClaimTaskLogic {
	return &ClaimTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ClaimTask 任务奖励领取（老项目 ClaimTask）：request_id 幂等重放原样回；
// CycleLimit 已领满 → 3047；SIGN_IN 需当日已签到且 streak≥Target；其余 progress≥Target*(claimed+1)。
// 奖励仅记录不发放（D3，legacy 字符串奖励）。
func (l *ClaimTaskLogic) ClaimTask(in *taskpb.ClaimTaskReq) (*taskpb.ClaimTaskResp, error) {
	if in.UserId <= 0 || in.TaskId <= 0 || in.Version <= 0 || in.RequestId == "" || len(in.RequestId) > 64 {
		return nil, errorx.NewCodeError(errorx.CodeTaskInvalid, "任务参数无效")
	}
	snapshot, err := loadTaskSnapshot(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	def, ok := findTaskDef(snapshot, uint64(in.TaskId), uint64(in.Version))
	if !ok {
		return nil, errorx.NewCodeError(errorx.CodeNotFound, "任务不存在")
	}
	now := taskNow()
	var claim task.Claim
	err = l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		claim, err = repo.ClaimTask(l.ctx, tx, task.TaskClaimCommand{
			RequestID: in.RequestId, UserID: uint64(in.UserId), Task: def,
			Period: task.PeriodStart(def.Period, now), Today: now.Format("2006-01-02"),
		})
		return err
	})
	if err != nil {
		return nil, mapClaimTaskErr(err)
	}
	return &taskpb.ClaimTaskResp{RewardTemplateId: claim.RewardTemplateID, RewardCount: claim.RewardCount, Claimed: claim.Claimed}, nil
}

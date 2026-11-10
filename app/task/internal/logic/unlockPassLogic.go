package logic

import (
	"context"

	"gacha_srv/app/task/internal/repo"
	"gacha_srv/app/task/internal/svc"
	taskpb "gacha_srv/app/task/task"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type UnlockPassLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnlockPassLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnlockPassLogic {
	return &UnlockPassLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UnlockPass 解锁付费通行证（老项目 UnlockPass）：仅 PAYMENT 线合法（MEMBER 线随会员等级走，
// UNSPECIFIED 无 premium）；赛季不存在 → 404。付费下单流程（D9 网关）成功后调用。
func (l *UnlockPassLogic) UnlockPass(in *taskpb.UnlockPassReq) (*taskpb.UnlockPassResp, error) {
	if in.UserId <= 0 || in.PassId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeTaskInvalid, "任务参数无效")
	}
	snapshot, err := loadTaskSnapshot(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	season, ok := snapshot.Passes[uint64(in.PassId)]
	if !ok {
		return nil, errorx.NewCodeError(errorx.CodeNotFound, "通行证赛季不存在")
	}
	if season.PremiumLine != taskpb.PassPremiumLine_PASS_PREMIUM_LINE_PAYMENT {
		return nil, errorx.NewCodeError(errorx.CodeTaskInvalid, "通行证解锁参数无效")
	}
	err = l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		return repo.UnlockPassUser(l.ctx, tx, uint64(in.UserId), season.ID)
	})
	if err != nil {
		return nil, mapUnlockErr(err)
	}
	return &taskpb.UnlockPassResp{}, nil
}

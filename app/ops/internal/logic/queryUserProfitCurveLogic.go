package logic

import (
	"context"

	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type QueryUserProfitCurveLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewQueryUserProfitCurveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QueryUserProfitCurveLogic {
	return &QueryUserProfitCurveLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// QueryUserProfitCurve 用户盈亏曲线——空态 stub（照抄 server_v2：
// 投影未迁入 → 恒 {points: [], data_ready: false}），校验同款。
func (l *QueryUserProfitCurveLogic) QueryUserProfitCurve(in *ops.QueryUserProfitCurveRequest) (*ops.QueryUserProfitCurveReply, error) {
	if in.GetTenantId() == 0 || in.GetUserId() == 0 {
		return nil, errorx.NewCodeError(errorx.CodeDashboardInvalid, "tenant/user 非法")
	}
	if g := in.GetGrain(); g != ops.TimeGrain_TIME_GRAIN_HOUR && g != ops.TimeGrain_TIME_GRAIN_DAY && g != ops.TimeGrain_TIME_GRAIN_MONTH {
		return nil, errorx.NewCodeError(errorx.CodeDashboardInvalid, "粒度非法")
	}
	from, to := in.GetFrom().AsTime(), in.GetTo().AsTime()
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		return nil, errorx.NewCodeError(errorx.CodeDashboardInvalid, "时间窗非法")
	}
	return &ops.QueryUserProfitCurveReply{Points: nil, DataReady: false}, nil
}

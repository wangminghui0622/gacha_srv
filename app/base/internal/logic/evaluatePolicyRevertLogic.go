package logic

import (
	"context"
	"database/sql"

	"gacha_srv/app/base/base"
	"gacha_srv/app/base/internal/svc"
	"gacha_srv/app/dtm/client/dtmgrpc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type EvaluatePolicyRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEvaluatePolicyRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EvaluatePolicyRevertLogic {
	return &EvaluatePolicyRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// EvaluatePolicyRevert SAGA 分支 01 补偿：空补偿（日志只追加）。
//
// 设计依据：决策日志是审计事实（只追加不撤销），拒绝时日志在返回 Aborted 前已提交，
// 无任何需回滚的副作用。故补偿仅需过屏障注册，让 DTM 正常收尾。
func (l *EvaluatePolicyRevertLogic) EvaluatePolicyRevert(in *base.EvaluatePolicyReq) (*base.EvaluatePolicyResp, error) {
	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("EvaluatePolicyRevert get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("EvaluatePolicyRevert get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		// 空补偿：无业务动作，仅屏障注册 + 防御日志
		return nil
	})
	if err != nil {
		l.Errorf("EvaluatePolicyRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if businessRan {
		l.Infof("EvaluatePolicyRevert success (no-op compensate), requestId: %s", in.RequestId)
	} else {
		l.Infof("EvaluatePolicyRevert barrier-skip (repeated compensate), requestId: %s", in.RequestId)
	}
	return &base.EvaluatePolicyResp{Allowed: true}, nil
}

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

type RiskEvaluateRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRiskEvaluateRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RiskEvaluateRevertLogic {
	return &RiskEvaluateRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RiskEvaluateRevert SAGA 分支 02 补偿：空补偿（案例人工处置）。
//
// 设计依据：命中案例是风控审计事实，走人工 Claim/Resolve 处置，不随补偿自动撤销。
// 故补偿仅需过屏障注册，让 DTM 正常收尾。
func (l *RiskEvaluateRevertLogic) RiskEvaluateRevert(in *base.RiskEvaluateReq) (*base.RiskEvaluateResp, error) {
	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("RiskEvaluateRevert get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("RiskEvaluateRevert get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		// 空补偿：无业务动作，仅屏障注册 + 防御日志
		return nil
	})
	if err != nil {
		l.Errorf("RiskEvaluateRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if businessRan {
		l.Infof("RiskEvaluateRevert success (no-op compensate), requestId: %s", in.RequestId)
	} else {
		l.Infof("RiskEvaluateRevert barrier-skip (repeated compensate), requestId: %s", in.RequestId)
	}
	return &base.RiskEvaluateResp{Passed: true}, nil
}

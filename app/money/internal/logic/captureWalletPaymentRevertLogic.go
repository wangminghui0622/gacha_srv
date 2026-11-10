package logic

import (
	"context"
	"database/sql"

	"gacha_srv/app/dtm/client/dtmgrpc"
	"gacha_srv/app/money/internal/svc"
	"gacha_srv/app/money/money"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CaptureWalletPaymentRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCaptureWalletPaymentRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CaptureWalletPaymentRevertLogic {
	return &CaptureWalletPaymentRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CaptureWalletPaymentRevert SAGA 分支 07 补偿：末步空补偿（注册防御）。
//
// 设计依据：捕获只做机械状态迁移，业务失败前置（04 冻结成功即足额）；
// Aborted 只会发生在任何 PostHold 之前的防御性状态校验——无已生效副作用需撤销。
// 若捕获途中系统错误，DTM 对 07 走重试（Internal 不触发补偿），PostHold 幂等收敛。
// 故补偿仅需过屏障注册，让 DTM 正常收尾。
func (l *CaptureWalletPaymentRevertLogic) CaptureWalletPaymentRevert(in *money.CaptureWalletPaymentReq) (*money.CaptureWalletPaymentResp, error) {
	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("CaptureWalletPaymentRevert get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("CaptureWalletPaymentRevert get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		// 空补偿：无业务动作，仅屏障注册 + 防御日志
		return nil
	})
	if err != nil {
		l.Errorf("CaptureWalletPaymentRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if businessRan {
		l.Infof("CaptureWalletPaymentRevert success (no-op compensate), requestId: %s", in.RequestId)
	} else {
		l.Infof("CaptureWalletPaymentRevert barrier-skip (repeated compensate), requestId: %s", in.RequestId)
	}
	return &money.CaptureWalletPaymentResp{PaymentNo: "P" + in.RequestId, Status: 0}, nil
}

package playkit

import (
	"context"
	"database/sql"

	"gacha_srv/app/dtm/client/dtmgrpc"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/pkg/utils/txutil"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// BarrierTx 统一 DTM 子事务屏障 + 事务连接：
//
//	BarrierFromGrpc → raw DB → CallWithDB → txutil.WithTx（业务用 *gorm.DB）
//
// 返回值 ran 对应当前代码里的 businessRan：
//   - true  = 业务回调实际执行（正常路径）；
//   - false = 屏障级跳过（分支已成功/重试），调用方应读库如实返回。
//
// op 仅用于错误日志前缀（如 "LockGashapon"）。
func BarrierTx(ctx context.Context, svcCtx *svc.ServiceContext, op string, fn func(db *gorm.DB) error) (ran bool, err error) {
	barrier, err := dtmgrpc.BarrierFromGrpc(ctx)
	if err != nil {
		logx.WithContext(ctx).Errorf("%s get barrier failed, err: %v", op, err)
		return false, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := svcCtx.DB.DB()
	if err != nil {
		logx.WithContext(ctx).Errorf("%s get raw db failed, err: %v", op, err)
		return false, status.Error(codes.Internal, err.Error())
	}
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		ran = true
		return fn(txutil.WithTx(ctx, svcCtx.DB, tx))
	})
	return ran, err
}

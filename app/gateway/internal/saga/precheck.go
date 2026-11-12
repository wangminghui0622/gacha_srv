// Package saga 网关统一 SAGA 组装器（builder 见 builder.go）。
package saga

import (
	"context"

	baseClient "gacha_srv/app/base/baseClient"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

// Precheck 同步预检（SAGA 之外）：资格评估 + 风控评估。
//
// 这两个是「闸门」而非「事务」——只读评估 + 追加日志（决策日志/风控案例均
// 只追加不撤销，无任何需回滚的状态），故从 SAGA 抽离为同步预检：
//   - 拒绝时立即返回业务码（4001 资格拒绝 / 4002 风控拒绝），前端当场可见，
//     免去 DTM 编排与「pending 后无记录」的静默失败；
//   - 分支数 7 → 5，事务更短、更不易出错。
//
// 竞态说明：评估通过 → 提交 SAGA 之间隔几毫秒，策略/风控状态可能变化，但该
// 窗口可忽略，且硬校验（冻结/开奖）仍由后续分支兜底，不影响正确性。
// 任一拒绝直接返回 CodeError；base 逻辑经拦截器把业务码（>=1000）原样跨 gRPC 传递。
func Precheck(ctx context.Context, log logx.Logger, baseRpc baseClient.Base, gid string, userId, boxId, amountFen int64, scene string) error {
	if _, err := baseRpc.EvaluatePolicy(ctx, &baseClient.EvaluatePolicyReq{
		RequestId: gid,
		UserId:    userId,
		BoxId:     boxId,
		Scene:     scene,
	}); err != nil {
		log.Errorf("Precheck EvaluatePolicy denied/failed, gid: %s, scene: %s, err: %v", gid, scene, err)
		return errorx.FromError(err)
	}
	if _, err := baseRpc.RiskEvaluate(ctx, &baseClient.RiskEvaluateReq{
		RequestId: gid,
		UserId:    userId,
		BoxId:     boxId,
		Scene:     scene,
		AmountFen: amountFen,
	}); err != nil {
		log.Errorf("Precheck RiskEvaluate rejected/failed, gid: %s, scene: %s, err: %v", gid, scene, err)
		return errorx.FromError(err)
	}
	return nil
}

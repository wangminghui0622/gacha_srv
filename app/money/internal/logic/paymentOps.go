package logic

// ══════════════ 支付公共常量（阶段2 M8）══════════════
// 资金一致性铁律同用户钱包：FOR UPDATE 行锁 + 盲更新 + 确定性单号幂等；
// 支付表全 nocache。子事务屏障 + 跨服务 sub-call 幂等（Freeze/Post/Void 各自 uk）。

// 支付单状态（payment_orders.status）
const (
	orderStatusCreated   int64 = 1 // 已创建（建单中）
	orderStatusAwaiting  int64 = 2 // 待支付（资金已冻结）
	orderStatusPaid      int64 = 3 // 已支付（捕获完成）
	orderStatusRefunding int64 = 4 // 退款中
	orderStatusRefunded  int64 = 5 // 已退款
	orderStatusCanceled  int64 = 6 // 已取消（SAGA 补偿）
)

// 资金腿状态（payment_legs.status）
const (
	legStatusPending int64 = 1 // 已冻结（待捕获/可作废）
	legStatusPosted  int64 = 2 // 已入账（捕获完成）
	legStatusVoided  int64 = 3 // 已作废（撤单/退款）
)

// 退款状态（payment_refunds.status）
const (
	refundStatusProcessing int64 = 1 // 处理中
	refundStatusSucceeded  int64 = 2 // 成功
	refundStatusFailed     int64 = 3 // 失败
)

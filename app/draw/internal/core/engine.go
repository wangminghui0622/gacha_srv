package core

// PrizeEntry 参与抽取的候选条目（与模型层解耦，仅引擎需要的字段）。
// 确定性抽取走 deterministic.go 的 DrawDeterministic（对齐 server_v2 Rust 引擎）；
// 本类型仅作调用方构造候选池用（settleDrawLogic 按 remaining/滚动剔除过滤后传引擎）。
type PrizeEntry struct {
	Id     int64
	Name   string
	Weight int
}

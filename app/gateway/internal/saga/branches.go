package saga

import (
	marketingClient "gacha_srv/app/marketing/marketingClient"

	"google.golang.org/protobuf/proto"
)

// 通用 play SAGA 分支路径（2026-11 接口化：7 个 Play* RPC 替代 36 个 typed 分支 RPC）。
const (
	PathPlayLock         = "/play.Play/PlayLock"
	PathPlayRelease      = "/play.Play/PlayRelease"
	PathPlaySettle       = "/play.Play/PlaySettle"
	PathPlaySettleRevert = "/play.Play/PlaySettleRevert"
	PathPlayGrant        = "/play.Play/PlayGrant"
	PathPlayGrantRevert  = "/play.Play/PlayGrantRevert"
)

// PlayBranchSet 一个玩法的 3 个业务分支（lock / settle / grant）。
// 02 冻结、05 捕获由 PlayBranches 统一插入，玩法不再各写一遍。
type PlayBranchSet struct {
	LockPath   string
	ReleasePath string
	LockPayload proto.Message

	SettlePath       string
	SettleRevertPath string
	SettlePayload    proto.Message

	GrantPath       string
	GrantRevertPath string
	GrantPayload    proto.Message
}

// PlayBranches 按 SAGA 固定顺序组装 5 分支：
//
//	01 lock → 02 authorize → 03 settle → 04 grant → 05 capture
//
// 补偿顺序由 DTM 反向执行：captureRevert → grantRevert → settleRevert → cancel → release。
// 这个顺序是玩法 SAGA 的语义不变量（奖品先于捕获、锁先于扣款），集中在这里防未来改错。
func PlayBranches(moneyRpcDirect, gid string, userID, boxID int64,
	quote *marketingClient.QuoteCheckoutResp, set PlayBranchSet) []Branch {
	return []Branch{
		{Action: set.LockPath, Revert: set.ReleasePath, Payload: set.LockPayload},
		WalletAuthorizeBranch(moneyRpcDirect, gid, userID, boxID, quote),
		{Action: set.SettlePath, Revert: set.SettleRevertPath, Payload: set.SettlePayload},
		{Action: set.GrantPath, Revert: set.GrantRevertPath, Payload: set.GrantPayload},
		WalletCaptureBranch(moneyRpcDirect, gid, quote),
	}
}

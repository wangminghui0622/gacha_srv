package logic

import (
	"context"
	"encoding/json"
	"errors"

	"gacha_srv/app/play/internal/core"
	"gacha_srv/app/play/internal/svc"
)

// ══════════════ M13 拼团抽（GROUP_BUY）公共操作 ═══════════════
// 状态 JSON 解析与落库助手（bets/awards/lucky_sent）。
// 回滚职责划分（dig/fullcar/gashapon 先例——唯一回滚点防双扣）：
//   - 03 补偿（ReleaseGroupBuy）唯一负责弹出 bet 记录 + remaining_bets+=count；
//   - 05 补偿（SettleGroupBuyRevert）只撤销自己造成的影响：记录撤销、
//     按 requestId 删结算奖、条目/子奖库存回补、lucky_sent 回退、
//     SETTLED→OPEN——不碰下注记录与 remaining_bets（防双扣）。

// 拼团抽状态（group_buy_state.status）
const (
	groupBuyStateOpen    int64 = 1
	groupBuyStateSettled int64 = 2
)

// 拼团抽记录状态（group_buy_records.status）
const (
	groupBuyRecordNormal   int64 = 1
	groupBuyRecordCanceled int64 = 2
)

// groupBuyStateJSON state 行三段 JSON 内存视图
type groupBuyStateJSON struct {
	Bets      []core.GroupBuyBet   `json:"bets"`
	Awards    []core.GroupBuyAward `json:"awards"`
	LuckySent []int32              `json:"lucky_sent"`
}

// groupBuyParseState state 行三段 JSON → 内存状态（空串取零值；坏 JSON → 错误）
func groupBuyParseState(betsJson, awardsJson, luckySentJson string) (*groupBuyStateJSON, error) {
	state := &groupBuyStateJSON{}
	if betsJson != "" {
		if err := json.Unmarshal([]byte(betsJson), &state.Bets); err != nil {
			return nil, err
		}
	}
	if state.Bets == nil {
		state.Bets = []core.GroupBuyBet{}
	}
	if awardsJson != "" {
		if err := json.Unmarshal([]byte(awardsJson), &state.Awards); err != nil {
			return nil, err
		}
	}
	if state.Awards == nil {
		state.Awards = []core.GroupBuyAward{}
	}
	if luckySentJson != "" {
		if err := json.Unmarshal([]byte(luckySentJson), &state.LuckySent); err != nil {
			return nil, err
		}
	}
	if state.LuckySent == nil {
		state.LuckySent = []int32{}
	}
	return state, nil
}

// groupBuyParseLucky lucky_prizes_json → 幸运奖配置（Remaining=quantity 原始值；
// 结算输入侧由调用方按 lucky_sent[prize_index] 扣减；空/坏 JSON → 错误，调用方报 3020）
func groupBuyParseLucky(jsonStr string) ([]core.GroupBuyLuckyStock, error) {
	if jsonStr == "" {
		return nil, errors.New("empty lucky prizes json")
	}
	var stocks []core.GroupBuyLuckyStock
	if err := json.Unmarshal([]byte(jsonStr), &stocks); err != nil {
		return nil, err
	}
	return stocks, nil
}

// groupBuyParseLuckyChildren lucky_children_json → 幸运子奖配置（空 → nil；坏 JSON → 错误）
func groupBuyParseLuckyChildren(jsonStr string) ([]core.GroupBuyLuckyChildStock, error) {
	if jsonStr == "" {
		return nil, nil
	}
	var children []core.GroupBuyLuckyChildStock
	if err := json.Unmarshal([]byte(jsonStr), &children); err != nil {
		return nil, err
	}
	return children, nil
}

// groupBuyBetsJSON 下注列表 → JSON（[]struct 恒可编码）
func groupBuyBetsJSON(bets []core.GroupBuyBet) string {
	b, _ := json.Marshal(bets)
	return string(b)
}

// groupBuyAwardsJSON 结算奖列表 → JSON（[]struct 恒可编码）
func groupBuyAwardsJSON(awards []core.GroupBuyAward) string {
	b, _ := json.Marshal(awards)
	return string(b)
}

// groupBuyLuckySentJSON 幸运已发计数 → JSON（[]int32 恒可编码）
func groupBuyLuckySentJSON(sent []int32) string {
	b, _ := json.Marshal(sent)
	return string(b)
}

// groupBuyAwardsOfRequest 取指定 requestId 归属的结算奖（06 发放本 gid 触发的结算奖）
func groupBuyAwardsOfRequest(awards []core.GroupBuyAward, requestId string) []core.GroupBuyAward {
	out := make([]core.GroupBuyAward, 0, len(awards))
	for _, a := range awards {
		if a.RequestID == requestId {
			out = append(out, a)
		}
	}
	return out
}

// grantOneGroupBuyAward 结算奖单奖幂等发放：
// 形状同 digCellPrize（kind/cash/points/coupon），复用 dig 的发放通道
// （现金 CreditWallet kind=1 / 积分 kind=4 / 券本地插，grant_no 全 uk 幂等）。
func grantOneGroupBuyAward(ctx context.Context, svcCtx *svc.ServiceContext, userId int64, grantNo string, a core.GroupBuyAward) error {
	return grantOneDigPrize(ctx, svcCtx, userId, grantNo, digCellPrize{
		Kind:             a.PrizeKind,
		CashFen:          a.CashFen,
		Points:           a.PointsAmount,
		CouponTemplateNo: a.CouponTemplateNo,
	})
}

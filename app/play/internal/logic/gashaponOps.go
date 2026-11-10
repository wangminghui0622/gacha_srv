package logic

import (
	"context"
	"encoding/json"
	"errors"

	"gacha_srv/app/play/internal/core"
	"gacha_srv/app/play/internal/svc"
)

// ══════════════ M12c 扭蛋（GASHAPON）公共操作 ═══════════════
// 状态 JSON 解析与落库助手（participants/records/sent/awards）。
// 回滚职责划分（dig/fullcar 先例——唯一回滚点防双扣）：
//   - 03 补偿（ReleaseGashapon）唯一负责弹出 records + participants-=count + 余票+=count；
//   - 05 补偿（SettleGashaponRevert）只撤销自己造成的影响：记录撤销、按 requestId
//     删特别奖、sent 回退、SETTLED→OPEN、regular 奖库存回补——不碰余票与参与记录。
//   （若 revert 也 +count，与 release 叠加会双加，余票溢出。）

// 扭蛋状态（gashapon_state.status）
const (
	gashaponStateOpen    int64 = 1
	gashaponStateSettled int64 = 2
)

// 扭蛋记录状态（gashapon_records.status）
const (
	gashaponRecordNormal   int64 = 1
	gashaponRecordCanceled int64 = 2
)

// gashaponOutcome regular 奖结果落库元素（entries_json 数组元素；谢谢参与无资产，仅名字+是否中奖）。
// EntryId：条目主键——05 补偿回库存按主键精确定位（name 可重复/被改名，
// config_id+name 匹配会错还库存；旧记录无 entry_id 时回退 name 匹配）
type gashaponOutcome struct {
	EntryId int64  `json:"entry_id"`
	Name    string `json:"name"`
	IsPrize bool   `json:"is_prize"`
}

// gashaponParseState state 行四段 JSON → 内存状态（空串取零值；坏 JSON → 错误）
func gashaponParseState(participantsJson, recordsJson, sentJson, awardsJson string) (*core.GashaponBoxState, error) {
	state := &core.GashaponBoxState{}
	if participantsJson != "" {
		if err := json.Unmarshal([]byte(participantsJson), &state.Participants); err != nil {
			return nil, err
		}
	}
	if state.Participants == nil {
		state.Participants = map[int64]int64{}
	}
	if recordsJson != "" {
		if err := json.Unmarshal([]byte(recordsJson), &state.Records); err != nil {
			return nil, err
		}
	}
	if sentJson != "" {
		if err := json.Unmarshal([]byte(sentJson), &state.Sent); err != nil {
			return nil, err
		}
	}
	if awardsJson != "" {
		if err := json.Unmarshal([]byte(awardsJson), &state.Awards); err != nil {
			return nil, err
		}
	}
	return state, nil
}

// gashaponParsePrizes special_prizes_json → 特别奖配置（空/坏 JSON → 错误，调用方报 3020）
func gashaponParsePrizes(jsonStr string) ([]core.GashaponSpecialPrize, error) {
	if jsonStr == "" {
		return nil, errors.New("empty special prizes json")
	}
	var prizes []core.GashaponSpecialPrize
	if err := json.Unmarshal([]byte(jsonStr), &prizes); err != nil {
		return nil, err
	}
	return prizes, nil
}

// gashaponParseOutcomes entries_json → regular 奖结果（空串/坏 JSON → 空列表，防御配置垃圾）
func gashaponParseOutcomes(jsonStr string) []gashaponOutcome {
	if jsonStr == "" {
		return nil
	}
	var outcomes []gashaponOutcome
	if err := json.Unmarshal([]byte(jsonStr), &outcomes); err != nil {
		return nil
	}
	return outcomes
}

// gashaponOutcomesJSON regular 奖结果 → JSON（[]struct 恒可编码）
func gashaponOutcomesJSON(outcomes []gashaponOutcome) string {
	b, _ := json.Marshal(outcomes)
	return string(b)
}

// gashaponAwardsJSON 特别奖列表 → JSON（[]struct 恒可编码）
func gashaponAwardsJSON(awards []core.GashaponAward) string {
	b, _ := json.Marshal(awards)
	return string(b)
}

// gashaponAwardsOfRequest 取指定 requestId 归属的特别奖（06 发放本 gid 触发的特别奖）
func gashaponAwardsOfRequest(awards []core.GashaponAward, requestId string) []core.GashaponAward {
	out := make([]core.GashaponAward, 0, len(awards))
	for _, a := range awards {
		if a.RequestID == requestId {
			out = append(out, a)
		}
	}
	return out
}

// gashaponSentJSON sent 计数 → JSON（[]int32 恒可编码）
func gashaponSentJSON(sent []int32) string {
	b, _ := json.Marshal(sent)
	return string(b)
}

// gashaponParticipantsJSON 参与数 map → JSON（map[int64]int64 恒可编码）
func gashaponParticipantsJSON(participants map[int64]int64) string {
	b, _ := json.Marshal(participants)
	return string(b)
}

// gashaponRecordsJSON 参与记录 → JSON（[]struct 恒可编码）
func gashaponRecordsJSON(records []core.GashaponParticipation) string {
	b, _ := json.Marshal(records)
	return string(b)
}

// grantOneGashaponPrize 特别奖单奖幂等发放：
// 形状同 digCellPrize（kind/cash/points/coupon），复用 dig 的发放通道
// （现金 CreditWallet kind=1 / 积分 kind=4 / 券本地插，grant_no 全 uk 幂等）。
func grantOneGashaponPrize(ctx context.Context, svcCtx *svc.ServiceContext, userId int64, grantNo string, a core.GashaponAward) error {
	return grantOneDigPrize(ctx, svcCtx, userId, grantNo, digCellPrize{
		Kind:             a.PrizeKind,
		CashFen:          a.CashFen,
		Points:           a.PointsAmount,
		CouponTemplateNo: a.CouponTemplateNo,
	})
}

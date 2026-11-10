package logic

import (
	"context"
	"encoding/json"

	"gacha_srv/app/play/internal/core"
	"gacha_srv/app/play/internal/svc"
)

// ══════════════ M12b 满车（FULL_CAR）公共操作 ═══════════════
// 座位/整车奖状态 JSON 解析与落库助手。
// 回滚职责划分（dig 先例——唯一回滚点防双扣）：
//   - 03 补偿（ReleaseFullCar）唯一负责座位释放 + 余票+1（锁的正向只做这两件事）；
//   - 05 补偿（SettleFullCarRevert）只撤销自己造成的影响：记录撤销、按 requestId
//     删整车奖、DEPARTED→OPEN、regular 奖库存回补——不碰余票。
//   （若 revert 也 +1，07 失败场景下与 release 叠加会 +2，余票溢出。）

// 满车状态（full_car_state.status）
const (
	fullCarStateOpen     int64 = 1
	fullCarStateDeparted int64 = 2
)

// 满车记录状态（full_car_records.status）
const (
	fullCarRecordNormal   int64 = 1
	fullCarRecordCanceled int64 = 2
)

// fullCarStoredAward 整车结算奖落库元素（awards_json 数组元素，含 requestId 供补偿精确定位）
type fullCarStoredAward struct {
	SeatIndex        int32  `json:"seat_index"`
	UserId           int64  `json:"user_id"`
	PrizeKind        int64  `json:"prize_kind"` // 1 CASH 2 POINTS 3 COUPON
	CashFen          int64  `json:"cash_fen"`
	PointsAmount     int64  `json:"points_amount"`
	CouponTemplateNo string `json:"coupon_template_no"`
	Tier             int32  `json:"tier"` // 1 LUCKY 2 NORMAL
	RequestId        string `json:"request_id"`
}

// fullCarAwardsToStored 整车结算结果 → 落库元素（requestId 标记归属）
func fullCarAwardsToStored(awards []core.FullCarAward, requestId string) []fullCarStoredAward {
	stored := make([]fullCarStoredAward, 0, len(awards))
	for _, a := range awards {
		stored = append(stored, fullCarStoredAward{
			SeatIndex:        a.SeatIndex,
			UserId:           a.UserID,
			PrizeKind:        a.Prize.PrizeKind,
			CashFen:          a.Prize.CashFen,
			PointsAmount:     a.Prize.PointsAmount,
			CouponTemplateNo: a.Prize.CouponTemplateNo,
			Tier:             a.Tier,
			RequestId:        requestId,
		})
	}
	return stored
}

// fullCarParseAwards awards_json → 落库元素列表（空串/坏 JSON → 空列表，防御配置垃圾）
func fullCarParseAwards(jsonStr string) []fullCarStoredAward {
	if jsonStr == "" {
		return nil
	}
	var awards []fullCarStoredAward
	if err := json.Unmarshal([]byte(jsonStr), &awards); err != nil {
		return nil
	}
	return awards
}

// fullCarAwardsJSON 整车奖列表 → JSON（[]struct 恒可编码）
func fullCarAwardsJSON(awards []fullCarStoredAward) string {
	b, _ := json.Marshal(awards)
	return string(b)
}

// fullCarAwardsOfRequest 取指定 requestId 归属的整车奖（06 发放本 gid 触发的整车奖）
func fullCarAwardsOfRequest(awards []fullCarStoredAward, requestId string) []fullCarStoredAward {
	out := make([]fullCarStoredAward, 0, len(awards))
	for _, a := range awards {
		if a.RequestId == requestId {
			out = append(out, a)
		}
	}
	return out
}

// fullCarParseSeats seats_json → 座位用户列表（0=空位；坏 JSON → nil，调用方判长度）
func fullCarParseSeats(jsonStr string) []int64 {
	if jsonStr == "" {
		return nil
	}
	var seats []int64
	if err := json.Unmarshal([]byte(jsonStr), &seats); err != nil {
		return nil
	}
	return seats
}

// fullCarMarshalSeats 座位用户列表 → JSON（Marshal 不会失败：[]int64 恒可编码）
func fullCarMarshalSeats(seats []int64) string {
	b, _ := json.Marshal(seats)
	return string(b)
}

// grantOneFullCarPrize 整车奖单奖幂等发放：
// 形状同 digCellPrize（kind/cash/points/coupon），复用 dig 的发放通道
// （现金 CreditWallet kind=1 / 积分 kind=4 / 券本地插，grant_no 全 uk 幂等）。
func grantOneFullCarPrize(ctx context.Context, svcCtx *svc.ServiceContext, userId int64, grantNo string, a fullCarStoredAward) error {
	return grantOneDigPrize(ctx, svcCtx, userId, grantNo, digCellPrize{
		Kind:             a.PrizeKind,
		CashFen:          a.CashFen,
		Points:           a.PointsAmount,
		CouponTemplateNo: a.CouponTemplateNo,
	})
}

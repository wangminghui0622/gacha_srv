package gameimpl

import (
	"context"
	"encoding/json"
	"errors"
	"gacha_srv/app/play/internal/core"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

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

type GetFullCarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// awardItems 整车奖落库元素 → 响应条目（券名尽力解析，展示用）
func (l *GetFullCarLogic) awardItems(awards []fullCarStoredAward) []*play.FullCarAwardItem {
	items := make([]*play.FullCarAwardItem, 0, len(awards))
	for _, a := range awards {
		item := &play.PrizeItem{Kind: a.PrizeKind, Points: a.PointsAmount, CouponTemplateNo: a.CouponTemplateNo}
		switch a.PrizeKind {
		case prizeCash:
			item.Amount = utils.FenToYuan(a.CashFen)
		case prizeCoupon:
			item.CouponName = couponNameOf(l.ctx, l.svcCtx, a.CouponTemplateNo)
		}
		items = append(items, &play.FullCarAwardItem{
			SeatIndex: int64(a.SeatIndex),
			UserId:    a.UserId,
			Prize:     item,
			Tier:      int64(a.Tier),
		})
	}
	return items
}

type GrantFullCarPrizesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type GrantFullCarPrizesRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

type ListFullCarRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// buildItem 记录行 → 响应条目（金额转元）
func (l *ListFullCarRecordsLogic) buildItem(rec *gamemodelnc.FullCarRecords) *play.FullCarRecordItem {
	return &play.FullCarRecordItem{
		RecordId:  rec.Id,
		RequestId: rec.RequestId,
		ConfigId:  rec.ConfigId,
		SeatIndex: rec.SeatIndex,
		EntryName: rec.EntryName,
		IsPrize:   rec.IsPrize == 1,
		SkuId:     rec.SkuId,
		Price:     utils.FenToYuan(rec.PriceFen),
		Status:    rec.Status,
		CreatedAt: rec.CreatedAt.Unix(),
	}
}

type LockFullCarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// seatOfUser 事务内读当前状态中用户的座位（幂等路径如实返回；未找到 → -1）
func (l *LockFullCarLogic) seatOfUser(db *gorm.DB, configId, userId int64) int32 {
	var st gamemodelnc.FullCarState
	if err := db.Where("config_id = ?", configId).First(&st).Error; err != nil {
		return -1
	}
	for i, u := range fullCarParseSeats(st.SeatsJson) {
		if u == userId {
			return int32(i)
		}
	}
	return -1
}

type ReleaseFullCarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// errFullCarRecordDup 事务内哨兵：同 requestId 的满车记录已存在（另一 gid 已结算）
var errFullCarRecordDup = errors.New("full car record duplicated")

type SettleFullCarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// isPrizeEntryName 由条目 id 取 is_prize（picked 只带回 id/name/weight）
func isPrizeEntryName(entries []gamemodel.FullCarEntries, id int64) int64 {
	for _, e := range entries {
		if e.Id == id {
			return e.IsPrize
		}
	}
	return 0
}

// readBackRecord 幂等/屏障跳过路径：读已有记录 + 当前状态如实返回
func (l *SettleFullCarLogic) readBackRecord(requestId string, userId int64) (*play.SettleFullCarResp, error) {
	var rec gamemodelnc.FullCarRecords
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", requestId, userId).
		First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("readBackRecord: record not found, requestId: %s", requestId)
			return nil, status.Error(codes.Internal, "满车记录不存在")
		}
		l.Errorf("readBackRecord failed, requestId: %s, err: %v", requestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	departed := false
	var st gamemodelnc.FullCarState
	if err := l.svcCtx.DB.Where("config_id = ?", rec.ConfigId).First(&st).Error; err == nil {
		departed = st.Status == fullCarStateDeparted
	}
	l.Infof("readBackRecord: requestId: %s, recordId: %d, departed: %v", requestId, rec.Id, departed)
	return &play.SettleFullCarResp{
		RecordId:  rec.Id,
		EntryName: rec.EntryName,
		IsPrize:   rec.IsPrize == 1,
		Departed:  departed,
		Status:    "SETTLED",
	}, nil
}

type SettleFullCarRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

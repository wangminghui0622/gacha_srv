package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gacha_srv/app/marketing/marketingClient"
	"gacha_srv/app/play/internal/core"
	"gacha_srv/app/play/internal/playkit"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	gamemodel "gacha_srv/pkg/model/gacha_play"
	gamemodelnc "gacha_srv/pkg/model_nocache/gacha_play"
	"gacha_srv/pkg/utils"
	"gacha_srv/pkg/utils/errorx"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
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

type GetFullCarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetFullCarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFullCarLogic {
	return &GetFullCarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetFullCar 满车详情：配置 + 座位 + 余票 + 状态；整车奖仅在 DEPARTED 后可见。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *GetFullCarLogic) GetFullCar(in *play.GetFullCarReq) (*play.GetFullCarResp, error) {
	if in.ConfigId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}

	cfg, err := l.svcCtx.FullCarCfgModel.FindOne(l.ctx, in.ConfigId)
	if err != nil {
		if errors.Is(err, gamemodel.ErrNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "满车配置不存在")
		}
		l.Errorf("GetFullCar query config failed, configId: %d, err: %v", in.ConfigId, err)
		return nil, errorx.FromMySQLError(err)
	}

	var st gamemodelnc.FullCarState
	if err := l.svcCtx.DB.Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "满车状态不存在")
		}
		l.Errorf("GetFullCar query state failed, configId: %d, err: %v", in.ConfigId, err)
		return nil, errorx.FromMySQLError(err)
	}

	seats := fullCarParseSeats(st.SeatsJson)
	seatItems := make([]*play.FullCarSeatItem, 0, len(seats))
	for i, u := range seats {
		seatItems = append(seatItems, &play.FullCarSeatItem{SeatIndex: int64(i), UserId: u})
	}

	resp := &play.GetFullCarResp{
		ConfigId:         cfg.Id,
		Name:             cfg.Name,
		Price:            utils.FenToYuan(cfg.PriceFen),
		SeatCount:        cfg.SeatCount,
		Seats:            seatItems,
		RemainingTickets: st.RemainingTickets,
		Status:           st.Status,
	}
	if st.Status == fullCarStateDeparted {
		resp.Awards = l.awardItems(fullCarParseAwards(st.AwardsJson))
	}
	return resp, nil
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

func NewGrantFullCarPrizesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantFullCarPrizesLogic {
	return &GrantFullCarPrizesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantFullCarPrizes SAGA 06 正向：发放本 gid 触发的整车结算奖（随 05 落 awards_json，
// 按 requestId 精确定位；非整车触发时为空集、直接成功）。
// 逐奖幂等发放（grant_no = "fcp_"+gid+"#"+idx，确定性派生）：
//   - 现金 → user.CreditWallet（kind=1，requestId=grant_no 幂等）
//   - 积分 → user.CreditWallet（kind=4，requestId=grant_no 幂等）
//   - 券   → 本地插入用户券（coupon_no="uc_"+grant_no，uk 撞键=已发过）
//
// regular 奖（满车抽中条目）只落记录展示、不在此发放（条目无 prize_kind，
// 形状同 draw_pool_entries 的资产化由 draw 域负责；满车条目仅展示+整车奖实发）。
//
// 铁律：**永不 Aborted**（奖品先于捕获，撤单不回退奖品，与 dig GrantDigPrizes 对齐）；
// 失败只回 Internal → DTM 退避重试，直到发放收敛。
func (l *GrantFullCarPrizesLogic) GrantFullCarPrizes(in *play.GrantFullCarPrizesReq) (*play.GrantFullCarPrizesResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 {
		return nil, status.Error(codes.Internal, "code=400 msg=请求参数非法")
	}

	// 整车奖由 05 正向落库（06 在其后执行）；缺失=异常，Internal 重试
	var st gamemodelnc.FullCarState
	if err := l.svcCtx.DB.Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("GrantFullCarPrizes state not found, configId: %d", in.ConfigId)
			return nil, status.Error(codes.Internal, fmt.Sprintf("code=%d msg=满车状态不存在", errorx.CodeFullCarStateInvalid))
		}
		l.Errorf("GrantFullCarPrizes query state failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	awards := fullCarAwardsOfRequest(fullCarParseAwards(st.AwardsJson), in.RequestId)
	prizes := make([]playkit.Prize, 0, len(awards))
	for _, a := range awards {
		prizes = append(prizes, playkit.Prize{
			UserID:           a.UserId,
			Kind:             a.PrizeKind,
			CashFen:          a.CashFen,
			Points:           a.PointsAmount,
			CouponTemplateNo: a.CouponTemplateNo,
		})
	}
	// grant_no = fcp_{gid}#{idx}（GrantAll 内派生）：DTM 重试/补偿重放命中同一键，
	// CreditWallet requestId 幂等 + 券 uk 撞键 = 已发过读回成功。
	if err := playkit.GrantAll(l.ctx, l.svcCtx, in.RequestId, "fcp", prizes); err != nil {
		// 永不 Aborted：失败 Internal → DTM 重试，grant_no 幂等收敛
		l.Errorf("GrantFullCarPrizes grant failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	// 高1 修复：发奖落库标记（04 补偿退款前置检查依据）；Internal 失败 → DTM 重试 06 幂等收敛
	if len(prizes) > 0 {
		if err := playkit.MarkGranted(l.ctx, l.svcCtx, in.RequestId, in.UserId); err != nil {
			l.Errorf("GrantFullCarPrizes mark failed, requestId: %s, err: %v", in.RequestId, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	l.Infof("GrantFullCarPrizes success, requestId: %s, awards: %d", in.RequestId, len(awards))
	return &play.GrantFullCarPrizesResp{Status: "GRANTED"}, nil
}

type GrantFullCarPrizesRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantFullCarPrizesRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantFullCarPrizesRevertLogic {
	return &GrantFullCarPrizesRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantFullCarPrizesRevert SAGA 06 补偿：空操作（整车奖先于捕获且不追回，
// 与 dig 同款决策；补偿注册仅为防御，即使调用也幂等返回）
func (l *GrantFullCarPrizesRevertLogic) GrantFullCarPrizesRevert(in *play.GrantFullCarPrizesReq) (*play.GrantFullCarPrizesResp, error) {
	l.Infof("GrantFullCarPrizesRevert noop, requestId: %s", in.RequestId)
	return &play.GrantFullCarPrizesResp{Status: "GRANTED"}, nil
}

type ListFullCarRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListFullCarRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFullCarRecordsLogic {
	return &ListFullCarRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListFullCarRecords 满车记录：requestId 非空=精确查一条（购买轮询），空=分页列表。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *ListFullCarRecordsLogic) ListFullCarRecords(in *play.ListFullCarRecordsReq) (*play.ListFullCarRecordsResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}

	// 精确查询（轮询开奖结果）
	if in.RequestId != "" {
		var rec gamemodelnc.FullCarRecords
		if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", in.RequestId, in.UserId).
			First(&rec).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// requestId 未命中是正常情况：返回空列表，避免每笔新购买打 error 日志。
				return &play.ListFullCarRecordsResp{Items: []*play.FullCarRecordItem{}, Total: 0}, nil
			}
			l.Errorf("ListFullCarRecords query failed, err: %v", err)
			return nil, err
		}
		return &play.ListFullCarRecordsResp{Items: []*play.FullCarRecordItem{l.buildItem(&rec)}, Total: 1}, nil
	}

	// 分页列表
	page, pageSize := in.Page, in.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	if err := l.svcCtx.DB.Model(&gamemodelnc.FullCarRecords{}).
		Where("user_id = ?", in.UserId).Count(&total).Error; err != nil {
		l.Errorf("ListFullCarRecords count failed, err: %v", err)
		return nil, err
	}
	var recs []gamemodelnc.FullCarRecords
	if err := l.svcCtx.DB.Where("user_id = ?", in.UserId).
		Order("id DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).
		Find(&recs).Error; err != nil {
		l.Errorf("ListFullCarRecords page failed, err: %v", err)
		return nil, err
	}
	items := make([]*play.FullCarRecordItem, 0, len(recs))
	for i := range recs {
		items = append(items, l.buildItem(&recs[i]))
	}
	return &play.ListFullCarRecordsResp{Items: items, Total: total}, nil
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

func NewLockFullCarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LockFullCarLogic {
	return &LockFullCarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LockFullCar SAGA 03 正向：barrier 事务内
//
//	① 报价只读校验（营销 GetQuote sub-call：userId/grossFen 取报价行）；
//	② state FOR UPDATE（uk config_id 串行化同车购买；种子缺失防御建行）；
//	③ 座位分配（显式 seatIndex 或首个空位；越界 3023 / 已占 3022 / 满车 3021）；
//	④ 价格一致（3018 复用：quote.gross == 单价×1）；
//	⑤ 报价锁（营销 LockQuote sub-call，置于全部玩法校验之后）；
//	⑥ 占座 + 余票-1。
//
// 报价锁放在最后：玩法自身校验失败回滚时不触碰营销锁（营销事务已独立提交，
// 无法随本事务回滚），与旧仓「锁与业务同事务」的最终状态一致——自中止后报价保持 OPEN。
// userId 取报价行（网关 QuoteCheckout 落库），不信请求参数。
// 错误语义（SAGA 分支铁律）：业务失败 → codes.Aborted（触发补偿）；系统错误 → codes.Internal（DTM 重试）。
func (l *LockFullCarLogic) LockFullCar(in *play.LockFullCarReq) (*play.LockFullCarResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	assignedSeat := int32(-1)
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "LockFullCar", func(db *gorm.DB) error {

		// ① 报价只读校验（营销 GetQuote sub-call；quote 不存在 → Aborted）
		quote, err := playkit.LoadQuote(l.ctx, l.svcCtx, in.QuoteToken)
		if err != nil {
			return err
		}
		userId := quote.UserID

		// 满车配置（纯种子配置只读，不走缓存锁）
		var cfg gamemodel.FullCarConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车配置不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		if cfg.Status != 1 || cfg.SeatCount < 2 || cfg.SeatCount > 1000 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车配置不可用", errorx.CodeGameConfigInvalid))
		}

		// ② state FOR UPDATE（uk(config_id) 串行化同车并发购买）
		var st gamemodelnc.FullCarState
		err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 防御：种子缺失时建行（空车，余票=座位数）
			st = gamemodelnc.FullCarState{
				ConfigId:         in.ConfigId,
				SeatsJson:        fullCarMarshalSeats(make([]int64, cfg.SeatCount)),
				AwardsJson:       "[]",
				RemainingTickets: cfg.SeatCount,
				Status:           fullCarStateOpen,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}
			if err := db.Create(&st).Error; err != nil {
				if errors.Is(err, gorm.ErrDuplicatedKey) {
					// 并发首建竞争：重读（行锁串行化后可见对方状态）
					if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
						Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
						return err
					}
				} else {
					return err
				}
			}
		} else if err != nil {
			return err
		}
		if st.Status != fullCarStateOpen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车状态不满足锁定", errorx.CodeFullCarStateInvalid))
		}
		if st.RemainingTickets <= 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车已出发", errorx.CodeFullCarDeparted))
		}
		seats := fullCarParseSeats(st.SeatsJson)
		if len(seats) != int(cfg.SeatCount) {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车座位数据异常", errorx.CodeGameConfigInvalid))
		}

		// ③ 座位分配：显式 seatIndex 或首个空位
		requested := in.SeatIndex
		if requested >= cfg.SeatCount {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=座位越界", errorx.CodeFullCarSeatInvalid))
		}
		// 中5 修复：同人重复占座拒绝——同一用户已在车内（任意座位）不可再占
		// （核心域已有 FullCarContainsUser 判定，锁座流程此前从未调用）
		if core.FullCarContainsUser(seats, userId) {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=同一用户已在车内，不可重复占座", errorx.CodeFullCarSeatOccupied))
		}
		var seat int32
		if requested >= 0 {
			if seats[requested] != 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=座位已被占用", errorx.CodeFullCarSeatOccupied))
			}
			seat = int32(requested)
		} else {
			seat = -1
			for i, u := range seats {
				if u == 0 {
					seat = int32(i)
					break
				}
			}
			if seat < 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车已出发", errorx.CodeFullCarDeparted))
			}
		}

		// ④ 价格一致（锁内重算）：单价×1 == 报价 gross == 请求总额
		if quote.GrossFen != cfg.PriceFen || in.TotalPriceFen != cfg.PriceFen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车价格校验不一致", errorx.CodeDigPriceMismatch))
		}

		// ⑤ 报价锁（营销 LockQuote sub-call，置于玩法校验之后；幂等：newLocked=false=已锁重发 → 业务已成功，读座位如实返回）
		newLocked, err := playkit.LockQuote(l.ctx, l.svcCtx, in.RequestId, in.QuoteToken)
		if err != nil {
			return err
		}
		if !newLocked {
			l.Infof("LockFullCar idempotent: quote already locked, quoteToken: %s", in.QuoteToken)
			assignedSeat = l.seatOfUser(db, in.ConfigId, userId)
			return nil
		}

		// ⑥ 占座 + 余票-1
		seats[seat] = userId
		if err := db.Model(&gamemodelnc.FullCarState{}).Where("id = ?", st.Id).
			Updates(map[string]interface{}{
				"seats_json":        fullCarMarshalSeats(seats),
				"remaining_tickets": st.RemainingTickets - 1,
				"status":            fullCarStateOpen,
				"updated_at":        time.Now(),
			}).Error; err != nil {
			return err
		}
		assignedSeat = seat
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("LockFullCar aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("LockFullCar failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（正向已成功）：读库如实返回
		return l.readBackLock(in.ConfigId, in.QuoteToken)
	}
	l.Infof("LockFullCar success, quoteToken: %s, seat: %d", in.QuoteToken, assignedSeat)
	return &play.LockFullCarResp{SeatIndex: int64(assignedSeat), Status: "LOCKED"}, nil
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

// readBackLock 屏障跳过时读库如实返回（报价经营销 RPC 读 userId）
func (l *LockFullCarLogic) readBackLock(configId int64, quoteToken string) (*play.LockFullCarResp, error) {
	q, err := l.svcCtx.MarketingRpc.GetQuote(l.ctx, &marketingClient.GetQuoteReq{QuoteToken: quoteToken})
	if err != nil {
		var ce *errorx.CodeError
		if errors.As(err, &ce) && ce.Code == errorx.CodeQuoteNotFound {
			return nil, status.Error(codes.Internal, "报价不存在")
		}
		l.Errorf("readBackLock failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	seat := l.seatOfUser(l.svcCtx.DB, configId, q.GetUserId())
	l.Infof("readBackLock: configId: %d, seat: %d", configId, seat)
	return &play.LockFullCarResp{SeatIndex: int64(seat), Status: "LOCKED"}, nil
}

type ReleaseFullCarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReleaseFullCarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReleaseFullCarLogic {
	return &ReleaseFullCarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ReleaseFullCar SAGA 03 补偿：barrier 事务内
//
//	① 报价 FOR UPDATE 先行——LOCKED ⇔ 本 gid 确实占座（需回滚）；OPEN ⇒ 正向
//	   已整体回滚（如 3022 座位已占），座位保持原样——绝不误释放他人座位；
//	② 座位释放 + 余票+1（仅 LOCKED 时；seats[i]==quote 用户才清，已回滚幂等跳过）。
//	   这是座位与余票的唯一回滚点（05 补偿只撤销记录/整车奖/库存，不碰余票——防双扣）；
//	③ 回退券（LOCKED 且 locked_by=本报价 → AVAILABLE）；
//	④ 报价 LOCKED/OPEN→RELEASED（幂等；USED 不回退）。
//
// 幂等：报价 RELEASED/USED/EXPIRED → 直接成功。
func (l *ReleaseFullCarLogic) ReleaseFullCar(in *play.ReleaseFullCarReq) (*play.ReleaseFullCarResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "releaseFullCar", func(db *gorm.DB) error {

		// ① 报价读（经营销 RPC）：USED（07 已捕获核销）不回退；RELEASED/EXPIRED 幂等无操作
		q, qerr := l.svcCtx.MarketingRpc.GetQuote(l.ctx, &marketingClient.GetQuoteReq{QuoteToken: in.QuoteToken})
		if qerr != nil {
			var ce *errorx.CodeError
			if errors.As(qerr, &ce) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
			}
			return qerr
		}
		if q.GetStatus() == "USED" || q.GetStatus() == "RELEASED" || q.GetStatus() == "EXPIRED" {
			l.Infof("ReleaseFullCar idempotent: quote status %s, quoteToken: %s", q.GetStatus(), in.QuoteToken)
			return nil
		}

		// ② 座位释放 + 余票+1：仅当报价 LOCKED（正向提交过占座）时执行
		if q.GetStatus() == "LOCKED" {
			var st gamemodelnc.FullCarState
			err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("config_id = ?", in.ConfigId).First(&st).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil {
				seats := fullCarParseSeats(st.SeatsJson)
				seat := in.SeatIndex
				if seat < 0 {
					// 自动选座：按 userId 反查座位释放
					for i, u := range seats {
						if u == q.GetUserId() {
							seat = int64(i)
							break
						}
					}
				}
				if seat >= 0 && int(seat) < len(seats) && seats[seat] == q.GetUserId() {
					seats[seat] = 0
					remaining := st.RemainingTickets + 1
					if remaining > int64(len(seats)) {
						remaining = int64(len(seats)) // 防御：余票不超座位数
					}
					if err := db.Model(&gamemodelnc.FullCarState{}).Where("id = ?", st.Id).
						Updates(map[string]interface{}{
							"seats_json":        fullCarMarshalSeats(seats),
							"remaining_tickets": remaining,
							"updated_at":        time.Now(),
						}).Error; err != nil {
						return err
					}
					l.Infof("ReleaseFullCar seat released, configId: %d, seat: %d", in.ConfigId, seat)
				}
			}
		}

		// ③④ 券回退+报价→RELEASED 经营销 ReleaseQuote sub-call（幂等；USED 不回退）
		if _, err := l.svcCtx.MarketingRpc.ReleaseQuote(l.ctx, &marketingClient.ReleaseQuoteReq{
			RequestId:  in.RequestId,
			QuoteToken: in.QuoteToken,
		}); err != nil {
			var ce *errorx.CodeError
			if errors.As(err, &ce) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", ce.Code, ce.Msg))
			}
			return err
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("ReleaseFullCar aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("ReleaseFullCar failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读报价如实返回（经营销 RPC；NotFound 保持 RELEASED 旧同款容错）
		statusOut := "RELEASED"
		q, qe := l.svcCtx.MarketingRpc.GetQuote(l.ctx, &marketingClient.GetQuoteReq{QuoteToken: in.QuoteToken})
		if qe == nil {
			statusOut = q.GetStatus()
		} else {
			var ce *errorx.CodeError
			if !errors.As(qe, &ce) || ce.Code != errorx.CodeQuoteNotFound {
				return nil, status.Error(codes.Internal, qe.Error())
			}
		}
		l.Infof("ReleaseFullCar success (readback), quoteToken: %s, status: %s", in.QuoteToken, statusOut)
		return &play.ReleaseFullCarResp{SeatIndex: in.SeatIndex, Status: statusOut}, nil
	}
	l.Infof("ReleaseFullCar success, quoteToken: %s", in.QuoteToken)
	return &play.ReleaseFullCarResp{SeatIndex: in.SeatIndex, Status: "RELEASED"}, nil
}

// errFullCarRecordDup 事务内哨兵：同 requestId 的满车记录已存在（另一 gid 已结算）
var errFullCarRecordDup = errors.New("full car record duplicated")

type SettleFullCarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleFullCarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleFullCarLogic {
	return &SettleFullCarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleFullCar SAGA 05 正向：barrier 事务内
//
//	① state FOR UPDATE（必须 OPEN）；② 座位校验（seats[seatIndex]==userId）；
//	③ 加权抽 regular 奖（full_car_entries，移植 draw-rpc core.PickPrize）+ 库存-1；
//	④ 插记录（uk request_id 撞键=已结算 → 读回幂等成功）；
//	⑤ 余票==0（最后一票已占满）→ 整车结算：确定性 SettleFullCar（seed=gid），
//	   awards 追加落库（含 requestId）、status→DEPARTED。
//
// 错误语义：业务失败 → codes.Aborted；系统错误 → codes.Internal。
func (l *SettleFullCarLogic) SettleFullCar(in *play.SettleFullCarReq) (*play.SettleFullCarResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	// 幂等重放快路径：同 requestId 记录已存在 → 读回成功（先于业务校验，M11b 教训）
	var existing gamemodelnc.FullCarRecords
	if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&existing).Error; err == nil {
		return l.readBackRecord(in.RequestId, in.UserId)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Errorf("SettleFullCar fast path query failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var outRecordId int64
	outEntry := ""
	outIsPrize := false
	outDeparted := false
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleFullCar", func(db *gorm.DB) error {

		// ① state FOR UPDATE
		var st gamemodelnc.FullCarState
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车状态不存在", errorx.CodeFullCarStateInvalid))
			}
			return err
		}
		if st.Status != fullCarStateOpen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车状态不满足结算", errorx.CodeFullCarStateInvalid))
		}

		// ② 座位必须已由 03 分配（防绕过锁定直接结算）
		seats := fullCarParseSeats(st.SeatsJson)
		seat := in.SeatIndex
		if seat < 0 {
			// 自动选座：03 自动分配后座位号不回传，按 userId 反查已分配座位
			seat = -1
			for i, u := range seats {
				if u == in.UserId {
					seat = int64(i)
					break
				}
			}
		}
		if seat < 0 || seat >= int64(len(seats)) || seats[seat] != in.UserId {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车座位与请求不一致", errorx.CodeFullCarStateInvalid))
		}

		// 满车配置（纯种子只读）
		var cfg gamemodel.FullCarConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车配置不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}

		// ③ 加权抽 regular 奖（remaining=0 售罄不参与；-1 不限量恒可抽）
		var entries []gamemodel.FullCarEntries
		if err := db.Where("config_id = ?", in.ConfigId).Find(&entries).Error; err != nil {
			return err
		}
		pool := make([]core.PrizeEntry, 0, len(entries))
		for _, e := range entries {
			if e.Remaining != 0 {
				pool = append(pool, core.PrizeEntry{Id: e.Id, Name: e.Name, Weight: int(e.Weight)})
			}
		}
		if len(pool) == 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车奖池已售罄", errorx.CodeGameConfigInvalid))
		}
		// 确定性抽取：seed=gid（同一 requestId 必抽同一条目，可回放核对）
		picked := core.PickPrizeDeterministic(pool, []byte(in.RequestId), "full-car-regular", 0)
		if picked == nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车奖池已售罄", errorx.CodeGameConfigInvalid))
		}
		// 库存-1（不限量 quantity=-1 跳过）
		if err := db.Model(&gamemodel.FullCarEntries{}).
			Where("id = ? AND quantity != -1 AND remaining > 0", picked.Id).
			Update("remaining", gorm.Expr("remaining - 1")).Error; err != nil {
			return err
		}

		// ④ 插记录（uk request_id 幂等；撞键 = 已结算读回）
		priceFen := int64(0)
		if in.Price != "" {
			f, err := utils.YuanToFen(in.Price)
			if err != nil || f < 0 {
				return status.Error(codes.Aborted, "code=400 msg=实付价格无效")
			}
			priceFen = f
		}
		rec := &gamemodelnc.FullCarRecords{
			RequestId: in.RequestId,
			UserId:    in.UserId,
			ConfigId:  in.ConfigId,
			SeatIndex: seat,
			EntryName: picked.Name,
			IsPrize:   isPrizeEntryName(entries, picked.Id),
			PriceFen:  priceFen,
			Status:    fullCarRecordNormal,
			CreatedAt: time.Now(),
		}
		if err := db.Create(rec).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errFullCarRecordDup
			}
			return err
		}
		outRecordId = rec.Id
		outEntry = picked.Name
		outIsPrize = rec.IsPrize == 1

		// ⑤ 最后一票 → 整车结算（确定性 seed=gid，可回放核对）
		if st.RemainingTickets == 0 {
			var prizes []core.FullCarSettlementPrize
			if err := json.Unmarshal([]byte(cfg.SettlementPrizesJson), &prizes); err != nil || len(prizes) == 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=满车整车结算奖配置无效", errorx.CodeGameConfigInvalid))
			}
			awards := core.SettleFullCar(seats, prizes, []byte(in.RequestId))
			stored := fullCarAwardsToStored(awards, in.RequestId)
			newAwards := append(fullCarParseAwards(st.AwardsJson), stored...)
			if err := db.Model(&gamemodelnc.FullCarState{}).Where("id = ?", st.Id).
				Updates(map[string]interface{}{
					"awards_json": fullCarAwardsJSON(newAwards),
					"status":      fullCarStateDeparted,
					"updated_at":  time.Now(),
				}).Error; err != nil {
				return err
			}
			outDeparted = true
			l.Infof("SettleFullCar departed, requestId: %s, awards: %d", in.RequestId, len(stored))
		}
		return nil
	})
	if errors.Is(err, errFullCarRecordDup) {
		// 同 requestId 已结算（另一 gid 已成功）：返回已有记录，幂等成功
		return l.readBackRecord(in.RequestId, in.UserId)
	}
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleFullCar aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleFullCar failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（分支已成功/重试）：读库如实返回已有记录
		return l.readBackRecord(in.RequestId, in.UserId)
	}
	l.Infof("SettleFullCar success, requestId: %s, recordId: %d, departed: %v", in.RequestId, outRecordId, outDeparted)
	return &play.SettleFullCarResp{
		RecordId:  outRecordId,
		EntryName: outEntry,
		IsPrize:   outIsPrize,
		Departed:  outDeparted,
		Status:    "SETTLED",
	}, nil
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

func NewSettleFullCarRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleFullCarRevertLogic {
	return &SettleFullCarRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleFullCarRevert SAGA 05 补偿：只撤销本分支造成的影响（dig 先例——唯一回滚点防双扣）：
//
//	① 记录 → 2 已撤销（无记录=05 正向从未成功，跳过）；
//	② regular 奖库存回补（按记录的 entry_name 定位，quantity=-1 不限量跳过）；
//	③ awards_json 按 requestId 删除本 gid 触发的整车奖；DEPARTED→OPEN。
//
// 座位释放与余票+1 由 03 补偿（ReleaseFullCar）唯一负责，本分支不碰余票。
func (l *SettleFullCarRevertLogic) SettleFullCarRevert(in *play.SettleFullCarReq) (*play.SettleFullCarResp, error) {
	if in.RequestId == "" || in.UserId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	var outRecordId int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleFullCarRevert", func(db *gorm.DB) error {

		// ① 记录 FOR UPDATE → 2 已撤销（盲更新，幂等；无记录=正向从未成功，跳过）
		var rec gamemodelnc.FullCarRecords
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&rec).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			outRecordId = rec.Id
			if rec.Status == fullCarRecordNormal {
				if err := db.Model(&gamemodelnc.FullCarRecords{}).
					Where("id = ? AND status = ?", rec.Id, fullCarRecordNormal).
					Update("status", fullCarRecordCanceled).Error; err != nil {
					return err
				}
			}
			// ② regular 奖库存回补（按记录条目名定位；-1 不限量不回补）
			if err := db.Model(&gamemodel.FullCarEntries{}).
				Where("config_id = ? AND name = ? AND quantity != -1", rec.ConfigId, rec.EntryName).
				Update("remaining", gorm.Expr("remaining + 1")).Error; err != nil {
				return err
			}
		}

		// ③ awards 按 requestId 删除 + DEPARTED→OPEN
		var st gamemodelnc.FullCarState
		err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			all := fullCarParseAwards(st.AwardsJson)
			kept := make([]fullCarStoredAward, 0, len(all))
			removed := 0
			for _, a := range all {
				if a.RequestId == in.RequestId {
					removed++
					continue
				}
				kept = append(kept, a)
			}
			updates := map[string]interface{}{"updated_at": time.Now()}
			if removed > 0 {
				updates["awards_json"] = fullCarAwardsJSON(kept)
			}
			if st.Status == fullCarStateDeparted {
				updates["status"] = fullCarStateOpen
			}
			if len(updates) > 1 {
				if err := db.Model(&gamemodelnc.FullCarState{}).Where("id = ?", st.Id).
					Updates(updates).Error; err != nil {
					return err
				}
			}
			if removed > 0 {
				l.Infof("SettleFullCarRevert awards removed, requestId: %s, removed: %d", in.RequestId, removed)
			}
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleFullCarRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleFullCarRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读记录如实返回
		var rec gamemodelnc.FullCarRecords
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&rec).Error; err == nil {
			outRecordId = rec.Id
		}
	}
	l.Infof("SettleFullCarRevert success, requestId: %s, recordId: %d", in.RequestId, outRecordId)
	return &play.SettleFullCarResp{RecordId: outRecordId, Status: "REVERTED"}, nil
}

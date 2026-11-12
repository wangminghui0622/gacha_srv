package gameimpl

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

// ══════════════ M13 连击链（COMBO_CHAIN）公共操作 ═══════════════
// 状态 JSON 解析与落库助手（participants/awards/tier_remaining/streaks_before）。
// 回滚职责划分（dig/fullcar/gashapon 先例——唯一回滚点防双扣）：
//   - 03 补偿（ReleaseComboChain）唯一负责弹出当前车参与者 + 余票+1；
//   - 05 补偿（SettleComboChainRevert）只撤销自己造成的影响：记录撤销、
//     按 requestId 删结算奖、档位库存回补、连击快照回滚、车次回退与
//     参与者重建——不碰余票（03 已回补，防双扣）。

// 连击链状态（combo_chain_state.status）
const (
	comboChainStateOpen    int64 = 1
	comboChainStateSettled int64 = 2
)

// 连击链记录状态（combo_chain_records.status）
const (
	comboChainRecordNormal   int64 = 1
	comboChainRecordCanceled int64 = 2
)

// comboChainStateJSON state 行四段 JSON 内存视图
type comboChainStateJSON struct {
	Participants  []int64                         `json:"participants"`
	Awards        []core.ComboChainAward          `json:"awards"`
	TierRemaining []int32                         `json:"tier_remaining"`
	StreaksBefore map[int64]core.ComboChainStreak `json:"streaks_before"`
}

// comboChainParseState state 行四段 JSON → 内存状态（空串取零值；坏 JSON → 错误）
func comboChainParseState(participantsJson, awardsJson, tierRemainingJson, streaksBeforeJson string) (*comboChainStateJSON, error) {
	state := &comboChainStateJSON{}
	if participantsJson != "" {
		if err := json.Unmarshal([]byte(participantsJson), &state.Participants); err != nil {
			return nil, err
		}
	}
	if state.Participants == nil {
		state.Participants = []int64{}
	}
	if awardsJson != "" {
		if err := json.Unmarshal([]byte(awardsJson), &state.Awards); err != nil {
			return nil, err
		}
	}
	if state.Awards == nil {
		state.Awards = []core.ComboChainAward{}
	}
	if tierRemainingJson != "" {
		if err := json.Unmarshal([]byte(tierRemainingJson), &state.TierRemaining); err != nil {
			return nil, err
		}
	}
	if state.TierRemaining == nil {
		state.TierRemaining = []int32{}
	}
	if streaksBeforeJson != "" {
		if err := json.Unmarshal([]byte(streaksBeforeJson), &state.StreaksBefore); err != nil {
			return nil, err
		}
	}
	if state.StreaksBefore == nil {
		state.StreaksBefore = map[int64]core.ComboChainStreak{}
	}
	return state, nil
}

// comboChainParseSlots global_slots_json → 全局奖槽（空/坏 JSON → 错误，调用方报 3020）
func comboChainParseSlots(jsonStr string) ([]core.ComboChainGlobalSlot, error) {
	if jsonStr == "" {
		return nil, errors.New("empty global slots json")
	}
	var slots []core.ComboChainGlobalSlot
	if err := json.Unmarshal([]byte(jsonStr), &slots); err != nil {
		return nil, err
	}
	return slots, nil
}

// comboChainParseShared shared_prizes_json → 共享奖配置（空/坏 JSON → 错误，调用方报 3020）
func comboChainParseShared(jsonStr string) ([]core.ComboChainSharedPrize, error) {
	if jsonStr == "" {
		return nil, errors.New("empty shared prizes json")
	}
	var prizes []core.ComboChainSharedPrize
	if err := json.Unmarshal([]byte(jsonStr), &prizes); err != nil {
		return nil, err
	}
	return prizes, nil
}

// comboChainParticipantsJSON 参与者列表 → JSON（[]int64 恒可编码）
func comboChainParticipantsJSON(participants []int64) string {
	b, _ := json.Marshal(participants)
	return string(b)
}

// comboChainAwardsJSON 结算奖列表 → JSON（[]struct 恒可编码）
func comboChainAwardsJSON(awards []core.ComboChainAward) string {
	b, _ := json.Marshal(awards)
	return string(b)
}

// comboChainTierRemainingJSON 档位库存 → JSON（[]int32 恒可编码）
func comboChainTierRemainingJSON(remaining []int32) string {
	b, _ := json.Marshal(remaining)
	return string(b)
}

// comboChainStreaksBeforeJSON 连击快照 → JSON（map 恒可编码）
func comboChainStreaksBeforeJSON(streaks map[int64]core.ComboChainStreak) string {
	b, _ := json.Marshal(streaks)
	return string(b)
}

// comboChainAwardsOfRequest 取指定 requestId 归属的结算奖（06 发放本 gid 触发的结算奖）
func comboChainAwardsOfRequest(awards []core.ComboChainAward, requestId string) []core.ComboChainAward {
	out := make([]core.ComboChainAward, 0, len(awards))
	for _, a := range awards {
		if a.RequestID == requestId {
			out = append(out, a)
		}
	}
	return out
}

// loadComboChainStreaks 事务内读连击计数：userId → 连击值
func loadComboChainStreaks(db *gorm.DB, configId int64) (map[int64]core.ComboChainStreak, error) {
	var rows []gamemodelnc.ComboChainStreaks
	if err := db.Where("config_id = ?", configId).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[int64]core.ComboChainStreak, len(rows))
	for _, r := range rows {
		out[r.UserId] = core.ComboChainStreak{EmptyStreak: int32(r.EmptyStreak), WinStreak: int32(r.WinStreak)}
	}
	return out, nil
}

// upsertComboChainStreak 连击计数落库（存在则更新，缺失则插入；快照回滚与结算推进共用）
func upsertComboChainStreak(db *gorm.DB, configId, userId int64, s core.ComboChainStreak) error {
	var row gamemodelnc.ComboChainStreaks
	err := db.Where("config_id = ? AND user_id = ?", configId, userId).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&gamemodelnc.ComboChainStreaks{
			ConfigId:    configId,
			UserId:      userId,
			EmptyStreak: int64(s.EmptyStreak),
			WinStreak:   int64(s.WinStreak),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}).Error
	}
	if err != nil {
		return err
	}
	return db.Model(&gamemodelnc.ComboChainStreaks{}).Where("id = ?", row.Id).
		Updates(map[string]interface{}{
			"empty_streak": s.EmptyStreak,
			"win_streak":   s.WinStreak,
			"updated_at":   time.Now(),
		}).Error
}

// prizeShapeItem 资产形状 → PrizeItem（现金金额转元；券名尽力解析，展示用）
func prizeShapeItem(ctx context.Context, svcCtx *svc.ServiceContext, shape core.PrizeShape) *play.PrizeItem {
	item := &play.PrizeItem{Kind: shape.PrizeKind, Points: shape.PointsAmount, CouponTemplateNo: shape.CouponTemplateNo}
	switch shape.PrizeKind {
	case prizeCash:
		item.Amount = utils.FenToYuan(shape.CashFen)
	case prizeCoupon:
		item.CouponName = couponNameOf(ctx, svcCtx, shape.CouponTemplateNo)
	}
	return item
}

type GetComboChainLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetComboChainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetComboChainLogic {
	return &GetComboChainLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetComboChain 连击链详情视图：配置 + 当前车进度 + 全局奖槽 + 共享奖（含档位剩余）
// + 我的连击计数。状态行缺失时按种子语义回退（车 1/余票 tickets_per_car/OPEN）。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *GetComboChainLogic) GetComboChain(in *play.GetComboChainReq) (*play.GetComboChainResp, error) {
	if in.ConfigId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	var cfg gamemodel.ComboChainConfigs
	if err := l.svcCtx.DB.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "连击链配置不存在")
		}
		l.Errorf("GetComboChain query config failed, err: %v", err)
		return nil, err
	}

	// 状态行（缺失回退种子语义：车 1、余票=tickets_per_car、OPEN）
	carIndex, remaining, status := int64(1), cfg.TicketsPerCar, comboChainStateOpen
	participants := int64(0)
	var tierRemaining []int32
	var st gamemodelnc.ComboChainState
	if err := l.svcCtx.DB.Where("config_id = ?", in.ConfigId).First(&st).Error; err == nil {
		carIndex, remaining, status = st.CarIndex, st.RemainingTickets, st.Status
		if state, perr := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson); perr == nil {
			participants = int64(len(state.Participants))
			tierRemaining = state.TierRemaining
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Errorf("GetComboChain query state failed, err: %v", err)
		return nil, err
	}

	// 全局奖槽（配置 JSON 无效 → 3020）
	slots, err := comboChainParseSlots(cfg.GlobalSlotsJson)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "连击链全局奖槽配置无效")
	}
	globalItems := make([]*play.PrizeItem, 0, len(slots))
	for _, slot := range slots {
		globalItems = append(globalItems, prizeShapeItem(l.ctx, l.svcCtx, slot.PrizeShape))
	}

	// 共享奖（含档位剩余；连击奖无限量剩余=-1）
	shared, err := comboChainParseShared(cfg.SharedPrizesJson)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "连击链共享奖配置无效")
	}
	sharedItems := make([]*play.ComboChainSharedPrizeItem, 0, len(shared))
	var comboIndex int32
	for _, p := range shared {
		item := &play.ComboChainSharedPrizeItem{
			Kind:                 p.Kind,
			Quantity:             int64(p.Quantity),
			Remaining:            -1,
			TierWeight:           int64(p.TierWeight),
			ConcentrateThreshold: int64(p.ConcentrateThreshold),
			StreakThreshold:      int64(p.StreakThreshold),
			Prize:                prizeShapeItem(l.ctx, l.svcCtx, p.PrizeShape),
		}
		if p.Kind == core.ComboSharedKindComboTier {
			if int(comboIndex) < len(tierRemaining) {
				item.Remaining = int64(tierRemaining[comboIndex])
			} else {
				item.Remaining = int64(p.Quantity)
			}
			comboIndex++
		}
		sharedItems = append(sharedItems, item)
	}

	// 我的连击计数
	var emptyStreak, winStreak int64
	if in.UserId > 0 {
		var row gamemodelnc.ComboChainStreaks
		if err := l.svcCtx.DB.Where("config_id = ? AND user_id = ?", in.ConfigId, in.UserId).
			First(&row).Error; err == nil {
			emptyStreak, winStreak = row.EmptyStreak, row.WinStreak
		}
	}

	return &play.GetComboChainResp{
		ConfigId:         cfg.Id,
		Name:             cfg.Name,
		Price:            utils.FenToYuan(cfg.PriceFen),
		TicketsPerCar:    cfg.TicketsPerCar,
		TotalCars:        cfg.TotalCars,
		CarIndex:         carIndex,
		Participants:     participants,
		RemainingTickets: remaining,
		Status:           status,
		GlobalSlots:      globalItems,
		SharedPrizes:     sharedItems,
		EmptyStreak:      emptyStreak,
		WinStreak:        winStreak,
	}, nil
}

type GrantComboChainPrizesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantComboChainPrizesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantComboChainPrizesLogic {
	return &GrantComboChainPrizesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantComboChainPrizes SAGA 06 正向：发放本 gid 触发的结算奖（随 05 落 awards_json，
// 按 requestId 精确定位；非结算票时为空集、直接成功）。
// 逐奖幂等发放（grant_no = "ccp_"+gid+"#"+idx，确定性派生）：
//   - 现金 → user.CreditWallet（kind=1，requestId=grant_no 幂等）
//   - 积分 → user.CreditWallet（kind=4，requestId=grant_no 幂等）
//   - 券   → 本地插入用户券（coupon_no="uc_"+grant_no，uk 撞键=已发过）
//
// 铁律：**永不 Aborted**（奖品先于捕获，撤单不回退奖品，与 dig GrantDigPrizes 对齐）；
// 失败只回 Internal → DTM 退避重试，直到发放收敛。
func (l *GrantComboChainPrizesLogic) GrantComboChainPrizes(in *play.GrantComboChainPrizesReq) (*play.GrantComboChainPrizesResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 {
		return nil, status.Error(codes.Internal, "code=400 msg=请求参数非法")
	}

	// 结算奖由 05 正向落库（06 在其后执行）；缺失=异常，Internal 重试
	var st gamemodelnc.ComboChainState
	if err := l.svcCtx.DB.Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("GrantComboChainPrizes state not found, configId: %d", in.ConfigId)
			return nil, status.Error(codes.Internal, fmt.Sprintf("code=%d msg=连击链状态不存在", errorx.CodeGameConfigInvalid))
		}
		l.Errorf("GrantComboChainPrizes query state failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	state, err := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson)
	if err != nil {
		l.Errorf("GrantComboChainPrizes parse state failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	awards := comboChainAwardsOfRequest(state.Awards, in.RequestId)
	prizes := make([]playkit.Prize, 0, len(awards))
	for _, a := range awards {
		prizes = append(prizes, playkit.Prize{
			UserID:           a.UserID,
			Kind:             a.PrizeKind,
			CashFen:          a.CashFen,
			Points:           a.PointsAmount,
			CouponTemplateNo: a.CouponTemplateNo,
		})
	}
	// grant_no = ccp_{gid}#{idx}（GrantAll 内派生）：DTM 重试/补偿重放命中同一键，
	// CreditWallet requestId 幂等 + 券 uk 撞键 = 已发过读回成功。
	if err := playkit.GrantAll(l.ctx, l.svcCtx, in.RequestId, "ccp", prizes); err != nil {
		// 永不 Aborted：失败 Internal → DTM 重试，grant_no 幂等收敛
		l.Errorf("GrantComboChainPrizes grant failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	// 高1 修复：发奖落库标记（04 补偿退款前置检查依据）；Internal 失败 → DTM 重试 06 幂等收敛
	if len(prizes) > 0 {
		if err := playkit.MarkGranted(l.ctx, l.svcCtx, in.RequestId, in.UserId); err != nil {
			l.Errorf("GrantComboChainPrizes mark failed, requestId: %s, err: %v", in.RequestId, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	l.Infof("GrantComboChainPrizes success, requestId: %s, awards: %d", in.RequestId, len(awards))
	return &play.GrantComboChainPrizesResp{Status: "GRANTED"}, nil
}

type GrantComboChainPrizesRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantComboChainPrizesRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantComboChainPrizesRevertLogic {
	return &GrantComboChainPrizesRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantComboChainPrizesRevert SAGA 06 补偿：空操作（结算奖先于捕获且不追回，
// 与 dig 同款决策；补偿注册仅为防御，即使调用也幂等返回）
func (l *GrantComboChainPrizesRevertLogic) GrantComboChainPrizesRevert(in *play.GrantComboChainPrizesReq) (*play.GrantComboChainPrizesResp, error) {
	l.Infof("GrantComboChainPrizesRevert noop, requestId: %s", in.RequestId)
	return &play.GrantComboChainPrizesResp{Status: "GRANTED"}, nil
}

type ListComboChainRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListComboChainRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListComboChainRecordsLogic {
	return &ListComboChainRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListComboChainRecords 连击链记录：requestId 非空=精确查一条（购买轮询），空=分页列表。
// 每条记录附本单触发的结算奖（state.awards_json 按 requestId 过滤，展示/轮询揭晓）。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *ListComboChainRecordsLogic) ListComboChainRecords(in *play.ListComboChainRecordsReq) (*play.ListComboChainRecordsResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}

	// 精确查询（轮询开奖结果）
	if in.RequestId != "" {
		var rec gamemodelnc.ComboChainRecords
		if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", in.RequestId, in.UserId).
			First(&rec).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// requestId 未命中是正常情况：返回空列表，避免每笔新购买打 error 日志。
				return &play.ListComboChainRecordsResp{Items: []*play.ComboChainRecordItem{}, Total: 0}, nil
			}
			l.Errorf("ListComboChainRecords query failed, err: %v", err)
			return nil, err
		}
		return &play.ListComboChainRecordsResp{Items: []*play.ComboChainRecordItem{l.buildItem(&rec)}, Total: 1}, nil
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
	if err := l.svcCtx.DB.Model(&gamemodelnc.ComboChainRecords{}).
		Where("user_id = ?", in.UserId).Count(&total).Error; err != nil {
		l.Errorf("ListComboChainRecords count failed, err: %v", err)
		return nil, err
	}
	var recs []gamemodelnc.ComboChainRecords
	if err := l.svcCtx.DB.Where("user_id = ?", in.UserId).
		Order("id DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).
		Find(&recs).Error; err != nil {
		l.Errorf("ListComboChainRecords page failed, err: %v", err)
		return nil, err
	}
	items := make([]*play.ComboChainRecordItem, 0, len(recs))
	for i := range recs {
		items = append(items, l.buildItem(&recs[i]))
	}
	return &play.ListComboChainRecordsResp{Items: items, Total: total}, nil
}

// buildItem 记录行 → 响应条目（金额转元；结算奖按 requestId 过滤自 state.awards_json）
func (l *ListComboChainRecordsLogic) buildItem(rec *gamemodelnc.ComboChainRecords) *play.ComboChainRecordItem {
	item := &play.ComboChainRecordItem{
		RecordId:  rec.Id,
		RequestId: rec.RequestId,
		ConfigId:  rec.ConfigId,
		CarIndex:  rec.CarIndex,
		Price:     utils.FenToYuan(rec.PriceFen),
		Status:    rec.Status,
		CreatedAt: rec.CreatedAt.Unix(),
	}
	var st gamemodelnc.ComboChainState
	if err := l.svcCtx.DB.Where("config_id = ?", rec.ConfigId).First(&st).Error; err == nil {
		if state, err := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson); err == nil {
			for _, a := range comboChainAwardsOfRequest(state.Awards, rec.RequestId) {
				item.Awards = append(item.Awards, prizeShapeItem(l.ctx, l.svcCtx, a.PrizeShape))
			}
		}
	}
	return item
}

type LockComboChainLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLockComboChainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LockComboChainLogic {
	return &LockComboChainLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LockComboChain SAGA 03 正向：barrier 事务内
//
//	① 报价只读校验（营销 GetQuote sub-call：userId/grossFen 取报价行）；
//	② state FOR UPDATE（uk config_id 串行化同机购买；种子缺失防御建行，
//	   档位库存=配置 COMBO_TIER quantity、当前车 1、余票=tickets_per_car）；
//	③ 状态校验（非 OPEN → 3029；当前车重复购买 → 3030；余票<1 → 3029）；
//	④ 价格一致（3018 复用：quote.gross == price_fen == 请求总额）；
//	⑤ 报价锁（营销 LockQuote sub-call，置于全部玩法校验之后）；
//	⑥ participants 追加、余票-1。
//
// 报价锁放在最后：玩法自身校验失败回滚时不触碰营销锁（营销事务已独立提交，
// 无法随本事务回滚），与旧仓「锁与业务同事务」的最终状态一致——自中止后报价保持 OPEN。
// userId 取报价行（网关 QuoteCheckout 落库），不信请求参数。
// 错误语义（SAGA 分支铁律）：业务失败 → codes.Aborted（触发补偿）；系统错误 → codes.Internal（DTM 重试）。
func (l *LockComboChainLogic) LockComboChain(in *play.LockComboChainReq) (*play.LockComboChainResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "LockComboChain", func(db *gorm.DB) error {

		// ① 报价只读校验（营销 GetQuote sub-call；quote 不存在 → Aborted）
		quote, err := playkit.LoadQuote(l.ctx, l.svcCtx, in.QuoteToken)
		if err != nil {
			return err
		}
		userId := quote.UserID

		// 连击链配置（纯种子配置只读）
		var cfg gamemodel.ComboChainConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链配置不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		if cfg.Status != 1 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链配置不可用", errorx.CodeGameConfigInvalid))
		}

		// ② state FOR UPDATE（uk(config_id) 串行化同机并发购买）
		var st gamemodelnc.ComboChainState
		err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 防御：种子缺失时建行（档位库存=配置 quantity、当前车 1、余票=tickets_per_car）
			shared, perr := comboChainParseShared(cfg.SharedPrizesJson)
			if perr != nil {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链共享奖配置无效", errorx.CodeGameConfigInvalid))
			}
			tiers := core.BuildComboTierStock(shared, nil)
			tierRemaining := make([]int32, len(tiers))
			for i := range tiers {
				tierRemaining[i] = tiers[i].Remaining
			}
			st = gamemodelnc.ComboChainState{
				ConfigId:          in.ConfigId,
				CarIndex:          1,
				ParticipantsJson:  "[]",
				AwardsJson:        "[]",
				TierRemainingJson: comboChainTierRemainingJSON(tierRemaining),
				StreaksBeforeJson: "{}",
				RemainingTickets:  cfg.TicketsPerCar,
				Status:            comboChainStateOpen,
				CreatedAt:         time.Now(),
				UpdatedAt:         time.Now(),
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
		if st.Status != comboChainStateOpen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链已结算或状态不满足锁定", errorx.CodeComboChainNoTickets))
		}

		// ③ 当前车校验：每用户每车限 1 票（3030）；余票 ≥1（3029）
		state, err := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链状态数据异常", errorx.CodeGameConfigInvalid))
		}
		for _, pid := range state.Participants {
			if pid == userId {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链当前车重复购买", errorx.CodeComboChainDupBuy))
			}
		}
		if st.RemainingTickets < 1 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链当前车已满或已结算", errorx.CodeComboChainNoTickets))
		}

		// ④ 价格一致（锁内重算）：单价 == 报价 gross == 请求总额
		if quote.GrossFen != cfg.PriceFen || in.TotalPriceFen != cfg.PriceFen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链价格校验不一致", errorx.CodeDigPriceMismatch))
		}

		// ⑤ 报价锁（营销 LockQuote sub-call，置于玩法校验之后；幂等：newLocked=false=已锁重发 → 业务已成功，直接返回）
		newLocked, err := playkit.LockQuote(l.ctx, l.svcCtx, in.RequestId, in.QuoteToken)
		if err != nil {
			return err
		}
		if !newLocked {
			l.Infof("LockComboChain idempotent: quote already locked, quoteToken: %s", in.QuoteToken)
			return nil
		}

		// ⑥ participants 追加、余票-1
		state.Participants = append(state.Participants, userId)
		if err := db.Model(&gamemodelnc.ComboChainState{}).Where("id = ?", st.Id).
			Updates(map[string]interface{}{
				"participants_json": comboChainParticipantsJSON(state.Participants),
				"remaining_tickets": st.RemainingTickets - 1,
				"status":            comboChainStateOpen,
				"updated_at":        time.Now(),
			}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("LockComboChain aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("LockComboChain failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（正向已成功）：读报价如实返回（经营销 RPC；NotFound 保持 LOCKED 旧同款容错）
		statusOut := "LOCKED"
		q, qe := l.svcCtx.MarketingRpc.GetQuote(l.ctx, &marketingClient.GetQuoteReq{QuoteToken: in.QuoteToken})
		if qe == nil {
			statusOut = q.GetStatus()
		} else {
			var ce *errorx.CodeError
			if !errors.As(qe, &ce) || ce.Code != errorx.CodeQuoteNotFound {
				return nil, status.Error(codes.Internal, qe.Error())
			}
		}
		l.Infof("LockComboChain success (readback), quoteToken: %s, status: %s", in.QuoteToken, statusOut)
		return &play.LockComboChainResp{Status: statusOut}, nil
	}
	l.Infof("LockComboChain success, quoteToken: %s", in.QuoteToken)
	return &play.LockComboChainResp{Status: "LOCKED"}, nil
}

type ReleaseComboChainLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReleaseComboChainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReleaseComboChainLogic {
	return &ReleaseComboChainLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ReleaseComboChain SAGA 03 补偿：barrier 事务内
//
//	① 报价 FOR UPDATE 先行——LOCKED ⇔ 本 gid 确实锁定购票（需回滚）；OPEN ⇒ 正向
//	   已整体回滚（如 3029/3030），参与者保持原样——绝不误回滚他人购票；
//	② 按 userId 弹出当前车参与者 + 余票+1（仅 LOCKED 且移除成功时；
//	   05 补偿重建参与者后本用户已不在其中 → 幂等跳过）。
//	   这是参与者与余票的唯一回滚点（05 补偿只撤销记录/结算奖/连击/车次，不碰余票——防双扣）；
//	③ 回退券（LOCKED 且 locked_by=本报价 → AVAILABLE）；
//	④ 报价 LOCKED/OPEN→RELEASED（幂等；USED 不回退）。
//
// 幂等：报价 RELEASED/USED/EXPIRED → 直接成功。
func (l *ReleaseComboChainLogic) ReleaseComboChain(in *play.ReleaseComboChainReq) (*play.ReleaseComboChainResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "releaseComboChain", func(db *gorm.DB) error {

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
			l.Infof("ReleaseComboChain idempotent: quote status %s, quoteToken: %s", q.GetStatus(), in.QuoteToken)
			return nil
		}

		// ② 弹出当前车参与者 + 余票+1：仅当报价 LOCKED（正向提交过锁定）时执行
		if q.GetStatus() == "LOCKED" {
			var st gamemodelnc.ComboChainState
			err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("config_id = ?", in.ConfigId).First(&st).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil {
				state, err := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson)
				if err == nil {
					removed := false
					kept := make([]int64, 0, len(state.Participants))
					for _, pid := range state.Participants {
						if !removed && pid == q.GetUserId() {
							removed = true
							continue
						}
						kept = append(kept, pid)
					}
					if removed {
						if err := db.Model(&gamemodelnc.ComboChainState{}).Where("id = ?", st.Id).
							Updates(map[string]interface{}{
								"participants_json": comboChainParticipantsJSON(kept),
								"remaining_tickets": st.RemainingTickets + 1,
								"updated_at":        time.Now(),
							}).Error; err != nil {
							return err
						}
						l.Infof("ReleaseComboChain participant popped, requestId: %s, userId: %d", in.RequestId, q.GetUserId())
					}
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
			l.Errorf("ReleaseComboChain aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("ReleaseComboChain failed, quoteToken: %s, err: %v", in.QuoteToken, err)
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
		l.Infof("ReleaseComboChain success (readback), quoteToken: %s, status: %s", in.QuoteToken, statusOut)
		return &play.ReleaseComboChainResp{Status: statusOut}, nil
	}
	l.Infof("ReleaseComboChain success, quoteToken: %s", in.QuoteToken)
	return &play.ReleaseComboChainResp{Status: "RELEASED"}, nil
}

// errComboChainRecordDup 事务内哨兵：同 requestId 的连击链记录已存在（另一 gid 已结算）
var errComboChainRecordDup = errors.New("combo chain record duplicated")

type SettleComboChainLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleComboChainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleComboChainLogic {
	return &SettleComboChainLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleComboChain SAGA 05 正向：barrier 事务内
//
//	① state FOR UPDATE（必须 OPEN，种子缺失 → 3020）；
//	② 插记录（uk request_id 撞键=已结算 → 读回幂等成功）；
//	③ 余票==0（最后一票）→ 整车结算：
//	   参与人数/全局奖槽守卫（3031）→ 档位库存构建 → 连击计数读入 →
//	   core.SettleComboCar（seed=gid 确定性）→ 结算奖标 requestId+车号追加、
//	   档位库存/连击推进/快照落库 → 车次推进（最后一车 → SETTLED 余票 0；
//	   否则车号+1、参与者清空、余票重置 tickets_per_car）。
//
// 错误语义：业务失败 → codes.Aborted；系统错误 → codes.Internal。
func (l *SettleComboChainLogic) SettleComboChain(in *play.SettleComboChainReq) (*play.SettleComboChainResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	// 幂等重放快路径：同 requestId 记录已存在 → 读回成功（先于业务校验，M11b 教训）
	var existing gamemodelnc.ComboChainRecords
	if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&existing).Error; err == nil {
		return l.readBackRecord(in.RequestId, in.UserId)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Errorf("SettleComboChain fast path query failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var outRecordId int64
	outAwards := make([]core.ComboChainAward, 0)
	outSettled := false
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleComboChain", func(db *gorm.DB) error {

		// ① state FOR UPDATE（uk config_id 串行化；种子缺失 → 3020）
		var st gamemodelnc.ComboChainState
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链状态不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		if st.Status != comboChainStateOpen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链已结算或状态不满足结算", errorx.CodeComboChainNoTickets))
		}

		// 连击链配置 + 全局奖槽 + 共享奖（配置 JSON 无效 → 3020）
		var cfg gamemodel.ComboChainConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链配置不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		slots, err := comboChainParseSlots(cfg.GlobalSlotsJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链全局奖槽配置无效", errorx.CodeGameConfigInvalid))
		}
		shared, err := comboChainParseShared(cfg.SharedPrizesJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链共享奖配置无效", errorx.CodeGameConfigInvalid))
		}

		// 状态 JSON → 内存
		state, err := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链状态数据异常", errorx.CodeGameConfigInvalid))
		}

		// ② 插记录（uk request_id 幂等；撞键 = 已结算读回）
		priceFen := int64(0)
		if in.Price != "" {
			f, err := utils.YuanToFen(in.Price)
			if err != nil || f < 0 {
				return status.Error(codes.Aborted, "code=400 msg=实付价格无效")
			}
			priceFen = f
		}
		rec := &gamemodelnc.ComboChainRecords{
			RequestId: in.RequestId,
			UserId:    in.UserId,
			ConfigId:  in.ConfigId,
			CarIndex:  st.CarIndex,
			PriceFen:  priceFen,
			Status:    comboChainRecordNormal,
			CreatedAt: time.Now(),
		}
		if err := db.Create(rec).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errComboChainRecordDup
			}
			return err
		}
		outRecordId = rec.Id

		// ③ 最后一票（余票==0）→ 整车结算
		if st.RemainingTickets == 0 {
			// 参与人数与配置守卫（3031 配置变更）：本车参与者必须满员
			if int64(len(state.Participants)) != cfg.TicketsPerCar {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链参与人数与配置不符", errorx.CodeComboChainConfigChanged))
			}
			// 本车全局奖槽（index%totalCars+1==carIndex）；缺失 → 3031
			carGlobals := core.AssignCarGlobals(slots, int(st.CarIndex), int(cfg.TotalCars))
			if len(carGlobals) == 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链当前车无全局奖", errorx.CodeComboChainConfigChanged))
			}
			// 档位库存 + 连击计数
			tiers := core.BuildComboTierStock(shared, state.TierRemaining)
			streaks, err := loadComboChainStreaks(db, in.ConfigId)
			if err != nil {
				return err
			}
			awards, streakNext, tierNext, err := core.SettleComboCar(
				state.Participants, carGlobals, tiers, int32(cfg.TierTriggerProbabilityBps),
				shared, []byte(in.RequestId), streaks)
			if err != nil {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=连击链结算失败", errorx.CodeComboChainConfigChanged))
			}
			// 结算奖标 requestId+车号追加
			for i := range awards {
				awards[i].RequestID = in.RequestId
				awards[i].CarIndex = int32(st.CarIndex)
			}
			outAwards = awards
			// 结算前连击快照（05 补偿回滚依据：只保留参与用户）
			snapshot := make(map[int64]core.ComboChainStreak, len(state.Participants))
			for _, pid := range state.Participants {
				snapshot[pid] = streaks[pid]
			}
			// 连击推进落库
			for userId, s := range streakNext {
				if err := upsertComboChainStreak(db, in.ConfigId, userId, s); err != nil {
					return err
				}
			}
			// 车次推进
			updates := map[string]interface{}{
				"awards_json":         comboChainAwardsJSON(append(state.Awards, awards...)),
				"tier_remaining_json": comboChainTierRemainingJSON(tierNext),
				"streaks_before_json": comboChainStreaksBeforeJSON(snapshot),
				"updated_at":          time.Now(),
			}
			if st.CarIndex >= cfg.TotalCars {
				// 最后一车：整链 SETTLED、余票 0（车号停在 total_cars）
				outSettled = true
				updates["status"] = comboChainStateSettled
				updates["remaining_tickets"] = 0
			} else {
				// 车次推进：参与者清空、余票重置
				updates["car_index"] = st.CarIndex + 1
				updates["participants_json"] = "[]"
				updates["remaining_tickets"] = cfg.TicketsPerCar
			}
			if err := db.Model(&gamemodelnc.ComboChainState{}).Where("id = ?", st.Id).
				Updates(updates).Error; err != nil {
				return err
			}
			l.Infof("SettleComboChain settled, requestId: %s, car: %d, awards: %d",
				in.RequestId, st.CarIndex, len(awards))
		}
		return nil
	})
	if errors.Is(err, errComboChainRecordDup) {
		// 同 requestId 已结算（另一 gid 已成功）：返回已有记录，幂等成功
		return l.readBackRecord(in.RequestId, in.UserId)
	}
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleComboChain aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleComboChain failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（分支已成功/重试）：读库如实返回已有记录
		return l.readBackRecord(in.RequestId, in.UserId)
	}
	l.Infof("SettleComboChain success, requestId: %s, recordId: %d, settled: %v, awards: %d",
		in.RequestId, outRecordId, outSettled, len(outAwards))
	return &play.SettleComboChainResp{
		RecordId: outRecordId,
		Awards:   l.comboChainAwardItems(outAwards),
		Settled:  outSettled,
		Status:   "SETTLED",
	}, nil
}

// comboChainAwardItems 结算奖 → PrizeItem 列表（券名尽力解析，展示用）
func (l *SettleComboChainLogic) comboChainAwardItems(awards []core.ComboChainAward) []*play.PrizeItem {
	items := make([]*play.PrizeItem, 0, len(awards))
	for _, a := range awards {
		items = append(items, prizeShapeItem(l.ctx, l.svcCtx, a.PrizeShape))
	}
	return items
}

// readBackRecord 幂等/屏障跳过路径：读已有记录 + 本 gid 结算奖 + 当前状态如实返回
func (l *SettleComboChainLogic) readBackRecord(requestId string, userId int64) (*play.SettleComboChainResp, error) {
	var rec gamemodelnc.ComboChainRecords
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", requestId, userId).
		First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("readBackRecord: record not found, requestId: %s", requestId)
			return nil, status.Error(codes.Internal, "连击链记录不存在")
		}
		l.Errorf("readBackRecord failed, requestId: %s, err: %v", requestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	settled := false
	var awards []core.ComboChainAward
	var st gamemodelnc.ComboChainState
	if err := l.svcCtx.DB.Where("config_id = ?", rec.ConfigId).First(&st).Error; err == nil {
		settled = st.Status == comboChainStateSettled
		if state, err := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson); err == nil {
			awards = comboChainAwardsOfRequest(state.Awards, requestId)
		}
	}
	l.Infof("readBackRecord: requestId: %s, recordId: %d, settled: %v, awards: %d", requestId, rec.Id, settled, len(awards))
	return &play.SettleComboChainResp{
		RecordId: rec.Id,
		Awards:   l.comboChainAwardItems(awards),
		Settled:  settled,
		Status:   "SETTLED",
	}, nil
}

type SettleComboChainRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleComboChainRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleComboChainRevertLogic {
	return &SettleComboChainRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleComboChainRevert SAGA 05 补偿：只撤销本分支造成的影响（dig/fullcar/gashapon 先例——唯一回滚点防双扣）：
//
//	① 记录 → 2 已撤销（无记录=05 正向从未成功，跳过）；
//	② 若本 gid 触发过整车结算（awards_json 有本 requestId 的结算奖）：
//	   按 requestId 删结算奖；COMBO_TIER 档位库存回补（combo_index++）；
//	   连击计数按 streaks_before_json 快照回滚；车次回退——car_index=记录车号、
//	   参与者由该车 status=1 记录重建（本 gid 记录已置 2 故自动排除）、
//	   余票=tickets_per_car-重建人数、状态 OPEN。
//
// 参与者弹出与余票回补由 03 补偿（ReleaseComboChain）唯一负责，本分支不碰余票
// 的普通回补（重建路径例外：整车结算回滚必须复原整辆车）。
func (l *SettleComboChainRevertLogic) SettleComboChainRevert(in *play.SettleComboChainReq) (*play.SettleComboChainResp, error) {
	if in.RequestId == "" || in.UserId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	var outRecordId int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleComboChainRevert", func(db *gorm.DB) error {

		// ① 记录 FOR UPDATE（盲撤销幂等；无记录=正向从未成功，跳过）
		var rec gamemodelnc.ComboChainRecords
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&rec).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			outRecordId = rec.Id
			if rec.Status == comboChainRecordNormal {
				if err := db.Model(&gamemodelnc.ComboChainRecords{}).
					Where("id = ? AND status = ?", rec.Id, comboChainRecordNormal).
					Update("status", comboChainRecordCanceled).Error; err != nil {
					return err
				}
				// ② 整车结算回滚（本 gid 触发过结算才有 awards；仅状态撤销幂等跳过）
				if err := l.revertSettlement(db, &rec); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleComboChainRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleComboChainRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读记录如实返回
		var rec gamemodelnc.ComboChainRecords
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&rec).Error; err == nil {
			outRecordId = rec.Id
		}
	}
	l.Infof("SettleComboChainRevert success, requestId: %s, recordId: %d", in.RequestId, outRecordId)
	return &play.SettleComboChainResp{RecordId: outRecordId, Status: "REVERTED"}, nil
}

// revertSettlement 整车结算回滚：删结算奖、档位回补、连击快照回滚、车次回退
func (l *SettleComboChainRevertLogic) revertSettlement(db *gorm.DB, rec *gamemodelnc.ComboChainRecords) error {
	var st gamemodelnc.ComboChainState
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("config_id = ?", rec.ConfigId).First(&st).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err != nil {
		return nil
	}
	state, perr := comboChainParseState(st.ParticipantsJson, st.AwardsJson, st.TierRemainingJson, st.StreaksBeforeJson)
	if perr != nil {
		return nil
	}
	// 按 requestId 删结算奖 + 档位库存回补
	kept := make([]core.ComboChainAward, 0, len(state.Awards))
	removed := 0
	for _, a := range state.Awards {
		if a.RequestID != rec.RequestId {
			kept = append(kept, a)
			continue
		}
		removed++
		if a.Kind == core.ComboAwardKindCombo {
			if idx := int(a.ComboIndex); idx >= 0 && idx < len(state.TierRemaining) {
				state.TierRemaining[idx]++
			}
		}
	}
	if removed == 0 {
		// 非结算票：无结算影响，只撤销记录（03 补偿负责弹出参与者与余票回补）
		return nil
	}
	// 连击计数按结算前快照回滚
	for userId, s := range state.StreaksBefore {
		if err := upsertComboChainStreak(db, rec.ConfigId, userId, s); err != nil {
			return err
		}
	}
	// 车次回退：参与者由该车 status=1 记录重建（本 gid 已置 2 自动排除）
	var recs []gamemodelnc.ComboChainRecords
	if err := db.Where("config_id = ? AND car_index = ? AND status = ?", rec.ConfigId, rec.CarIndex, comboChainRecordNormal).
		Order("id ASC").Find(&recs).Error; err != nil {
		return err
	}
	participants := make([]int64, 0, len(recs))
	for _, r := range recs {
		participants = append(participants, r.UserId)
	}
	updates := map[string]interface{}{
		"awards_json":         comboChainAwardsJSON(kept),
		"tier_remaining_json": comboChainTierRemainingJSON(state.TierRemaining),
		"car_index":           rec.CarIndex,
		"participants_json":   comboChainParticipantsJSON(participants),
		"status":              comboChainStateOpen,
		"updated_at":          time.Now(),
	}
	// 余票复原 = tickets_per_car - 重建人数（配置缺失时保持原值防御）
	var cfg gamemodel.ComboChainConfigs
	if err := db.Where("id = ?", rec.ConfigId).First(&cfg).Error; err == nil {
		updates["remaining_tickets"] = cfg.TicketsPerCar - int64(len(participants))
	}
	if err := db.Model(&gamemodelnc.ComboChainState{}).Where("id = ?", st.Id).
		Updates(updates).Error; err != nil {
		return err
	}
	l.Infof("revertSettlement: requestId: %s, removed: %d, participants rebuilt: %d",
		rec.RequestId, removed, len(participants))
	return nil
}

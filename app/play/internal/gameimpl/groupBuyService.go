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

type GetGroupBuyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGroupBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGroupBuyLogic {
	return &GetGroupBuyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetGroupBuy 拼团详情视图：配置 + 余位 + 我的下注数 + 参与人数（去重）
// + 幸运奖（含已发进度）。状态行缺失时按种子语义回退（余位=target_bet_count/OPEN）。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *GetGroupBuyLogic) GetGroupBuy(in *play.GetGroupBuyReq) (*play.GetGroupBuyResp, error) {
	if in.ConfigId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	var cfg gamemodel.GroupBuyConfigs
	if err := l.svcCtx.DB.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "拼团配置不存在")
		}
		l.Errorf("GetGroupBuy query config failed, err: %v", err)
		return nil, err
	}

	// 幸运奖配置（含已发进度）
	luckyCfg, err := groupBuyParseLucky(cfg.LuckyPrizesJson)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "拼团幸运奖配置无效")
	}

	// 状态行（缺失回退种子语义：余位=target_bet_count、OPEN）
	remaining, status := cfg.TargetBetCount, groupBuyStateOpen
	myBets, participants := int64(0), int64(0)
	sent := make([]int32, len(luckyCfg))
	var st gamemodelnc.GroupBuyState
	if err := l.svcCtx.DB.Where("config_id = ?", in.ConfigId).First(&st).Error; err == nil {
		remaining, status = st.RemainingBets, st.Status
		if state, perr := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson); perr == nil {
			if len(state.LuckySent) == len(luckyCfg) {
				sent = state.LuckySent
			}
			seen := map[int64]bool{}
			for _, b := range state.Bets {
				seen[b.UserID] = true
				if b.UserID == in.UserId {
					myBets += int64(b.Count)
				}
			}
			participants = int64(len(seen))
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Errorf("GetGroupBuy query state failed, err: %v", err)
		return nil, err
	}

	// 幸运奖视图（含进度）
	luckyItems := make([]*play.GroupBuyLuckyPrizeItem, 0, len(luckyCfg))
	for _, stock := range luckyCfg {
		sentCount := int64(0)
		if int(stock.Index) < len(sent) {
			sentCount = int64(sent[stock.Index])
		}
		luckyItems = append(luckyItems, &play.GroupBuyLuckyPrizeItem{
			PrizeIndex: int64(stock.Index),
			Quantity:   int64(stock.Remaining),
			Sent:       sentCount,
			Prize:      prizeShapeItem(l.ctx, l.svcCtx, stock.PrizeShape),
		})
	}

	return &play.GetGroupBuyResp{
		ConfigId:         cfg.Id,
		Name:             cfg.Name,
		Price:            utils.FenToYuan(cfg.PriceFen),
		TargetBetCount:   cfg.TargetBetCount,
		PerBetAwardLimit: cfg.PerBetAwardLimit,
		RemainingBets:    remaining,
		MyBets:           myBets,
		Participants:     participants,
		Status:           status,
		LuckyPrizes:      luckyItems,
	}, nil
}

type GrantGroupBuyPrizesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantGroupBuyPrizesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantGroupBuyPrizesLogic {
	return &GrantGroupBuyPrizesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantGroupBuyPrizes SAGA 06 正向：发放本 gid 触发的结算奖（随 05 落 awards_json，
// 按 requestId 精确定位；非成团注时为空集、直接成功）。
// 逐奖幂等发放（grant_no = "gbp_"+gid+"#"+idx，确定性派生）：
//   - 现金 → user.CreditWallet（kind=1，requestId=grant_no 幂等）
//   - 积分 → user.CreditWallet（kind=4，requestId=grant_no 幂等）
//   - 券   → 本地插入用户券（coupon_no="uc_"+grant_no，uk 撞键=已发过）
//
// 铁律：**永不 Aborted**（奖品先于捕获，撤单不回退奖品，与 dig GrantDigPrizes 对齐）；
// 失败只回 Internal → DTM 退避重试，直到发放收敛。
func (l *GrantGroupBuyPrizesLogic) GrantGroupBuyPrizes(in *play.GrantGroupBuyPrizesReq) (*play.GrantGroupBuyPrizesResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 {
		return nil, status.Error(codes.Internal, "code=400 msg=请求参数非法")
	}

	// 结算奖由 05 正向落库（06 在其后执行）；缺失=异常，Internal 重试
	var st gamemodelnc.GroupBuyState
	if err := l.svcCtx.DB.Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("GrantGroupBuyPrizes state not found, configId: %d", in.ConfigId)
			return nil, status.Error(codes.Internal, fmt.Sprintf("code=%d msg=拼团状态不存在", errorx.CodeGameConfigInvalid))
		}
		l.Errorf("GrantGroupBuyPrizes query state failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	state, err := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson)
	if err != nil {
		l.Errorf("GrantGroupBuyPrizes parse state failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	awards := groupBuyAwardsOfRequest(state.Awards, in.RequestId)
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
	// grant_no = gbp_{gid}#{idx}（GrantAll 内派生）：DTM 重试/补偿重放命中同一键，
	// CreditWallet requestId 幂等 + 券 uk 撞键 = 已发过读回成功。
	if err := playkit.GrantAll(l.ctx, l.svcCtx, in.RequestId, "gbp", prizes); err != nil {
		// 永不 Aborted：失败 Internal → DTM 重试，grant_no 幂等收敛
		l.Errorf("GrantGroupBuyPrizes grant failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	// 高1 修复：发奖落库标记（04 补偿退款前置检查依据）；Internal 失败 → DTM 重试 06 幂等收敛
	if len(prizes) > 0 {
		if err := playkit.MarkGranted(l.ctx, l.svcCtx, in.RequestId, in.UserId); err != nil {
			l.Errorf("GrantGroupBuyPrizes mark failed, requestId: %s, err: %v", in.RequestId, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	l.Infof("GrantGroupBuyPrizes success, requestId: %s, awards: %d", in.RequestId, len(awards))
	return &play.GrantGroupBuyPrizesResp{Status: "GRANTED"}, nil
}

type GrantGroupBuyPrizesRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantGroupBuyPrizesRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantGroupBuyPrizesRevertLogic {
	return &GrantGroupBuyPrizesRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantGroupBuyPrizesRevert SAGA 06 补偿：空操作（结算奖先于捕获且不追回，
// 与 dig 同款决策；补偿注册仅为防御，即使调用也幂等返回）
func (l *GrantGroupBuyPrizesRevertLogic) GrantGroupBuyPrizesRevert(in *play.GrantGroupBuyPrizesReq) (*play.GrantGroupBuyPrizesResp, error) {
	l.Infof("GrantGroupBuyPrizesRevert noop, requestId: %s", in.RequestId)
	return &play.GrantGroupBuyPrizesResp{Status: "GRANTED"}, nil
}

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

type ListGroupBuyRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListGroupBuyRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListGroupBuyRecordsLogic {
	return &ListGroupBuyRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListGroupBuyRecords 拼团记录：requestId 非空=精确查一条（购买轮询），空=分页列表。
// 每条记录附本单触发的结算奖（state.awards_json 按 requestId 过滤，展示/轮询揭晓）。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *ListGroupBuyRecordsLogic) ListGroupBuyRecords(in *play.ListGroupBuyRecordsReq) (*play.ListGroupBuyRecordsResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}

	// 精确查询（轮询开奖结果）
	if in.RequestId != "" {
		var rec gamemodelnc.GroupBuyRecords
		if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", in.RequestId, in.UserId).
			First(&rec).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// requestId 未命中是正常情况：返回空列表，避免每笔新购买打 error 日志。
				return &play.ListGroupBuyRecordsResp{Items: []*play.GroupBuyRecordItem{}, Total: 0}, nil
			}
			l.Errorf("ListGroupBuyRecords query failed, err: %v", err)
			return nil, err
		}
		return &play.ListGroupBuyRecordsResp{Items: []*play.GroupBuyRecordItem{l.buildItem(&rec)}, Total: 1}, nil
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
	if err := l.svcCtx.DB.Model(&gamemodelnc.GroupBuyRecords{}).
		Where("user_id = ?", in.UserId).Count(&total).Error; err != nil {
		l.Errorf("ListGroupBuyRecords count failed, err: %v", err)
		return nil, err
	}
	var recs []gamemodelnc.GroupBuyRecords
	if err := l.svcCtx.DB.Where("user_id = ?", in.UserId).
		Order("id DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).
		Find(&recs).Error; err != nil {
		l.Errorf("ListGroupBuyRecords page failed, err: %v", err)
		return nil, err
	}
	items := make([]*play.GroupBuyRecordItem, 0, len(recs))
	for i := range recs {
		items = append(items, l.buildItem(&recs[i]))
	}
	return &play.ListGroupBuyRecordsResp{Items: items, Total: total}, nil
}

// buildItem 记录行 → 响应条目（金额转元；结算奖按 requestId 过滤自 state.awards_json）
func (l *ListGroupBuyRecordsLogic) buildItem(rec *gamemodelnc.GroupBuyRecords) *play.GroupBuyRecordItem {
	item := &play.GroupBuyRecordItem{
		RecordId:  rec.Id,
		RequestId: rec.RequestId,
		ConfigId:  rec.ConfigId,
		Count:     rec.Count,
		Price:     utils.FenToYuan(rec.PriceFen),
		Status:    rec.Status,
		CreatedAt: rec.CreatedAt.Unix(),
	}
	var st gamemodelnc.GroupBuyState
	if err := l.svcCtx.DB.Where("config_id = ?", rec.ConfigId).First(&st).Error; err == nil {
		if state, err := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson); err == nil {
			for _, a := range groupBuyAwardsOfRequest(state.Awards, rec.RequestId) {
				item.Awards = append(item.Awards, prizeShapeItem(l.ctx, l.svcCtx, a.PrizeShape))
			}
		}
	}
	return item
}

type LockGroupBuyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLockGroupBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LockGroupBuyLogic {
	return &LockGroupBuyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LockGroupBuy SAGA 03 正向：barrier 事务内
//
//	① 报价只读校验（营销 GetQuote sub-call：userId/grossFen 取报价行）；
//	② state FOR UPDATE（uk config_id 串行化同机下注；种子缺失防御建行，
//	   幸运已发=全 0、余位=target_bet_count）；
//	③ 下注校验（count≤0 → 3033；状态非 OPEN / 余位<count → 3032）；
//	④ 价格一致（3018 复用：quote.gross == 单价×count == 请求总额）；
//	⑤ 报价锁（营销 LockQuote sub-call，置于全部玩法校验之后）；
//	⑥ bets 追加 {user_id, draw_id=requestId, count}、余位-=count。
//
// 报价锁放在最后：玩法自身校验失败回滚时不触碰营销锁（营销事务已独立提交，
// 无法随本事务回滚），与旧仓「锁与业务同事务」的最终状态一致——自中止后报价保持 OPEN。
// userId 取报价行（网关 QuoteCheckout 落库），不信请求参数。
// 错误语义（SAGA 分支铁律）：业务失败 → codes.Aborted（触发补偿）；系统错误 → codes.Internal（DTM 重试）。
func (l *LockGroupBuyLogic) LockGroupBuy(in *play.LockGroupBuyReq) (*play.LockGroupBuyResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	outCount := in.Count
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "LockGroupBuy", func(db *gorm.DB) error {

		// ① 报价只读校验（营销 GetQuote sub-call；quote 不存在 → Aborted）
		quote, err := playkit.LoadQuote(l.ctx, l.svcCtx, in.QuoteToken)
		if err != nil {
			return err
		}
		userId := quote.UserID

		// 拼团配置（纯种子配置只读）
		var cfg gamemodel.GroupBuyConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团配置不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		if cfg.Status != 1 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团配置不可用", errorx.CodeGameConfigInvalid))
		}

		// ② state FOR UPDATE（uk(config_id) 串行化同机并发下注）
		var st gamemodelnc.GroupBuyState
		err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 防御：种子缺失时建行（幸运已发=全 0、余位=target_bet_count）
			lucky, perr := groupBuyParseLucky(cfg.LuckyPrizesJson)
			if perr != nil {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团幸运奖配置无效", errorx.CodeGameConfigInvalid))
			}
			sent := make([]int32, len(lucky))
			st = gamemodelnc.GroupBuyState{
				ConfigId:      in.ConfigId,
				BetsJson:      "[]",
				AwardsJson:    "[]",
				LuckySentJson: groupBuyLuckySentJSON(sent),
				RemainingBets: cfg.TargetBetCount,
				Status:        groupBuyStateOpen,
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
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
		if st.Status != groupBuyStateOpen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团已结算或状态不满足锁定", errorx.CodeGroupBuyNoSlots))
		}

		// ③ 下注校验：count>0（3033）；余位 ≥ count（3032）
		if in.Count <= 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团下注数非法", errorx.CodeGroupBuyCountInvalid))
		}
		if st.RemainingBets < in.Count {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团余位不足", errorx.CodeGroupBuyNoSlots))
		}

		// ④ 价格一致（锁内重算）：单价×count == 报价 gross == 请求总额
		if quote.GrossFen != cfg.PriceFen*in.Count || in.TotalPriceFen != cfg.PriceFen*in.Count {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团价格校验不一致", errorx.CodeDigPriceMismatch))
		}

		// ⑤ 报价锁（营销 LockQuote sub-call，置于玩法校验之后；幂等：newLocked=false=已锁重发 → 业务已成功，读下注记录如实返回）
		newLocked, err := playkit.LockQuote(l.ctx, l.svcCtx, in.RequestId, in.QuoteToken)
		if err != nil {
			return err
		}
		if !newLocked {
			l.Infof("LockGroupBuy idempotent: quote already locked, quoteToken: %s", in.QuoteToken)
			outCount = l.countOfRequest(db, in.ConfigId, in.RequestId, in.Count)
			return nil
		}

		// ⑥ bets 追加、余位-=count
		state, err := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团状态数据异常", errorx.CodeGameConfigInvalid))
		}
		state.Bets = append(state.Bets, core.GroupBuyBet{UserID: userId, DrawID: in.RequestId, Count: int32(in.Count)})
		if err := db.Model(&gamemodelnc.GroupBuyState{}).Where("id = ?", st.Id).
			Updates(map[string]interface{}{
				"bets_json":      groupBuyBetsJSON(state.Bets),
				"remaining_bets": st.RemainingBets - in.Count,
				"status":         groupBuyStateOpen,
				"updated_at":     time.Now(),
			}).Error; err != nil {
			return err
		}
		outCount = in.Count
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("LockGroupBuy aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("LockGroupBuy failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（正向已成功）：读库如实返回
		outCount = l.countOfRequest(l.svcCtx.DB, in.ConfigId, in.RequestId, in.Count)
	}
	l.Infof("LockGroupBuy success, quoteToken: %s, count: %d", in.QuoteToken, outCount)
	return &play.LockGroupBuyResp{Count: outCount, Status: "LOCKED"}, nil
}

// countOfRequest 幂等/屏障跳过路径：按 requestId 读下注记录如实返回；未找到 → 回退请求值
func (l *LockGroupBuyLogic) countOfRequest(db *gorm.DB, configId int64, requestId string, fallback int64) int64 {
	var st gamemodelnc.GroupBuyState
	if err := db.Where("config_id = ?", configId).First(&st).Error; err != nil {
		return fallback
	}
	state, err := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson)
	if err != nil {
		return fallback
	}
	for _, b := range state.Bets {
		if b.DrawID == requestId {
			return int64(b.Count)
		}
	}
	return fallback
}

type ReleaseGroupBuyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReleaseGroupBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReleaseGroupBuyLogic {
	return &ReleaseGroupBuyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ReleaseGroupBuy SAGA 03 补偿：barrier 事务内
//
//	① 报价 FOR UPDATE 先行——LOCKED ⇔ 本 gid 确实锁定下注（需回滚）；OPEN ⇒ 正向
//	   已整体回滚（如 3032/3033），下注保持原样——绝不误回滚他人下注；
//	② 按 draw_id（=requestId）弹出下注 + 余位+=count（仅 LOCKED 且弹出成功时；
//	   已弹出则幂等跳过）。这是下注记录与余位的唯一回滚点
//	   （05 补偿只撤销记录/结算奖/库存/幸运，不碰余位——防双扣）；
//	③ 回退券（LOCKED 且 locked_by=本报价 → AVAILABLE）；
//	④ 报价 LOCKED/OPEN→RELEASED（幂等；USED 不回退）。
//
// 幂等：报价 RELEASED/USED/EXPIRED → 直接成功。
func (l *ReleaseGroupBuyLogic) ReleaseGroupBuy(in *play.ReleaseGroupBuyReq) (*play.ReleaseGroupBuyResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	outCount := in.Count
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "releaseGroupBuy", func(db *gorm.DB) error {

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
			l.Infof("ReleaseGroupBuy idempotent: quote status %s, quoteToken: %s", q.GetStatus(), in.QuoteToken)
			return nil
		}

		// ② 弹出下注 + 余位+=count：仅当报价 LOCKED（正向提交过锁定）时执行
		if q.GetStatus() == "LOCKED" {
			var st gamemodelnc.GroupBuyState
			err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("config_id = ?", in.ConfigId).First(&st).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil {
				state, err := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson)
				if err == nil {
					removed := int64(0)
					kept := make([]core.GroupBuyBet, 0, len(state.Bets))
					for _, b := range state.Bets {
						if b.DrawID == in.RequestId {
							removed = int64(b.Count)
							continue
						}
						kept = append(kept, b)
					}
					if removed > 0 {
						if err := db.Model(&gamemodelnc.GroupBuyState{}).Where("id = ?", st.Id).
							Updates(map[string]interface{}{
								"bets_json":      groupBuyBetsJSON(kept),
								"remaining_bets": st.RemainingBets + removed,
								"updated_at":     time.Now(),
							}).Error; err != nil {
							return err
						}
						l.Infof("ReleaseGroupBuy bet popped, requestId: %s, count: %d", in.RequestId, removed)
					}
					outCount = removed
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
			l.Errorf("ReleaseGroupBuy aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("ReleaseGroupBuy failed, quoteToken: %s, err: %v", in.QuoteToken, err)
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
		l.Infof("ReleaseGroupBuy success (readback), quoteToken: %s, status: %s", in.QuoteToken, statusOut)
		return &play.ReleaseGroupBuyResp{Count: outCount, Status: statusOut}, nil
	}
	l.Infof("ReleaseGroupBuy success, quoteToken: %s, count: %d", in.QuoteToken, outCount)
	return &play.ReleaseGroupBuyResp{Count: outCount, Status: "RELEASED"}, nil
}

// errGroupBuyRecordDup 事务内哨兵：同 requestId 的拼团记录已存在（另一 gid 已结算）
var errGroupBuyRecordDup = errors.New("group buy record duplicated")

type SettleGroupBuyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleGroupBuyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleGroupBuyLogic {
	return &SettleGroupBuyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleGroupBuy SAGA 05 正向：barrier 事务内
//
//	① state FOR UPDATE（必须 OPEN，种子缺失 → 3020）；
//	② 插记录（uk request_id 撞键=已结算 → 读回幂等成功）；
//	③ 余位==0（最后一注成团）→ 整批结算：
//	   普通槽=普通条目按 remaining 展开（不足 target → 3020）、条目子奖、
//	   幸运奖（lucky_sent 长度守卫 3034，remaining=quantity-sent）→
//	   core.SettleGroupBuyBatch（seed=gid 确定性）→ 结算奖标 requestId 追加、
//	   条目/子奖库存按结算奖计数扣减、lucky_sent 推进、状态 SETTLED。
//
// 错误语义：业务失败 → codes.Aborted；系统错误 → codes.Internal。
func (l *SettleGroupBuyLogic) SettleGroupBuy(in *play.SettleGroupBuyReq) (*play.SettleGroupBuyResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 || in.Count <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	// 幂等重放快路径：同 requestId 记录已存在 → 读回成功（先于业务校验，M11b 教训）
	var existing gamemodelnc.GroupBuyRecords
	if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&existing).Error; err == nil {
		return l.readBackRecord(in.RequestId, in.UserId)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Errorf("SettleGroupBuy fast path query failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var outRecordId int64
	outAwards := make([]core.GroupBuyAward, 0)
	outSettled := false
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleGroupBuy", func(db *gorm.DB) error {

		// ① state FOR UPDATE（uk config_id 串行化；种子缺失 → 3020）
		var st gamemodelnc.GroupBuyState
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团状态不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		if st.Status != groupBuyStateOpen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团已结算或状态不满足结算", errorx.CodeGroupBuyNoSlots))
		}

		// 拼团配置 + 幸运奖配置（配置 JSON 无效 → 3020）
		var cfg gamemodel.GroupBuyConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团配置不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		luckyCfg, err := groupBuyParseLucky(cfg.LuckyPrizesJson)
		if err != nil || len(luckyCfg) == 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团幸运奖配置无效", errorx.CodeGameConfigInvalid))
		}
		luckyChildren, err := groupBuyParseLuckyChildren(cfg.LuckyChildrenJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团幸运子奖配置无效", errorx.CodeGameConfigInvalid))
		}

		// 状态 JSON → 内存
		state, err := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团状态数据异常", errorx.CodeGameConfigInvalid))
		}
		// 3034 守卫：lucky_sent 长度与幸运奖配置数不一致 = 配置变更（空数组同样视为变更），Aborted
		if len(state.LuckySent) != len(luckyCfg) {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团配置变更，请重置", errorx.CodeGroupBuyConfigChanged))
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
		rec := &gamemodelnc.GroupBuyRecords{
			RequestId: in.RequestId,
			UserId:    in.UserId,
			ConfigId:  in.ConfigId,
			Count:     in.Count,
			PriceFen:  priceFen,
			Status:    groupBuyRecordNormal,
			CreatedAt: time.Now(),
		}
		if err := db.Create(rec).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errGroupBuyRecordDup
			}
			return err
		}
		outRecordId = rec.Id

		// ③ 最后一注（余位==0）→ 整批结算
		if st.RemainingBets == 0 {
			// 条目快照：普通槽展开（parent=0 且 remaining>0）、条目子奖（parent!=0 且 remaining>0）
			var entries []gamemodel.GroupBuyEntries
			if err := db.Where("config_id = ?", in.ConfigId).Find(&entries).Error; err != nil {
				return err
			}
			if len(entries) == 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团条目配置缺失", errorx.CodeGameConfigInvalid))
			}
			slots := make([]core.GroupBuySlot, 0)
			children := make([]core.GroupBuyChildStock, 0)
			for _, e := range entries {
				if e.ParentEntryId == 0 {
					for i := int64(0); i < e.Remaining; i++ {
						slots = append(slots, core.GroupBuySlot{
							EntryID:   e.Id,
							EntryName: e.Name,
							IsPrize:   e.IsPrize == 1,
							Remaining: int32(e.Remaining),
						})
					}
					continue
				}
				if e.Remaining > 0 && e.PrizeKind != 0 {
					children = append(children, core.GroupBuyChildStock{
						EntryID:    e.Id,
						ParentID:   e.ParentEntryId,
						Remaining:  int32(e.Remaining),
						PrizeShape: core.PrizeShape{PrizeKind: e.PrizeKind, CashFen: e.CashFen, PointsAmount: e.PointsAmount, CouponTemplateNo: e.CouponTemplateNo},
					})
				}
			}
			if len(slots) < int(cfg.TargetBetCount) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团普通槽不足", errorx.CodeGameConfigInvalid))
			}
			// 幸运奖库存 = quantity - lucky_sent（3034 守卫已过）；幸运子奖按 lucky_index 挂接
			lucky := make([]core.GroupBuyLuckyStock, 0, len(luckyCfg))
			for _, stock := range luckyCfg {
				sent := int32(0)
				if int(stock.Index) < len(state.LuckySent) {
					sent = state.LuckySent[stock.Index]
				}
				stock.Remaining = stock.Remaining - sent
				if stock.Remaining <= 0 {
					continue
				}
				for _, child := range luckyChildren {
					if child.Index == stock.Index {
						stock.Children = append(stock.Children, child)
					}
				}
				lucky = append(lucky, stock)
			}
			awards, err := core.SettleGroupBuyBatch(state.Bets, int(cfg.TargetBetCount),
				int(cfg.PerBetAwardLimit), slots, children, lucky, []byte(in.RequestId))
			if err != nil {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=拼团结算失败", errorx.CodeGroupBuyConfigChanged))
			}
			for i := range awards {
				awards[i].RequestID = in.RequestId
			}
			outAwards = awards
			// 库存推进按结算奖计数反推：normal/条目子奖扣 entries.remaining、lucky 推进 lucky_sent
			entryDelta := map[int64]int64{}
			luckyDelta := map[int32]int32{}
			for _, a := range awards {
				switch a.Kind {
				case core.GroupBuyAwardKindNormal:
					entryDelta[a.EntryID]++
				case core.GroupBuyAwardKindChild:
					if a.EntryID > 0 {
						entryDelta[a.EntryID]++
					}
				case core.GroupBuyAwardKindLucky:
					luckyDelta[a.LuckyIndex]++
				}
			}
			for entryId, n := range entryDelta {
				if err := db.Model(&gamemodel.GroupBuyEntries{}).
					Where("id = ? AND remaining >= ?", entryId, n).
					Update("remaining", gorm.Expr("remaining - ?", n)).Error; err != nil {
					return err
				}
			}
			for index, n := range luckyDelta {
				if int(index) < len(state.LuckySent) {
					state.LuckySent[index] += n
				}
			}
			outSettled = true
			if err := db.Model(&gamemodelnc.GroupBuyState{}).Where("id = ?", st.Id).
				Updates(map[string]interface{}{
					"awards_json":     groupBuyAwardsJSON(append(state.Awards, awards...)),
					"lucky_sent_json": groupBuyLuckySentJSON(state.LuckySent),
					"status":          groupBuyStateSettled,
					"updated_at":      time.Now(),
				}).Error; err != nil {
				return err
			}
			l.Infof("SettleGroupBuy settled, requestId: %s, awards: %d", in.RequestId, len(awards))
		}
		return nil
	})
	if errors.Is(err, errGroupBuyRecordDup) {
		// 同 requestId 已结算（另一 gid 已成功）：返回已有记录，幂等成功
		return l.readBackRecord(in.RequestId, in.UserId)
	}
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleGroupBuy aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleGroupBuy failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（分支已成功/重试）：读库如实返回已有记录
		return l.readBackRecord(in.RequestId, in.UserId)
	}
	l.Infof("SettleGroupBuy success, requestId: %s, recordId: %d, settled: %v, awards: %d",
		in.RequestId, outRecordId, outSettled, len(outAwards))
	return &play.SettleGroupBuyResp{
		RecordId: outRecordId,
		Awards:   l.groupBuyAwardItems(outAwards),
		Settled:  outSettled,
		Status:   "SETTLED",
	}, nil
}

// groupBuyAwardItems 结算奖 → PrizeItem 列表（券名尽力解析，展示用）
func (l *SettleGroupBuyLogic) groupBuyAwardItems(awards []core.GroupBuyAward) []*play.PrizeItem {
	items := make([]*play.PrizeItem, 0, len(awards))
	for _, a := range awards {
		items = append(items, prizeShapeItem(l.ctx, l.svcCtx, a.PrizeShape))
	}
	return items
}

// readBackRecord 幂等/屏障跳过路径：读已有记录 + 本 gid 结算奖 + 当前状态如实返回
func (l *SettleGroupBuyLogic) readBackRecord(requestId string, userId int64) (*play.SettleGroupBuyResp, error) {
	var rec gamemodelnc.GroupBuyRecords
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", requestId, userId).
		First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("readBackRecord: record not found, requestId: %s", requestId)
			return nil, status.Error(codes.Internal, "拼团记录不存在")
		}
		l.Errorf("readBackRecord failed, requestId: %s, err: %v", requestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	settled := false
	var awards []core.GroupBuyAward
	var st gamemodelnc.GroupBuyState
	if err := l.svcCtx.DB.Where("config_id = ?", rec.ConfigId).First(&st).Error; err == nil {
		settled = st.Status == groupBuyStateSettled
		if state, err := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson); err == nil {
			awards = groupBuyAwardsOfRequest(state.Awards, requestId)
		}
	}
	l.Infof("readBackRecord: requestId: %s, recordId: %d, settled: %v, awards: %d", requestId, rec.Id, settled, len(awards))
	return &play.SettleGroupBuyResp{
		RecordId: rec.Id,
		Awards:   l.groupBuyAwardItems(awards),
		Settled:  settled,
		Status:   "SETTLED",
	}, nil
}

type SettleGroupBuyRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleGroupBuyRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleGroupBuyRevertLogic {
	return &SettleGroupBuyRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleGroupBuyRevert SAGA 05 补偿：只撤销本分支造成的影响（dig/fullcar/gashapon 先例——唯一回滚点防双扣）：
//
//	① 记录 → 2 已撤销（无记录=05 正向从未成功，跳过）；
//	② 若本 gid 触发过整批结算（awards_json 有本 requestId 的结算奖）：
//	   按 requestId 删结算奖；条目库存回补（normal 按 entry_id 计数、
//	   条目子奖按 entry_id 计数；幸运子奖无库存不回补）；lucky_sent 逐幸运奖回退；
//	   SETTLED→OPEN。
//
// 下注记录弹出与余位回补由 03 补偿（ReleaseGroupBuy）唯一负责，本分支不碰余位。
func (l *SettleGroupBuyRevertLogic) SettleGroupBuyRevert(in *play.SettleGroupBuyReq) (*play.SettleGroupBuyResp, error) {
	if in.RequestId == "" || in.UserId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	var outRecordId int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleGroupBuyRevert", func(db *gorm.DB) error {

		// ① 记录 FOR UPDATE（盲撤销幂等；无记录=正向从未成功，跳过）
		var rec gamemodelnc.GroupBuyRecords
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&rec).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			outRecordId = rec.Id
			if rec.Status == groupBuyRecordNormal {
				if err := db.Model(&gamemodelnc.GroupBuyRecords{}).
					Where("id = ? AND status = ?", rec.Id, groupBuyRecordNormal).
					Update("status", groupBuyRecordCanceled).Error; err != nil {
					return err
				}
				// ② 整批结算回滚（本 gid 触发过结算才有 awards；仅状态撤销幂等跳过）
				if err := l.revertSettlement(db, &rec); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleGroupBuyRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleGroupBuyRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读记录如实返回
		var rec gamemodelnc.GroupBuyRecords
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&rec).Error; err == nil {
			outRecordId = rec.Id
		}
	}
	l.Infof("SettleGroupBuyRevert success, requestId: %s, recordId: %d", in.RequestId, outRecordId)
	return &play.SettleGroupBuyResp{RecordId: outRecordId, Status: "REVERTED"}, nil
}

// revertSettlement 整批结算回滚：删结算奖、条目库存回补、lucky_sent 回退、SETTLED→OPEN
func (l *SettleGroupBuyRevertLogic) revertSettlement(db *gorm.DB, rec *gamemodelnc.GroupBuyRecords) error {
	var st gamemodelnc.GroupBuyState
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("config_id = ?", rec.ConfigId).First(&st).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err != nil {
		return nil
	}
	state, perr := groupBuyParseState(st.BetsJson, st.AwardsJson, st.LuckySentJson)
	if perr != nil {
		return nil
	}
	// 按 requestId 删结算奖 + 库存计数
	kept := make([]core.GroupBuyAward, 0, len(state.Awards))
	removed := 0
	entryDelta := map[int64]int64{}
	luckyDelta := map[int32]int32{}
	for _, a := range state.Awards {
		if a.RequestID != rec.RequestId {
			kept = append(kept, a)
			continue
		}
		removed++
		switch a.Kind {
		case core.GroupBuyAwardKindNormal:
			entryDelta[a.EntryID]++
		case core.GroupBuyAwardKindChild:
			if a.EntryID > 0 {
				entryDelta[a.EntryID]++
			}
		case core.GroupBuyAwardKindLucky:
			luckyDelta[a.LuckyIndex]++
		}
	}
	if removed == 0 {
		// 非成团注：无结算影响，只撤销记录（03 补偿负责弹出下注与余位回补）
		return nil
	}
	// 条目库存回补（normal/条目子奖按 entry_id 计数；幸运子奖无库存跳过）
	for entryId, n := range entryDelta {
		if err := db.Model(&gamemodel.GroupBuyEntries{}).
			Where("id = ?", entryId).
			Update("remaining", gorm.Expr("remaining + ?", n)).Error; err != nil {
			return err
		}
	}
	// lucky_sent 回退（防御下限 0）
	for index, n := range luckyDelta {
		if int(index) >= 0 && int(index) < len(state.LuckySent) {
			state.LuckySent[index] -= n
			if state.LuckySent[index] < 0 {
				state.LuckySent[index] = 0
			}
		}
	}
	// 结算奖删除 + lucky_sent 落库 + SETTLED→OPEN（余位不碰——03 补偿负责）
	updates := map[string]interface{}{
		"awards_json":     groupBuyAwardsJSON(kept),
		"lucky_sent_json": groupBuyLuckySentJSON(state.LuckySent),
		"status":          groupBuyStateOpen,
		"updated_at":      time.Now(),
	}
	if err := db.Model(&gamemodelnc.GroupBuyState{}).Where("id = ?", st.Id).
		Updates(updates).Error; err != nil {
		return err
	}
	l.Infof("revertSettlement: requestId: %s, removed: %d", rec.RequestId, removed)
	return nil
}

package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gacha_srv/app/marketing/marketingClient"
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
	"math/bits"
	"sort"
	"time"
)

// ══════════════ M11b 挖宝箱（DIG_BOX）公共操作 ═══════════════
// 会话按 (user_id, config_id) 唯一（uk），BIGINT 位图（cell_count≤62，cell_index 为位号）；
// 满格 COMPLETED 的会话在下一次购买时回收（位图/计数清零）。
// 发奖（SAGA 06）与 campaign 同款幂等模式：grant_no 确定性派生 + 全 uk 幂等外部调用。

// 挖宝会话状态（dig_box_sessions.status）
const (
	digSessionPlaying   int64 = 1
	digSessionCompleted int64 = 2
	digSessionCanceled  int64 = 3
)

// 挖宝记录状态（dig_box_records.status）
const (
	digRecordNormal   int64 = 1
	digRecordCanceled int64 = 2
)

// digCellPrize 挖宝格奖品（prizes_json 数组元素；kind=0 空格不落 JSON）
type digCellPrize struct {
	Kind             int64  `json:"kind"`                       // 1 CASH 2 POINTS 3 COUPON
	CashFen          int64  `json:"cashFen,omitempty"`          // 现金金额（分）
	Points           int64  `json:"points,omitempty"`           // 积分数
	CouponTemplateNo string `json:"couponTemplateNo,omitempty"` // 券模板单号
	CouponName       string `json:"couponName,omitempty"`       // 券名称（展示）
}

// digCellMask 格子列表 → 位图掩码（cell_index 为位号；调用方保证索引合法）
func digCellMask(cells []int64) int64 {
	var mask int64
	for _, c := range cells {
		mask |= 1 << c
	}
	return mask
}

// digCellsOfMask 位图 → 已置位格子列表（升序）
func digCellsOfMask(mask int64) []int64 {
	cells := make([]int64, 0, bits.OnesCount64(uint64(mask)))
	for i := 0; i < 62; i++ {
		if mask&(1<<i) != 0 {
			cells = append(cells, int64(i))
		}
	}
	return cells
}

// digJoinCells 格子列表 → 逗号分隔串（升序，记录落库用）
func digJoinCells(cells []int64) string {
	sorted := append([]int64(nil), cells...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	out := ""
	for i, c := range sorted {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprintf("%d", c)
	}
	return out
}

// digParseCells 逗号分隔串 → 格子列表
func digParseCells(s string) []int64 {
	if s == "" {
		return nil
	}
	var out []int64
	var cur int64 = -1
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if cur >= 0 {
				out = append(out, cur)
				cur = -1
			}
			continue
		}
		if s[i] >= '0' && s[i] <= '9' {
			if cur < 0 {
				cur = 0
			}
			cur = cur*10 + int64(s[i]-'0')
		}
	}
	return out
}

// digCellsValid 校验格子列表：非空、无重复、索引 < cellCount
func digCellsValid(cells []int64, cellCount int64) bool {
	if len(cells) == 0 || cellCount <= 0 {
		return false
	}
	seen := make(map[int64]bool, len(cells))
	for _, c := range cells {
		if c < 0 || c >= cellCount || seen[c] {
			return false
		}
		seen[c] = true
	}
	return true
}

// digPrizesOfCells 格子行 → 揭晓奖品（空格 prize_kind=0 跳过；券名尽力解析）。
// 返回 JSON 串（记录落库）与响应 PrizeItem 列表（顺序与 grants 的 #idx 一致）。
func digPrizesOfCells(ctx context.Context, svcCtx *svc.ServiceContext, cells []*gamemodel.DigBoxCells) (jsonStr string, items []*play.PrizeItem) {
	prizes := make([]digCellPrize, 0, len(cells))
	items = make([]*play.PrizeItem, 0, len(cells))
	for _, c := range cells {
		if c.PrizeKind == 0 {
			continue
		}
		p := digCellPrize{Kind: c.PrizeKind, CashFen: c.CashFen, Points: c.PointsAmount, CouponTemplateNo: c.CouponTemplateNo}
		item := &play.PrizeItem{Kind: c.PrizeKind, Points: c.PointsAmount, CouponTemplateNo: c.CouponTemplateNo}
		switch c.PrizeKind {
		case prizeCash:
			item.Amount = utils.FenToYuan(c.CashFen)
		case prizeCoupon:
			p.CouponName = couponNameOf(ctx, svcCtx, c.CouponTemplateNo)
			item.CouponName = p.CouponName
		}
		prizes = append(prizes, p)
		items = append(items, item)
	}
	if len(prizes) == 0 {
		return "", items
	}
	b, err := json.Marshal(prizes)
	if err != nil {
		return "", items
	}
	return string(b), items
}

// digGrantPrizesOfRecord 记录 prizes_json → 发放队列（kind=0 空格已在落库时过滤）
func digGrantPrizesOfRecord(prizesJSON string) ([]digCellPrize, error) {
	if prizesJSON == "" {
		return nil, nil
	}
	var prizes []digCellPrize
	if err := json.Unmarshal([]byte(prizesJSON), &prizes); err != nil {
		return nil, err
	}
	return prizes, nil
}

// digPrizeItemOf 揭晓奖品 → 响应 PrizeItem（列表/记录读回组装）
func digPrizeItemOf(p digCellPrize) *play.PrizeItem {
	item := &play.PrizeItem{Kind: p.Kind, Points: p.Points, CouponTemplateNo: p.CouponTemplateNo, CouponName: p.CouponName}
	if p.Kind == prizeCash {
		item.Amount = utils.FenToYuan(p.CashFen)
	}
	return item
}

type GetDigBoxSessionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDigBoxSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDigBoxSessionLogic {
	return &GetDigBoxSessionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetDigBoxSession 挖宝会话（uk(user_id,config_id)；无会话返回 hasSession=false + 配置格数。
// 非 SAGA RPC：业务错误 CodeError）
func (l *GetDigBoxSessionLogic) GetDigBoxSession(in *play.GetDigBoxSessionReq) (*play.GetDigBoxSessionResp, error) {
	if in.UserId <= 0 || in.BoxId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}
	var cfg gamemodel.DigBoxConfigs
	if err := l.svcCtx.DB.Where("id = ?", in.BoxId).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeDigBoxNotFound, "挖宝箱不存在")
		}
		l.Errorf("GetDigBoxSession query config failed, err: %v", err)
		return nil, err
	}

	var sess gamemodelnc.DigBoxSessions
	err := l.svcCtx.DB.Where("user_id = ? AND config_id = ?", in.UserId, in.BoxId).First(&sess).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &play.GetDigBoxSessionResp{
			HasSession: false,
			CellCount:  cfg.CellCount,
		}, nil
	}
	if err != nil {
		l.Errorf("GetDigBoxSession query failed, err: %v", err)
		return nil, err
	}
	return &play.GetDigBoxSessionResp{
		HasSession:     true,
		SessionNo:      sess.SessionNo,
		OccupiedBitmap: sess.OccupiedBitmap,
		TotalCellsPaid: sess.TotalCellsPaid,
		Status:         sess.Status,
		CellCount:      cfg.CellCount,
	}, nil
}

type GrantDigPrizesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantDigPrizesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantDigPrizesLogic {
	return &GrantDigPrizesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantDigPrizes SAGA 06 正向：按记录 prizes_json 逐奖幂等发放。
//   - 现金 → user.CreditWallet（kind=1，requestId=grant_no 幂等）
//   - 积分 → user.CreditWallet（kind=4，requestId=grant_no 幂等）
//   - 券   → 本地插入用户券（coupon_no="uc_"+grant_no，uk 撞键=已发过）
//     grant_no = "dgp_"+requestId+"#"+idx（确定性派生，重试/重放只补发不重复）。
//
// 铁律：**永不 Aborted**（奖品先于捕获，撤单不回退奖品，与 draw 域 GrantAsset 对齐）；
// 失败只回 Internal → DTM 退避重试，直到发放收敛。
func (l *GrantDigPrizesLogic) GrantDigPrizes(in *play.GrantDigPrizesReq) (*play.GrantDigPrizesResp, error) {
	if in.RequestId == "" || in.UserId <= 0 {
		return nil, status.Error(codes.Internal, "code=400 msg=请求参数非法")
	}

	// 记录由 05 正向落库（06 在其后执行）；缺失=异常，Internal 重试
	var rec gamemodelnc.DigBoxRecords
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", in.RequestId, in.UserId).
		First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("GrantDigPrizes record not found, requestId: %s", in.RequestId)
			return nil, status.Error(codes.Internal, fmt.Sprintf("code=%d msg=挖宝记录不存在", errorx.CodeDigRecordNotFound))
		}
		l.Errorf("GrantDigPrizes query record failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	prizes, err := digGrantPrizesOfRecord(rec.PrizesJson)
	if err != nil {
		l.Errorf("GrantDigPrizes parse prizes failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	items := make([]playkit.Prize, 0, len(prizes))
	for _, p := range prizes {
		items = append(items, playkit.Prize{
			UserID:           in.UserId,
			Kind:             p.Kind,
			CashFen:          p.CashFen,
			Points:           p.Points,
			CouponTemplateNo: p.CouponTemplateNo,
		})
	}
	// grant_no = dgp_{gid}#{idx}（GrantAll 内派生）：dtm 重试/补偿重放命中同一键，
	// CreditWallet requestId 幂等 + 券 uk 撞键 = 已发过读回成功。
	if err := playkit.GrantAll(l.ctx, l.svcCtx, in.RequestId, "dgp", items); err != nil {
		// 永不 Aborted：失败 Internal → DTM 重试，grant_no 幂等收敛
		l.Errorf("GrantDigPrizes grant failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	// 高1 修复：发奖落库标记（04 补偿退款前置检查依据）；Internal 失败 → DTM 重试 06 幂等收敛
	if len(items) > 0 {
		if err := playkit.MarkGranted(l.ctx, l.svcCtx, in.RequestId, in.UserId); err != nil {
			l.Errorf("GrantDigPrizes mark failed, requestId: %s, err: %v", in.RequestId, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	l.Infof("GrantDigPrizes success, requestId: %s, prizes: %d", in.RequestId, len(items))
	return &play.GrantDigPrizesResp{Status: "GRANTED"}, nil
}

type GrantDigPrizesRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantDigPrizesRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantDigPrizesRevertLogic {
	return &GrantDigPrizesRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantDigPrizesRevert SAGA 06 补偿：空操作（D4 决策——奖品先于捕获且不追回，
// 补偿注册仅为防御；即使调用也幂等返回）
func (l *GrantDigPrizesRevertLogic) GrantDigPrizesRevert(in *play.GrantDigPrizesReq) (*play.GrantDigPrizesResp, error) {
	l.Infof("GrantDigPrizesRevert noop, requestId: %s", in.RequestId)
	return &play.GrantDigPrizesResp{Status: "GRANTED"}, nil
}

type ListDigBoxesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListDigBoxesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDigBoxesLogic {
	return &ListDigBoxesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListDigBoxes 挖宝箱列表（status=1 上架；非 SAGA RPC：业务错误 CodeError，系统错误 Internal）
func (l *ListDigBoxesLogic) ListDigBoxes(in *play.ListDigBoxesReq) (*play.ListDigBoxesResp, error) {
	var cfgs []gamemodel.DigBoxConfigs
	if err := l.svcCtx.DB.Where("status = ?", 1).Order("id ASC").Find(&cfgs).Error; err != nil {
		l.Errorf("ListDigBoxes query failed, err: %v", err)
		return nil, err
	}
	items := make([]*play.DigBoxItem, 0, len(cfgs))
	for i := range cfgs {
		items = append(items, &play.DigBoxItem{
			BoxId:     cfgs[i].Id,
			Name:      cfgs[i].Name,
			Price:     utils.FenToYuan(cfgs[i].PriceFen),
			CellCount: cfgs[i].CellCount,
		})
	}
	return &play.ListDigBoxesResp{Items: items}, nil
}

type ListDigRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListDigRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDigRecordsLogic {
	return &ListDigRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListDigRecords 挖宝记录：requestId 非空=精确查一条（购买轮询），空=分页列表。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *ListDigRecordsLogic) ListDigRecords(in *play.ListDigRecordsReq) (*play.ListDigRecordsResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}

	// 精确查询（轮询开奖结果）
	if in.RequestId != "" {
		var rec gamemodelnc.DigBoxRecords
		if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", in.RequestId, in.UserId).
			First(&rec).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// requestId 未命中是正常情况：返回空列表，避免每笔新购买打 error 日志。
				return &play.ListDigRecordsResp{Items: []*play.DigRecordItem{}, Total: 0}, nil
			}
			l.Errorf("ListDigRecords query failed, err: %v", err)
			return nil, err
		}
		item, err := l.buildItem(&rec)
		if err != nil {
			return nil, err
		}
		return &play.ListDigRecordsResp{Items: []*play.DigRecordItem{item}, Total: 1}, nil
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
	if err := l.svcCtx.DB.Model(&gamemodelnc.DigBoxRecords{}).
		Where("user_id = ?", in.UserId).Count(&total).Error; err != nil {
		l.Errorf("ListDigRecords count failed, err: %v", err)
		return nil, err
	}
	var recs []gamemodelnc.DigBoxRecords
	if err := l.svcCtx.DB.Where("user_id = ?", in.UserId).
		Order("id DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).
		Find(&recs).Error; err != nil {
		l.Errorf("ListDigRecords page failed, err: %v", err)
		return nil, err
	}
	items := make([]*play.DigRecordItem, 0, len(recs))
	for i := range recs {
		item, err := l.buildItem(&recs[i])
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return &play.ListDigRecordsResp{Items: items, Total: total}, nil
}

// buildItem 记录行 → 响应条目（prizes_json 解析 + 金额转元）
func (l *ListDigRecordsLogic) buildItem(rec *gamemodelnc.DigBoxRecords) (*play.DigRecordItem, error) {
	prizes, err := digGrantPrizesOfRecord(rec.PrizesJson)
	if err != nil {
		l.Errorf("ListDigRecords parse prizes failed, recordId: %d, err: %v", rec.Id, err)
		return nil, err
	}
	items := make([]*play.PrizeItem, 0, len(prizes))
	for _, p := range prizes {
		items = append(items, digPrizeItemOf(p))
	}
	return &play.DigRecordItem{
		RecordId:    rec.Id,
		RequestId:   rec.RequestId,
		ConfigId:    rec.ConfigId,
		CellsChosen: rec.CellsChosen,
		Prizes:      items,
		Price:       utils.FenToYuan(rec.TotalPriceFen),
		Status:      rec.Status,
		CreatedAt:   rec.CreatedAt.Unix(),
	}, nil
}

type LockDigBoxLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLockDigBoxLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LockDigBoxLogic {
	return &LockDigBoxLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LockDigBox SAGA 03 正向：barrier 事务内
//
//	① 报价只读校验（营销 GetQuote sub-call：userId/grossFen/boxId 取报价行）；
//	② 会话 FOR UPDATE（COMPLETED/CANCELED → 回收清零位图与计数）；
//	③ 校验格子未占用（3016）与价格一致（3018：quote.gross == 单价×格数）；
//	④ 报价锁（营销 LockQuote sub-call，置于全部玩法校验之后）；
//	⑤ 位图置位 + 计数累加。
//
// 报价锁放在最后：玩法自身校验失败回滚时不触碰营销锁（营销事务已独立提交，
// 无法随本事务回滚），与旧仓「锁与业务同事务」的最终状态一致——自中止后报价保持 OPEN。
// userId 取报价行（网关 QuoteCheckout 落库），不信请求参数。
// 错误语义（SAGA 分支铁律）：业务失败 → codes.Aborted（触发补偿）；系统错误 → codes.Internal（DTM 重试）。
func (l *LockDigBoxLogic) LockDigBox(in *play.LockDigBoxReq) (*play.LockDigBoxResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.SessionNo == "" || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "LockDigBox", func(db *gorm.DB) error {

		// ① 报价只读校验（营销 GetQuote sub-call；quote 不存在 → Aborted）
		quote, err := playkit.LoadQuote(l.ctx, l.svcCtx, in.QuoteToken)
		if err != nil {
			return err
		}
		userId := quote.UserID

		// 挖宝箱配置（纯种子配置只读，不走缓存锁）
		var cfg gamemodel.DigBoxConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝箱不存在", errorx.CodeDigBoxNotFound))
			}
			return err
		}
		if cfg.Status != 1 || cfg.CellCount <= 0 || cfg.CellCount > 62 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝箱不可用", errorx.CodeDigBoxNotFound))
		}

		// ② 会话 FOR UPDATE（uk(user_id,config_id) 串行化同箱并发购买）
		var sess gamemodelnc.DigBoxSessions
		err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND config_id = ?", userId, in.ConfigId).First(&sess).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 首次挖该箱：建会话（bitmap=0）
			sess = gamemodelnc.DigBoxSessions{
				SessionNo:      in.SessionNo,
				UserId:         userId,
				ConfigId:       in.ConfigId,
				OccupiedBitmap: 0,
				TotalCellsPaid: 0,
				TotalPriceFen:  0,
				Status:         digSessionPlaying,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}
			if err := db.Create(&sess).Error; err != nil {
				if errors.Is(err, gorm.ErrDuplicatedKey) {
					// 并发首建竞争：重读（行锁串行化后可见对方会话）
					if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
						Where("user_id = ? AND config_id = ?", userId, in.ConfigId).First(&sess).Error; err != nil {
						return err
					}
				} else {
					return err
				}
			}
		} else if err != nil {
			return err
		}
		// 满格/取消会话回收：位图计数清零重开（D5）
		if sess.Status != digSessionPlaying {
			if err := db.Model(&gamemodelnc.DigBoxSessions{}).Where("id = ?", sess.Id).
				Updates(map[string]interface{}{
					"occupied_bitmap":  0,
					"total_cells_paid": 0,
					"total_price_fen":  0,
					"status":           digSessionPlaying,
				}).Error; err != nil {
				return err
			}
			sess.OccupiedBitmap = 0
			sess.TotalCellsPaid = 0
			sess.TotalPriceFen = 0
			sess.Status = digSessionPlaying
		}

		// ③ 格子解析与校验（takeAll=锁内解析全部空格；网关侧已解析为显式格子）
		var cells []int64
		if in.TakeAll {
			for i := int64(0); i < cfg.CellCount; i++ {
				if sess.OccupiedBitmap&(1<<i) == 0 {
					cells = append(cells, i)
				}
			}
		} else {
			cells = in.CellIndexes
		}
		if !digCellsValid(cells, cfg.CellCount) {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=所选格子无效", errorx.CodeDigCellOccupied))
		}
		mask := digCellMask(cells)
		if sess.OccupiedBitmap&mask != 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=所选格子已被挖开", errorx.CodeDigCellOccupied))
		}

		// ④ 价格一致（锁内重算）：count×单价 == 报价 gross == 请求总额
		expected := int64(len(cells)) * cfg.PriceFen
		if quote.GrossFen != expected || in.TotalPriceFen != expected {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝价格校验不一致", errorx.CodeDigPriceMismatch))
		}
		if quote.BoxID != 0 && quote.BoxID != in.ConfigId {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝价格校验不一致", errorx.CodeDigPriceMismatch))
		}

		// ⑤ 报价锁（营销 LockQuote sub-call，置于玩法校验之后；幂等：newLocked=false=已锁重发 → 业务已成功，直接返回）
		newLocked, err := playkit.LockQuote(l.ctx, l.svcCtx, in.RequestId, in.QuoteToken)
		if err != nil {
			return err
		}
		if !newLocked {
			l.Infof("LockDigBox idempotent: quote already locked, quoteToken: %s", in.QuoteToken)
			return nil
		}

		// ⑥ 位图置位 + 计数累加
		newBitmap := sess.OccupiedBitmap | mask
		if err := db.Model(&gamemodelnc.DigBoxSessions{}).Where("id = ?", sess.Id).
			Updates(map[string]interface{}{
				"occupied_bitmap":  newBitmap,
				"total_cells_paid": sess.TotalCellsPaid + int64(len(cells)),
				"total_price_fen":  sess.TotalPriceFen + expected,
				"status":           digSessionPlaying,
				"updated_at":       time.Now(),
			}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("LockDigBox aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("LockDigBox failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（正向已成功）：读库如实返回
		return l.readBackLock(in.SessionNo)
	}
	l.Infof("LockDigBox success, sessionNo: %s, cells: %d", in.SessionNo, len(in.CellIndexes))
	return &play.LockDigBoxResp{SessionNo: in.SessionNo, Status: "LOCKED"}, nil
}

// readBackLock 屏障跳过时读会话如实返回
func (l *LockDigBoxLogic) readBackLock(sessionNo string) (*play.LockDigBoxResp, error) {
	var sess gamemodelnc.DigBoxSessions
	if err := l.svcCtx.DB.Where("session_no = ?", sessionNo).First(&sess).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Error(codes.Internal, "挖宝会话不存在")
		}
		l.Errorf("readBackLock failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	l.Infof("readBackLock: sessionNo: %s, paid: %d, bitmap: %d",
		sessionNo, sess.TotalCellsPaid, sess.OccupiedBitmap)
	return &play.LockDigBoxResp{SessionNo: sessionNo, Status: "LOCKED"}, nil
}

type ReleaseDigBoxLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReleaseDigBoxLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReleaseDigBoxLogic {
	return &ReleaseDigBoxLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ReleaseDigBox SAGA 03 补偿：barrier 事务内
//
//	① 报价 FOR UPDATE 先行——状态决定会话是否回滚：正向 LockDigBox 在同一
//	   barrier 事务内提交「报价 LOCKED + 会话置位」，两者同生共死，故
//	   quote LOCKED ⇔ 本 gid 确实置位（需回滚）；quote OPEN ⇒ 正向已整体
//	   回滚（如 3016 格已挖），会话保持原样——绝不误回滚预置的已挖格；
//	② 会话回滚（仅 LOCKED 时）：mine = bitmap & mask(cells)——只回滚本 gid
//	   实际置位的格子（已回滚则 mine=0 幂等跳过）；
//	③ 计数/实付回滚（唯一回滚点，05 补偿只撤销记录不碰计数，杜绝双扣）；
//	④ 报价 LOCKED/OPEN→RELEASED + 券 LOCKED→AVAILABLE（幂等；USED 不回退）。
//
// 幂等：报价 RELEASED/USED/EXPIRED → 直接成功。
func (l *ReleaseDigBoxLogic) ReleaseDigBox(in *play.ReleaseDigBoxReq) (*play.ReleaseDigBoxResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.SessionNo == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "releaseDigBox", func(db *gorm.DB) error {

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
			l.Infof("ReleaseDigBox idempotent: quote status %s, quoteToken: %s", q.GetStatus(), in.QuoteToken)
			return nil
		}

		// ② 会话回滚：仅当报价 LOCKED（正向提交过置位）时执行（按 session_no 精确定位）
		if q.GetStatus() == "LOCKED" {
			var sess gamemodelnc.DigBoxSessions
			err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("session_no = ?", in.SessionNo).First(&sess).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil {
				// mine = 本 gid 实际置位的格子（已回滚/未锁定则 mine=0 幂等跳过）
				mask := digCellMask(in.CellIndexes)
				mine := sess.OccupiedBitmap & mask
				if mine != 0 {
					newBitmap := sess.OccupiedBitmap &^ mine
					newPaid := sess.TotalCellsPaid - int64(bits.OnesCount64(uint64(mine)))
					if newPaid < 0 {
						newPaid = 0
					}
					newPrice := sess.TotalPriceFen - in.TotalPriceFen
					if newPrice < 0 {
						newPrice = 0
					}
					newStatus := sess.Status
					if newStatus == digSessionCompleted {
						newStatus = digSessionPlaying // 满格状态由本 gid 造成 → 回退
					}
					if err := db.Model(&gamemodelnc.DigBoxSessions{}).Where("id = ?", sess.Id).
						Updates(map[string]interface{}{
							"occupied_bitmap":  newBitmap,
							"total_cells_paid": newPaid,
							"total_price_fen":  newPrice,
							"status":           newStatus,
							"updated_at":       time.Now(),
						}).Error; err != nil {
						return err
					}
					l.Infof("ReleaseDigBox session rolled back, sessionNo: %s, cells: %d", in.SessionNo, bits.OnesCount64(uint64(mine)))
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
			l.Errorf("ReleaseDigBox aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("ReleaseDigBox failed, quoteToken: %s, err: %v", in.QuoteToken, err)
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
		l.Infof("ReleaseDigBox success (readback), quoteToken: %s, status: %s", in.QuoteToken, statusOut)
		return &play.ReleaseDigBoxResp{SessionNo: in.SessionNo, Status: statusOut}, nil
	}
	l.Infof("ReleaseDigBox success, sessionNo: %s, quoteToken: %s", in.SessionNo, in.QuoteToken)
	return &play.ReleaseDigBoxResp{SessionNo: in.SessionNo, Status: "RELEASED"}, nil
}

// errDigRecordDup 事务内哨兵：同 requestId 的挖宝记录已存在（另一 gid 已结算）
var errDigRecordDup = errors.New("dig record duplicated")

type SettleDigBoxLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleDigBoxLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleDigBoxLogic {
	return &SettleDigBoxLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleDigBox SAGA 05 正向：barrier 事务内
//
//	① 会话 FOR UPDATE（必须 PLAYING）；② 校验格子已由 03 置位（3017）；
//	③ 揭晓奖品（dig_box_cells 固定奖品，空格跳过）落 prizes_json；
//	④ 插挖宝记录（uk request_id 撞键=已结算 → 读回幂等成功）；
//	⑤ 满格（位图全 1）→ 会话 COMPLETED（下次购买回收）。
//
// 错误语义：业务失败 → codes.Aborted；系统错误 → codes.Internal。
func (l *SettleDigBoxLogic) SettleDigBox(in *play.SettleDigBoxReq) (*play.SettleDigBoxResp, error) {
	if in.RequestId == "" || in.SessionNo == "" || in.UserId <= 0 || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	var outRecordId int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleDigBox", func(db *gorm.DB) error {

		// ① 会话 FOR UPDATE
		var sess gamemodelnc.DigBoxSessions
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("session_no = ? AND user_id = ?", in.SessionNo, in.UserId).First(&sess).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝会话不存在", errorx.CodeDigSessionInvalid))
			}
			return err
		}
		if sess.Status != digSessionPlaying {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝会话状态不满足结算", errorx.CodeDigSessionInvalid))
		}

		// ② 格子必须已由 03 置位（防绕过锁定直接结算）；格数上限取配置
		var cfg gamemodel.DigBoxConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=挖宝箱不存在", errorx.CodeDigBoxNotFound))
			}
			return err
		}
		if !digCellsValid(in.CellIndexes, cfg.CellCount) {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=所选格子无效", errorx.CodeDigCellOccupied))
		}
		mask := digCellMask(in.CellIndexes)
		if sess.OccupiedBitmap&mask != mask {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=会话位图与所选格子不一致", errorx.CodeDigSessionInvalid))
		}

		// ③ 揭晓奖品（格行按 cell_index 排序，与 grants #idx 对齐）
		var cells []*gamemodel.DigBoxCells
		if err := db.Where("config_id = ? AND cell_index IN ?", in.ConfigId, in.CellIndexes).
			Order("cell_index ASC").Find(&cells).Error; err != nil {
			return err
		}
		prizesJSON, _ := digPrizesOfCells(l.ctx, l.svcCtx, cells)

		// ④ 插记录（uk request_id 幂等；撞键 = 已结算读回）
		priceFen := int64(0)
		if in.Price != "" {
			f, err := utils.YuanToFen(in.Price)
			if err != nil || f < 0 {
				return status.Error(codes.Aborted, "code=400 msg=实付价格无效")
			}
			priceFen = f
		}
		rec := &gamemodelnc.DigBoxRecords{
			RequestId:     in.RequestId,
			UserId:        in.UserId,
			ConfigId:      in.ConfigId,
			SessionNo:     in.SessionNo,
			CellsChosen:   digJoinCells(in.CellIndexes),
			PrizesJson:    prizesJSON,
			TotalPriceFen: priceFen,
			Status:        digRecordNormal,
			CreatedAt:     time.Now(),
		}
		if err := db.Create(rec).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errDigRecordDup
			}
			return err
		}
		outRecordId = rec.Id

		// ⑤ 满格 → COMPLETED（下次购买时回收）
		if sess.OccupiedBitmap == (int64(1)<<cfg.CellCount)-1 {
			if err := db.Model(&gamemodelnc.DigBoxSessions{}).Where("id = ?", sess.Id).
				Updates(map[string]interface{}{"status": digSessionCompleted, "updated_at": time.Now()}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, errDigRecordDup) {
		// 同 requestId 已结算（另一 gid 已成功）：返回已有记录，幂等成功
		return l.readBackRecord(in.RequestId, in.UserId)
	}
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleDigBox aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleDigBox failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（分支已成功/重试）：读库如实返回已有记录
		return l.readBackRecord(in.RequestId, in.UserId)
	}
	l.Infof("SettleDigBox success, requestId: %s, recordId: %d", in.RequestId, outRecordId)
	return &play.SettleDigBoxResp{RecordId: outRecordId, Status: "SETTLED"}, nil
}

// readBackRecord 幂等/屏障跳过路径：读已有记录如实返回
func (l *SettleDigBoxLogic) readBackRecord(requestId string, userId int64) (*play.SettleDigBoxResp, error) {
	var rec gamemodelnc.DigBoxRecords
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", requestId, userId).
		First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("readBackRecord: record not found, requestId: %s", requestId)
			return nil, status.Error(codes.Internal, "挖宝记录不存在")
		}
		l.Errorf("readBackRecord failed, requestId: %s, err: %v", requestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	l.Infof("readBackRecord: requestId: %s, recordId: %d", requestId, rec.Id)
	return &play.SettleDigBoxResp{RecordId: rec.Id, Status: "SETTLED"}, nil
}

type SettleDigBoxRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleDigBoxRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleDigBoxRevertLogic {
	return &SettleDigBoxRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleDigBoxRevert SAGA 05 补偿：记录置撤销 + 满格状态回退。
// 位图清位与计数回滚由 03 补偿（ReleaseDigBox）唯一负责——避免双扣；
// 本分支只撤销自己造成的影响（记录 2 已撤销、会话 COMPLETED→PLAYING），幂等。
func (l *SettleDigBoxRevertLogic) SettleDigBoxRevert(in *play.SettleDigBoxReq) (*play.SettleDigBoxResp, error) {
	if in.RequestId == "" || in.SessionNo == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	var outRecordId int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleDigBoxRevert", func(db *gorm.DB) error {

		// 记录 FOR UPDATE → 2 已撤销（盲更新，幂等；无记录=05 正向从未成功，跳过）
		var rec gamemodelnc.DigBoxRecords
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&rec).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			outRecordId = rec.Id
			if rec.Status == digRecordNormal {
				if err := db.Model(&gamemodelnc.DigBoxRecords{}).
					Where("id = ? AND status = ?", rec.Id, digRecordNormal).
					Update("status", digRecordCanceled).Error; err != nil {
					return err
				}
			}
		}

		// 会话满格状态回退（满格必然由本 gid 造成：此前 COMPLETED 会话在 03 已回收）
		if err := db.Model(&gamemodelnc.DigBoxSessions{}).
			Where("session_no = ? AND status = ?", in.SessionNo, digSessionCompleted).
			Updates(map[string]interface{}{"status": digSessionPlaying, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleDigBoxRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleDigBoxRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读记录如实返回
		var rec gamemodelnc.DigBoxRecords
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&rec).Error; err == nil {
			outRecordId = rec.Id
		}
	}
	l.Infof("SettleDigBoxRevert success, requestId: %s, recordId: %d", in.RequestId, outRecordId)
	return &play.SettleDigBoxResp{RecordId: outRecordId, Status: "REVERTED"}, nil
}

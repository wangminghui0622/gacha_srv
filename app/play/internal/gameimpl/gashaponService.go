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

type AssignGashaponLuckyUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAssignGashaponLuckyUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssignGashaponLuckyUserLogic {
	return &AssignGashaponLuckyUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AssignGashaponLuckyUser 幸运指派（仅 LUCKY 特别奖可指派；userId=0 删除）。
// 无 HTTP 路由（网关无管理员鉴权，挂上等于人人可指派——D4），供未来 ops-rpc/管理网关直调。
//
// 偏离记录：server_v2 校验用户存在；本仓 identity.proto 无查询用户 RPC，只校验
// prize 为 LUCKY（3027）。指派用户未参与时核心判定天然忽略（gashaponUserTouchesRange
// / participants 守卫），不产生错误发奖。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *AssignGashaponLuckyUserLogic) AssignGashaponLuckyUser(in *play.AssignGashaponLuckyUserReq) (*play.AssignGashaponLuckyUserResp, error) {
	if in.ConfigId <= 0 || in.PrizeIndex < 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}

	// 校验 prize 存在且为 LUCKY（3027）
	cfg, err := l.svcCtx.GashaponCfgModel.FindOne(l.ctx, in.ConfigId)
	if err != nil {
		if errors.Is(err, gamemodel.ErrNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "扭蛋配置不存在")
		}
		l.Errorf("AssignGashaponLuckyUser query config failed, configId: %d, err: %v", in.ConfigId, err)
		return nil, errorx.FromMySQLError(err)
	}
	prizes, err := gashaponParsePrizes(cfg.SpecialPrizesJson)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "扭蛋特别奖配置无效")
	}
	var target *core.GashaponSpecialPrize
	for i := range prizes {
		if int64(prizes[i].PrizeIndex) == in.PrizeIndex {
			target = &prizes[i]
			break
		}
	}
	if target == nil || target.SpecialKind != core.GashaponKindLucky {
		return nil, errorx.NewCodeError(errorx.CodeLuckyAssignInvalid, "幸运指派仅限 LUCKY 特别奖")
	}

	// 删除指派（userId=0）
	if in.UserId == 0 {
		if err := l.svcCtx.DB.Where("config_id = ? AND prize_index = ?", in.ConfigId, in.PrizeIndex).
			Delete(&gamemodelnc.GashaponLuckyAssignments{}).Error; err != nil {
			l.Errorf("AssignGashaponLuckyUser delete failed, configId: %d, prizeIndex: %d, err: %v", in.ConfigId, in.PrizeIndex, err)
			return nil, errorx.FromMySQLError(err)
		}
		l.Infof("AssignGashaponLuckyUser cleared, configId: %d, prizeIndex: %d", in.ConfigId, in.PrizeIndex)
		return &play.AssignGashaponLuckyUserResp{Status: "CLEARED"}, nil
	}

	// upsert（uk(config_id, prize_index)）：存在更新 user_id，缺失插入
	var existing gamemodelnc.GashaponLuckyAssignments
	err = l.svcCtx.DB.Where("config_id = ? AND prize_index = ?", in.ConfigId, in.PrizeIndex).
		First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Errorf("AssignGashaponLuckyUser query failed, configId: %d, err: %v", in.ConfigId, err)
		return nil, errorx.FromMySQLError(err)
	}
	if err == nil {
		if err := l.svcCtx.DB.Model(&gamemodelnc.GashaponLuckyAssignments{}).
			Where("id = ?", existing.Id).
			Updates(map[string]interface{}{"user_id": in.UserId, "updated_at": time.Now()}).Error; err != nil {
			l.Errorf("AssignGashaponLuckyUser update failed, err: %v", err)
			return nil, errorx.FromMySQLError(err)
		}
	} else {
		row := &gamemodelnc.GashaponLuckyAssignments{
			ConfigId:   in.ConfigId,
			PrizeIndex: in.PrizeIndex,
			UserId:     in.UserId,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if err := l.svcCtx.DB.Create(row).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				// 并发 upsert 竞争：重读后更新
				if err := l.svcCtx.DB.Model(&gamemodelnc.GashaponLuckyAssignments{}).
					Where("config_id = ? AND prize_index = ?", in.ConfigId, in.PrizeIndex).
					Updates(map[string]interface{}{"user_id": in.UserId, "updated_at": time.Now()}).Error; err != nil {
					l.Errorf("AssignGashaponLuckyUser upsert retry failed, err: %v", err)
					return nil, errorx.FromMySQLError(err)
				}
			} else {
				l.Errorf("AssignGashaponLuckyUser insert failed, err: %v", err)
				return nil, errorx.FromMySQLError(err)
			}
		}
	}
	l.Infof("AssignGashaponLuckyUser assigned, configId: %d, prizeIndex: %d, userId: %d", in.ConfigId, in.PrizeIndex, in.UserId)
	return &play.AssignGashaponLuckyUserResp{Status: "ASSIGNED"}, nil
}

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

type GetGashaponLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGashaponLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGashaponLogic {
	return &GetGashaponLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetGashapon 扭蛋详情：配置 + 特别奖进度（sent/quantity）+ 余票/总票数/参与人数/状态。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *GetGashaponLogic) GetGashapon(in *play.GetGashaponReq) (*play.GetGashaponResp, error) {
	if in.ConfigId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}

	cfg, err := l.svcCtx.GashaponCfgModel.FindOne(l.ctx, in.ConfigId)
	if err != nil {
		if errors.Is(err, gamemodel.ErrNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "扭蛋配置不存在")
		}
		l.Errorf("GetGashapon query config failed, configId: %d, err: %v", in.ConfigId, err)
		return nil, errorx.FromMySQLError(err)
	}

	var st gamemodelnc.GashaponState
	if err := l.svcCtx.DB.Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "扭蛋状态不存在")
		}
		l.Errorf("GetGashapon query state failed, configId: %d, err: %v", in.ConfigId, err)
		return nil, errorx.FromMySQLError(err)
	}

	// 总票数 = Σ entries quantity（全部有限量）
	var total int64
	if err := l.svcCtx.DB.Model(&gamemodel.GashaponEntries{}).
		Where("config_id = ?", in.ConfigId).
		Select("COALESCE(SUM(quantity), 0)").Scan(&total).Error; err != nil {
		l.Errorf("GetGashapon sum entries failed, configId: %d, err: %v", in.ConfigId, err)
		return nil, errorx.FromMySQLError(err)
	}

	prizes, err := gashaponParsePrizes(cfg.SpecialPrizesJson)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "扭蛋特别奖配置无效")
	}
	state, err := gashaponParseState(st.ParticipantsJson, st.RecordsJson, st.SentJson, st.AwardsJson)
	if err != nil {
		return nil, errorx.NewCodeError(errorx.CodeGameConfigInvalid, "扭蛋状态数据异常")
	}

	resp := &play.GetGashaponResp{
		ConfigId:         cfg.Id,
		Name:             cfg.Name,
		Price:            utils.FenToYuan(cfg.PriceFen),
		RemainingTickets: st.RemainingTickets,
		TotalTickets:     total,
		Participants:     int64(len(state.Participants)),
		Status:           st.Status,
	}
	for _, p := range prizes {
		item := &play.PrizeItem{Kind: p.PrizeKind, Points: p.PointsAmount, CouponTemplateNo: p.CouponTemplateNo}
		switch p.PrizeKind {
		case prizeCash:
			item.Amount = utils.FenToYuan(p.CashFen)
		case prizeCoupon:
			item.CouponName = couponNameOf(l.ctx, l.svcCtx, p.CouponTemplateNo)
		}
		var sent int64
		if int(p.PrizeIndex) < len(state.Sent) {
			sent = int64(state.Sent[p.PrizeIndex])
		}
		resp.SpecialPrizes = append(resp.SpecialPrizes, &play.GashaponSpecialPrizeItem{
			PrizeIndex:  int64(p.PrizeIndex),
			SpecialKind: p.SpecialKind,
			Quantity:    int64(p.Quantity),
			Sent:        sent,
			StageOrder:  int64(p.StageOrder),
			Prize:       item,
		})
	}
	return resp, nil
}

type GrantGashaponPrizesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantGashaponPrizesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantGashaponPrizesLogic {
	return &GrantGashaponPrizesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantGashaponPrizes SAGA 06 正向：发放本 gid 触发的特别奖（随 05 落 awards_json，
// 按 requestId 精确定位；无特别奖时为空集、直接成功）。
// 逐奖幂等发放（grant_no = "gcp_"+gid+"#"+idx，确定性派生）：
//   - 现金 → user.CreditWallet（kind=1，requestId=grant_no 幂等）
//   - 积分 → user.CreditWallet（kind=4，requestId=grant_no 幂等）
//   - 券   → 本地插入用户券（coupon_no="uc_"+grant_no，uk 撞键=已发过）
//
// regular 奖（扭蛋抽中条目）只落记录展示、不在此发放（条目无 prize_kind，
// 形状同 draw_pool_entries 的资产化由 draw 域负责；扭蛋条目仅展示+特别奖实发）。
//
// 铁律：**永不 Aborted**（奖品先于捕获，撤单不回退奖品，与 dig GrantDigPrizes 对齐）；
// 失败只回 Internal → DTM 退避重试，直到发放收敛。
func (l *GrantGashaponPrizesLogic) GrantGashaponPrizes(in *play.GrantGashaponPrizesReq) (*play.GrantGashaponPrizesResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 {
		return nil, status.Error(codes.Internal, "code=400 msg=请求参数非法")
	}

	// 特别奖由 05 正向落库（06 在其后执行）；缺失=异常，Internal 重试
	var st gamemodelnc.GashaponState
	if err := l.svcCtx.DB.Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("GrantGashaponPrizes state not found, configId: %d", in.ConfigId)
			return nil, status.Error(codes.Internal, fmt.Sprintf("code=%d msg=扭蛋状态不存在", errorx.CodeGameConfigInvalid))
		}
		l.Errorf("GrantGashaponPrizes query state failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	state, err := gashaponParseState(st.ParticipantsJson, st.RecordsJson, st.SentJson, st.AwardsJson)
	if err != nil {
		l.Errorf("GrantGashaponPrizes parse state failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	awards := gashaponAwardsOfRequest(state.Awards, in.RequestId)
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
	// grant_no = gcp_{gid}#{idx}（GrantAll 内派生）：DTM 重试/补偿重放命中同一键，
	// CreditWallet requestId 幂等 + 券 uk 撞键 = 已发过读回成功。
	if err := playkit.GrantAll(l.ctx, l.svcCtx, in.RequestId, "gcp", prizes); err != nil {
		// 永不 Aborted：失败 Internal → DTM 重试，grant_no 幂等收敛
		l.Errorf("GrantGashaponPrizes grant failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	// 高1 修复：发奖落库标记（04 补偿退款前置检查依据）；Internal 失败 → DTM 重试 06 幂等收敛
	if len(prizes) > 0 {
		if err := playkit.MarkGranted(l.ctx, l.svcCtx, in.RequestId, in.UserId); err != nil {
			l.Errorf("GrantGashaponPrizes mark failed, requestId: %s, err: %v", in.RequestId, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	l.Infof("GrantGashaponPrizes success, requestId: %s, awards: %d", in.RequestId, len(awards))
	return &play.GrantGashaponPrizesResp{Status: "GRANTED"}, nil
}

type GrantGashaponPrizesRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantGashaponPrizesRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantGashaponPrizesRevertLogic {
	return &GrantGashaponPrizesRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GrantGashaponPrizesRevert SAGA 06 补偿：空操作（特别奖先于捕获且不追回，
// 与 dig 同款决策；补偿注册仅为防御，即使调用也幂等返回）
func (l *GrantGashaponPrizesRevertLogic) GrantGashaponPrizesRevert(in *play.GrantGashaponPrizesReq) (*play.GrantGashaponPrizesResp, error) {
	l.Infof("GrantGashaponPrizesRevert noop, requestId: %s", in.RequestId)
	return &play.GrantGashaponPrizesResp{Status: "GRANTED"}, nil
}

type ListGashaponRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListGashaponRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListGashaponRecordsLogic {
	return &ListGashaponRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListGashaponRecords 扭蛋记录：requestId 非空=精确查一条（购买轮询），空=分页列表。
// 非 SAGA RPC：业务错误 CodeError（跨服务码不失真）。
func (l *ListGashaponRecordsLogic) ListGashaponRecords(in *play.ListGashaponRecordsReq) (*play.ListGashaponRecordsResp, error) {
	if in.UserId <= 0 {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "请求参数非法")
	}

	// 精确查询（轮询开奖结果）
	if in.RequestId != "" {
		var rec gamemodelnc.GashaponRecords
		if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", in.RequestId, in.UserId).
			First(&rec).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// requestId 未命中是正常情况（幂等快路径/轮询尚未出记录）：
				// 返回空列表而非业务错误，避免每笔新购买都打 error 日志。
				return &play.ListGashaponRecordsResp{Items: []*play.GashaponRecordItem{}, Total: 0}, nil
			}
			l.Errorf("ListGashaponRecords query failed, err: %v", err)
			return nil, err
		}
		return &play.ListGashaponRecordsResp{Items: []*play.GashaponRecordItem{l.buildItem(&rec)}, Total: 1}, nil
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
	if err := l.svcCtx.DB.Model(&gamemodelnc.GashaponRecords{}).
		Where("user_id = ?", in.UserId).Count(&total).Error; err != nil {
		l.Errorf("ListGashaponRecords count failed, err: %v", err)
		return nil, err
	}
	var recs []gamemodelnc.GashaponRecords
	if err := l.svcCtx.DB.Where("user_id = ?", in.UserId).
		Order("id DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).
		Find(&recs).Error; err != nil {
		l.Errorf("ListGashaponRecords page failed, err: %v", err)
		return nil, err
	}
	items := make([]*play.GashaponRecordItem, 0, len(recs))
	for i := range recs {
		items = append(items, l.buildItem(&recs[i]))
	}
	return &play.ListGashaponRecordsResp{Items: items, Total: total}, nil
}

// buildItem 记录行 → 响应条目（金额转元；entries_json → 结果条目）
func (l *ListGashaponRecordsLogic) buildItem(rec *gamemodelnc.GashaponRecords) *play.GashaponRecordItem {
	return &play.GashaponRecordItem{
		RecordId:  rec.Id,
		RequestId: rec.RequestId,
		ConfigId:  rec.ConfigId,
		Count:     rec.Count,
		Entries:   gashaponOutcomesToGame(gashaponParseOutcomes(rec.EntriesJson)),
		Price:     utils.FenToYuan(rec.PriceFen),
		Status:    rec.Status,
		CreatedAt: rec.CreatedAt.Unix(),
	}
}

type LockGashaponLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLockGashaponLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LockGashaponLogic {
	return &LockGashaponLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LockGashapon SAGA 03 正向：barrier 事务内
//
//	① 报价只读校验（营销 GetQuote sub-call：userId/grossFen 取报价行）；
//	② state FOR UPDATE（uk config_id 串行化同机购买；种子缺失防御建行，总票数=Σ entries quantity）；
//	③ 余票校验（count≤0 → 3028；status 非 OPEN / remaining<count → 3025）；
//	④ 价格一致（3018 复用：quote.gross == 单价×count == 请求总额）；
//	⑤ 报价锁（营销 LockQuote sub-call，置于全部玩法校验之后）；
//	⑥ participants+=count、records 追加、余票-=count。
//
// 报价锁放在最后：玩法自身校验失败回滚时不触碰营销锁（营销事务已独立提交，
// 无法随本事务回滚），与旧仓「锁与业务同事务」的最终状态一致——自中止后报价保持 OPEN。
// userId 取报价行（网关 QuoteCheckout 落库），不信请求参数。
// 错误语义（SAGA 分支铁律）：业务失败 → codes.Aborted（触发补偿）；系统错误 → codes.Internal（DTM 重试）。
func (l *LockGashaponLogic) LockGashapon(in *play.LockGashaponReq) (*play.LockGashaponResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	outCount := in.Count
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "LockGashapon", func(db *gorm.DB) error {

		// ① 报价只读校验（营销 GetQuote sub-call；quote 不存在 → Aborted）
		quote, err := playkit.LoadQuote(l.ctx, l.svcCtx, in.QuoteToken)
		if err != nil {
			return err
		}
		userId := quote.UserID

		// 扭蛋配置（纯种子配置只读，不走缓存锁）
		var cfg gamemodel.GashaponConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋配置不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		if cfg.Status != 1 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋配置不可用", errorx.CodeGameConfigInvalid))
		}

		// 特别奖配置（sent 长度=特别奖数；防御建行时预置 sent）
		prizes, err := gashaponParsePrizes(cfg.SpecialPrizesJson)
		if err != nil || len(prizes) == 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋特别奖配置无效", errorx.CodeGameConfigInvalid))
		}

		// ② state FOR UPDATE（uk(config_id) 串行化同机并发购买）
		var st gamemodelnc.GashaponState
		err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 防御：种子缺失时建行（总票数=Σ entries quantity；全部有限量，-1 非法）
			var entries []gamemodel.GashaponEntries
			if err := db.Where("config_id = ?", in.ConfigId).Find(&entries).Error; err != nil {
				return err
			}
			var total int64
			for _, e := range entries {
				if e.Quantity < 0 {
					return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋条目必须有限量", errorx.CodeGameConfigInvalid))
				}
				total += e.Quantity
			}
			if total <= 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋总票数非法", errorx.CodeGameConfigInvalid))
			}
			sent := make([]int32, len(prizes))
			st = gamemodelnc.GashaponState{
				ConfigId:         in.ConfigId,
				ParticipantsJson: "{}",
				RecordsJson:      "[]",
				SentJson:         gashaponSentJSON(sent),
				AwardsJson:       "[]",
				RemainingTickets: total,
				Status:           gashaponStateOpen,
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
		if st.Status != gashaponStateOpen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋已结算或状态不满足锁定", errorx.CodeGashaponNoTickets))
		}

		// ③ 余票校验
		if in.Count <= 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=抽数非法", errorx.CodeGashaponCountInvalid))
		}
		if st.RemainingTickets < in.Count {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋余票不足", errorx.CodeGashaponNoTickets))
		}

		// ④ 价格一致（锁内重算）：单价×count == 报价 gross == 请求总额
		if quote.GrossFen != cfg.PriceFen*in.Count || in.TotalPriceFen != cfg.PriceFen*in.Count {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋价格校验不一致", errorx.CodeDigPriceMismatch))
		}

		// ⑤ 报价锁（营销 LockQuote sub-call，置于玩法校验之后；幂等：newLocked=false=已锁重发 → 业务已成功，读参与记录如实返回）
		newLocked, err := playkit.LockQuote(l.ctx, l.svcCtx, in.RequestId, in.QuoteToken)
		if err != nil {
			return err
		}
		if !newLocked {
			l.Infof("LockGashapon idempotent: quote already locked, quoteToken: %s", in.QuoteToken)
			outCount = l.countOfRequest(db, in.ConfigId, in.RequestId, in.Count)
			return nil
		}

		// ⑥ participants+=count、records 追加、余票-=count
		state, err := gashaponParseState(st.ParticipantsJson, st.RecordsJson, st.SentJson, st.AwardsJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋状态数据异常", errorx.CodeGameConfigInvalid))
		}
		state.Participants[userId] += in.Count
		state.Records = append(state.Records, core.GashaponParticipation{
			UserID:    userId,
			RequestID: in.RequestId,
			Count:     in.Count,
		})
		if err := db.Model(&gamemodelnc.GashaponState{}).Where("id = ?", st.Id).
			Updates(map[string]interface{}{
				"participants_json": gashaponParticipantsJSON(state.Participants),
				"records_json":      gashaponRecordsJSON(state.Records),
				"remaining_tickets": st.RemainingTickets - in.Count,
				"status":            gashaponStateOpen,
				"updated_at":        time.Now(),
			}).Error; err != nil {
			return err
		}
		outCount = in.Count
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("LockGashapon aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("LockGashapon failed, quoteToken: %s, err: %v", in.QuoteToken, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（正向已成功）：读库如实返回
		outCount = l.countOfRequest(l.svcCtx.DB, in.ConfigId, in.RequestId, in.Count)
	}
	l.Infof("LockGashapon success, quoteToken: %s, count: %d", in.QuoteToken, outCount)
	return &play.LockGashaponResp{Count: outCount, Status: "LOCKED"}, nil
}

// countOfRequest 幂等/屏障跳过路径：按 requestId 读参与记录如实返回；未找到 → 回退请求值
func (l *LockGashaponLogic) countOfRequest(db *gorm.DB, configId int64, requestId string, fallback int64) int64 {
	var st gamemodelnc.GashaponState
	if err := db.Where("config_id = ?", configId).First(&st).Error; err != nil {
		return fallback
	}
	state, err := gashaponParseState(st.ParticipantsJson, st.RecordsJson, st.SentJson, st.AwardsJson)
	if err != nil {
		return fallback
	}
	for _, r := range state.Records {
		if r.RequestID == requestId {
			return r.Count
		}
	}
	return fallback
}

type ReleaseGashaponLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReleaseGashaponLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReleaseGashaponLogic {
	return &ReleaseGashaponLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ReleaseGashapon SAGA 03 补偿：barrier 事务内
//
//	① 报价 FOR UPDATE 先行——LOCKED ⇔ 本 gid 确实锁定抽数（需回滚）；OPEN ⇒ 正向
//	   已整体回滚（如 3025 余票不足），参与记录保持原样——绝不误回滚他人抽数；
//	② 按 requestId 弹出记录 + participants-=count + 余票+=count（仅 LOCKED 时；
//	   记录已弹出则幂等跳过）。这是参与记录与余票的唯一回滚点
//	   （05 补偿只撤销记录/特别奖/sent/库存，不碰余票——防双扣）；
//	③ 回退券（LOCKED 且 locked_by=本报价 → AVAILABLE）；
//	④ 报价 LOCKED/OPEN→RELEASED（幂等；USED 不回退）。
//
// 幂等：报价 RELEASED/USED/EXPIRED → 直接成功。
func (l *ReleaseGashaponLogic) ReleaseGashapon(in *play.ReleaseGashaponReq) (*play.ReleaseGashaponResp, error) {
	if in.RequestId == "" || in.QuoteToken == "" || in.ConfigId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "releaseGashapon", func(db *gorm.DB) error {

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
			l.Infof("ReleaseGashapon idempotent: quote status %s, quoteToken: %s", q.GetStatus(), in.QuoteToken)
			return nil
		}

		// ② 弹出参与记录 + participants-=count + 余票+=count：仅当报价 LOCKED（正向提交过锁定）时执行
		if q.GetStatus() == "LOCKED" {
			var st gamemodelnc.GashaponState
			err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("config_id = ?", in.ConfigId).First(&st).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil {
				state, err := gashaponParseState(st.ParticipantsJson, st.RecordsJson, st.SentJson, st.AwardsJson)
				if err == nil {
					removed := int64(0)
					kept := make([]core.GashaponParticipation, 0, len(state.Records))
					for _, r := range state.Records {
						if r.RequestID == in.RequestId {
							removed = r.Count
							continue
						}
						kept = append(kept, r)
					}
					if removed > 0 {
						// 回退参与数（归零即删除键——参与人数=len(map)，0 值键会虚增人数）
						state.Participants[q.GetUserId()] -= removed
						if state.Participants[q.GetUserId()] <= 0 {
							delete(state.Participants, q.GetUserId())
						}
						if err := db.Model(&gamemodelnc.GashaponState{}).Where("id = ?", st.Id).
							Updates(map[string]interface{}{
								"participants_json": gashaponParticipantsJSON(state.Participants),
								"records_json":      gashaponRecordsJSON(kept),
								"remaining_tickets": st.RemainingTickets + removed,
								"updated_at":        time.Now(),
							}).Error; err != nil {
							return err
						}
						l.Infof("ReleaseGashapon record popped, requestId: %s, count: %d", in.RequestId, removed)
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
			l.Errorf("ReleaseGashapon aborted, quoteToken: %s, reason: %s", in.QuoteToken, s.Message())
			return nil, err
		}
		l.Errorf("ReleaseGashapon failed, quoteToken: %s, err: %v", in.QuoteToken, err)
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
		l.Infof("ReleaseGashapon success (readback), quoteToken: %s, status: %s", in.QuoteToken, statusOut)
		return &play.ReleaseGashaponResp{Count: in.Count, Status: statusOut}, nil
	}
	l.Infof("ReleaseGashapon success, quoteToken: %s", in.QuoteToken)
	return &play.ReleaseGashaponResp{Count: in.Count, Status: "RELEASED"}, nil
}

// errGashaponRecordDup 事务内哨兵：同 requestId 的扭蛋记录已存在（另一 gid 已结算）
var errGashaponRecordDup = errors.New("gashapon record duplicated")

type SettleGashaponLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleGashaponLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleGashaponLogic {
	return &SettleGashaponLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleGashapon SAGA 05 正向：barrier 事务内
//
//	① state FOR UPDATE（必须 OPEN）；sent 初始化/长度守卫（3026 配置变更）；
//	② 抽 count 个 regular 奖（gashapon_entries，每抽消耗一个条目实例，按条目余量
//	   递减——与 server_v2 persistent_draw 同款"每票耗一实例"语义）；
//	③ 插记录（uk request_id 撞键=已结算 → 读回幂等成功）；
//	④ 分阶段 LUCKY 判定（drawn=total-remaining_after_lock，seed=gid，含本次票据）；
//	⑤ 清空抽（remaining==0）→ 终局 LAST/MVP/LUCKY + status→SETTLED；
//	⑥ 新特别奖标 requestId 追加落库，sent 推进持久化。
//
// 错误语义：业务失败 → codes.Aborted；系统错误 → codes.Internal。
func (l *SettleGashaponLogic) SettleGashapon(in *play.SettleGashaponReq) (*play.SettleGashaponResp, error) {
	if in.RequestId == "" || in.UserId <= 0 || in.ConfigId <= 0 || in.Count <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	// 幂等重放快路径：同 requestId 记录已存在 → 读回成功（先于业务校验，M11b 教训）
	var existing gamemodelnc.GashaponRecords
	if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&existing).Error; err == nil {
		return l.readBackRecord(in.RequestId, in.UserId)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Errorf("SettleGashapon fast path query failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var outRecordId int64
	outOutcomes := make([]gashaponOutcome, 0, in.Count)
	outSpecial := make([]core.GashaponAward, 0)
	outSettled := false
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleGashapon", func(db *gorm.DB) error {

		// ① state FOR UPDATE（uk config_id 串行化；种子缺失 → 3020）
		var st gamemodelnc.GashaponState
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋状态不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		if st.Status != gashaponStateOpen {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋已结算或状态不满足结算", errorx.CodeGashaponNoTickets))
		}
		if st.RemainingTickets < 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋状态数据异常", errorx.CodeGameConfigInvalid))
		}

		// 扭蛋配置 + 特别奖（sent 长度与配置守卫：3026 配置变更）
		var cfg gamemodel.GashaponConfigs
		if err := db.Where("id = ?", in.ConfigId).First(&cfg).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋配置不存在", errorx.CodeGameConfigInvalid))
			}
			return err
		}
		prizes, err := gashaponParsePrizes(cfg.SpecialPrizesJson)
		if err != nil || len(prizes) == 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋特别奖配置无效", errorx.CodeGameConfigInvalid))
		}

		// 状态 JSON → 内存（票据区间/参与数/已发计数/已发奖）
		state, err := gashaponParseState(st.ParticipantsJson, st.RecordsJson, st.SentJson, st.AwardsJson)
		if err != nil {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋状态数据异常", errorx.CodeGameConfigInvalid))
		}
		if len(state.Sent) == 0 {
			state.Sent = make([]int32, len(prizes))
		}
		if len(state.Sent) != len(prizes) {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋配置变更，请重置", errorx.CodeGashaponConfigChanged))
		}

		// ② 抽 count 个 regular 奖：总票数=Σ quantity（-1 非法）；奖池余量≥count
		var entries []gamemodel.GashaponEntries
		if err := db.Where("config_id = ?", in.ConfigId).Find(&entries).Error; err != nil {
			return err
		}
		var total int64
		remainingByName := map[int64]int64{}
		pool := make([]core.PrizeEntry, 0, len(entries))
		for _, e := range entries {
			if e.Quantity < 0 {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋条目必须有限量", errorx.CodeGameConfigInvalid))
			}
			total += e.Quantity
			remainingByName[e.Id] = e.Remaining
			if e.Remaining > 0 {
				pool = append(pool, core.PrizeEntry{Id: e.Id, Name: e.Name, Weight: int(e.Weight)})
			}
		}
		if total <= 0 {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋总票数非法", errorx.CodeGameConfigInvalid))
		}
		// ②.5 余量校验：全部条目 remaining 之和 ≥ count（有货条目数不含分摊余量，
		//      用条目数判断会在"少数条目余量充足"时误判余量不足）
		var totalRemaining int64
		for _, e := range pool {
			totalRemaining += remainingByName[e.Id]
		}
		if totalRemaining < in.Count {
			return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋奖池余量不足", errorx.CodeGashaponNoTickets))
		}

		// 每抽耗一个条目实例（remaining 递减，0 移出候选——每票一实例，与 server_v2 同款）
		// 确定性抽取：seed=gid，index=第几抽（同一 (entries, gid, count) 结果确定，可回放核对）
		entryByName := map[int64]gamemodel.GashaponEntries{}
		for _, e := range entries {
			entryByName[e.Id] = e
		}
		for i := int64(0); i < in.Count; i++ {
			picked := core.PickPrizeDeterministic(pool, []byte(in.RequestId), "gashapon-regular", uint32(i))
			if picked == nil {
				return status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=扭蛋奖池余量不足", errorx.CodeGashaponNoTickets))
			}
			if err := db.Model(&gamemodel.GashaponEntries{}).
				Where("id = ? AND remaining > 0", picked.Id).
				Update("remaining", gorm.Expr("remaining - 1")).Error; err != nil {
				return err
			}
			remainingByName[picked.Id]--
			outOutcomes = append(outOutcomes, gashaponOutcome{
				EntryId: picked.Id,
				Name:    picked.Name,
				IsPrize: entryByName[picked.Id].IsPrize == 1,
			})
			// 候选池同步：余量归零移出
			pool = filterGashaponPool(pool, picked.Id, remainingByName)
		}

		// ③ 插记录（uk request_id 幂等；撞键 = 已结算读回）
		priceFen := int64(0)
		if in.Price != "" {
			f, err := utils.YuanToFen(in.Price)
			if err != nil || f < 0 {
				return status.Error(codes.Aborted, "code=400 msg=实付价格无效")
			}
			priceFen = f
		}
		rec := &gamemodelnc.GashaponRecords{
			RequestId:   in.RequestId,
			UserId:      in.UserId,
			ConfigId:    in.ConfigId,
			Count:       in.Count,
			EntriesJson: gashaponOutcomesJSON(outOutcomes),
			PriceFen:    priceFen,
			Status:      gashaponRecordNormal,
			CreatedAt:   time.Now(),
		}
		if err := db.Create(rec).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errGashaponRecordDup
			}
			return err
		}
		outRecordId = rec.Id

		// ④ 分阶段 LUCKY（drawn=已抽总票数，含本次——03 已扣余票；seed=gid 确定性）
		assignees := l.loadAssignees(db, in.ConfigId)
		drawn := total - st.RemainingTickets
		staged := core.StagedGashaponAwards(state, prizes, total, drawn, []byte(in.RequestId), assignees)

		// ⑤ 清空抽 → 终局 LAST/MVP/LUCKY 剩余 + SETTLED
		var final []core.GashaponAward
		if st.RemainingTickets == 0 {
			final = core.FinalGashaponAwards(state, prizes, in.UserId, []byte(in.RequestId), assignees)
			outSettled = true
		}

		// ⑥ 新特别奖标 requestId 追加落库 + sent 持久化
		newAwards := make([]core.GashaponAward, 0, len(staged)+len(final))
		for _, a := range append(append([]core.GashaponAward{}, staged...), final...) {
			a.RequestID = in.RequestId
			newAwards = append(newAwards, a)
		}
		outSpecial = newAwards
		updates := map[string]interface{}{
			"sent_json":   gashaponSentJSON(state.Sent),
			"awards_json": gashaponAwardsJSON(append(state.Awards, newAwards...)),
			"updated_at":  time.Now(),
		}
		if outSettled {
			updates["status"] = gashaponStateSettled
		}
		if err := db.Model(&gamemodelnc.GashaponState{}).Where("id = ?", st.Id).
			Updates(updates).Error; err != nil {
			return err
		}
		if outSettled {
			l.Infof("SettleGashapon settled, requestId: %s, special: %d", in.RequestId, len(newAwards))
		}
		return nil
	})
	if errors.Is(err, errGashaponRecordDup) {
		// 同 requestId 已结算（另一 gid 已成功）：返回已有记录，幂等成功
		return l.readBackRecord(in.RequestId, in.UserId)
	}
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleGashapon aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleGashapon failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过（分支已成功/重试）：读库如实返回已有记录
		return l.readBackRecord(in.RequestId, in.UserId)
	}
	l.Infof("SettleGashapon success, requestId: %s, recordId: %d, settled: %v, special: %d",
		in.RequestId, outRecordId, outSettled, len(outSpecial))
	return &play.SettleGashaponResp{
		RecordId:      outRecordId,
		RegularPrizes: gashaponOutcomesToGame(outOutcomes),
		SpecialAwards: l.specialAwardItems(outSpecial),
		Settled:       outSettled,
		Status:        "SETTLED",
	}, nil
}

// filterGashaponPool 候选池同步：抽中的条目余量归零后移出（其余原样保留）
func filterGashaponPool(pool []core.PrizeEntry, pickedId int64, remainingByName map[int64]int64) []core.PrizeEntry {
	if remainingByName[pickedId] > 0 {
		return pool
	}
	out := make([]core.PrizeEntry, 0, len(pool))
	for _, p := range pool {
		if p.Id == pickedId {
			continue
		}
		out = append(out, p)
	}
	return out
}

// loadAssignees 事务内读幸运指派：prize_index → 指派用户（0 值条目过滤）
func (l *SettleGashaponLogic) loadAssignees(db *gorm.DB, configId int64) map[int32]int64 {
	var assigns []gamemodelnc.GashaponLuckyAssignments
	if err := db.Where("config_id = ?", configId).Find(&assigns).Error; err != nil {
		l.Errorf("SettleGashapon load assignments failed, configId: %d, err: %v", configId, err)
		return map[int32]int64{}
	}
	out := make(map[int32]int64, len(assigns))
	for _, a := range assigns {
		out[int32(a.PrizeIndex)] = a.UserId
	}
	return out
}

// gashaponOutcomesToGame regular 奖结果 → 响应条目
func gashaponOutcomesToGame(outcomes []gashaponOutcome) []*play.GashaponEntryOutcome {
	items := make([]*play.GashaponEntryOutcome, 0, len(outcomes))
	for _, o := range outcomes {
		items = append(items, &play.GashaponEntryOutcome{Name: o.Name, IsPrize: o.IsPrize})
	}
	return items
}

// specialAwardItems 特别奖 → PrizeItem 列表（券名尽力解析，展示用）
func (l *SettleGashaponLogic) specialAwardItems(awards []core.GashaponAward) []*play.PrizeItem {
	items := make([]*play.PrizeItem, 0, len(awards))
	for _, a := range awards {
		item := &play.PrizeItem{Kind: a.PrizeKind, Points: a.PointsAmount, CouponTemplateNo: a.CouponTemplateNo}
		switch a.PrizeKind {
		case prizeCash:
			item.Amount = utils.FenToYuan(a.CashFen)
		case prizeCoupon:
			item.CouponName = couponNameOf(l.ctx, l.svcCtx, a.CouponTemplateNo)
		}
		items = append(items, item)
	}
	return items
}

// readBackRecord 幂等/屏障跳过路径：读已有记录 + 本 gid 特别奖 + 当前状态如实返回
func (l *SettleGashaponLogic) readBackRecord(requestId string, userId int64) (*play.SettleGashaponResp, error) {
	var rec gamemodelnc.GashaponRecords
	if err := l.svcCtx.DB.Where("request_id = ? AND user_id = ?", requestId, userId).
		First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			l.Errorf("readBackRecord: record not found, requestId: %s", requestId)
			return nil, status.Error(codes.Internal, "扭蛋记录不存在")
		}
		l.Errorf("readBackRecord failed, requestId: %s, err: %v", requestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	settled := false
	var special []core.GashaponAward
	var st gamemodelnc.GashaponState
	if err := l.svcCtx.DB.Where("config_id = ?", rec.ConfigId).First(&st).Error; err == nil {
		settled = st.Status == gashaponStateSettled
		if state, err := gashaponParseState(st.ParticipantsJson, st.RecordsJson, st.SentJson, st.AwardsJson); err == nil {
			special = gashaponAwardsOfRequest(state.Awards, requestId)
		}
	}
	l.Infof("readBackRecord: requestId: %s, recordId: %d, settled: %v, special: %d", requestId, rec.Id, settled, len(special))
	return &play.SettleGashaponResp{
		RecordId:      rec.Id,
		RegularPrizes: gashaponOutcomesToGame(gashaponParseOutcomes(rec.EntriesJson)),
		SpecialAwards: l.specialAwardItems(special),
		Settled:       settled,
		Status:        "SETTLED",
	}, nil
}

type SettleGashaponRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleGashaponRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleGashaponRevertLogic {
	return &SettleGashaponRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SettleGashaponRevert SAGA 05 补偿：只撤销本分支造成的影响（dig/fullcar 先例——唯一回滚点防双扣）：
//
//	① 记录 → 2 已撤销（无记录=05 正向从未成功，跳过）；
//	② regular 奖库存回补（按记录 entries_json 逐条回补；同名条目多行时
//	   每单位回补一行 LIMIT 1——总库存按名复原，与抽中行号无关）；
//	③ awards_json 按 requestId 删本 gid 触发的特别奖、sent 逐奖回退；SETTLED→OPEN。
//
// 参与记录弹出与余票回补由 03 补偿（ReleaseGashapon）唯一负责，本分支不碰余票。
func (l *SettleGashaponRevertLogic) SettleGashaponRevert(in *play.SettleGashaponReq) (*play.SettleGashaponResp, error) {
	if in.RequestId == "" || in.UserId <= 0 {
		return nil, status.Error(codes.Aborted, "code=400 msg=请求参数非法")
	}

	var outRecordId int64
	businessRan, err := playkit.BarrierTx(l.ctx, l.svcCtx, "settleGashaponRevert", func(db *gorm.DB) error {

		// ① 记录 FOR UPDATE → 2 已撤销（盲更新，幂等；无记录=正向从未成功，跳过）
		var rec gamemodelnc.GashaponRecords
		err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", in.RequestId).First(&rec).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			outRecordId = rec.Id
			if rec.Status == gashaponRecordNormal {
				if err := db.Model(&gamemodelnc.GashaponRecords{}).
					Where("id = ? AND status = ?", rec.Id, gashaponRecordNormal).
					Update("status", gashaponRecordCanceled).Error; err != nil {
					return err
				}
			}
			// ② regular 奖库存回补：优先按条目主键精确定位；旧记录无主键回退按名回补
			//   一行（同名多行时总库存按名复原）
			for _, o := range gashaponParseOutcomes(rec.EntriesJson) {
				if o.EntryId > 0 {
					if err := db.Model(&gamemodel.GashaponEntries{}).
						Where("id = ?", o.EntryId).
						Update("remaining", gorm.Expr("remaining + 1")).Error; err != nil {
						return err
					}
					continue
				}
				if err := db.Model(&gamemodel.GashaponEntries{}).
					Where("config_id = ? AND name = ?", rec.ConfigId, o.Name).
					Order("id ASC").Limit(1).
					Update("remaining", gorm.Expr("remaining + 1")).Error; err != nil {
					return err
				}
			}
		}

		// ③ awards 按 requestId 删除 + sent 回退 + SETTLED→OPEN
		var st gamemodelnc.GashaponState
		err = db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_id = ?", in.ConfigId).First(&st).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			state, perr := gashaponParseState(st.ParticipantsJson, st.RecordsJson, st.SentJson, st.AwardsJson)
			updates := map[string]interface{}{"updated_at": time.Now()}
			if perr == nil {
				kept := make([]core.GashaponAward, 0, len(state.Awards))
				removed := 0
				for _, a := range state.Awards {
					if a.RequestID == in.RequestId {
						removed++
						// sent 逐奖回退（防御下限 0）
						if idx := int(a.PrizeIndex); idx >= 0 && idx < len(state.Sent) && state.Sent[idx] > 0 {
							state.Sent[idx]--
						}
						continue
					}
					kept = append(kept, a)
				}
				if removed > 0 {
					updates["awards_json"] = gashaponAwardsJSON(kept)
					updates["sent_json"] = gashaponSentJSON(state.Sent)
					l.Infof("SettleGashaponRevert awards removed, requestId: %s, removed: %d", in.RequestId, removed)
				}
			}
			if st.Status == gashaponStateSettled {
				updates["status"] = gashaponStateOpen
			}
			if len(updates) > 1 {
				if err := db.Model(&gamemodelnc.GashaponState{}).Where("id = ?", st.Id).
					Updates(updates).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Aborted {
			l.Errorf("SettleGashaponRevert aborted, requestId: %s, reason: %s", in.RequestId, s.Message())
			return nil, err
		}
		l.Errorf("SettleGashaponRevert failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !businessRan {
		// 屏障级跳过：读记录如实返回
		var rec gamemodelnc.GashaponRecords
		if err := l.svcCtx.DB.Where("request_id = ?", in.RequestId).First(&rec).Error; err == nil {
			outRecordId = rec.Id
		}
	}
	l.Infof("SettleGashaponRevert success, requestId: %s, recordId: %d", in.RequestId, outRecordId)
	return &play.SettleGashaponResp{RecordId: outRecordId, Status: "REVERTED"}, nil
}

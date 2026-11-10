// Package task M15c 任务玩家体系纯函数层：语义逐条移植自 server_v2 app/service/task/internal/biz
// （Board 内存快照 + 周期键 + 视图状态机 + 领取/签到判定 + 进度事件应用）。
// 错误哨兵沿用老项目语义；logic 层按上下文映射 errorx 3046-3054。
// 类型用 game 生成枚举与 ops 结构化奖励（D2：计划 blob 复用 ops.TaskPlan 解码）。
package task

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"gacha_srv/app/ops/ops"
	taskpb "gacha_srv/app/task/task"
)

var (
	ErrTaskNotFound   = errors.New("task not found")
	ErrTaskNotReady   = errors.New("task progress is below target")
	ErrTaskFinished   = errors.New("task cycle limit reached")
	ErrInvalidTask    = errors.New("invalid task")
	ErrDuplicateEvent = errors.New("task event already applied")
	ErrPlanConflict   = errors.New("task plan version conflict")
)

type Definition struct {
	ID               uint64
	Version          uint64
	Name             string
	Period           taskpb.TaskPeriod
	Metric           taskpb.TaskMetric
	Target           int64
	CycleLimit       uint32
	RewardTemplateID string
	RewardCount      uint32
	Reward           *ops.TaskRewardDefinition
}

type progressKey struct {
	userID uint64
	taskID uint64
	period string
}

type progressRow struct {
	amount  int64
	claimed uint32
}

// Board is the published task set and per-user progress.
// Listing does not write. Progress adds the event amount. CycleLimit always caps claims.
type Board struct {
	mu           sync.Mutex
	tasks        []Definition
	progress     map[progressKey]progressRow
	events       map[string]struct{}
	claims       map[string]Claim
	signIns      map[progressKey]signInRow
	passes       map[uint64]PassSeason
	passUsers    map[passUserKey]passProgress
	passEvents   map[string]struct{}
	passClaims   map[string]PassClaim
	memberTiers  []MemberTier
	members      map[uint64]memberState
	memberEvents map[string]struct{}
	memberClaims map[string]Claim
	now          func() time.Time
	location     *time.Location
}

type signInRow struct {
	lastDate string
	streak   uint32
}

type Claim struct {
	RewardTemplateID string
	RewardCount      uint32
	Claimed          uint32
}

func NewBoard(tasks []Definition, now func() time.Time) (*Board, error) {
	if now == nil {
		now = time.Now
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return nil, err
	}
	seen := map[uint64]struct{}{}
	copied := make([]Definition, 0, len(tasks))
	for _, task := range tasks {
		if task.ID == 0 || task.Version == 0 || task.Target <= 0 || task.Metric == taskpb.TaskMetric_TASK_METRIC_UNSPECIFIED || task.Period == taskpb.TaskPeriod_TASK_PERIOD_UNSPECIFIED || !validReward(task.Reward, task.RewardTemplateID, task.RewardCount) {
			return nil, ErrInvalidTask
		}
		if _, ok := seen[task.ID]; ok {
			return nil, ErrInvalidTask
		}
		seen[task.ID] = struct{}{}
		copied = append(copied, task)
	}
	sort.Slice(copied, func(i, j int) bool { return copied[i].ID < copied[j].ID })
	return &Board{tasks: copied, progress: map[progressKey]progressRow{}, events: map[string]struct{}{}, claims: map[string]Claim{}, signIns: map[progressKey]signInRow{}, now: now, location: loc}, nil
}

// List 内存模式分页列表（逻辑层走 ListFromState + 存储行）
func (b *Board) List(userID uint64, period taskpb.TaskPeriod, pageSize int, pageToken string) ([]View, string, error) {
	if userID == 0 {
		return nil, "", ErrInvalidTask
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	start := 0
	if pageToken != "" {
		var tokenID uint64
		if _, err := fmt.Sscanf(pageToken, "%d", &tokenID); err != nil {
			return nil, "", ErrInvalidTask
		}
		for i, task := range b.tasks {
			if task.ID > tokenID {
				start = i
				break
			}
			if i == len(b.tasks)-1 {
				start = len(b.tasks)
			}
		}
	}
	var views []View
	var last uint64
	for _, task := range b.tasks[start:] {
		if period != taskpb.TaskPeriod_TASK_PERIOD_UNSPECIFIED && task.Period != period {
			continue
		}
		views = append(views, b.view(userID, task))
		last = task.ID
		if len(views) == pageSize {
			break
		}
	}
	next := ""
	if len(views) == pageSize {
		for _, task := range b.tasks {
			if task.ID > last && (period == taskpb.TaskPeriod_TASK_PERIOD_UNSPECIFIED || task.Period == period) {
				next = fmt.Sprint(last)
				break
			}
		}
	}
	return views, next, nil
}

// ListFromState 存储模式分页视图（老项目 listFromState；progress/signIns 为全用户行，按 userID 键区分）
func ListFromState(tasks []Definition, userID uint64, when time.Time, period taskpb.TaskPeriod, pageSize int, pageToken string, state UserState) ([]View, string, error) {
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	progress := make(map[progressKey]progressRow, len(state.Progress))
	for _, row := range state.Progress {
		progress[progressKey{userID: row.UserID, taskID: row.TaskID, period: row.Period}] = progressRow{amount: row.Amount, claimed: row.Claimed}
	}
	signIns := make(map[progressKey]signInRow, len(state.SignIns))
	for _, row := range state.SignIns {
		signIns[progressKey{userID: row.UserID, taskID: row.TaskID}] = signInRow{lastDate: row.LastDate, streak: row.Streak}
	}
	start := 0
	if pageToken != "" {
		var tokenID uint64
		if _, err := fmt.Sscanf(pageToken, "%d", &tokenID); err != nil {
			return nil, "", ErrInvalidTask
		}
		start = len(tasks)
		for i, task := range tasks {
			if task.ID > tokenID {
				start = i
				break
			}
		}
	}
	views := make([]View, 0, pageSize)
	var last uint64
	for _, task := range tasks[start:] {
		if period != taskpb.TaskPeriod_TASK_PERIOD_UNSPECIFIED && task.Period != period {
			continue
		}
		views = append(views, viewFromState(userID, task, when, progress, signIns))
		last = task.ID
		if len(views) == pageSize {
			break
		}
	}
	next := ""
	if len(views) == pageSize {
		for _, task := range tasks {
			if task.ID > last && (period == taskpb.TaskPeriod_TASK_PERIOD_UNSPECIFIED || task.Period == period) {
				next = fmt.Sprint(last)
				break
			}
		}
	}
	return views, next, nil
}

type View struct {
	Definition Definition
	Progress   int64
	Claimed    uint32
	State      taskpb.TaskProgressState
}

func (b *Board) ApplyPaidSpend(userID, orderID uint64, businessType string, payable int64, currency string) error {
	return ApplyPaidSpendContext(b, userID, orderID, businessType, payable, currency)
}

// ApplyPaidSpend 付费事件过滤：business_type 仅 gacha_draw/memory_match、user/order>0、payable>0、
// currency=CNY 才转为 DRAW_SPEND_MINOR 事件 "pay:<orderID>"；否则静默跳过（老项目语义）。
// 网关购买挂钩（D9）直接调 ApplyContext，本函数留给单测与内存 Board 使用。
func ApplyPaidSpendContext(b *Board, userID, orderID uint64, businessType string, payable int64, currency string) error {
	if businessType != "gacha_draw" && businessType != "memory_match" {
		return nil
	}
	if userID == 0 || orderID == 0 || payable <= 0 || currency != "CNY" {
		return nil
	}
	return b.Apply(userID, fmt.Sprintf("pay:%d", orderID), taskpb.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR, payable)
}

func (b *Board) Apply(userID uint64, eventID string, metric taskpb.TaskMetric, amount int64) error {
	if userID == 0 || eventID == "" || len(eventID) > 64 || amount <= 0 || metric == taskpb.TaskMetric_TASK_METRIC_UNSPECIFIED || metric == taskpb.TaskMetric_TASK_METRIC_SIGN_IN {
		return ErrInvalidTask
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.events[eventID]; ok {
		return nil
	}
	when := b.now().In(b.location)
	for _, task := range b.tasks {
		if task.Metric != metric {
			continue
		}
		key := progressKey{userID: userID, taskID: task.ID, period: PeriodStart(task.Period, when)}
		row := b.progress[key]
		row.amount += amount
		b.progress[key] = row
	}
	b.events[eventID] = struct{}{}
	b.creditPassLocked(userID, metric, amount, eventID)
	if metric == taskpb.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR {
		b.creditMemberLocked(userID, amount, eventID)
	}
	return nil
}

// ApplyDeltaFor 存储模式事件增量计算（老项目 ApplyContext store 分支）：
// 匹配 metric 的任务进度行 + DRAW_SPEND_MINOR 时全部通行证 XP（封顶）与会员 spend 增量。
func ApplyDeltaFor(snapshot Snapshot, userID uint64, eventID string, metric taskpb.TaskMetric, amount int64, when time.Time) (ApplyDelta, error) {
	if userID == 0 || eventID == "" || len(eventID) > 64 || amount <= 0 || metric == taskpb.TaskMetric_TASK_METRIC_UNSPECIFIED || metric == taskpb.TaskMetric_TASK_METRIC_SIGN_IN {
		return ApplyDelta{}, ErrInvalidTask
	}
	delta := ApplyDelta{EventID: eventID}
	for _, task := range snapshot.Tasks {
		if task.Metric == metric {
			delta.Progress = append(delta.Progress, ProgressRow{UserID: userID, TaskID: task.ID, Period: PeriodStart(task.Period, when), Amount: amount})
		}
	}
	if metric == taskpb.TaskMetric_TASK_METRIC_DRAW_SPEND_MINOR {
		for _, season := range snapshot.Passes {
			if season.XPPerSpendMinor <= 0 || amount > (1<<62)/season.XPPerSpendMinor {
				continue
			}
			delta.Passes = append(delta.Passes, PassRow{UserID: userID, PassID: season.ID, XP: amount * season.XPPerSpendMinor, MaxXP: season.XPPerLevel * int64(season.MaxLevel)})
		}
		if len(snapshot.MemberTiers) > 0 {
			delta.Member = &MemberDelta{UserID: userID, SpendMinor: amount, Tiers: snapshot.MemberTiers}
		}
	}
	return delta, nil
}

func (b *Board) Claim(userID, taskID, version uint64, requestID string) (Claim, error) {
	if userID == 0 || taskID == 0 || version == 0 || requestID == "" || len(requestID) > 64 {
		return Claim{}, ErrInvalidTask
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var task Definition
	found := false
	for _, item := range b.tasks {
		if item.ID == taskID && item.Version == version {
			task = item
			found = true
			break
		}
	}
	if !found {
		return Claim{}, ErrTaskNotFound
	}
	if existing, ok := b.claims[requestID]; ok {
		return existing, nil
	}
	key := progressKey{userID: userID, taskID: task.ID, period: PeriodStart(task.Period, b.now().In(b.location))}
	row := b.progress[key]
	if task.Metric == taskpb.TaskMetric_TASK_METRIC_SIGN_IN {
		return b.claimSignIn(task, key, row, requestID)
	}
	if task.CycleLimit > 0 && row.claimed >= task.CycleLimit {
		return Claim{}, ErrTaskFinished
	}
	need := task.Target * int64(row.claimed+1)
	if row.amount < need {
		return Claim{}, ErrTaskNotReady
	}
	row.claimed++
	b.progress[key] = row
	claim := Claim{RewardTemplateID: task.RewardTemplateID, RewardCount: task.RewardCount, Claimed: row.claimed}
	b.claims[requestID] = claim
	return claim, nil
}

func (b *Board) SignIn(userID, taskID, version uint64) (uint32, string, bool, error) {
	if userID == 0 || taskID == 0 || version == 0 {
		return 0, "", false, ErrInvalidTask
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var task Definition
	found := false
	for _, item := range b.tasks {
		if item.ID == taskID && item.Version == version && item.Metric == taskpb.TaskMetric_TASK_METRIC_SIGN_IN {
			task = item
			found = true
			break
		}
	}
	if !found {
		return 0, "", false, ErrTaskNotFound
	}
	today := b.now().In(b.location).Format("2006-01-02")
	key := progressKey{userID: userID, taskID: task.ID}
	row := b.signIns[key]
	if row.lastDate == today {
		return row.streak, today, true, nil
	}
	if row.lastDate == PreviousDate(today, b.location) && row.streak > 0 {
		row.streak++
	} else {
		row.streak = 1
	}
	row.lastDate = today
	b.signIns[key] = row
	return row.streak, today, false, nil
}

func (b *Board) claimSignIn(task Definition, key progressKey, row progressRow, requestID string) (Claim, error) {
	signed := b.signIns[progressKey{userID: key.userID, taskID: key.taskID}]
	today := b.now().In(b.location).Format("2006-01-02")
	if signed.lastDate != today {
		return Claim{}, ErrTaskNotReady
	}
	if task.CycleLimit > 0 && row.claimed >= task.CycleLimit {
		return Claim{}, ErrTaskFinished
	}
	if int64(signed.streak) < task.Target {
		return Claim{}, ErrTaskNotReady
	}
	row.claimed++
	b.progress[key] = row
	claim := Claim{RewardTemplateID: task.RewardTemplateID, RewardCount: task.RewardCount, Claimed: row.claimed}
	b.claims[requestID] = claim
	return claim, nil
}

// PreviousDate 前一上海日历日（老项目 previousDate）
func PreviousDate(date string, loc *time.Location) string {
	parsed, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return ""
	}
	return parsed.AddDate(0, 0, -1).Format("2006-01-02")
}

func (b *Board) view(userID uint64, task Definition) View {
	when := b.now().In(b.location)
	return viewFromState(userID, task, when, b.progress, b.signIns)
}

// viewFromState 视图状态机（老项目 viewFromState）：when 须为上海时区时刻
func viewFromState(userID uint64, task Definition, when time.Time, progress map[progressKey]progressRow, signIns map[progressKey]signInRow) View {
	row := progress[progressKey{userID: userID, taskID: task.ID, period: PeriodStart(task.Period, when)}]
	value := row.amount
	if task.Metric == taskpb.TaskMetric_TASK_METRIC_SIGN_IN {
		value = int64(signIns[progressKey{userID: userID, taskID: task.ID}].streak)
	}
	state := taskpb.TaskProgressState_TASK_PROGRESS_STATE_IN_PROGRESS
	if task.CycleLimit > 0 && row.claimed >= task.CycleLimit {
		state = taskpb.TaskProgressState_TASK_PROGRESS_STATE_FINISHED
	} else if task.Metric == taskpb.TaskMetric_TASK_METRIC_SIGN_IN {
		signed := signIns[progressKey{userID: userID, taskID: task.ID}]
		if signed.lastDate == when.Format("2006-01-02") && value >= task.Target {
			state = taskpb.TaskProgressState_TASK_PROGRESS_STATE_CLAIMABLE
		}
	} else if row.amount >= task.Target*int64(row.claimed+1) {
		state = taskpb.TaskProgressState_TASK_PROGRESS_STATE_CLAIMABLE
	}
	return View{Definition: task, Progress: value, Claimed: row.claimed, State: state}
}

// PeriodStart 周期键：ONCE="once" / DAILY="2006-01-02" / WEEKLY=ISOWeek "%04d-W%02d"（when 须为上海时间）
func PeriodStart(period taskpb.TaskPeriod, when time.Time) string {
	switch period {
	case taskpb.TaskPeriod_TASK_PERIOD_DAILY:
		return when.Format("2006-01-02")
	case taskpb.TaskPeriod_TASK_PERIOD_WEEKLY:
		year, week := when.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", year, week)
	default:
		return "once"
	}
}

// 存储模式数据类型（老项目 store.go ApplyDelta/UserState；逻辑层与 repo 层共用）

type ApplyDelta struct {
	EventID  string
	Progress []ProgressRow
	Passes   []PassRow
	Member   *MemberDelta
}

type ApplyResult struct {
	Duplicate bool
	Member    *MemberRow
}

type MemberDelta struct {
	UserID     uint64
	SpendMinor int64
	Tiers      []MemberTier
}

type ProgressRow struct {
	UserID  uint64
	TaskID  uint64
	Period  string
	Amount  int64
	Claimed uint32
}

type SignInRow struct {
	UserID   uint64
	TaskID   uint64
	LastDate string
	Streak   uint32
}

type PassRow struct {
	UserID  uint64
	PassID  uint64
	XP      int64
	MaxXP   int64
	Premium bool
}

type MemberRow struct {
	UserID      uint64
	SpendMinor  int64
	Level       uint32
	FactVersion uint64
}

type UserState struct {
	Progress []ProgressRow
	SignIns  []SignInRow
}

// TaskClaimCommand 任务领取命令（老项目 TaskClaimCommand 去租户版）
type TaskClaimCommand struct {
	PlanVersion uint64
	RequestID   string
	UserID      uint64
	Task        Definition
	Period      string
	Today       string
}

// SignInCommand / SignInResult 签到命令与结果（老项目同款）
type SignInCommand struct {
	UserID, TaskID  uint64
	Today, Previous string
}

type SignInResult struct {
	Streak  uint32
	Date    string
	Already bool
}

// PassClaimCommand 通行证奖励领取命令
type PassClaimCommand struct {
	PlanVersion uint64
	RequestID   string
	UserID      uint64
	Season      PassSeason
	Reward      PassReward
}

// MemberClaimCommand 会员奖励领取命令
type MemberClaimCommand struct {
	PlanVersion uint64
	RequestID   string
	UserID      uint64
	Tier        MemberTier
}

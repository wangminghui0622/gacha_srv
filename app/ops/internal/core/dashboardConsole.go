// 报表看板核心（M14e + M15d）：看板校验 + 查询参数/缓存键/桶差量纯函数。
// 语义照抄 server_v2 reporting internal/biz/reporting.go：
//   - validateDashboard：编码/标题/widget 网格/重叠校验（逐条同款）
//   - ValidateDashboardQuery：查询参数校验（tenant/dashboard/编码/时间窗/max）
//   - DashboardCacheKey：sha256(dashboard 标识+查询参数) → "reporting:dashboard:v1:%x"
//   - grainSeconds/floorTime/ceilTime/sumPoints：查询路径时间与 NUMBER 聚合
//   - Deltas（M15d）：快照值变更 → 按币种桶差量
package core

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"gacha_srv/app/ops/ops"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// ValidateDashboard 校验看板（server_v2 validateDashboard 逐条同款）。
// 不修改入参。
func ValidateDashboard(d *ops.Dashboard) error {
	if d == nil || d.GetTenantId() == 0 || !ValidCode(d.GetDashboardCode(), 64) ||
		len(strings.TrimSpace(d.GetTitle())) == 0 || len(d.GetTitle()) > 128 ||
		len(d.GetWidgets()) == 0 || len(d.GetWidgets()) > 100 {
		return fmt.Errorf("看板非法（tenant/code/title/widgets 数量）")
	}
	seen := make(map[string]bool, len(d.GetWidgets()))
	for i, w := range d.GetWidgets() {
		if !ValidCode(w.GetWidgetCode(), 64) || seen[w.GetWidgetCode()] ||
			len(strings.TrimSpace(w.GetTitle())) == 0 || len(w.GetTitle()) > 128 ||
			!ValidCode(w.GetMetricCode(), 96) || !ValidCode(w.GetDimensionCode(), 64) ||
			!ValidWidgetKind(w.GetKind()) ||
			(w.GetGrain() != ops.TimeGrain_TIME_GRAIN_HOUR && w.GetGrain() != ops.TimeGrain_TIME_GRAIN_DAY) {
			return fmt.Errorf("看板 widget 非法（code/title/metric/dimension/kind/grain）")
		}
		if w.GetGridWidth() == 0 || w.GetGridWidth() > 24 || w.GetGridX()+w.GetGridWidth() > 24 ||
			w.GetGridHeight() == 0 || w.GetGridHeight() > 100 || w.GetGridY() > 10000 ||
			len(w.GetDisplayOptionsJson()) > 8192 || !ValidJSONObject(w.GetDisplayOptionsJson()) {
			return fmt.Errorf("看板 widget 网格/展示选项非法")
		}
		seen[w.GetWidgetCode()] = true
		for j := 0; j < i; j++ {
			x := d.GetWidgets()[j]
			if w.GetGridX() < x.GetGridX()+x.GetGridWidth() && x.GetGridX() < w.GetGridX()+w.GetGridWidth() &&
				w.GetGridY() < x.GetGridY()+x.GetGridHeight() && x.GetGridY() < w.GetGridY()+w.GetGridHeight() {
				return fmt.Errorf("看板 widget 网格重叠")
			}
		}
	}
	return nil
}

// ValidateDashboardQuery 校验查询参数（server_v2 QueryDashboard 入参同款；
// max_data_points<=0 由调用方先归一为 500，此处只拒 >2000）。
func ValidateDashboardQuery(q *ops.QueryDashboardRequest, from, to time.Time) error {
	if q.GetDashboardId() == 0 || q.GetTenantId() == 0 ||
		!ValidCode(q.GetProviderCode(), 64) || !ValidCode(q.GetExternalAccountId(), 128) ||
		from.IsZero() || !to.After(from) || q.GetMaxDataPoints() > 2000 {
		return fmt.Errorf("报表查询非法（id/provider/account/时间窗/max_data_points）")
	}
	return nil
}

// ValidCode 编码合法性（非空、≤max、无首尾空白、无控制字符）。
func ValidCode(s string, max int) bool {
	return len(s) > 0 && len(s) <= max && s == strings.TrimSpace(s) && !strings.ContainsAny(s, "\x00\n\r\t")
}

// ValidWidgetKind widget 类型合法性（1 NUMBER / 2 LINE / 3 BAR / 4 TABLE）。
func ValidWidgetKind(k ops.WidgetKind) bool {
	return k == ops.WidgetKind_WIDGET_KIND_NUMBER || k == ops.WidgetKind_WIDGET_KIND_LINE ||
		k == ops.WidgetKind_WIDGET_KIND_BAR || k == ops.WidgetKind_WIDGET_KIND_TABLE
}

// ValidJSONObject 字符串必须是合法 JSON 对象（{} 也算）。
func ValidJSONObject(value string) bool {
	if !json.Valid([]byte(value)) {
		return false
	}
	var object map[string]json.RawMessage
	return json.Unmarshal([]byte(value), &object) == nil && object != nil
}

// GrainSeconds 粒度 → 秒（HOUR 3600 / DAY 86400；其余 0）。
func GrainSeconds(g ops.TimeGrain) uint32 {
	if g == ops.TimeGrain_TIME_GRAIN_HOUR {
		return 3600
	}
	if g == ops.TimeGrain_TIME_GRAIN_DAY {
		return 86400
	}
	return 0
}

// FloorTime 向下取整到粒度边界。
func FloorTime(t time.Time, seconds uint32) time.Time {
	return time.Unix(t.Unix()/int64(seconds)*int64(seconds), 0).UTC()
}

// CeilTime 向上取整到粒度边界（已在边界则原值）。
func CeilTime(t time.Time, seconds uint32) time.Time {
	floor := FloorTime(t, seconds)
	if floor.Equal(t) {
		return floor
	}
	return floor.Add(time.Duration(seconds) * time.Second)
}

// SumPoints NUMBER widget 聚合：按币种求和，币种升序，bucket_start 取锚点。
func SumPoints(points []*ops.WidgetDataPoint, at time.Time) []*ops.WidgetDataPoint {
	totals := map[string]int64{}
	for _, x := range points {
		totals[x.GetCurrency()] += x.GetValue()
	}
	currencies := make([]string, 0, len(totals))
	for c := range totals {
		currencies = append(currencies, c)
	}
	slices.Sort(currencies)
	out := make([]*ops.WidgetDataPoint, 0, len(currencies))
	for _, c := range currencies {
		out = append(out, &ops.WidgetDataPoint{BucketStart: timestamppb.New(at), Currency: c, Value: totals[c]})
	}
	return out
}

// Deltas 快照值变更 → 按币种桶差量（server_v2 biz.Deltas 同款）：
// 旧币种 -旧值、新币种 +新值，零差量剔除（旧币种==新币种时相消）。
func Deltas(oldValue, newValue int64, oldCurrency, newCurrency string) map[string]int64 {
	out := map[string]int64{}
	// 旧币/新币双侧都无条件入账（计数指标币种为 ""，同样要扣旧值），
	// 同键相抵后剔除 0 值。
	out[oldCurrency] -= oldValue
	out[newCurrency] += newValue
	for currency, value := range out {
		if value == 0 {
			delete(out, currency)
		}
	}
	return out
}

// DashboardCacheKey 查询结果缓存键（server_v2 dashboardCacheKey 同款
// 载荷：dashboard 标识+版本+查询参数，sha256 后十六进制）。
func DashboardCacheKey(d *ops.Dashboard, q *ops.QueryDashboardRequest) (string, error) {
	payload, err := json.Marshal(struct {
		ID, TenantID, Version uint64
		Provider, Account     string
		From, To              int64
		Dimensions            map[string]string
		Max                   uint32
	}{d.GetDashboardId(), q.GetTenantId(), d.GetVersion(), q.GetProviderCode(), q.GetExternalAccountId(),
		q.GetFrom().GetSeconds(), q.GetTo().GetSeconds(), q.GetDimensionValues(), q.GetMaxDataPoints()})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("reporting:dashboard:v1:%x", sum[:]), nil
}

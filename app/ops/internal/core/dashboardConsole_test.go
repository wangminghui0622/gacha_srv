package core

import (
	"strings"
	"testing"
	"time"

	"gacha_srv/app/ops/ops"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func validWidget(code string) *ops.DashboardWidget {
	return &ops.DashboardWidget{
		WidgetCode: code, Title: "标题", Kind: ops.WidgetKind_WIDGET_KIND_NUMBER,
		MetricCode: "provider.impressions", Grain: ops.TimeGrain_TIME_GRAIN_DAY,
		DimensionCode: "all", GridX: 0, GridY: 0, GridWidth: 12, GridHeight: 6,
		DisplayOptionsJson: "{}",
	}
}

func validDashboard() *ops.Dashboard {
	return &ops.Dashboard{
		DashboardId: 0, DashboardCode: "overview", Title: "总览",
		TenantId: 1, Widgets: []*ops.DashboardWidget{
			validWidget("w1"),
			func() *ops.DashboardWidget { w := validWidget("w2"); w.GridX = 12; return w }(),
		},
	}
}

func TestValidateDashboard(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*ops.Dashboard)
		wantErr string
	}{
		{"合法", func(d *ops.Dashboard) {}, ""},
		{"nil", func(d *ops.Dashboard) {}, ""},
		{"tenant=0", func(d *ops.Dashboard) { d.TenantId = 0 }, "非法"},
		{"code 空", func(d *ops.Dashboard) { d.DashboardCode = "" }, "非法"},
		{"code 带空白", func(d *ops.Dashboard) { d.DashboardCode = " a" }, "非法"},
		{"code 超长", func(d *ops.Dashboard) { d.DashboardCode = strings.Repeat("a", 65) }, "非法"},
		{"title 空", func(d *ops.Dashboard) { d.Title = "  " }, "非法"},
		{"title 超长", func(d *ops.Dashboard) { d.Title = strings.Repeat("长", 129) }, "非法"},
		{"widgets 空", func(d *ops.Dashboard) { d.Widgets = nil }, "非法"},
		{"widget code 重复", func(d *ops.Dashboard) { d.Widgets[1].WidgetCode = "w1" }, "非法"},
		{"widget title 空", func(d *ops.Dashboard) { d.Widgets[0].Title = "" }, "非法"},
		{"metric_code 非法", func(d *ops.Dashboard) { d.Widgets[0].MetricCode = "" }, "非法"},
		{"dimension_code 非法", func(d *ops.Dashboard) { d.Widgets[0].DimensionCode = "\x00" }, "非法"},
		{"kind 未指定", func(d *ops.Dashboard) { d.Widgets[0].Kind = ops.WidgetKind_WIDGET_KIND_UNSPECIFIED }, "非法"},
		{"grain MONTH 非法", func(d *ops.Dashboard) { d.Widgets[0].Grain = ops.TimeGrain_TIME_GRAIN_MONTH }, "非法"},
		{"宽度 0", func(d *ops.Dashboard) { d.Widgets[0].GridWidth = 0 }, "非法"},
		{"x+w 越界", func(d *ops.Dashboard) { d.Widgets[0].GridX = 20; d.Widgets[0].GridWidth = 5 }, "非法"},
		{"高度 101", func(d *ops.Dashboard) { d.Widgets[0].GridHeight = 101 }, "非法"},
		{"y 越界", func(d *ops.Dashboard) { d.Widgets[0].GridY = 10001 }, "非法"},
		{"display 超长", func(d *ops.Dashboard) { d.Widgets[0].DisplayOptionsJson = `{"k":"` + strings.Repeat("a", 8193) + `"}` }, "非法"},
		{"display 非 JSON", func(d *ops.Dashboard) { d.Widgets[0].DisplayOptionsJson = "nope" }, "非法"},
		{"display 数组非对象", func(d *ops.Dashboard) { d.Widgets[0].DisplayOptionsJson = "[1]" }, "非法"},
		{"网格重叠", func(d *ops.Dashboard) { d.Widgets[1].GridX = 0 }, "重叠"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "nil" {
				if err := ValidateDashboard(nil); err == nil {
					t.Fatal("nil 看板应报错")
				}
				return
			}
			d := validDashboard()
			tc.mutate(d)
			err := ValidateDashboard(d)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("期望通过，得到 %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("期望错误含 %q，得到 %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateDashboardQuery(t *testing.T) {
	base := func() *ops.QueryDashboardRequest {
		return &ops.QueryDashboardRequest{
			DashboardId: 1, TenantId: 1, ProviderCode: "ad", ExternalAccountId: "acc",
			From: timestamppb.New(time.Unix(1000, 0)), To: timestamppb.New(time.Unix(2000, 0)),
		}
	}
	from, to := time.Unix(1000, 0), time.Unix(2000, 0)
	cases := []struct {
		name    string
		mutate  func(*ops.QueryDashboardRequest, *time.Time, *time.Time)
		wantErr bool
	}{
		{"合法", func(q *ops.QueryDashboardRequest, f, t *time.Time) {}, false},
		{"dashboard=0", func(q *ops.QueryDashboardRequest, f, t *time.Time) { q.DashboardId = 0 }, true},
		{"tenant=0", func(q *ops.QueryDashboardRequest, f, t *time.Time) { q.TenantId = 0 }, true},
		{"provider 空", func(q *ops.QueryDashboardRequest, f, t *time.Time) { q.ProviderCode = "" }, true},
		{"provider 超长", func(q *ops.QueryDashboardRequest, f, t *time.Time) { q.ProviderCode = strings.Repeat("a", 65) }, true},
		{"account 超长", func(q *ops.QueryDashboardRequest, f, t *time.Time) { q.ExternalAccountId = strings.Repeat("a", 129) }, true},
		{"from 零值", func(q *ops.QueryDashboardRequest, f, t *time.Time) { *f = time.Time{} }, true},
		{"to<=from", func(q *ops.QueryDashboardRequest, f, t *time.Time) { *t = time.Unix(1000, 0) }, true},
		{"max>2000", func(q *ops.QueryDashboardRequest, f, t *time.Time) { q.MaxDataPoints = 2001 }, true},
		{"max=2000 合法", func(q *ops.QueryDashboardRequest, f, t *time.Time) { q.MaxDataPoints = 2000 }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := base()
			f, t2 := from, to
			tc.mutate(q, &f, &t2)
			err := ValidateDashboardQuery(q, f, t2)
			if tc.wantErr && err == nil {
				t.Fatal("期望报错")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("期望通过，得到 %v", err)
			}
		})
	}
}

func TestGrainSecondsAndTimeRounding(t *testing.T) {
	if GrainSeconds(ops.TimeGrain_TIME_GRAIN_HOUR) != 3600 || GrainSeconds(ops.TimeGrain_TIME_GRAIN_DAY) != 86400 {
		t.Fatal("HOUR/DAY 粒度秒数错误")
	}
	if GrainSeconds(ops.TimeGrain_TIME_GRAIN_MONTH) != 0 || GrainSeconds(ops.TimeGrain_TIME_GRAIN_UNSPECIFIED) != 0 {
		t.Fatal("MONTH/UNSPECIFIED 应不支持")
	}
	// 2026-10-23T05:30:00Z → floor 05:00，ceil 06:00
	at := time.Date(2026, 10, 23, 5, 30, 0, 0, time.UTC)
	if got := FloorTime(at, 3600); !got.Equal(time.Date(2026, 10, 23, 5, 0, 0, 0, time.UTC)) {
		t.Fatalf("floor 错误: %v", got)
	}
	if got := CeilTime(at, 3600); !got.Equal(time.Date(2026, 10, 23, 6, 0, 0, 0, time.UTC)) {
		t.Fatalf("ceil 错误: %v", got)
	}
	// 已在边界：ceil == floor == 原值
	exact := time.Date(2026, 10, 23, 5, 0, 0, 0, time.UTC)
	if got := CeilTime(exact, 3600); !got.Equal(exact) {
		t.Fatalf("边界 ceil 错误: %v", got)
	}
}

func TestSumPoints(t *testing.T) {
	anchor := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	points := []*ops.WidgetDataPoint{
		{Currency: "USD", Value: 100},
		{Currency: "CNY", Value: 200},
		{Currency: "USD", Value: 50},
	}
	sum := SumPoints(points, anchor)
	if len(sum) != 2 || sum[0].Currency != "CNY" || sum[0].Value != 200 ||
		sum[1].Currency != "USD" || sum[1].Value != 150 {
		t.Fatalf("聚合错误: %+v", sum)
	}
	for _, p := range sum {
		if !p.BucketStart.AsTime().Equal(anchor) {
			t.Fatalf("锚点错误: %v", p.BucketStart.AsTime())
		}
	}
	if got := SumPoints(nil, anchor); len(got) != 0 {
		t.Fatalf("空集应聚合为空: %+v", got)
	}
}

func TestDashboardCacheKey(t *testing.T) {
	d := &ops.Dashboard{DashboardId: 7, TenantId: 1, Version: 3, Status: ops.DashboardStatus_DASHBOARD_STATUS_PUBLISHED}
	q := &ops.QueryDashboardRequest{
		DashboardId: 7, TenantId: 1, ProviderCode: "ad", ExternalAccountId: "acc",
		From: timestamppb.New(time.Unix(1000, 0)), To: timestamppb.New(time.Unix(2000, 0)),
		DimensionValues: map[string]string{"geo": "cn"}, MaxDataPoints: 500,
	}
	key, err := DashboardCacheKey(d, q)
	if err != nil || !strings.HasPrefix(key, "reporting:dashboard:v1:") || len(key) != len("reporting:dashboard:v1:")+64 {
		t.Fatalf("缓存键非法: %q %v", key, err)
	}
	// 查询参数任一变化 → 键变化
	other := &ops.QueryDashboardRequest{DashboardId: 7, TenantId: 1, ProviderCode: "ad",
		ExternalAccountId: "acc", From: timestamppb.New(time.Unix(1000, 0)),
		To: timestamppb.New(time.Unix(2000, 0)), DimensionValues: map[string]string{"geo": "cn"}, MaxDataPoints: 501}
	key2, _ := DashboardCacheKey(d, other)
	if key == key2 {
		t.Fatal("max_data_points 变化应改变缓存键")
	}
	// 版本变化 → 键变化
	d2 := &ops.Dashboard{DashboardId: 7, TenantId: 1, Version: 4}
	key3, _ := DashboardCacheKey(d2, q)
	if key == key3 {
		t.Fatal("dashboard 版本变化应改变缓存键")
	}
	// 确定性：同参数两次一致
	key4, _ := DashboardCacheKey(d, q)
	if key != key4 {
		t.Fatal("缓存键应确定")
	}
}

func TestDeltas(t *testing.T) {
	eq := func(got map[string]int64, want map[string]int64) bool {
		if len(got) != len(want) {
			return false
		}
		for k, v := range want {
			if got[k] != v {
				return false
			}
		}
		return true
	}
	cases := []struct {
		name                     string
		oldValue, newValue       int64
		oldCurrency, newCurrency string
		want                     map[string]int64
	}{
		{"新建金额", 0, 100, "", "CNY", map[string]int64{"CNY": 100}},
		{"同币种上涨", 50, 80, "CNY", "CNY", map[string]int64{"CNY": 30}},
		{"同币种下跌", 50, 30, "CNY", "CNY", map[string]int64{"CNY": -20}},
		{"同币种持平", 50, 50, "CNY", "CNY", map[string]int64{}},
		{"换币种", 5000, 700, "CNY", "USD", map[string]int64{"CNY": -5000, "USD": 700}},
		{"金额转计数（币种清空）", 5000, 3, "CNY", "", map[string]int64{"CNY": -5000, "": 3}},
		{"计数转金额", 3, 5000, "", "CNY", map[string]int64{"": -3, "CNY": 5000}},
		{"计数指标替换（同槽高修订）", 100, 300, "", "", map[string]int64{"": 200}},
		{"零值持平", 0, 0, "", "", map[string]int64{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Deltas(tc.oldValue, tc.newValue, tc.oldCurrency, tc.newCurrency)
			if !eq(got, tc.want) {
				t.Fatalf("Deltas(%d,%d,%q,%q) = %v; want %v",
					tc.oldValue, tc.newValue, tc.oldCurrency, tc.newCurrency, got, tc.want)
			}
		})
	}
}

func TestValidCodeAndJSONObject(t *testing.T) {
	if ValidCode("", 10) || ValidCode(strings.Repeat("a", 11), 10) || ValidCode(" a", 10) || ValidCode("a ", 10) || ValidCode("a\n", 10) || ValidCode("a\t", 10) {
		t.Fatal("ValidCode 应拒绝空/超长/空白/控制字符")
	}
	if !ValidCode("abc", 10) {
		t.Fatal("ValidCode 应接受普通编码")
	}
	if !ValidJSONObject("{}") || !ValidJSONObject(`{"k":1}`) {
		t.Fatal("ValidJSONObject 应接受对象")
	}
	if ValidJSONObject("") || ValidJSONObject("null") || ValidJSONObject("[1]") || ValidJSONObject(`"s"`) || ValidJSONObject("{bad") {
		t.Fatal("ValidJSONObject 应拒绝非对象/非法 JSON")
	}
}

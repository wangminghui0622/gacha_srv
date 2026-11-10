// 报表存储（M14e + M15d）：指标目录 + 看板 CRUD/CAS + 查询结果缓存 + 指标事件应用。
// 语义照抄 server_v2 reporting internal/data/dashboard.go + biz/reporting.go：
//   - SaveDashboard：id=0 创建（version 须=1&&DRAFT）；否则 UPDATE
//     WHERE tenant_id AND id AND version AND status=1（version++）→ CAS 失败 CONFLICT；
//     delete+reinsert widgets，逐个 GetMetricCapability（metric+dimension+status=1+粒度支持）
//   - PublishDashboard：SET status=2,version=version+1,published_at
//     WHERE tenant_id AND id AND version AND status=1 → 0 行 CONFLICT
//   - ListMetricDefinitions：metric_definition⋈metric_dimension（status=1）聚合
//   - ApplyMetric/ListBuckets（M15d 指标管线，见 metricApply.go）
// 偏离：dashboard/widget id 用 AUTO_INCREMENT（server_v2 雪花）；其余逐字同款。
package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gacha_srv/app/ops/ops"
	opsmodel "gacha_srv/pkg/model_nocache/gacha_ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"
)

const (
	dashboardStatusDraft  = 1
	dashboardStatusLive   = 2
	// dashboardCacheTTL 查询结果缓存 TTL（server_v2 默认同款）。
	dashboardCacheTTL = 60 * time.Second
)

// ReportingStore 报表数据访问。
type ReportingStore struct {
	db *gorm.DB
}

// NewReportingStore 构造报表存储。
func NewReportingStore(db *gorm.DB) *ReportingStore {
	return &ReportingStore{db: db}
}

// ListMetricDefinitions 指标目录（metric⋈dimension，status=1，聚合维度列表）。
func (s *ReportingStore) ListMetricDefinitions(ctx context.Context) ([]*ops.MetricDefinition, error) {
	type row struct {
		MetricCode      string
		Title           string
		StandardMetric  uint64
		ValueKind       uint64
		SupportedGrains string
		DimensionCode   string
	}
	var rows []row
	err := s.db.WithContext(ctx).Raw(
		"SELECT m.metric_code,m.title,m.standard_metric,m.value_kind,m.supported_grains,d.dimension_code "+
			"FROM `reporting_metric_definition` m JOIN `reporting_metric_dimension` d ON d.metric_code=m.metric_code "+
			"WHERE m.status=1 ORDER BY m.metric_code,d.dimension_code").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list metric definitions: %w", err)
	}
	out := make([]*ops.MetricDefinition, 0)
	for _, x := range rows {
		if len(out) == 0 || out[len(out)-1].GetMetricCode() != x.MetricCode {
			out = append(out, &ops.MetricDefinition{
				MetricCode: x.MetricCode, Title: x.Title,
				StandardMetric: ops.StandardMetric(x.StandardMetric),
				ValueKind:      ops.MetricValueKind(x.ValueKind),
				SupportedGrains: parseGrains(x.SupportedGrains),
			})
		}
		last := out[len(out)-1]
		last.DimensionCodes = append(last.GetDimensionCodes(), x.DimensionCode)
	}
	return out, nil
}

// SaveDashboard 保存看板（创建/更新，widgets 全量替换）。
func (s *ReportingStore) SaveDashboard(ctx context.Context, d *ops.Dashboard) (*ops.Dashboard, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		var current opsmodel.ReportingDashboard
		lookupErr := tx.Where("`tenant_id` = ? AND `id` = ?", d.GetTenantId(), d.GetDashboardId()).Take(&current).Error
		create := errors.Is(lookupErr, gorm.ErrRecordNotFound)
		if lookupErr != nil && !create {
			return lookupErr
		}
		if create {
			if d.GetVersion() != 1 || d.GetStatus() != ops.DashboardStatus_DASHBOARD_STATUS_DRAFT {
				return s.conflictf("创建看板须 version=1 且 DRAFT")
			}
			row := opsmodel.ReportingDashboard{
				TenantId: d.GetTenantId(), DashboardCode: d.GetDashboardCode(), Title: d.GetTitle(),
				Status: dashboardStatusDraft, Version: 1, CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&row).Error; err != nil {
				if errors.Is(err, gorm.ErrDuplicatedKey) {
					return s.conflictf("看板编码 %s 已存在", d.GetDashboardCode())
				}
				return err
			}
			d.DashboardId = row.Id
		} else {
			if current.Status != dashboardStatusDraft {
				return s.conflictf("看板 %d 非草稿状态", d.GetDashboardId())
			}
			res := tx.Exec("UPDATE `reporting_dashboard` SET `dashboard_code`=?,`title`=?,`version`=`version`+1,`updated_at`=? WHERE `tenant_id`=? AND `id`=? AND `version`=? AND `status`=?",
				d.GetDashboardCode(), d.GetTitle(), now, d.GetTenantId(), d.GetDashboardId(), d.GetVersion(), dashboardStatusDraft)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return s.conflictf("看板 %d 版本 CAS 失败", d.GetDashboardId())
			}
			d.Version++
		}
		if err := tx.Exec("DELETE FROM `reporting_dashboard_widget` WHERE `dashboard_id`=?", d.GetDashboardId()).Error; err != nil {
			return err
		}
		for _, w := range d.GetWidgets() {
			var capability struct{ SupportedGrains string }
			err := tx.Raw("SELECT m.supported_grains FROM `reporting_metric_definition` m JOIN `reporting_metric_dimension` d ON d.metric_code=m.metric_code WHERE m.metric_code=? AND d.dimension_code=? AND m.status=1 LIMIT 1",
				w.GetMetricCode(), w.GetDimensionCode()).Scan(&capability).Error
			if err != nil {
				return err
			}
			if capability.SupportedGrains == "" || !supportsGrain(capability.SupportedGrains, w.GetGrain()) {
				return errorx.NewCodeError(errorx.CodeDashboardInvalid, fmt.Sprintf("指标 %s 与维度 %s 不匹配或不支持粒度 %s", w.GetMetricCode(), w.GetDimensionCode(), w.GetGrain()))
			}
			widget := opsmodel.ReportingDashboardWidget{
				DashboardId: d.GetDashboardId(), WidgetCode: w.GetWidgetCode(), Title: w.GetTitle(),
				Kind: uint64(w.GetKind()), MetricCode: w.GetMetricCode(), Grain: uint64(w.GetGrain()),
				DimensionCode: w.GetDimensionCode(), GridX: uint64(w.GetGridX()), GridY: uint64(w.GetGridY()),
				GridWidth: uint64(w.GetGridWidth()), GridHeight: uint64(w.GetGridHeight()),
				DisplayOptions: w.GetDisplayOptionsJson(), CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&widget).Error; err != nil {
				return err
			}
			w.WidgetId = widget.Id
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return d, nil
}

// PublishDashboard 发布看板（version/status CAS）。
func (s *ReportingStore) PublishDashboard(ctx context.Context, tenantID, id, version uint64) (*ops.Dashboard, error) {
	now := time.Now().UTC()
	res := s.db.WithContext(ctx).Exec("UPDATE `reporting_dashboard` SET `status`=?,`version`=`version`+1,`published_at`=?,`updated_at`=? WHERE `tenant_id`=? AND `id`=? AND `version`=? AND `status`=?",
		dashboardStatusLive, now, now, tenantID, id, version, dashboardStatusDraft)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected != 1 {
		return nil, s.conflictf("看板 %d 发布 CAS 失败", id)
	}
	return s.GetDashboard(ctx, tenantID, id)
}

// GetDashboard 读取看板（含 widgets，grid_y/grid_x/id 排序）。
func (s *ReportingStore) GetDashboard(ctx context.Context, tenantID, id uint64) (*ops.Dashboard, error) {
	var row opsmodel.ReportingDashboard
	err := s.db.WithContext(ctx).Where("`tenant_id` = ? AND `id` = ?", tenantID, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errorx.NewCodeError(errorx.CodeDashboardNotFound, fmt.Sprintf("看板 %d 不存在", id))
	}
	if err != nil {
		return nil, err
	}
	var widgets []opsmodel.ReportingDashboardWidget
	if err = s.db.WithContext(ctx).Where("`dashboard_id` = ?", id).
		Order("`grid_y`,`grid_x`,`id`").Find(&widgets).Error; err != nil {
		return nil, err
	}
	d := &ops.Dashboard{
		DashboardId: row.Id, DashboardCode: row.DashboardCode, Title: row.Title,
		Status: ops.DashboardStatus(row.Status), Version: row.Version, TenantId: row.TenantId,
		Widgets: make([]*ops.DashboardWidget, 0, len(widgets)),
	}
	for i := range widgets {
		w := &widgets[i]
		d.Widgets = append(d.Widgets, &ops.DashboardWidget{
			WidgetId: w.Id, WidgetCode: w.WidgetCode, Title: w.Title, Kind: ops.WidgetKind(w.Kind),
			MetricCode: w.MetricCode, Grain: ops.TimeGrain(w.Grain), DimensionCode: w.DimensionCode,
			GridX: uint32(w.GridX), GridY: uint32(w.GridY), GridWidth: uint32(w.GridWidth),
			GridHeight: uint32(w.GridHeight), DisplayOptionsJson: w.DisplayOptions,
		})
	}
	return d, nil
}

func (s *ReportingStore) conflictf(format string, args ...any) error {
	return errorx.NewCodeError(errorx.CodeDashboardConflict, fmt.Sprintf(format, args...))
}

// parseGrains SET 列（"hour,day"）→ 粒度列表（server_v2 parseGrains 同款顺序）。
func parseGrains(value string) []ops.TimeGrain {
	out := make([]ops.TimeGrain, 0, 2)
	if supportsGrain(value, ops.TimeGrain_TIME_GRAIN_HOUR) {
		out = append(out, ops.TimeGrain_TIME_GRAIN_HOUR)
	}
	if supportsGrain(value, ops.TimeGrain_TIME_GRAIN_DAY) {
		out = append(out, ops.TimeGrain_TIME_GRAIN_DAY)
	}
	if supportsGrain(value, ops.TimeGrain_TIME_GRAIN_MONTH) {
		out = append(out, ops.TimeGrain_TIME_GRAIN_MONTH)
	}
	return out
}

func supportsGrain(value string, g ops.TimeGrain) bool {
	want := ""
	switch g {
	case ops.TimeGrain_TIME_GRAIN_HOUR:
		want = "hour"
	case ops.TimeGrain_TIME_GRAIN_DAY:
		want = "day"
	case ops.TimeGrain_TIME_GRAIN_MONTH:
		want = "month"
	default:
		return false
	}
	for _, v := range strings.Split(value, ",") {
		if v == want {
			return true
		}
	}
	return false
}

// ResultCache 查询结果缓存（Redis，protojson 往返；key 由 core.DashboardCacheKey 生成）。
type ResultCache struct {
	rdb *redis.Client
	ttl time.Duration
}

// NewResultCache 构造查询结果缓存。
func NewResultCache(rdb *redis.Client, ttl time.Duration) *ResultCache {
	if ttl <= 0 {
		ttl = dashboardCacheTTL
	}
	return &ResultCache{rdb: rdb, ttl: ttl}
}

// Get 读取缓存结果；未命中/解析失败返回 (nil, false)。
func (c *ResultCache) Get(ctx context.Context, key string) (*ops.QueryDashboardReply, bool) {
	data, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false
	}
	if err != nil {
		return nil, false
	}
	reply := new(ops.QueryDashboardReply)
	if protojson.Unmarshal(data, reply) != nil {
		return nil, false
	}
	reply.CacheHit = true
	return reply, true
}

// Set 写入缓存结果（失败静默——缓存是加速层，照抄 server_v2 `_ =`）。
func (c *ResultCache) Set(ctx context.Context, key string, reply *ops.QueryDashboardReply) error {
	data, err := protojson.Marshal(reply)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, data, c.ttl).Err()
}

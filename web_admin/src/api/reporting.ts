// 报表 typed service：方法名与参考 ReportingDashboardView 调用点一致
// （Metrics/Get/Save/Publish/Query），axios 直连本仓 /reporting 路由。
// from/to 为 RFC3339 时间串（protojson Timestamp）；value 为分（int64 字符串）。
import { http } from '@/utils/request'

export interface MetricDefinition {
  metricCode?: string
  title?: string
  standardMetric?: string
  valueKind?: string
  supportedGrains?: string[]
  dimensionCodes?: string[]
}

export interface DashboardWidget {
  widgetId?: string
  widgetCode?: string
  title?: string
  kind?: string | number
  metricCode?: string
  grain?: string | number
  dimensionCode?: string
  gridX?: number
  gridY?: number
  gridWidth?: number
  gridHeight?: number
  displayOptionsJson?: string
}

export interface Dashboard {
  dashboardId?: string
  dashboardCode?: string
  title?: string
  status?: string
  version?: string
  widgets?: DashboardWidget[]
  tenantId?: string
}

export interface WidgetDataPoint {
  bucketStart?: string
  currency?: string
  value?: string
}

export interface WidgetQueryResult {
  widgetId?: string
  widgetCode?: string
  metricCode?: string
  grain?: string
  dimensionCode?: string
  points?: WidgetDataPoint[]
  truncated?: boolean
}

export interface QueryDashboardReply {
  dashboard?: Dashboard
  results?: WidgetQueryResult[]
  cacheHit?: boolean
  generatedAt?: string
}

export const reportingClient = {
  Metrics(): Promise<{ metrics?: MetricDefinition[] }> {
    return http.get('/console/v2/api/reporting/metrics')
  },
  Get({ dashboardId }: { dashboardId: string }): Promise<Dashboard> {
    return http.get(`/console/v2/api/reporting/dashboards/${encodeURIComponent(dashboardId)}`)
  },
  Save(payload: {
    dashboardId: string | number
    expectedVersion: string | number
    dashboardCode: string
    title: string
    widgets: DashboardWidget[]
  }): Promise<Dashboard> {
    return http.post('/console/v2/api/reporting/dashboards', payload)
  },
  Publish({ dashboardId, expectedVersion }: { dashboardId: string; expectedVersion: string }): Promise<Dashboard> {
    return http.post(`/console/v2/api/reporting/dashboards/${encodeURIComponent(dashboardId)}/publish`, { expectedVersion })
  },
  Query(payload: {
    dashboardId: string
    providerCode: string
    externalAccountId: string
    from: string
    to: string
    dimensionValues?: Record<string, string>
    maxDataPoints: number
    skipCache: boolean
  }): Promise<QueryDashboardReply> {
    const { dashboardId, ...body } = payload
    return http.post(`/console/v2/api/reporting/dashboards/${encodeURIComponent(dashboardId)}/query`, body)
  },
}

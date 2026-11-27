import { http } from '@/utils/request'

export interface RealtimeSummary {
  totalRegistrations: number
  totalPayingUsers: number
  totalPayFen: number
  totalRechargeUsers: number
  totalRechargeFen: number
  totalRefundFen: number
  arppuFen: number
  todayRegistrations: number
  todayPayFen: number
  todayRechargeFen: number
}

export type RealtimeKind = 'count' | 'money'

export interface RealtimeSeries {
  name: string
  kind: RealtimeKind
  values: number[]
}

export interface RealtimeChart {
  title: string
  series: RealtimeSeries[]
}

export interface RealtimeReportResponse {
  summary: RealtimeSummary
  labels: string[]
  charts: RealtimeChart[]
}

export function getRealtimeReport(): Promise<RealtimeReportResponse> {
  return http.get('/console/v2/api/reporting/realtime')
}

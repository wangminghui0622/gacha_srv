import { http } from '@/utils/request'

export type RetentionType = 'new' | 'active' | 'new_pay' | 'join' | 'pay'

export interface RetentionCell {
  dayNo: number
  totalUsers: number
  retainedUsers: number
  rateBps: number
  available: boolean
}

export interface RetentionRow {
  cohortDate: string
  totalUsers: number
  cells: RetentionCell[]
}

export interface RetentionReportResponse {
  type: RetentionType
  dayNos: number[]
  rows: RetentionRow[]
  total: RetentionRow
}

export function getRetentionReport(params: {
  type: RetentionType
  startDate: string
  endDate: string
}): Promise<RetentionReportResponse> {
  return http.get('/console/v2/api/reporting/retention', { params })
}

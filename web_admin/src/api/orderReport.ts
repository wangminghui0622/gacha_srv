import { http } from '@/utils/request'

export interface OrderReportRow {
  date: string
  userId: number
  nickname: string
  costFen: number
}

export interface OrderReportResponse {
  rows: OrderReportRow[]
}

export function getOrderReport(params: {
  startDate: string
  endDate: string
  userId?: string
  nickname?: string
  phone?: string
}): Promise<OrderReportResponse> {
  return http.get('/console/v2/api/reporting/orders', { params })
}

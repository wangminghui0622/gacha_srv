import { http } from '@/utils/request'

export interface MarketReportRow {
  date: string
  participants: number
  newOrders: number
  amountTotalFen: number
  amountNormalFen: number
  amountSampleFen: number
}

export interface MarketReportResponse {
  rows: MarketReportRow[]
}

export function getMarketReport(params: {
  startDate: string
  endDate: string
}): Promise<MarketReportResponse> {
  return http.get('/console/v2/api/reporting/market', { params })
}

import { http } from '@/utils/request'

// 经营日报（金额单位：分，比率单位：万分比）
export interface DailyReportSummary {
  registrations: number
  payingUsers: number
  drawCount: number
  drawAmountFen: number
  prizeAmountFen: number
  grossProfitFen: number
  grossMarginBps: number
  totalReceivedFen: number
  totalRechargeFen: number
  totalRefundFen: number
  alipayRechargeFen: number
  huifuRechargeFen: number
  alipayRefundFen: number
  huifuRefundFen: number
  rechargeOriginalRefundFen: number
  userRefundFen: number
  rechargeMinusRefundFen: number
  refundRateBps: number
}

export interface DailyReportRow {
  date: string
  registrations: number
  activeUsers: number
  payingUsers: number
  refundableFen: number
  balanceFen: number
  inventoryValueFen: number
  drawAmountFen: number
  prizeAmountFen: number
  grossProfitFen: number
  grossMarginBps: number
  totalReceivedFen: number
  totalRechargeFen: number
  totalRefundFen: number
  alipayRechargeFen: number
  huifuRechargeFen: number
  alipayRefundFen: number
  huifuRefundFen: number
  rechargeOriginalRefundFen: number
  userRefundFen: number
  rechargeMinusRefundFen: number
  refundRateBps: number
}

export interface DailyReportResponse {
  summary: DailyReportSummary
  month: DailyReportSummary
  rows: DailyReportRow[]
}

export function getDailyReport(params: { startDate: string; endDate: string }): Promise<DailyReportResponse> {
  return http.get('/console/v2/api/reporting/business-daily', { params })
}

import { http } from '@/utils/request'

export interface RevenueReportRow {
  date: string
  registrations: number
  activeUsers: number
  newActiveUsers: number
  oldActiveUsers: number
  activeUsers7d: number
  activeUsers15d: number
  churn1dBps: number
  churn3dBps: number
  payAmountFen: number
  newPayAmountFen: number
  oldPayAmountFen: number
  refundAmountFen: number
  refundUsers: number
  payingUsers: number
  newPayingUsers: number
  oldPayingUsers: number
  arpuFen: number
  arppuFen: number
  penetrationBps: number
  netRevenueFen: number
  wechatPayFen: number
  alipayPayFen: number
  huifuPayFen: number
  wechatGiftFen: number
  alipayGiftFen: number
  huifuGiftFen: number
  wechatRefundFen: number
  alipayRefundFen: number
  huifuRefundFen: number
  wechatGiftRefundFen: number
  alipayGiftRefundFen: number
  huifuGiftRefundFen: number
  taxFen: number
  newTaxFen: number
  oldTaxFen: number
  walletBalanceFen: number
  bonusBalanceFen: number
  redeemPointBalance: number
  loyaltyPointBalance: number
  platformCurrencyPayFen: number
  arpuNewFen: number
  arpuOldFen: number
  penetrationNewBps: number
  penetrationOldBps: number
  refundRateBps: number
  revenueRateBps: number
  revenueArpuFen: number
}

export interface RevenueReportResponse {
  rows: RevenueReportRow[]
}

export function getRevenueReport(params: {
  startDate: string
  endDate: string
}): Promise<RevenueReportResponse> {
  return http.get('/console/v2/api/reporting/revenue', { params })
}

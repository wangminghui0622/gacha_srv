import { get, post } from './api'
import type {
  GroupBuyBuyResponse,
  GroupBuyDetailResponse,
  GroupBuyRecordsResponse,
} from './types'

// 拼团抽服务（M13）：端点 /group-buy（与后端 routes.go 一致）

/** 拼团抽详情（余位/我的下注/参与人数/幸运奖进度） */
export function getGroupBuy(configId: number): Promise<GroupBuyDetailResponse> {
  return get<GroupBuyDetailResponse>(`/group-buy/${configId}`)
}

/** 拼团抽下注（SAGA 提交，异步整批结算）：count 本次下注数；同幂等键重试复用同一事务 */
export function groupBuyBuy(
  configId: number,
  count: number,
  idempotencyKey: string,
): Promise<GroupBuyBuyResponse> {
  return post<GroupBuyBuyResponse>(
    `/group-buy/${configId}/buy`,
    { count },
    { headers: { 'Idempotency-Key': idempotencyKey } },
  )
}

/** 拼团记录：requestId 精确查询或分页列表 */
export function listGroupBuyRecords(params?: {
  requestId?: string
  page?: number
  pageSize?: number
}): Promise<GroupBuyRecordsResponse> {
  return get<GroupBuyRecordsResponse>('/group-buy/records', params)
}

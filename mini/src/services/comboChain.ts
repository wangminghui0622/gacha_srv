import { get, post } from './api'
import type {
  ComboChainBuyResponse,
  ComboChainDetailResponse,
  ComboChainRecordsResponse,
} from './types'

// 连击链服务（M13）：端点 /combo-chain（与后端 routes.go 一致）

/** 连击链详情（当前车进度/全局奖槽/共享奖档位/我的连击计数） */
export function getComboChain(configId: number): Promise<ComboChainDetailResponse> {
  return get<ComboChainDetailResponse>(`/combo-chain/${configId}`)
}

/** 连击链购票（SAGA 提交，异步整车结算）：一人一车一票；同幂等键重试复用同一事务 */
export function comboChainBuy(
  configId: number,
  idempotencyKey: string,
): Promise<ComboChainBuyResponse> {
  return post<ComboChainBuyResponse>(
    `/combo-chain/${configId}/buy`,
    {},
    { headers: { 'Idempotency-Key': idempotencyKey } },
  )
}

/** 连击链记录：requestId 精确查询或分页列表 */
export function listComboChainRecords(params?: {
  requestId?: string
  page?: number
  pageSize?: number
}): Promise<ComboChainRecordsResponse> {
  return get<ComboChainRecordsResponse>('/combo-chain/records', params)
}

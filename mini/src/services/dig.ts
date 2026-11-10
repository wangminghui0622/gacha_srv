import { get, post } from './api'
import type {
  DigBoxListResponse,
  DigBoxSessionResponse,
  DigBuyResponse,
  DigQuoteResponse,
  DigRecordsResponse,
} from './types'

// 挖宝箱服务（M11b）：端点 /dig-boxes（与后端 routes.go 一致）

/** 挖宝箱列表 */
export function listDigBoxes(): Promise<DigBoxListResponse> {
  return get<DigBoxListResponse>('/dig-boxes')
}

/** 挖宝会话（位图/已挖数/状态） */
export function getDigSession(boxId: number): Promise<DigBoxSessionResponse> {
  return get<DigBoxSessionResponse>(`/dig-boxes/${boxId}/session`)
}

/** 报价预览：cells=逗号分隔格子（如 "0,1,2"），takeAll=true 时按会话空位算 */
export function digQuote(boxId: number, cells?: number[], takeAll?: boolean): Promise<DigQuoteResponse> {
  const params: Record<string, string | boolean> = {}
  if (cells && cells.length > 0) params.cells = cells.join(',')
  if (takeAll) params.takeAll = true
  return get<DigQuoteResponse>(`/dig-boxes/${boxId}/quote`, params)
}

/** 挖宝购买（SAGA 提交，异步开格）：同幂等键重试复用同一事务 */
export function digBuy(
  boxId: number,
  idempotencyKey: string,
  cellIndexes?: number[],
  takeAll?: boolean,
): Promise<DigBuyResponse> {
  return post<DigBuyResponse>(
    `/dig-boxes/${boxId}/buy`,
    { boxId, cellIndexes: cellIndexes && cellIndexes.length > 0 ? cellIndexes : undefined, takeAll: !!takeAll },
    { headers: { 'Idempotency-Key': idempotencyKey } },
  )
}

/** 挖宝记录：requestId 精确查询或分页列表 */
export function listDigRecords(params?: {
  requestId?: string
  page?: number
  pageSize?: number
}): Promise<DigRecordsResponse> {
  return get<DigRecordsResponse>('/dig-boxes/records', params)
}

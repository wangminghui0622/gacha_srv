import { get, post } from './api'
import type {
  GashaponBuyResponse,
  GashaponDetailResponse,
  GashaponRecordsResponse,
} from './types'

// 扭蛋服务（M12c）：端点 /gashapon（与后端 routes.go 一致）

/** 扭蛋详情（特别奖进度/余票/参与人数/单价） */
export function getGashapon(configId: number): Promise<GashaponDetailResponse> {
  return get<GashaponDetailResponse>(`/gashapon/${configId}`)
}

/** 扭蛋抽取（SAGA 提交，异步开奖）：count 本次抽数；同幂等键重试复用同一事务 */
export function gashaponBuy(
  configId: number,
  count: number,
  idempotencyKey: string,
): Promise<GashaponBuyResponse> {
  return post<GashaponBuyResponse>(
    `/gashapon/${configId}/buy`,
    { count },
    { headers: { 'Idempotency-Key': idempotencyKey } },
  )
}

/** 扭蛋记录：requestId 精确查询或分页列表 */
export function listGashaponRecords(params?: {
  requestId?: string
  page?: number
  pageSize?: number
}): Promise<GashaponRecordsResponse> {
  return get<GashaponRecordsResponse>('/gashapon/records', params)
}

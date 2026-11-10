import { get, post } from './api'
import type {
  FullCarBuyResponse,
  FullCarDetailResponse,
  FullCarRecordsResponse,
} from './types'

// 满车服务（M12b）：端点 /full-car（与后端 routes.go 一致）

/** 满车详情（座位/整车奖/余票/单价） */
export function getFullCar(configId: number): Promise<FullCarDetailResponse> {
  return get<FullCarDetailResponse>(`/full-car/${configId}`)
}

/** 满车购座（SAGA 提交，异步占座开奖）：seatIndex 缺省=自动分配空位；同幂等键重试复用同一事务 */
export function fullCarBuy(
  configId: number,
  idempotencyKey: string,
  seatIndex?: number,
): Promise<FullCarBuyResponse> {
  return post<FullCarBuyResponse>(
    `/full-car/${configId}/buy`,
    { seatIndex: seatIndex ?? -1 },
    { headers: { 'Idempotency-Key': idempotencyKey } },
  )
}

/** 满车记录：requestId 精确查询或分页列表 */
export function listFullCarRecords(params?: {
  requestId?: string
  page?: number
  pageSize?: number
}): Promise<FullCarRecordsResponse> {
  return get<FullCarRecordsResponse>('/full-car/records', params)
}

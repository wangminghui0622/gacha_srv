import { get, post } from './api'
import type {
  BoxDetailResponse,
  BoxListResponse,
  BoxQuoteResponse,
  BuyBoxResponse,
  RecordsListResponse,
  WarehouseListResponse,
} from './types'

// 游戏域服务：端点与后端 .api 定义一致（/boxes、/box、/draw、/warehouse）

/** 宝箱列表（公开） */
export function listBoxes(): Promise<BoxListResponse> {
  return get<BoxListResponse>('/boxes')
}

/** 宝箱详情 + 概率公示（公开） */
export function boxDetail(boxId: number): Promise<BoxDetailResponse> {
  return get<BoxDetailResponse>(`/boxes/${boxId}`)
}

/** 购买开奖：提交 DTM SAGA 事务；idempotencyKey 复用时后端幂等返回同一单号。
 *  M10 起可选券（userCouponId）/积分抵扣（useLoyaltyOffset），价格永远服务端算 */
export function buyBox(
  boxId: number,
  idempotencyKey: string,
  options?: { userCouponId?: number; useLoyaltyOffset?: boolean },
): Promise<BuyBoxResponse> {
  return post<BuyBoxResponse>('/box/buy', { boxId, ...options }, {
    headers: { 'Idempotency-Key': idempotencyKey },
  })
}

/** 购买页报价：宝箱原价 + 券/积分抵扣试算（服务端算价，纯预览） */
export function getQuote(
  boxId: number,
  options?: { userCouponId?: number; useLoyaltyOffset?: boolean },
): Promise<BoxQuoteResponse> {
  return get<BoxQuoteResponse>('/box/quote', { boxId, ...options })
}

/** 按全局事务单号查一条记录（购买轮询；未产生时后端返回 2004） */
export function getRecordByRequestId(requestId: string): Promise<RecordsListResponse> {
  return get<RecordsListResponse>('/draw/records', { requestId })
}

/** 抽赏记录分页列表 */
export function listRecords(page: number, pageSize: number): Promise<RecordsListResponse> {
  return get<RecordsListResponse>('/draw/records', { page, pageSize })
}

/** 吉仓列表 */
export function listWarehouse(): Promise<WarehouseListResponse> {
  return get<WarehouseListResponse>('/warehouse')
}

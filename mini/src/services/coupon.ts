import { get, post } from './api'
import type { ClaimCouponResponse, CouponListResponse, CouponTemplateListResponse } from './types'

// 玩法域服务（M10）：端点 /coupons、/coupon-templates（与后端 routes.go 一致）

/** 我的券列表（券中心 + 购买页选择器） */
export function listUserCoupons(): Promise<CouponListResponse> {
  return get<CouponListResponse>('/coupons')
}

/** 可领券模板列表（券中心领取页） */
export function listTemplates(): Promise<CouponTemplateListResponse> {
  return get<CouponTemplateListResponse>('/coupon-templates')
}

/** 领取券：模板单号 → 用户券（后端幂等，网络重试安全） */
export function claimCoupon(templateNo: string): Promise<ClaimCouponResponse> {
  return post<ClaimCouponResponse>('/coupons/claim', { templateNo })
}

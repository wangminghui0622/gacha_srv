// 营销 typed service：方法名与参考 CouponManagementView 调用点一致
// （ListCouponTemplates/GetCouponTemplate/PublishCouponTemplate/GetLoyalty/
// PutLoyalty/GrantCoupons），axios 直连本仓 /promotion/tenants/:tenantId 路由。
// requestId → X-Request-Id 头（幂等，照抄参考 promotionClient(requestId)）。
import { http } from '@/utils/request'

export interface CouponTemplate {
  templateId?: string | number
  version?: string | number
  tenantId?: string | number
  status?: string | number
  kind?: string | number
  amountMinor?: string | number
  minGrossMinor?: string | number
  currency?: string
  funder?: string | number
  scene?: string | number
  percentBps?: number
  minUnitPriceMinor?: string | number
  maxUnitPriceMinor?: string | number
  minItemCount?: number
  maxItemCount?: number
  targetBusinessKey?: string
  validityDays?: number
  rewardTemplateId?: string | number
  rewardCount?: number
}

export interface UserCoupon {
  id?: string
  tenantId?: string
  userId?: string
  templateId?: string
  templateVersion?: string
  status?: string
  customAmountMinor?: string
  expiresAtUnixMillis?: string
  source?: string
  sourceId?: string
}

export interface TenantLoyaltyDefaults {
  tenantId?: string
  loyaltyPointsPerMajor?: string
  maxGrossBps?: number
}

export interface ListCouponTemplatesReply {
  templates?: CouponTemplate[]
  nextAfterTemplateId?: string
  nextAfterVersion?: string
}

export interface GrantCouponsReply {
  coupons?: UserCoupon[]
}

export function promotionClient(requestId?: string) {
  const headers = requestId ? { 'X-Request-Id': requestId } : undefined
  return {
    ListCouponTemplates({ tenantId, afterTemplateId, afterVersion, pageSize }: { tenantId: string | number; afterTemplateId?: string; afterVersion?: string; pageSize?: number }): Promise<ListCouponTemplatesReply> {
      return http.get(`/console/v2/api/promotion/tenants/${encodeURIComponent(String(tenantId))}/coupon-templates`, {
        params: {
          ...(afterTemplateId ? { afterTemplateId, afterVersion } : {}),
          pageSize: pageSize ?? 20,
        },
      })
    },
    GetCouponTemplate({ tenantId, templateId }: { tenantId: string | number; templateId: string }): Promise<CouponTemplate> {
      return http.get(`/console/v2/api/promotion/tenants/${encodeURIComponent(String(tenantId))}/coupon-templates/${encodeURIComponent(templateId)}`)
    },
    PublishCouponTemplate({ template }: { template: CouponTemplate }): Promise<CouponTemplate> {
      return http.post(
        `/console/v2/api/promotion/tenants/${encodeURIComponent(String(template.tenantId ?? 1))}/coupon-templates`,
        template,
        { headers },
      )
    },
    GetLoyalty({ tenantId }: { tenantId: string | number }): Promise<TenantLoyaltyDefaults> {
      return http.get(`/console/v2/api/promotion/tenants/${encodeURIComponent(String(tenantId))}/loyalty-defaults`)
    },
    PutLoyalty({ defaults }: { defaults: TenantLoyaltyDefaults }): Promise<TenantLoyaltyDefaults> {
      return http.put(
        `/console/v2/api/promotion/tenants/${encodeURIComponent(String(defaults.tenantId ?? 1))}/loyalty-defaults`,
        { loyaltyPointsPerMajor: defaults.loyaltyPointsPerMajor, maxGrossBps: defaults.maxGrossBps },
      )
    },
    GrantCoupons(payload: {
      tenantId: string | number
      userId: string | number
      templateId: string | number
      count: number
      customAmountMinor?: string | number
    }): Promise<GrantCouponsReply> {
      return http.post(
        `/console/v2/api/promotion/tenants/${encodeURIComponent(String(payload.tenantId))}/coupon-grants`,
        payload,
        { headers },
      )
    },
  }
}

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
  name?: string
  description?: string
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
  naturalDay?: boolean
  enabled?: boolean
}

export interface CouponPoolEntry {
  id?: string | number
  templateId?: string | number
  templateName?: string
  count?: number
  remainingCount?: number
  drawCount?: number
  startAtUnixMillis?: string | number
  enabled?: boolean
  createdAtUnixMillis?: string | number
  updatedAtUnixMillis?: string | number
}

export interface CouponLedgerEntry {
  id?: string | number
  entryId?: string | number
  userCouponId?: string | number
  userId?: string | number
  eventType?: string
  delta?: number
  beforeCount?: number
  afterCount?: number
  beforeStatus?: number
  afterStatus?: number
  operatorType?: string
  operatorId?: string | number
  orderId?: string
  remark?: string
  createdAtUnixMillis?: string | number
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

export interface ListCouponPoolEntriesReply {
  entries?: CouponPoolEntry[]
  total?: string
}

export interface ListCouponLedgerReply {
  entries?: CouponLedgerEntry[]
  total?: string
}

export interface BatchGrantCouponsReply {
  matchedUsers?: number
  grantedUsers?: number
  failedUsers?: number
  failedUserIds?: Array<string | number>
  stoppedReason?: string
  remainingCount?: number
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
    ListCouponPoolEntries({ tenantId, page, pageSize }: { tenantId: string | number; page?: number; pageSize?: number }): Promise<ListCouponPoolEntriesReply> {
      return http.get(`/console/v2/api/promotion/tenants/${encodeURIComponent(String(tenantId))}/coupon-pool-entries`, {
        params: { page: page ?? 1, pageSize: pageSize ?? 20 },
      })
    },
    CreateCouponPoolEntry({ tenantId, entry }: { tenantId: string | number; entry: CouponPoolEntry }): Promise<CouponPoolEntry> {
      return http.post(
        `/console/v2/api/promotion/tenants/${encodeURIComponent(String(tenantId))}/coupon-pool-entries`,
        entry,
        { headers },
      )
    },
    UpdateCouponPoolEntry({ tenantId, entryId, entry }: { tenantId: string | number; entryId: string; entry: CouponPoolEntry }): Promise<CouponPoolEntry> {
      return http.put(
        `/console/v2/api/promotion/tenants/${encodeURIComponent(String(tenantId))}/coupon-pool-entries/${encodeURIComponent(entryId)}`,
        entry,
        { headers },
      )
    },
    AdjustCouponPoolEntry({ tenantId, entryId, delta, remark }: { tenantId: string | number; entryId: string; delta: number; remark?: string }): Promise<CouponPoolEntry> {
      return http.post(
        `/console/v2/api/promotion/tenants/${encodeURIComponent(String(tenantId))}/coupon-pool-entries/${encodeURIComponent(entryId)}/adjust`,
        { delta, remark },
        { headers },
      )
    },
    ListCouponLedger({ tenantId, entryId, userCouponId, userId, orderId, page, pageSize }: { tenantId: string | number; entryId?: string; userCouponId?: string; userId?: string; orderId?: string; page?: number; pageSize?: number }): Promise<ListCouponLedgerReply> {
      return http.get(`/console/v2/api/promotion/tenants/${encodeURIComponent(String(tenantId))}/coupon-ledger`, {
        params: {
          ...(entryId ? { entryId } : {}),
          ...(userCouponId ? { userCouponId } : {}),
          ...(userId ? { userId } : {}),
          ...(orderId ? { orderId } : {}),
          page: page ?? 1,
          pageSize: pageSize ?? 20,
        },
      })
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
      poolEntryId?: string | number
    }): Promise<GrantCouponsReply> {
      return http.post(
        `/console/v2/api/promotion/tenants/${encodeURIComponent(String(payload.tenantId))}/coupon-grants`,
        payload,
        { headers },
      )
    },
    BatchGrantCoupons(payload: {
      tenantId: string | number
      poolEntryId: string | number
      minSpendMinor: number
      maxSpendMinor: number
      perUserCount: number
      maxUsers: number
      customAmountMinor?: number
    }): Promise<BatchGrantCouponsReply> {
      return http.post(
        `/console/v2/api/promotion/tenants/${encodeURIComponent(String(payload.tenantId))}/coupon-grants/batch`,
        payload,
        { headers },
      )
    },
  }
}

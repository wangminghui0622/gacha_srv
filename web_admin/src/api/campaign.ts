// 活动 typed service：方法名与参考 CampaignManagementView 调用点一致
// （Get/CreateDraft/Publish），axios 直连本仓 /campaigns 路由。
// 本仓仅三种活动（SIGN_IN=1/REDEMPTION_CODE=2/PRIZE_WHEEL=13），config oneof。
import { http } from '@/utils/request'

export interface SignInConfig { rewards?: Array<{ day?: number; loyaltyPoints?: number; prizes?: unknown[] }> }
export interface RedemptionConfig { codes?: unknown[] }
export interface PrizeWheelConfig { slots?: unknown[]; dailyLimit?: number }

export interface CampaignConfig {
  signIn?: SignInConfig
  redemption?: RedemptionConfig
  prizeWheel?: PrizeWheelConfig
}

export interface Campaign {
  tenantId?: string
  campaignId?: string
  campaignKey?: string
  kind?: string | number
  name?: string
  state?: string
  version?: number
  startAtUnixMillis?: string
  endAtUnixMillis?: string
  config?: CampaignConfig
  publishedVersion?: number
}

export const campaignClient = {
  Get({ campaignId }: { campaignId: string; tenantId?: number }): Promise<Campaign> {
    return http.get(`/console/v2/api/campaigns/${encodeURIComponent(campaignId)}`)
  },
  CreateDraft(payload: {
    campaignId?: string | number
    campaignKey?: string
    kind: number
    name: string
    startAtUnixMillis: string
    endAtUnixMillis: string
    config: CampaignConfig
    tenantId?: number
  }): Promise<Campaign> {
    return http.post('/console/v2/api/campaigns/drafts', payload)
  },
  Publish({ campaignId }: { campaignId: string; tenantId?: number }): Promise<Campaign> {
    return http.post(`/console/v2/api/campaigns/${encodeURIComponent(campaignId)}/publish`, {})
  },
}

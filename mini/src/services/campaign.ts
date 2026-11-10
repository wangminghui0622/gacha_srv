import { get, post } from './api'
import { randomUUID } from '@/utils/id'
import type {
  CampaignListResponse,
  CampaignParticipateResponse,
  CampaignPreviewResponse,
  EntitlementListResponse,
  OpenCampaignBoxResponse,
} from './types'

// 活动域服务（M11）：端点 /campaigns、/campaign-boxes（与后端 routes.go 一致）

/** 活动列表 + 我的状态（签到连续天数 / 转盘剩余次数） */
export function listCampaigns(): Promise<CampaignListResponse> {
  return get<CampaignListResponse>('/campaigns')
}

/** 参与预览：签到下一日奖励 / 兑换码奖品 / 转盘槽位（code 仅兑换码传） */
export function campaignPreview(campaignKey: string, code?: string): Promise<CampaignPreviewResponse> {
  return get<CampaignPreviewResponse>(`/campaigns/${campaignKey}/preview`, code ? { code } : undefined)
}

/** 生成幂等键：参与接口强制 Idempotency-Key，同一逻辑动作网络重试须复用同键
 *  （mp-weixin 无 crypto.randomUUID，统一走 utils/id 本地 v4 兜底，与玩法页一致） */
export function newIdemKey(): string {
  return randomUUID()
}

/** 统一参与（签到/兑换码/转盘）：后端按 Idempotency-Key 幂等（重放返回首次结果） */
export function campaignParticipate(
  campaignKey: string,
  code: string,
  idemKey: string,
): Promise<CampaignParticipateResponse> {
  return post<CampaignParticipateResponse>(
    '/campaigns/participate',
    { campaignKey, code: code || undefined },
    { headers: { 'Idempotency-Key': idemKey } },
  )
}

/** 我的活动宝箱资格列表（开箱端点 M11c 上线） */
export function listCampaignBoxes(): Promise<EntitlementListResponse> {
  return get<EntitlementListResponse>('/campaign-boxes')
}

/** 活动宝箱开箱（entitlement 资格全抵零付）：提交 DTM SAGA，客户端轮询 /draw/records。
 *  资格单号含 '#'，路径段必须编码，否则浏览器当片段剥掉 */
export function openCampaignBox(
  entitlementNo: string,
  idemKey: string,
): Promise<OpenCampaignBoxResponse> {
  return post<OpenCampaignBoxResponse>(
    `/campaign-boxes/${encodeURIComponent(entitlementNo)}/open`,
    {},
    { headers: { 'Idempotency-Key': idemKey } },
  )
}

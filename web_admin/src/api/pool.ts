// 赏池 typed service：方法名与参考 PoolEditor/BoxManagement 调用点一致
// （Get/Create/RequestPublication/ApprovePublication/Publish/GetBoxView），
// axios 直连本仓 /gacha 路由。相对参考的刻意差异（M14 裁决服务可调）：
// - 池草稿 = 本仓 BoxDraft（boxName/price/entries），无参考 mode/drawPrice；
// - 无 CreateBox 路由 → 箱子只读；幸运指派走扭蛋配置 AssignLuckyUser
//   （POST /gacha/gashapons/:configId/lucky-assignment，body {prizeIndex,userId}）。
import { http } from '@/utils/request'

export interface PoolEntryDraft {
  entryId?: string | number
  name?: string
  weight?: number
  quantity?: number
  isPrize?: boolean
  displayValueFen?: string | number
  rollingLimited?: boolean
}

export interface BoxDraft {
  poolId?: string | number
  version?: string | number
  status?: string | number
  boxName?: string
  price?: string | number
  description?: string
  entries?: PoolEntryDraft[]
  rollingRulesJson?: string
  configHash?: string
  publishedAtUnixMillis?: string
}

export interface PoolPublicationApproval {
  poolId?: string
  version?: string
  configHash?: string
  requesterId?: string
  approverId?: string
  approved?: boolean
}

export interface BoxDetailEntry {
  entryId?: string
  name?: string
  weight?: number
  remaining?: number
  isPrize?: boolean
  probability?: string
}

export interface BoxDetail {
  boxId?: string
  boxName?: string
  price?: string
  description?: string
  soldOut?: boolean
  entries?: BoxDetailEntry[]
}

export function poolClient(idempotencyKey?: string) {
  const headers = idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : undefined
  return {
    Get({ poolId, version }: { poolId: string; version: string }): Promise<BoxDraft> {
      return http.get(`/console/v2/api/gacha/pools/${encodeURIComponent(poolId)}/versions/${encodeURIComponent(version)}`)
    },
    Create({ box }: { box: BoxDraft }): Promise<BoxDraft> {
      return http.post('/console/v2/api/gacha/pools/drafts', { box }, { headers })
    },
    RequestPublication({ poolId, version, expectedConfigHash }: { poolId: string; version: string; expectedConfigHash: string }): Promise<PoolPublicationApproval> {
      return http.post(`/console/v2/api/gacha/pools/${encodeURIComponent(poolId)}/versions/${encodeURIComponent(version)}/publication-request`, { expectedConfigHash })
    },
    ApprovePublication({ poolId, version, expectedConfigHash }: { poolId: string; version: string; expectedConfigHash: string }): Promise<PoolPublicationApproval> {
      return http.post(`/console/v2/api/gacha/pools/${encodeURIComponent(poolId)}/versions/${encodeURIComponent(version)}/approval`, { expectedConfigHash })
    },
    Publish({ poolId, version, expectedConfigHash }: { poolId: string; version: string; expectedConfigHash: string }): Promise<BoxDraft> {
      return http.post(`/console/v2/api/gacha/pools/${encodeURIComponent(poolId)}/versions/${encodeURIComponent(version)}/publish`, { expectedConfigHash })
    },
    GetBoxView({ boxId }: { boxId: string }, options?: { silentError?: boolean }): Promise<BoxDetail> {
      return http.get(`/console/v2/api/gacha/boxes/${encodeURIComponent(boxId)}/view`, { silentError: options?.silentError })
    },
    AssignLuckyUser({ configId, prizeIndex, userId }: { configId: string; prizeIndex: number; userId: string }): Promise<{ status?: string }> {
      return http.post(`/console/v2/api/gacha/gashapons/${encodeURIComponent(configId)}/lucky-assignment`, { prizeIndex, userId })
    },
  }
}

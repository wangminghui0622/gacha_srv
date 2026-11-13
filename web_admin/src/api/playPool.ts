import { http } from '@/utils/request'

export interface PlayPoolPrize {
  prizeIndex: number
  prizeGrade?: string
  specialKind?: string
  prizeType?: number
  name: string
  skuId?: number
  childPoolId?: number
  weight?: number
  quantity?: number
  remaining?: number
  isPrize?: boolean
  targetValue?: number
  cashFen?: number
  pointsAmount?: number
  couponTemplateNo?: string
  costFen?: number
  imageUrl?: string
  parentIndex?: number
  hasParent?: boolean
  extraJson?: string
}

export interface PlayPoolSummary {
  poolId?: string
  latestVersion?: string
  mode?: string
  name?: string
  status?: string | number
  publishedAt?: string
}

export interface PlayPoolVersion {
  poolId?: string
  version?: string
  mode?: string
  name?: string
  priceFen?: string
  configJson?: string
  configHash?: string
  status?: string | number
  publishedAt?: string
  prizes?: PlayPoolPrize[]
}

export const playPoolClient = {
  List(params: { afterPoolId?: string; pageSize?: number; mode?: string } = {}): Promise<{ items?: PlayPoolSummary[] }> {
    return http.get('/console/v2/api/gacha/play-pools', { params })
  },
  Get({ poolId, version }: { poolId: string; version: string }): Promise<PlayPoolVersion> {
    return http.get(`/console/v2/api/gacha/play-pools/${encodeURIComponent(poolId)}/versions/${encodeURIComponent(version)}`)
  },
  CreateDraft(payload: {
    idempotencyKey: string
    poolId?: string
    mode: string
    name: string
    priceFen: number
    configJson: string
    prizes: PlayPoolPrize[]
  }): Promise<{ poolId?: string; version?: string; status?: string }> {
    return http.post('/console/v2/api/gacha/play-pools/drafts', payload, {
      headers: { 'Idempotency-Key': payload.idempotencyKey },
    })
  },
  Publish({ poolId, version }: { poolId: string; version: string }): Promise<{ status?: string }> {
    return http.post(`/console/v2/api/gacha/play-pools/${encodeURIComponent(poolId)}/versions/${encodeURIComponent(version)}/publish`, {})
  },
}

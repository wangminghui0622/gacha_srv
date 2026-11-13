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

export interface AdminPlayPool {
  poolId?: string
  name?: string
  priceFen?: string
  startAt?: string
  status?: string | number
  mode?: string
  tags?: string[]
  periods?: string
  weight?: string
  ipId?: string
  latestVersion?: string
  createdAt?: string
}

export interface ListAdminPlayPoolsReply {
  items?: AdminPlayPool[]
  total?: string
  page?: number
  pageSize?: number
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
  AdminList(params: {
    keyword: string
    mode: string
    ipId: string
    status: string
    tag: string
    sortOrder: string
    page: number
    pageSize: number
  }): Promise<ListAdminPlayPoolsReply> {
    return http.get('/console/v2/api/gacha/admin/pools', { params })
  },
  AdminUpdate(poolId: string, payload: {
    name: string
    ipId: string
    tags: string[]
    periods: number
    weight: number
    priceFen: number
  }): Promise<AdminPlayPool> {
    return http.put(`/console/v2/api/gacha/admin/pools/${encodeURIComponent(poolId)}`, payload)
  },
  SaveAdminWeights(weights: Array<{ poolId: string; weight: number }>): Promise<{ saved?: boolean }> {
    return http.put('/console/v2/api/gacha/admin/pool-weights', { weights })
  },
  DeleteAdminPools(poolIds: string[]): Promise<{ deleted?: string }> {
    return http.post('/console/v2/api/gacha/admin/pools/delete', { poolIds })
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
    ipId?: string
    tags?: string[]
    periods?: number
    weight?: number
  }): Promise<{ poolId?: string; version?: string; status?: string }> {
    return http.post('/console/v2/api/gacha/play-pools/drafts', payload, {
      headers: { 'Idempotency-Key': payload.idempotencyKey },
    })
  },
  Publish({ poolId, version }: { poolId: string; version: string }): Promise<{ status?: string }> {
    return http.post(`/console/v2/api/gacha/play-pools/${encodeURIComponent(poolId)}/versions/${encodeURIComponent(version)}/publish`, {})
  },
}

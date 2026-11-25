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

export interface PlayPoolCreateInitial {
  poolId?: string
  mode?: string
  name?: string
  priceYuan?: number
  ipId?: string
  tags?: string[]
  periods?: number
  weight?: number
  configJson?: string
  prizes?: PlayPoolPrize[]
  coverImage?: string
  coverThumb?: string
  startAt?: string
  endAt?: string
  discount?: number
  isOnSale?: boolean
  isNew?: boolean
  isRecommended?: boolean
  isBargain?: boolean
  isPresale?: boolean
  isMiniapp?: boolean
  isWelfare?: boolean
  excludeTask?: boolean
  allowBoxInBox?: boolean
  coupons?: Array<{ id: number; name: string; description: string; status: string }>
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

export interface PlayPoolReferenceItem {
  poolId?: string | number
  poolName?: string
  mode?: string
  periods?: string | number
  status?: string | number
  tags?: string[]
  source?: string
  refType?: string | number
  refId?: string | number
  prizeGrade?: string
  quantity?: string | number
}

export interface QueryPlayPoolReferencesResp {
  items?: PlayPoolReferenceItem[]
  total?: string | number
  page?: string | number
  pageSize?: string | number
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
  adminJson?: string
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
  QueryReferences(params: {
    refType: number
    refId: number
    page?: number
    pageSize?: number
  }): Promise<QueryPlayPoolReferencesResp> {
    return http.get('/console/v2/api/gacha/play-pool-references', { params })
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
    adminJson?: string
  }): Promise<{ poolId?: string; version?: string; status?: string }> {
    return http.post('/console/v2/api/gacha/play-pools/drafts', payload, {
      headers: { 'Idempotency-Key': payload.idempotencyKey },
    })
  },
  Publish({ poolId, version }: { poolId: string; version: string }): Promise<{ status?: string }> {
    return http.post(`/console/v2/api/gacha/play-pools/${encodeURIComponent(poolId)}/versions/${encodeURIComponent(version)}/publish`, {})
  },
  Retire({ poolId }: { poolId: string }): Promise<{ status?: string; retiredVersions?: string | number }> {
    return http.post(`/console/v2/api/gacha/play-pools/${encodeURIComponent(poolId)}/retire`, {})
  },
}

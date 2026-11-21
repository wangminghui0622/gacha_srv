import { http } from '@/utils/request'

export interface CollectionSetItem {
  id: number
  code: string
  name: string
  version: number
  status: number
  accumulateType: number
  cycleType: number
  sort: number
  configJson: string
  startAt: string
  endAt: string
  createdAt: string
  updatedAt: string
}

export interface CollectionSetListResp {
  items: CollectionSetItem[]
  total: number
  page: number
  pageSize: number
}

export interface CollectionSetCreatePayload {
  code: string
  name: string
  accumulateType: number
  cycleType: number
  sort: number
  status: number
  configJson?: string
  startAt?: string
  endAt?: string
}

export interface CollectionSetUpdatePayload {
  name: string
  accumulateType: number
  cycleType: number
  sort: number
  status: number
  configJson?: string
  startAt?: string
  endAt?: string
}

export interface CollectionRewardStat {
  name: string
  claimed: number
  stock: number
}

export interface CollectionSetData {
  participants: number
  completed: number
  claims: number
  pendingClaims: number
  rewards: CollectionRewardStat[]
}

export const codexClient = {
  List(params: { keyword?: string; status?: number; page?: number; pageSize?: number }): Promise<CollectionSetListResp> {
    return http.get('/console/v2/api/gacha/collection-sets', { params })
  },
  Create(payload: CollectionSetCreatePayload): Promise<CollectionSetItem> {
    return http.post('/console/v2/api/gacha/collection-sets', payload)
  },
  Update(id: number, payload: CollectionSetUpdatePayload): Promise<CollectionSetItem> {
    return http.put(`/console/v2/api/gacha/collection-sets/${encodeURIComponent(String(id))}`, payload)
  },
  SetStatus(id: number, status: number): Promise<CollectionSetItem> {
    return http.put(`/console/v2/api/gacha/collection-sets/${encodeURIComponent(String(id))}/status`, { status })
  },
  Data(id: number): Promise<CollectionSetData> {
    return http.get(`/console/v2/api/gacha/collection-sets/${encodeURIComponent(String(id))}/data`)
  },
}

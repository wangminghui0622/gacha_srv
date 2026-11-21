import { http } from '@/utils/request'

export type IntLike = number | string

export interface MemoryMatchGlobalRules {
  id?: IntLike
  name?: string
  pairProbabilityMode?: IntLike
  initialHandCount?: IntLike
  pairSize?: IntLike
  replenishCount?: IntLike
  secondDestinyEnabled?: IntLike
  destinyDoubleEnabled?: IntLike
  destinyIncome?: IntLike
  doubleDestinyBuff?: IntLike
  destinyReplenishCost?: IntLike
  resumeEnabled?: IntLike
  hostModeEnabled?: IntLike
  hostMinPairs?: IntLike
  hostShareBps?: IntLike
  revenueBase?: IntLike
  ledgerType?: IntLike
  status?: IntLike
  createdAt?: string
  updatedAt?: string
}

export interface MemoryMatchDeckTemplate {
  id?: IntLike
  name?: string
  status?: IntLike
  totalCards?: IntLike
  weight?: IntLike
  configJson?: string
  createdAt?: string
  updatedAt?: string
}

export interface MemoryMatchAppearance {
  id?: IntLike
  type?: IntLike
  name?: string
  imageUrl?: string
  backImageUrl?: string
  status?: IntLike
  free?: IntLike
  exchangePoints?: string
  functionType?: string
  functionConfig?: string
  weight?: IntLike
  createdAt?: string
  updatedAt?: string
}

export interface ListMemoryMatchDeckTemplatesReply {
  items?: MemoryMatchDeckTemplate[]
  total?: IntLike
}

export interface ListMemoryMatchAppearancesReply {
  items?: MemoryMatchAppearance[]
  total?: IntLike
}

export interface UploadFileReply {
  objectKey?: string
  url?: string
  size?: IntLike
}

export function uploadImageFile(file: File, dir = 'gacha/match'): Promise<UploadFileReply> {
  const form = new FormData()
  form.append('file', file)
  form.append('dir', dir)
  return http.post('/console/v2/api/upload', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

export const gachaMatchClient = {
  GetRules(): Promise<{ rules?: MemoryMatchGlobalRules }> {
    return http.get('/console/v2/api/gacha/match/rules')
  },
  SaveRules(rules: MemoryMatchGlobalRules): Promise<{ rules?: MemoryMatchGlobalRules }> {
    return http.put('/console/v2/api/gacha/match/rules', { rules })
  },
  ListDeckTemplates(params: {
    keyword?: string
    status?: number
    page?: number
    pageSize?: number
  }): Promise<ListMemoryMatchDeckTemplatesReply> {
    return http.get('/console/v2/api/gacha/match/deck-templates', { params })
  },
  SaveDeckTemplate(template: MemoryMatchDeckTemplate): Promise<{ template?: MemoryMatchDeckTemplate }> {
    return http.post('/console/v2/api/gacha/match/deck-templates', { template })
  },
  DeleteDeckTemplate(id: IntLike): Promise<{ deleted?: boolean }> {
    return http.delete(`/console/v2/api/gacha/match/deck-templates/${encodeURIComponent(String(id))}`)
  },
  ListAppearances(params: {
    type?: number
    status?: number
    keyword?: string
    page?: number
    pageSize?: number
  }): Promise<ListMemoryMatchAppearancesReply> {
    return http.get('/console/v2/api/gacha/match/appearances', { params })
  },
  SaveAppearance(item: MemoryMatchAppearance): Promise<{ item?: MemoryMatchAppearance }> {
    return http.post('/console/v2/api/gacha/match/appearances', { item })
  },
  DeleteAppearance(id: IntLike): Promise<{ deleted?: boolean }> {
    return http.delete(`/console/v2/api/gacha/match/appearances/${encodeURIComponent(String(id))}`)
  },
}

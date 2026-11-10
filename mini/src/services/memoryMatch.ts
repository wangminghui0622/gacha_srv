import { get, post, put } from './api'

import type {
  MemoryMatchActionResponse,
  MemoryMatchAppearanceView,
  MemoryMatchCompleteResponse,
  MemoryMatchConfigResponse,
  MemoryMatchPurchaseResponse,
  MemoryMatchRoundActionResponse,
  MemoryMatchRoundResponse,
  MemoryMatchSessionResponse,
  MemoryMatchSkipResponse,
} from './types'

// M15b 记忆翻牌：services/ 唯一 API 层（规范 05-api-layer）

export function getMemoryMatchConfig(configId: number): Promise<MemoryMatchConfigResponse> {
  return get(`/memory-match/configs/${configId}`)
}

// 购买：强制 Idempotency-Key（网关轮询 SAGA 至会话建立后返回同步形态响应）
export function purchaseMemoryMatch(
  body: {
    configId: number
    facePresetId?: number
    backPresetId?: number
    userCouponId?: number
    paymentMethod?: string
  },
  idempotencyKey: string,
): Promise<MemoryMatchPurchaseResponse> {
  return post('/memory-match/purchase', body, { headers: { 'Idempotency-Key': idempotencyKey } })
}

export function getMemoryMatchAppearance(configId: number): Promise<MemoryMatchAppearanceView> {
  return get('/memory-match/appearance', { configId })
}

export function setMemoryMatchAppearance(
  configId: number,
  facePresetId: number,
  backPresetId: number,
): Promise<MemoryMatchAppearanceView> {
  return put('/memory-match/appearance', { configId, facePresetId, backPresetId })
}

export function getMemoryMatchSession(sessionId: number): Promise<MemoryMatchSessionResponse> {
  return get(`/memory-match/sessions/${sessionId}`)
}

export function selectMemoryMatchDestiny(
  sessionId: number,
  patternId: number,
): Promise<MemoryMatchActionResponse> {
  return post(`/memory-match/sessions/${sessionId}/destiny`, { patternId })
}

export function selectMemoryMatchSecondDestiny(
  sessionId: number,
  patternId: number,
): Promise<MemoryMatchActionResponse> {
  return post(`/memory-match/sessions/${sessionId}/second-destiny`, { patternId })
}

export function getMemoryMatchRound(
  sessionId: number,
  roundNo: number,
): Promise<MemoryMatchRoundResponse> {
  return get(`/memory-match/sessions/${sessionId}/rounds/${roundNo}`)
}

export function applyMemoryMatchPair(
  sessionId: number,
  roundNo: number,
  firstCard: number,
  secondCard: number,
): Promise<MemoryMatchRoundActionResponse> {
  return post(`/memory-match/sessions/${sessionId}/rounds/${roundNo}/pair`, {
    firstCard,
    secondCard,
  })
}

export function applyMemoryMatchDestinyDraw(
  sessionId: number,
  roundNo: number,
): Promise<MemoryMatchRoundActionResponse> {
  return post(`/memory-match/sessions/${sessionId}/rounds/${roundNo}/destiny-draw`)
}

export function completeMemoryMatchRound(
  sessionId: number,
  roundNo: number,
): Promise<MemoryMatchCompleteResponse> {
  return post(`/memory-match/sessions/${sessionId}/rounds/${roundNo}/complete`)
}

export function skipMemoryMatch(sessionId: number): Promise<MemoryMatchSkipResponse> {
  return post(`/memory-match/sessions/${sessionId}/skip`)
}

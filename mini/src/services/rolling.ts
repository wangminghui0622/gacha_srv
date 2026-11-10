import { get } from './api'

import type { RollingRecentWinsResponse } from './types'

// 滚动中奖 feed（首页 marquee；公开端点）

/** 最近中奖列表（marqueeOnly=true 只回高档/限量，过滤普通中奖） */
export function listRecentWins(options?: {
  marqueeOnly?: boolean
  limit?: number
  boxId?: number
}): Promise<RollingRecentWinsResponse> {
  return get<RollingRecentWinsResponse>('/rolling-draw/recent-wins', {
    marqueeOnly: options?.marqueeOnly ?? true,
    limit: options?.limit,
    boxId: options?.boxId,
  })
}

// 用户盈亏曲线 typed service（M14e 空态 stub：dataReady=false 恒空）。
import { http } from '@/utils/request'
import type { TimeGrainKey } from '@/navigation/entityRoutes'

const grainToProto: Record<TimeGrainKey, number> = {
  hour: 1,
  day: 2,
  month: 3,
}

export interface UserProfitCurvePoint {
  bucketStart?: string
  spendMinor?: string
  awardCostMinor?: string
  profitMinor?: string
  currency?: string
}

export interface UserProfitCurveReply {
  points?: UserProfitCurvePoint[]
  dataReady?: boolean
}

export function fetchUserProfitCurve(params: {
  userId: number
  tenantId?: number
  grain: TimeGrainKey
  from: string
  to: string
}): Promise<UserProfitCurveReply> {
  return http.get(`/console/v2/api/analytics/users/${params.userId}/profit-curve`, {
    params: {
      tenantId: params.tenantId,
      grain: grainToProto[params.grain],
      from: params.from,
      to: params.to,
    },
  })
}

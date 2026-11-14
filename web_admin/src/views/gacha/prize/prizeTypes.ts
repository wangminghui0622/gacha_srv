import type { PlayPoolPrize } from '@/api/playPool'

export interface DisplayPrize extends PlayPoolPrize {
  __rowKey: string
  __copyIndex: number
  __displayIndex: number
  __expanded: boolean
  __badgeIndex: number
}

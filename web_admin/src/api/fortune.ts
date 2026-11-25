import { http } from '@/utils/request'

export type FortuneRewardItemType =
  | 'FORTUNE_REWARD_ITEM_TYPE_FORTUNE'
  | 'FORTUNE_REWARD_ITEM_TYPE_POINTS'
  | 'FORTUNE_REWARD_ITEM_TYPE_COUPON'
  | 'FORTUNE_REWARD_ITEM_TYPE_ITEM'
  | 'FORTUNE_REWARD_ITEM_TYPE_BOX'
  | 'FORTUNE_REWARD_ITEM_TYPE_MATCH_APPEARANCE'

export interface FortuneRewardItem {
  type?: FortuneRewardItemType
  value?: string
  quantity?: number
  customAmountMinor?: string | number
  name?: string
  thumbnail?: string
  extra?: string
}

export interface FortuneReward {
  id?: string | number
  name?: string
  exchangePoints?: string | number
  directBuyMinor?: string | number
  stock?: number
  startAtUnixMillis?: string | number
  endAtUnixMillis?: string | number
  cashbackMinor?: string | number
  rewards?: FortuneRewardItem[]
  coverUrl?: string
  tag?: string
  enabled?: boolean
  createdAtUnixMillis?: string | number
  updatedAtUnixMillis?: string | number
}

export interface ListFortuneRewardsReply {
  items?: FortuneReward[]
  total?: string
}

export function fortuneRewardClient(requestId?: string) {
  const headers = requestId ? { 'X-Request-Id': requestId } : undefined
  return {
    List(params: { tenantId: string | number; page?: number; pageSize?: number }): Promise<ListFortuneRewardsReply> {
      return http.get(`/console/v2/api/campaigns/fortune-rewards`, {
        params: { page: params.page ?? 1, pageSize: params.pageSize ?? 20 },
      })
    },
    Create(reward: FortuneReward): Promise<FortuneReward> {
      return http.post('/console/v2/api/campaigns/fortune-rewards', reward, { headers })
    },
    Update(id: string | number, reward: FortuneReward): Promise<FortuneReward> {
      return http.put(`/console/v2/api/campaigns/fortune-rewards/${encodeURIComponent(String(id))}`, reward, { headers })
    },
  }
}

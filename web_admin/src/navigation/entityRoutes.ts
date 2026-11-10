export type TimeGrainKey = 'hour' | 'day' | 'month'

export const entityRoutes = {
  userOverview(userId: number | string) {
    return { name: 'user-overview', params: { userId: String(userId) } } as const
  },
  userProfit(userId: number | string, query?: { grain?: TimeGrainKey }) {
    return { name: 'user-profit', params: { userId: String(userId) }, query } as const
  },
  product(productId: number | string) {
    return { name: 'catalog-product', params: { productId: String(productId) } } as const
  },
  sku(productId: number | string, skuId: number | string) {
    return {
      name: 'catalog-sku',
      params: { productId: String(productId), skuId: String(skuId) },
    } as const
  },
  pool(poolId: number | string) {
    return { name: 'gacha-pool', params: { poolId: String(poolId) } } as const
  },
  box(boxId: number | string) {
    return { name: 'gacha-box', params: { boxId: String(boxId) } } as const
  },
}

// tokenStorage：认证令牌唯一存取门面（规范 04-state-routing 第 3 条）
// 统一 key 与存取逻辑；小程序形态用 uni storage 替代 web 的 js-cookie
const TOKEN_KEY = 'gacha_token'

export const tokenStorage = {
  get(): string {
    try {
      return uni.getStorageSync(TOKEN_KEY) || ''
    } catch {
      return ''
    }
  },
  set(token: string) {
    uni.setStorageSync(TOKEN_KEY, token)
  },
  remove() {
    uni.removeStorageSync(TOKEN_KEY)
  },
}

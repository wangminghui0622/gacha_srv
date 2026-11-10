import { defineStore } from 'pinia'

import { tokenStorage } from '@/utils/storage'

import type { UserProfile } from '@/services/types'

// 认证域 store（规范 04-state-routing 第 1-3 条，Options 式统一写法）
export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null as UserProfile | null,
  }),
  getters: {
    isAuthenticated: (state) => state.user !== null,
    balance: (state) => state.user?.balance ?? '0.00',
  },
  actions: {
    /** 登录/注册成功：写入令牌并缓存用户资料（资料可在随后异步补入） */
    setAuth(token: string, user: UserProfile | null = null) {
      tokenStorage.set(token)
      this.user = user
    },
    /** 刷新用户资料（余额变动后同步） */
    setUser(user: UserProfile) {
      this.user = user
    },
    /** 清除登录态（退出/令牌失效） */
    clearAuth() {
      tokenStorage.remove()
      this.user = null
    },
  },
})

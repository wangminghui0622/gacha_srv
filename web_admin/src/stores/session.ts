import { defineStore } from 'pinia'
import { nextTick } from 'vue'
import {
  clearSessionCookies,
  getAccessToken,
  setAccessToken,
  setRefreshToken,
  setSessionId,
} from '@/utils/cache'
import { parseConsoleJwt, type ConsoleJwtClaims } from '@/utils/jwt'
import {
  fetchConsolePermissions,
  loginConsole,
  logoutConsole,
  refreshConsoleSession,
  type ConsoleLoginReply,
  type ConsoleSessionTokens,
} from '@/api/console'
import type { PermissionNode } from '@/access/grants'
import router from '@/router'
import { resetDynamicRoutes, syncDynamicRoutes } from '@/router/dynamic'
import { canAccessRoute } from '@/navigation/routeCatalog'
import { i18n } from '@/i18n'

export interface SessionState {
  accessToken: string | null
  claims: ConsoleJwtClaims | null
  permissionNodes: PermissionNode[]
  permissionsLoaded: boolean
  permissionsLoading: boolean
}

function applyTokens(tokens: ConsoleSessionTokens): ConsoleJwtClaims | null {
  setAccessToken(tokens.accessToken)
  setRefreshToken(tokens.refreshToken)
  setSessionId(tokens.sessionId)
  return parseConsoleJwt(tokens.accessToken)
}

export const useSessionStore = defineStore('session', {
  state: (): SessionState => ({
    accessToken: getAccessToken() ?? null,
    claims: getAccessToken() ? parseConsoleJwt(getAccessToken()!) : null,
    permissionNodes: [],
    permissionsLoaded: false,
    permissionsLoading: false,
  }),
  getters: {
    roles(state): string[] {
      return state.claims?.roles ?? []
    },
    displayName(state): string {
      return state.claims?.name || `${i18n.global.t('dashboard.operator')} ${state.claims?.actor_id ?? ''}`
    },
    tenantId(state): number | undefined {
      return state.claims?.tenant_id
    },
  },
  actions: {
    async login(payload: {
      tenantId?: number
      username?: string
      password?: string
      otpCode?: string
      loginTicket?: string
    }): Promise<ConsoleLoginReply> {
      const reply = await loginConsole(payload)
      if (reply.session?.accessToken) {
        this.accessToken = reply.session.accessToken
        this.claims = applyTokens(reply.session)
        await this.loadPermissions()
      }
      return reply
    },
    async refresh() {
      const tokens = await refreshConsoleSession()
      this.accessToken = tokens.accessToken
      this.claims = applyTokens(tokens)
      await this.loadPermissions()
    },
    async loadPermissions() {
      if (!this.accessToken) {
        this.permissionNodes = []
        this.permissionsLoaded = false
        return
      }
      const token = this.accessToken
      this.permissionsLoading = true
      try {
        const tree = await fetchConsolePermissions()
        // 注销或换账号期间返回的旧请求不得重新恢复权限。
        if (this.accessToken !== token) return
        this.permissionNodes = tree.nodes ?? []
        this.permissionsLoaded = true
        const name = router.currentRoute.value.name
        if (typeof name === 'string' && name !== 'login' && !canAccessRoute(name, this.permissionNodes)) {
          await router.replace('/dashboard')
          await nextTick()
        }
        syncDynamicRoutes(router, this.permissionNodes)
      } finally {
        this.permissionsLoading = false
      }
    },
    async ensurePermissions() {
      if (this.permissionsLoaded) {
        return
      }
      await this.loadPermissions()
    },
    async logout() {
      try {
        if (this.accessToken) {
          await logoutConsole()
        }
      } finally {
        clearSessionCookies()
        this.accessToken = null
        this.claims = null
        this.permissionNodes = []
        this.permissionsLoaded = false
        // 先卸载业务页，再删除其命名路由，避免存活的 RouterLink 解析失败。
        await router.replace('/login')
        await nextTick()
        resetDynamicRoutes(router)
      }
    },
    hasRole(role: string): boolean {
      return this.roles.includes(role)
    },
  },
})

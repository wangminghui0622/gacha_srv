import NProgress from 'nprogress'
import 'nprogress/nprogress.css'
import type { Router } from 'vue-router'
import { canAccessRoute } from '@/navigation/routeCatalog'
import { useSessionStore } from '@/stores/session'
import { getAccessToken } from '@/utils/cache'
import { i18n } from '@/i18n'
import { installDynamicRoutes } from './dynamic'

NProgress.configure({ showSpinner: false })

export function installRouterGuard(router: Router): void {
  router.beforeEach(async (to, _from, next) => {
    NProgress.start()
    // 路由目录 title 存 i18n KEY（双语是相对参考的刻意差异）。
    const title = (to.meta.title as string | undefined) ?? import.meta.env.VITE_APP_TITLE
    document.title = i18n.global.t(title)

    const token = getAccessToken()
    if (!token) {
      if (to.meta.public) {
        next()
        return
      }
      next({ path: '/login', query: { redirect: to.fullPath } })
      return
    }

    if (to.path === '/login') {
      next({ path: '/catalog' })
      return
    }

    const session = useSessionStore()
    try {
      await session.ensurePermissions()
    } catch {
      next({ path: '/login', query: { redirect: to.fullPath } })
      return
    }

    if (!router.hasRoute('console-root')) {
      installDynamicRoutes(router, session.permissionNodes)
      next({ ...to, replace: true })
      return
    }

    const name = typeof to.name === 'string' ? to.name : undefined
    if (!canAccessRoute(name, session.permissionNodes)) {
      next({ path: '/catalog' })
      return
    }
    next()
  })

  router.afterEach(() => {
    NProgress.done()
  })
}

import type { Router, RouteRecordRaw } from 'vue-router'
import type { PermissionNode } from '@/access/grants'
import ConsoleLayout from '@/layout/ConsoleLayout.vue'
import { accessibleRoutes, consoleRouteCatalog, toRouteRecords } from '@/navigation/routeCatalog'

export const consoleRootName = 'console-root'

export function installDynamicRoutes(router: Router, nodes: PermissionNode[]): void {
  if (router.hasRoute(consoleRootName)) {
    return
  }
  const children = toRouteRecords(accessibleRoutes(nodes))
  const root: RouteRecordRaw = {
    path: '/',
    name: consoleRootName,
    component: ConsoleLayout,
    redirect: '/dashboard',
    children,
  }
  router.addRoute(root)
  router.addRoute({
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    redirect: '/dashboard',
  })
}

export function resetDynamicRoutes(router: Router): void {
  if (router.hasRoute('not-found')) {
    router.removeRoute('not-found')
  }
  if (router.hasRoute(consoleRootName)) {
    router.removeRoute(consoleRootName)
  }
}

/** 保留布局，仅同步权限发生变化的子路由，避免刷新权限重建整个页面壳层。 */
export function syncDynamicRoutes(router: Router, nodes: PermissionNode[]): void {
  if (!router.hasRoute(consoleRootName)) return
  const allowed = accessibleRoutes(nodes)
  const names = new Set(allowed.map((item) => item.name))
  for (const item of consoleRouteCatalog) {
    if (!names.has(item.name) && router.hasRoute(item.name)) router.removeRoute(item.name)
  }
  for (const record of toRouteRecords(allowed)) {
    if (record.name && !router.hasRoute(record.name)) router.addRoute(consoleRootName, record)
  }
}

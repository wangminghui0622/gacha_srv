// 路由目录：结构与 server_v2 参考控制台一致；title 与菜单分组标题存 i18n KEY
// （web_admin 双语是刻意差异），渲染处（NavigationMenu/ConsoleLayout/路由守卫/
// 工作台快捷入口）统一 $t()。
import type { Component } from 'vue'
import type { RouteRecordRaw } from 'vue-router'
import {
  AppstoreOutlined,
  DashboardOutlined,
  FireOutlined,
  FlagOutlined,
  FilterOutlined,
  GiftOutlined,
  LineChartOutlined,
  SafetyOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  ShoppingOutlined,
  TeamOutlined,
  UnorderedListOutlined,
  UserOutlined,
} from '@ant-design/icons-vue'
import { allows, type PermissionNode } from '@/access/grants'

export interface ConsolePermission {
  resource: string
  action: string
}

export interface ConsoleRouteDef {
  name: string
  path: string
  title: string
  /** 空表示登录后始终可见，例如工作台。 */
  permission?: ConsolePermission
  permissions?: ConsolePermission[]
  showInMenu: boolean
  menuGroup?: 'operations' | 'business' | 'governance'
  icon?: Component
  backFallback?: string
  component: () => Promise<unknown>
  /** 未实现页面：有权限时菜单可见但不可进入。 */
  placeholder?: boolean
}

export const consoleRouteCatalog: ConsoleRouteDef[] = [
  {
    name: 'dashboard',
    path: 'dashboard',
    title: 'nav.dashboard',
    permission: undefined,
    showInMenu: true,
    icon: DashboardOutlined,
    backFallback: '/dashboard',
    component: () => import('@/views/dashboard/DashboardView.vue'),
  },
  {
    name: 'task-progress',
    path: 'operations/tasks',
    title: 'nav.taskProgress',
    permission: { resource: 'task.progress', action: 'read' },
    showInMenu: true,
    menuGroup: 'operations',
    icon: UnorderedListOutlined,
    backFallback: '/dashboard',
    component: () => import('@/views/operations/TaskProgressView.vue'),
  },
  {
    name: 'task-plan',
    path: 'operations/task-plans',
    title: 'nav.taskPlan',
    permissions: ['read', 'write', 'request_publication', 'approve_publication', 'publish'].map(action => ({ resource: 'task.plan', action })),
    showInMenu: true,
    menuGroup: 'operations',
    icon: UnorderedListOutlined,
    backFallback: '/dashboard',
    component: () => import('@/views/operations/TaskPlanView.vue'),
  },
  {
    name: 'user-profit',
    path: 'analytics/users/:userId/profit',
    title: 'nav.userProfit',
    permission: { resource: 'reporting.metric', action: 'read' },
    showInMenu: false,
    icon: LineChartOutlined,
    backFallback: '/dashboard',
    component: () => import('@/views/analytics/UserProfitCurveView.vue'),
  },
  {
    name: 'user-overview',
    path: 'users/:userId',
    title: 'nav.userOverview',
    permission: { resource: 'reporting.metric', action: 'read' },
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/users/UserOverviewView.vue'),
  },
  {
    name: 'catalog-product',
    path: 'catalog/products/:productId',
    title: 'nav.product',
    permission: { resource: 'catalog.product', action: 'read' },
    showInMenu: false,
    icon: ShoppingOutlined,
    backFallback: '/catalog',
    component: () => import('@/views/catalog/ProductDetailView.vue'),
  },
  {
    name: 'catalog-sku',
    path: 'catalog/products/:productId/sku/:skuId',
    title: 'nav.sku',
    permission: { resource: 'catalog.product', action: 'read' },
    showInMenu: false,
    backFallback: '/catalog/products/:productId',
    component: () => import('@/views/catalog/SkuDetailView.vue'),
  },
  {
    name: 'gacha-pool',
    path: 'gacha/pools/:poolId',
    title: 'nav.pool',
    permission: { resource: 'gacha.pool', action: 'read' },
    showInMenu: false,
    icon: AppstoreOutlined,
    backFallback: '/gacha',
    component: () => import('@/views/gacha/PoolDetailView.vue'),
  },
  {
    name: 'gacha-box',
    path: 'gacha/boxes/:boxId',
    title: 'nav.box',
    permissions: ['read', 'create', 'assign_lucky'].map(action => ({ resource: 'gacha.pool', action })),
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/gacha/BoxDetailView.vue'),
  },
  {
    name: 'promotion',
    path: 'promotion',
    title: 'nav.promotion',
    permissions: [
      { resource: 'promotion.coupon_template', action: 'read' },
      { resource: 'promotion.coupon_template', action: 'publish' },
      { resource: 'promotion.user_coupon', action: 'grant' },
      { resource: 'promotion.tenant_loyalty', action: 'read' },
      { resource: 'promotion.tenant_loyalty', action: 'write' },
    ],
    showInMenu: true,
    icon: GiftOutlined,
    menuGroup: 'business',
    component: () => import('@/views/promotion/CouponManagementView.vue'),
  },
  {
    name: 'campaign',
    path: 'campaigns',
    title: 'nav.campaign',
    permissions: [
      { resource: 'campaign.definition', action: 'read' },
      { resource: 'campaign.definition', action: 'write' },
      { resource: 'campaign.definition', action: 'publish' },
    ],
    showInMenu: true,
    icon: FlagOutlined,
    menuGroup: 'operations',
    component: () => import('@/views/campaign/CampaignManagementView.vue'),
  },
  {
    name: 'catalog',
    path: 'catalog',
    title: 'nav.catalog',
    permissions: ['read', 'create', 'write', 'publish'].map(action => ({ resource: 'catalog.product', action })),
    showInMenu: true,
    menuGroup: 'business',
    icon: ShoppingOutlined,
    backFallback: '/dashboard',
    component: () => import('@/views/catalog/CatalogManagementView.vue'),
  },
  {
    name: 'gacha',
    path: 'gacha',
    title: 'nav.gacha',
    permissions: ['read', 'create', 'request_publication', 'approve_publication', 'publish', 'assign_lucky'].map(action => ({ resource: 'gacha.pool', action })),
    showInMenu: true,
    menuGroup: 'business',
    icon: AppstoreOutlined,
    backFallback: '/dashboard',
    component: () => import('@/views/gacha/PlayPoolManagementView.vue'),
  },
  {
    name: 'customers',
    path: 'customers',
    title: 'nav.customers',
    permission: { resource: 'reporting.metric', action: 'read' },
    showInMenu: true,
    icon: UserOutlined,
    backFallback: '/catalog',
    component: () => import('@/views/customers/CustomerManagementView.vue'),
  },
  {
    name: 'system',
    path: 'system',
    title: 'nav.system',
    showInMenu: true,
    icon: SettingOutlined,
    backFallback: '/catalog',
    component: () => import('@/views/system/SystemManagementView.vue'),
  },
  {
    name: 'accounts',
    path: 'accounts',
    title: 'nav.accounts',
    permission: { resource: 'authorization.role', action: 'read' },
    showInMenu: true,
    icon: TeamOutlined,
    backFallback: '/catalog',
    component: () => import('@/views/authorization/RoleManagementView.vue'),
  },
  {
    name: 'eligibility',
    path: 'eligibility',
    title: 'nav.eligibility',
    permissions: ['read', 'create', 'write', 'publish'].map(action => ({ resource: 'eligibility.policy', action })),
    showInMenu: true,
    menuGroup: 'business',
    icon: FilterOutlined,
    backFallback: '/dashboard',
    component: () => import('@/views/eligibility/EligibilityPolicyView.vue'),
  },
  {
    name: 'authorization-roles',
    path: 'authorization/roles',
    title: 'nav.authorization',
    permission: { resource: 'authorization.role', action: 'read' },
    showInMenu: true,
    menuGroup: 'governance',
    icon: SafetyCertificateOutlined,
    backFallback: '/dashboard',
    component: () => import('@/views/authorization/RoleManagementView.vue'),
  },
  {
    name: 'reporting',
    path: 'reporting',
    title: 'nav.reporting',
    permissions: [
      { resource: 'reporting.metric', action: 'read' },
      { resource: 'reporting.dashboard', action: 'read' },
      { resource: 'reporting.dashboard', action: 'write' },
      { resource: 'reporting.dashboard', action: 'publish' },
      { resource: 'reporting.dashboard', action: 'query' },
    ],
    showInMenu: true,
    menuGroup: 'operations',
    icon: LineChartOutlined,
    component: () => import('@/views/reporting/ReportingDashboardView.vue'),
  },
  {
    name: 'risk',
    path: 'risk',
    title: 'nav.risk',
    permissions: [
      { resource: 'risk.case', action: 'list' },
      { resource: 'risk.case', action: 'claim' },
      { resource: 'risk.case', action: 'resolve' },
    ],
    showInMenu: true,
    icon: SafetyOutlined,
    menuGroup: 'governance',
    component: () => import('@/views/risk/RiskCaseView.vue'),
  },
]

function granted(item: ConsoleRouteDef, nodes: PermissionNode[]): boolean {
  if (item.permissions) return item.permissions.some((permission) => allows(nodes, permission.resource, permission.action))
  if (!item.permission) {
    return true
  }
  return allows(nodes, item.permission.resource, item.permission.action)
}

// 主菜单：与设计稿一致的一级导航（顺序即展示顺序）
const PRIMARY_MENU = ['catalog', 'gacha', 'customers', 'campaign', 'system', 'accounts'] as const

export function accessibleRoutes(nodes: PermissionNode[]): ConsoleRouteDef[] {
  return consoleRouteCatalog.filter((item) => !item.placeholder && granted(item, nodes))
}

export function menuRoutes(nodes: PermissionNode[]): ConsoleRouteDef[] {
  const byName = new Map(consoleRouteCatalog.map((item) => [item.name, item]))
  return PRIMARY_MENU.map((name) => byName.get(name))
    .filter((item): item is ConsoleRouteDef => !!item && !item.placeholder && granted(item, nodes))
}

export const menuGroupTitles = { operations: 'menu.operations', business: 'menu.business', governance: 'menu.governance' } as const

export function menuGroups(nodes: PermissionNode[]) {
  const routes = menuRoutes(nodes)
  return (Object.keys(menuGroupTitles) as Array<keyof typeof menuGroupTitles>)
    .map((key) => ({ key, title: menuGroupTitles[key], items: routes.filter((item) => item.menuGroup === key) }))
    .filter((group) => group.items.length > 0)
}

export function toRouteRecords(defs: ConsoleRouteDef[]): RouteRecordRaw[] {
  return defs.map((item) => ({
    path: item.path,
    name: item.name,
    component: item.component,
    meta: {
      title: item.title,
      backFallback: item.backFallback,
    },
  }))
}

export function canAccessRoute(routeName: string | undefined, nodes: PermissionNode[]): boolean {
  if (!routeName || routeName === 'console-root' || routeName === 'not-found') {
    return true
  }
  const item = consoleRouteCatalog.find((entry) => entry.name === routeName)
  if (!item || item.placeholder) {
    return false
  }
  return granted(item, nodes)
}

export function menuPath(item: ConsoleRouteDef): string {
  return `/${item.path}`
}

// 路由目录：结构与 server_v2 参考控制台一致；title 与菜单分组标题存 i18n KEY
// （web_admin 双语是刻意差异），渲染处（NavigationMenu/ConsoleLayout/路由守卫/
// 工作台快捷入口）统一 $t()。
import type { Component } from 'vue'
import type { RouteRecordRaw } from 'vue-router'
import {
  AppstoreOutlined,
  BarChartOutlined,
  DashboardOutlined,
  FireOutlined,
  FlagOutlined,
  FilterOutlined,
  GiftOutlined,
  LineChartOutlined,
  SafetyOutlined,
  SafetyCertificateOutlined,
  SearchOutlined,
  SettingOutlined,
  ShoppingOutlined,
  TagsOutlined,
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
  /** 菜单文案（缺省用 title；子菜单需要与页面标题不同时使用） */
  menuTitle?: string
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
    menuTitle: 'nav.campaignWheel',
    permissions: [
      { resource: 'campaign.definition', action: 'read' },
      { resource: 'campaign.definition', action: 'write' },
      { resource: 'campaign.definition', action: 'publish' },
    ],
    showInMenu: true,
    icon: FlagOutlined,
    menuGroup: 'operations',
    backFallback: '/campaigns',
    component: () => import('@/views/campaign/CampaignManagementView.vue'),
  },
  {
    name: 'campaign-fortune',
    path: 'campaigns/fortune',
    title: 'nav.campaignFortune',
    permission: { resource: 'campaign.definition', action: 'read' },
    showInMenu: false,
    backFallback: '/campaigns',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'campaign-fortune-rank',
    path: 'campaigns/fortune-rank',
    title: 'nav.campaignFortuneRank',
    permission: { resource: 'campaign.definition', action: 'read' },
    showInMenu: false,
    backFallback: '/campaigns',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'campaign-rank',
    path: 'campaigns/rank',
    title: 'nav.campaignRank',
    permission: { resource: 'campaign.definition', action: 'read' },
    showInMenu: false,
    backFallback: '/campaigns',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'campaign-fortune-multiplier',
    path: 'campaigns/fortune-multiplier',
    title: 'nav.campaignFortuneMultiplier',
    permission: { resource: 'campaign.definition', action: 'read' },
    showInMenu: false,
    backFallback: '/campaigns',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'campaign-spend-coupon',
    path: 'campaigns/spend-coupon',
    title: 'nav.campaignSpendCoupon',
    permission: { resource: 'campaign.definition', action: 'read' },
    showInMenu: false,
    backFallback: '/campaigns',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'campaign-exchange',
    path: 'campaigns/exchange',
    title: 'nav.campaignExchange',
    permission: { resource: 'campaign.definition', action: 'read' },
    showInMenu: false,
    backFallback: '/campaigns',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'campaign-coupon-batch',
    path: 'campaigns/coupon-batch',
    title: 'nav.campaignCouponBatch',
    permission: { resource: 'promotion.coupon_template', action: 'read' },
    showInMenu: false,
    backFallback: '/campaigns',
    component: () => import('@/views/promotion/CouponManagementView.vue'),
  },
  {
    name: 'campaign-item-batch',
    path: 'campaigns/item-batch',
    title: 'nav.campaignItemBatch',
    permission: { resource: 'campaign.definition', action: 'read' },
    showInMenu: false,
    backFallback: '/campaigns',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'campaign-random-discount',
    path: 'campaigns/random-discount',
    title: 'nav.campaignRandomDiscount',
    permission: { resource: 'campaign.definition', action: 'read' },
    showInMenu: false,
    backFallback: '/campaigns',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'campaign-chaodan',
    path: 'campaigns/chaodan',
    title: 'nav.campaignChaodan',
    permission: { resource: 'campaign.definition', action: 'read' },
    showInMenu: false,
    backFallback: '/campaigns',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'campaign-redeem',
    path: 'campaigns/redeem-code',
    title: 'nav.campaignRedeem',
    permission: { resource: 'campaign.definition', action: 'read' },
    showInMenu: false,
    backFallback: '/campaigns',
    component: () => import('@/views/campaign/CampaignManagementView.vue'),
  },
  {
    name: 'catalog-ip',
    path: 'catalog/ip',
    title: 'nav.catalogIp',
    permission: { resource: 'catalog.product', action: 'read' },
    showInMenu: false,
    backFallback: '/catalog',
    component: () => import('@/views/catalog/IpManagementView.vue'),
  },
  {
    name: 'catalog-brand',
    path: 'catalog/brand',
    title: 'nav.catalogBrand',
    permission: { resource: 'catalog.product', action: 'read' },
    showInMenu: false,
    backFallback: '/catalog',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'catalog-series',
    path: 'catalog/series',
    title: 'nav.catalogSeries',
    permission: { resource: 'catalog.product', action: 'read' },
    showInMenu: false,
    backFallback: '/catalog',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'catalog',
    path: 'catalog',
    title: 'nav.catalog',
    menuTitle: 'nav.catalogGoods',
    permissions: ['read', 'create', 'write', 'publish'].map(action => ({ resource: 'catalog.product', action })),
    showInMenu: true,
    menuGroup: 'business',
    icon: ShoppingOutlined,
    backFallback: '/catalog',
    component: () => import('@/views/catalog/CatalogManagementView.vue'),
  },
  {
    name: 'gacha-grade',
    path: 'gacha/grades',
    title: 'nav.gachaGrade',
    permission: { resource: 'gacha.pool', action: 'read' },
    showInMenu: false,
    backFallback: '/gacha',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'gacha',
    path: 'gacha',
    title: 'nav.gacha',
    menuTitle: 'nav.gachaPools',
    permissions: ['read', 'create', 'request_publication', 'approve_publication', 'publish', 'assign_lucky'].map(action => ({ resource: 'gacha.pool', action })),
    showInMenu: true,
    menuGroup: 'business',
    icon: AppstoreOutlined,
    backFallback: '/gacha',
    component: () => import('@/views/gacha/PlayPoolManagementView.vue'),
  },
  {
    name: 'gacha-match',
    path: 'gacha/match',
    title: 'nav.gachaMatch',
    permission: { resource: 'gacha.pool', action: 'read' },
    showInMenu: false,
    backFallback: '/gacha',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'gacha-codex',
    path: 'gacha/codex',
    title: 'nav.gachaCodex',
    permission: { resource: 'gacha.pool', action: 'read' },
    showInMenu: false,
    backFallback: '/gacha',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'customers',
    path: 'customers',
    title: 'nav.customers',
    menuTitle: 'nav.customerUsers',
    permission: { resource: 'reporting.metric', action: 'read' },
    showInMenu: true,
    icon: UserOutlined,
    backFallback: '/catalog',
    component: () => import('@/views/customers/CustomerManagementView.vue'),
  },
  {
    name: 'customer-shipments',
    path: 'customers/shipments',
    title: 'nav.customerShipments',
    showInMenu: false,
    backFallback: '/customers',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'customer-sms',
    path: 'customers/sms',
    title: 'nav.customerSms',
    showInMenu: false,
    backFallback: '/customers',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'customer-refunds',
    path: 'customers/refunds',
    title: 'nav.customerRefunds',
    showInMenu: false,
    backFallback: '/customers',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'customer-recharges',
    path: 'customers/recharges',
    title: 'nav.customerRecharges',
    showInMenu: false,
    backFallback: '/customers',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'customer-stored-refunds',
    path: 'customers/stored-refunds',
    title: 'nav.customerStoredRefunds',
    showInMenu: false,
    backFallback: '/customers',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'customer-jishi-orders',
    path: 'customers/jishi-orders',
    title: 'nav.customerJishiOrders',
    showInMenu: false,
    backFallback: '/customers',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'customer-wallet-reset',
    path: 'customers/wallet-reset',
    title: 'nav.customerWalletReset',
    showInMenu: false,
    backFallback: '/customers',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'system',
    path: 'system',
    title: 'nav.system',
    menuTitle: 'nav.systemConfig',
    showInMenu: true,
    icon: SettingOutlined,
    backFallback: '/system',
    component: () => import('@/views/system/SystemManagementView.vue'),
  },
  {
    name: 'system-campaigns',
    path: 'system/campaigns',
    title: 'nav.systemCampaigns',
    permission: { resource: 'campaign.definition', action: 'read' },
    showInMenu: false,
    backFallback: '/system',
    component: () => import('@/views/campaign/CampaignManagementView.vue'),
  },
  {
    name: 'system-coupons',
    path: 'system/coupons',
    title: 'nav.systemCoupons',
    permission: { resource: 'promotion.coupon_template', action: 'read' },
    showInMenu: false,
    backFallback: '/system',
    component: () => import('@/views/promotion/CouponManagementView.vue'),
  },
  {
    name: 'system-announcements',
    path: 'system/announcements',
    title: 'nav.systemAnnouncements',
    showInMenu: false,
    backFallback: '/system',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'system-banners',
    path: 'system/banners',
    title: 'nav.systemBanners',
    showInMenu: false,
    backFallback: '/system',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'system-popups',
    path: 'system/popups',
    title: 'nav.systemPopups',
    showInMenu: false,
    backFallback: '/system',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'system-agreements',
    path: 'system/agreements',
    title: 'nav.systemAgreements',
    showInMenu: false,
    backFallback: '/system',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'system-version',
    path: 'system/version',
    title: 'nav.systemVersion',
    showInMenu: false,
    backFallback: '/system',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'system-stored-value',
    path: 'system/stored-value',
    title: 'nav.systemStoredValue',
    showInMenu: false,
    backFallback: '/system',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'system-tasks',
    path: 'system/tasks',
    title: 'nav.systemTasks',
    permission: { resource: 'task.plan', action: 'read' },
    showInMenu: false,
    backFallback: '/system',
    component: () => import('@/views/operations/TaskPlanView.vue'),
  },
  {
    name: 'accounts',
    path: 'accounts',
    title: 'nav.accountRoles',
    menuTitle: 'nav.accountRoles',
    permission: { resource: 'authorization.role', action: 'read' },
    showInMenu: true,
    icon: TeamOutlined,
    backFallback: '/accounts',
    component: () => import('@/views/authorization/RoleManagementView.vue'),
  },
  {
    name: 'account-list',
    path: 'accounts/list',
    title: 'nav.accountList',
    showInMenu: false,
    backFallback: '/accounts',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'account-audit',
    path: 'accounts/audit',
    title: 'nav.accountAudit',
    showInMenu: false,
    backFallback: '/accounts',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'report-download',
    path: 'reports/download',
    title: 'nav.reportDownload',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'report-daily',
    path: 'reports/daily',
    title: 'nav.reportDaily',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'report-retention',
    path: 'reports/retention',
    title: 'nav.reportRetention',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'report-revenue',
    path: 'reports/revenue',
    title: 'nav.reportRevenue',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'report-realtime',
    path: 'reports/realtime',
    title: 'nav.reportRealtime',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'report-market',
    path: 'reports/market',
    title: 'nav.reportMarket',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'report-invite',
    path: 'reports/invite',
    title: 'nav.reportInvite',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'report-orders',
    path: 'reports/orders',
    title: 'nav.reportOrders',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'query-item-flow',
    path: 'query/item-flow',
    title: 'nav.queryItemFlow',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'query-user-profit',
    path: 'query/user-profit',
    title: 'nav.queryUserProfit',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'query-user-bag',
    path: 'query/user-bag',
    title: 'nav.queryUserBag',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'query-box-revenue',
    path: 'query/box-revenue',
    title: 'nav.queryBoxRevenue',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'query-balance-flow',
    path: 'query/balance-flow',
    title: 'nav.queryBalanceFlow',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'query-coupon-flow',
    path: 'query/coupon-flow',
    title: 'nav.queryCouponFlow',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'query-invite',
    path: 'query/invite',
    title: 'nav.queryInvite',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'query-task-flow',
    path: 'query/task-flow',
    title: 'nav.queryTaskFlow',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'activity-exchange',
    path: 'activities/exchange',
    title: 'nav.activityExchange',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'activity-points-exchange',
    path: 'activities/points-exchange',
    title: 'nav.activityPointsExchange',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/common/ModulePlaceholderView.vue'),
  },
  {
    name: 'activity-redeem',
    path: 'activities/redeem',
    title: 'nav.activityRedeem',
    showInMenu: false,
    backFallback: '/dashboard',
    component: () => import('@/views/campaign/CampaignManagementView.vue'),
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

// 主菜单：与设计稿一致的一级导航（顺序即展示顺序）；商品管理为右侧弹出子菜单
export interface MenuNode {
  key: string
  title: string
  icon?: Component
  path?: string
  children?: MenuNode[]
}

const CATALOG_CHILDREN = ['catalog-ip', 'catalog-brand', 'catalog-series', 'catalog'] as const
const GACHA_CHILDREN = ['gacha-grade', 'gacha', 'gacha-match', 'gacha-codex'] as const
const CUSTOMER_CHILDREN = [
  'customers',
  'customer-shipments',
  'customer-sms',
  'customer-refunds',
  'customer-recharges',
  'customer-stored-refunds',
  'customer-jishi-orders',
  'customer-wallet-reset',
] as const
const CAMPAIGN_CHILDREN = [
  'campaign-fortune',
  'campaign-fortune-rank',
  'campaign-rank',
  'campaign-fortune-multiplier',
  'campaign-spend-coupon',
  'campaign-exchange',
  'campaign-coupon-batch',
  'campaign-item-batch',
  'campaign-random-discount',
  'campaign',
  'campaign-chaodan',
  'campaign-redeem',
] as const
const SYSTEM_CHILDREN = [
  'system-campaigns',
  'system-coupons',
  'system-announcements',
  'system-banners',
  'system-popups',
  'system-agreements',
  'system-version',
  'system-stored-value',
  'system',
  'system-tasks',
] as const
const ACCOUNT_CHILDREN = ['account-list', 'accounts', 'account-audit'] as const
const REPORT_CHILDREN = [
  'report-download',
  'report-daily',
  'report-retention',
  'report-revenue',
  'report-realtime',
  'report-market',
  'report-invite',
  'report-orders',
] as const
const QUERY_CHILDREN = [
  'query-item-flow',
  'query-user-profit',
  'query-user-bag',
  'query-box-revenue',
  'query-balance-flow',
  'query-coupon-flow',
  'query-invite',
  'query-task-flow',
] as const
const ACTIVITY_CHILDREN = ['activity-exchange', 'activity-points-exchange', 'activity-redeem'] as const
const PRIMARY_LEAVES: readonly string[] = []

export function accessibleRoutes(nodes: PermissionNode[]): ConsoleRouteDef[] {
  return consoleRouteCatalog.filter((item) => !item.placeholder && granted(item, nodes))
}

function childNodes(names: readonly string[], nodes: PermissionNode[], byName: Map<string, ConsoleRouteDef>): MenuNode[] {
  return names
    .map((name) => byName.get(name))
    .filter((item): item is ConsoleRouteDef => !!item && !item.placeholder && granted(item, nodes))
    .map((item) => ({ key: item.name, title: item.menuTitle ?? item.title, path: menuPath(item) }))
}

export function menuTree(nodes: PermissionNode[]): MenuNode[] {
  const byName = new Map(consoleRouteCatalog.map((item) => [item.name, item]))
  const tree: MenuNode[] = []
  const dashboard = byName.get('dashboard')
  if (dashboard && !dashboard.placeholder && granted(dashboard, nodes)) {
    tree.push({ key: dashboard.name, title: dashboard.title, icon: dashboard.icon, path: menuPath(dashboard) })
  }
  const catalogChildren = childNodes(CATALOG_CHILDREN, nodes, byName)
  if (catalogChildren.length > 0) {
    tree.push({ key: 'catalog-group', title: 'nav.catalog', icon: ShoppingOutlined, children: catalogChildren })
  }
  const gachaChildren = childNodes(GACHA_CHILDREN, nodes, byName)
  if (gachaChildren.length > 0) {
    tree.push({ key: 'gacha-group', title: 'nav.gacha', icon: GiftOutlined, children: gachaChildren })
  }
  const customerChildren = childNodes(CUSTOMER_CHILDREN, nodes, byName)
  if (customerChildren.length > 0) {
    tree.push({ key: 'customer-group', title: 'nav.customers', icon: UserOutlined, children: customerChildren })
  }
  const campaignChildren = childNodes(CAMPAIGN_CHILDREN, nodes, byName)
  if (campaignChildren.length > 0) {
    tree.push({ key: 'campaign-group', title: 'nav.campaign', icon: FireOutlined, children: campaignChildren })
  }
  const systemChildren = childNodes(SYSTEM_CHILDREN, nodes, byName)
  if (systemChildren.length > 0) {
    tree.push({ key: 'system-group', title: 'nav.system', icon: SettingOutlined, children: systemChildren })
  }
  const accountChildren = childNodes(ACCOUNT_CHILDREN, nodes, byName)
  if (accountChildren.length > 0) {
    tree.push({ key: 'account-group', title: 'nav.accounts', icon: TeamOutlined, children: accountChildren })
  }
  const reportChildren = childNodes(REPORT_CHILDREN, nodes, byName)
  if (reportChildren.length > 0) {
    tree.push({ key: 'report-group', title: 'nav.reports', icon: BarChartOutlined, children: reportChildren })
  }
  const queryChildren = childNodes(QUERY_CHILDREN, nodes, byName)
  if (queryChildren.length > 0) {
    tree.push({ key: 'query-group', title: 'nav.query', icon: SearchOutlined, children: queryChildren })
  }
  const activityChildren = childNodes(ACTIVITY_CHILDREN, nodes, byName)
  if (activityChildren.length > 0) {
    tree.push({ key: 'activity-group', title: 'nav.activity', icon: TagsOutlined, children: activityChildren })
  }
  for (const name of PRIMARY_LEAVES) {
    const item = byName.get(name)
    if (item && !item.placeholder && granted(item, nodes)) {
      tree.push({ key: item.name, title: item.title, icon: item.icon, path: menuPath(item) })
    }
  }
  return tree
}

export function menuRoutes(nodes: PermissionNode[]): ConsoleRouteDef[] {
  const byName = new Map(consoleRouteCatalog.map((item) => [item.name, item]))
  return menuTree(nodes).flatMap((node) => {
    if (node.children) {
      return node.children
        .map((child) => byName.get(child.key))
        .filter((item): item is ConsoleRouteDef => !!item)
    }
    const item = byName.get(node.key)
    return item ? [item] : []
  })
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

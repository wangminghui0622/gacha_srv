// 手写 typed services（本仓栈无 protoc-gen-typescript-http，设计文档刻意差异）：
// 方法名与 server_v2 参考控制台 views 调用点一致，axios 直连本仓管理网关
// /console/v2/api 路由；响应形状对齐本仓 protojson（uint64→string、
// 枚举→名称、bytes→base64），请求体照场景 25-28 实测 wire。
import { http } from '@/utils/request'
import { getClientFingerprint, getRefreshToken, getSessionId } from '@/utils/cache'
import type { PermissionTree } from '@/access/grants'

const consoleBase = '/console/v2/api'

const dataScopeNames = ['DATA_SCOPE_UNSPECIFIED', 'DATA_SCOPE_SELF', 'DATA_SCOPE_TEAM', 'DATA_SCOPE_DEPARTMENT', 'DATA_SCOPE_ORGANIZATION', 'DATA_SCOPE_CUSTOM', 'DATA_SCOPE_ALL'] as const
const approvalNames = ['APPROVAL_MODE_UNSPECIFIED', 'APPROVAL_MODE_NONE', 'APPROVAL_MODE_SINGLE', 'APPROVAL_MODE_DUAL_CONTROL'] as const
function enumNumber(value: string | number | undefined, names: readonly string[]): number {
  return typeof value === 'number' ? value : Math.max(0, names.indexOf(value ?? ''))
}

export interface ConsoleSessionTokens {
  accessToken: string
  refreshToken: string
  tokenType: string
  accessExpiresAtUnix: string
  refreshExpiresAtUnix: string
  sessionId: string
}

export interface ConsoleTotpChallenge {
  loginTicket: string
  setupRequired: boolean
  totpSecret?: string
  totpUri?: string
}

export interface ConsoleLoginReply {
  session?: ConsoleSessionTokens
  totpChallenge?: ConsoleTotpChallenge
}

// 本仓健康检查为 GET /health 且只回 {status}（无参考实现的 service 字段）。
export interface ConsoleHealthReply {
  status: string
}

export function loginConsole(payload: {
  tenantId?: number
  username?: string
  password?: string
  otpCode?: string
  loginTicket?: string
}): Promise<ConsoleLoginReply> {
  return http.post(`${consoleBase}/session/login`, {
    tenantId: payload.tenantId ?? 0,
    username: payload.username ?? '',
    password: payload.password ?? '',
    otpCode: payload.otpCode ?? '',
    loginTicket: payload.loginTicket ?? '',
    clientFingerprint: getClientFingerprint(),
  })
}

export function refreshConsoleSession(): Promise<ConsoleSessionTokens> {
  const refreshToken = getRefreshToken()
  if (!refreshToken) {
    return Promise.reject(new Error('missing refresh token'))
  }
  return http.post(`${consoleBase}/session/refresh`, {
    refreshToken,
    clientFingerprint: getClientFingerprint(),
  })
}

export function logoutConsole(): Promise<{ revoked: boolean }> {
  const sessionId = getSessionId() ?? ''
  return http.post(`${consoleBase}/session/logout`, { sessionId })
}

export function fetchConsolePermissions(): Promise<PermissionTree> {
  return http.get(`${consoleBase}/session/permissions`)
}

export interface ConsoleTab {
  path: string
  titleKey: string
}

export interface ConsoleTabsReply {
  tabs?: ConsoleTab[]
  activePath?: string
}

export function fetchConsoleTabs(): Promise<ConsoleTabsReply> {
  return http.get(`${consoleBase}/session/tabs`)
}

export function saveConsoleTabs(tabs: ConsoleTab[], activePath: string): Promise<ConsoleTabsReply> {
  return http.put(`${consoleBase}/session/tabs`, { tabs, activePath })
}

export function fetchConsoleHealth(): Promise<ConsoleHealthReply> {
  return http.get(`${consoleBase}/health`)
}

// ── 任务进度（本仓 TaskView 嵌套 definition——相对参考扁平 taskId 的 wire 差异）──

export interface TaskRewardView {
  count?: number
  coupon?: { templateId?: string; templateVersion?: string }
  inventory?: { productId?: string; skuId?: string; productVersion?: string }
}

export interface TaskDefinitionView {
  taskId?: string
  version?: string
  name?: string
  period?: string
  metric?: string
  target?: string
  cycleLimit?: number
  rewardTemplateId?: string
  rewardCount?: number
  reward?: TaskRewardView
}

export interface TaskView {
  definition?: TaskDefinitionView
  progress?: string
  claimed?: number
  state?: string
}

export interface ListTasksReply {
  tasks?: TaskView[]
  nextPageToken?: string
}

export function listUserTasks(params: {
  tenantId: number
  userId: number
  pageSize?: number
  pageToken?: string
}): Promise<ListTasksReply> {
  return http.get(`${consoleBase}/tasks`, { params })
}

// ── 商品视图（本仓 catalog.Product wire）──

export interface ProductView {
  productId?: string
  productCode?: string
  title?: string
  status?: string | number
  version?: string
  options?: Array<{ optionId?: string; name?: string; values?: Array<{ optionValueId?: string; value?: string }> }>
  skus?: Array<{ skuId?: string; skuCode?: string; title?: string; status?: string | number }>
}

export function getConsoleProduct(productId: string): Promise<ProductView> {
  return http.get(`${consoleBase}/catalog/products/${encodeURIComponent(productId)}`)
}

export interface ProductSummaryView {
  productId?: string
  productCode?: string
  title?: string
  status?: string | number
  version?: string
}

export interface ListProductsView {
  products?: ProductSummaryView[]
  nextAfterProductId?: string
}

export function listConsoleProducts(afterProductId = '', pageSize = 20): Promise<ListProductsView> {
  return http.get(`${consoleBase}/catalog/products`, { params: { ...(afterProductId ? { afterProductId } : {}), pageSize } })
}

// ── 赏池版本视图（本仓 BoxDraft wire——无参考实现的 mode 字段）──

export interface PoolEntryView {
  entryId?: string
  name?: string
  weight?: number
  quantity?: number
  isPrize?: boolean
  displayValueFen?: string
  rollingLimited?: boolean
}

export interface PoolVersionView {
  poolId?: string
  version?: string
  status?: string | number
  boxName?: string
  price?: string
  description?: string
  entries?: PoolEntryView[]
  rollingRulesJson?: string
  configHash?: string
  publishedAtUnixMillis?: string
}

export function getConsolePoolVersion(poolId: string, version: string): Promise<PoolVersionView> {
  return http.get(`${consoleBase}/gacha/pools/${encodeURIComponent(poolId)}/versions/${encodeURIComponent(version)}`)
}

export interface PoolVersionSummaryView {
  poolId?: string
  version?: string
  status?: string | number
}

export interface ListPoolVersionsView {
  versions?: PoolVersionSummaryView[]
  nextAfterPoolId?: string
  nextAfterVersion?: string
}

export function listConsolePoolVersions(afterPoolId = '', afterVersion = '', pageSize = 20): Promise<ListPoolVersionsView> {
  return http.get(`${consoleBase}/gacha/pools`, { params: { ...(afterPoolId ? { afterPoolId, afterVersion } : {}), pageSize } })
}

// ── 授权（本仓 /authorization 路由直连；枚举 wire 名称→视图数字，与参考 roleFromWire 同构）──

export interface PermissionCatalogAction { code: string; name: string }
export interface PermissionCatalogResource { code: string; name: string; actions?: PermissionCatalogAction[] }

export interface RolePermission {
  resource: string
  action: string
  dataScope?: number
  allowedFields?: string[]
  deniedFields?: string[]
  condition?: string
  approvalMode?: number
}

export interface ConsoleRole {
  id: string
  tenantId: string
  code: string
  name: string
  enabled: boolean
  version: string
  permissions?: RolePermission[]
}

interface RoleWire {
  id?: string
  tenantId?: string
  code?: string
  name?: string
  enabled?: boolean
  version?: string
  permissions?: Array<{
    resource?: string
    action?: string
    dataScope?: string | number
    allowedFields?: string[]
    deniedFields?: string[]
    condition?: string
    approvalMode?: string | number
  }>
}

function roleFromWire(role: RoleWire): ConsoleRole {
  return {
    id: role.id ?? '',
    tenantId: role.tenantId ?? '',
    code: role.code ?? '',
    name: role.name ?? '',
    enabled: role.enabled ?? false,
    version: role.version ?? '',
    permissions: (role.permissions ?? []).map((p) => ({
      resource: p.resource ?? '',
      action: p.action ?? '',
      dataScope: enumNumber(p.dataScope, dataScopeNames),
      approvalMode: enumNumber(p.approvalMode, approvalNames),
      allowedFields: p.allowedFields ?? [],
      deniedFields: p.deniedFields ?? [],
      condition: p.condition ?? '',
    })),
  }
}

export async function fetchPermissionCatalog(): Promise<{ resources?: PermissionCatalogResource[] }> {
  const response = (await http.get(`${consoleBase}/authorization/permissions`)) as {
    resources?: Array<{ code?: string; name?: string; actions?: Array<{ code?: string; name?: string }> }>
  }
  return {
    resources: (response.resources ?? []).map((resource) => ({
      code: resource.code ?? '',
      name: resource.name ?? '',
      actions: (resource.actions ?? []).map((action) => ({ code: action.code ?? '', name: action.name ?? '' })),
    })),
  }
}

export async function listConsoleRoles(): Promise<{ roles?: ConsoleRole[] }> {
  const response = (await http.get(`${consoleBase}/authorization/roles`)) as { roles?: RoleWire[] }
  return { roles: (response.roles ?? []).map(roleFromWire) }
}

export async function getConsoleRole(roleId: string): Promise<ConsoleRole> {
  return roleFromWire((await http.get(`${consoleBase}/authorization/roles/${encodeURIComponent(roleId)}`)) as RoleWire)
}

export async function createConsoleRole(payload: { code: string; name: string }): Promise<ConsoleRole> {
  return roleFromWire((await http.post(`${consoleBase}/authorization/roles`, payload)) as RoleWire)
}

export async function updateConsoleRole(roleId: string, payload: { name: string; enabled: boolean; expectedVersion: string }): Promise<ConsoleRole> {
  return roleFromWire((await http.put(`${consoleBase}/authorization/roles/${encodeURIComponent(roleId)}`, payload)) as RoleWire)
}

export async function putConsoleRolePermissions(roleId: string, expectedVersion: string, permissions: RolePermission[]): Promise<ConsoleRole> {
  const wirePermissions = permissions.map((p) => ({
    resource: p.resource,
    action: p.action,
    dataScope: dataScopeNames[p.dataScope ?? 6] as (typeof dataScopeNames)[number],
    approvalMode: approvalNames[p.approvalMode ?? 1] as (typeof approvalNames)[number],
    allowedFields: p.allowedFields,
    deniedFields: p.deniedFields,
    condition: p.condition,
  }))
  return roleFromWire(
    (await http.put(`${consoleBase}/authorization/roles/${encodeURIComponent(roleId)}/permissions`, {
      expectedVersion,
      permissions: wirePermissions,
    })) as RoleWire,
  )
}

export interface ConsoleRoleMember { principalId: string; displayName: string }
export interface ConsolePrincipalRoles { principalId: string; tenantId: string; version: string; roleIds?: string[] }

export async function listConsoleRoleMembers(roleId: string, afterPrincipalId?: string): Promise<{ members?: ConsoleRoleMember[]; nextPrincipalId?: string }> {
  const response = (await http.get(`${consoleBase}/authorization/roles/${encodeURIComponent(roleId)}/members`, {
    params: { pageSize: 50, ...(afterPrincipalId ? { afterPrincipalId } : {}) },
  })) as { members?: Array<{ principalId?: string; displayName?: string }>; nextPrincipalId?: string }
  return {
    members: (response.members ?? []).map((m) => ({ principalId: m.principalId ?? '', displayName: m.displayName ?? '' })),
    nextPrincipalId: response.nextPrincipalId,
  }
}

export async function getConsolePrincipalRoles(principalId: string): Promise<ConsolePrincipalRoles> {
  const response = (await http.get(`${consoleBase}/authorization/principals/${encodeURIComponent(principalId)}/roles`)) as {
    principalId?: string
    tenantId?: string
    version?: string
    roleIds?: string[]
  }
  return {
    principalId: response.principalId ?? '',
    tenantId: response.tenantId ?? '',
    version: response.version ?? '',
    roleIds: response.roleIds ?? [],
  }
}

export async function putConsolePrincipalRoles(principalId: string, expectedVersion: string, roleIds: string[]): Promise<ConsolePrincipalRoles> {
  const response = (await http.put(`${consoleBase}/authorization/principals/${encodeURIComponent(principalId)}/roles`, {
    expectedVersion,
    roleIds,
  })) as { principalId?: string; tenantId?: string; version?: string; roleIds?: string[] }
  return {
    principalId: response.principalId ?? '',
    tenantId: response.tenantId ?? '',
    version: response.version ?? '',
    roleIds: response.roleIds ?? [],
  }
}

// 会话令牌本地存储。
// 与参考实现（server_v2 web/console 用 js-cookie）的刻意差异：本仓管理网关只读
// Authorization: Bearer 头、不读写 Cookie，令牌存 localStorage 更契合栈；键名保留
// lb_console_* 前缀便于辨认。localStorage 在私密窗口可能抛异常，全部读写带守卫。
const TOKEN_KEY = 'lb_console_access'
const REFRESH_KEY = 'lb_console_refresh'
const SESSION_KEY = 'lb_console_session'
const FINGERPRINT_KEY = 'lb_console_fp'

function storageGet(key: string): string | undefined {
  try {
    return localStorage.getItem(key) ?? undefined
  } catch {
    return undefined
  }
}

function storageSet(key: string, value: string): void {
  try {
    localStorage.setItem(key, value)
  } catch {
    // 存储不可用时仅丢失跨刷新会话，不阻断当前会话。
  }
}

function storageRemove(key: string): void {
  try {
    localStorage.removeItem(key)
  } catch {
    // 忽略。
  }
}

export function getAccessToken(): string | undefined {
  return storageGet(TOKEN_KEY)
}

export function setAccessToken(token: string): void {
  storageSet(TOKEN_KEY, token)
}

export function getRefreshToken(): string | undefined {
  return storageGet(REFRESH_KEY)
}

export function setRefreshToken(token: string): void {
  storageSet(REFRESH_KEY, token)
}

export function getSessionId(): string | undefined {
  return storageGet(SESSION_KEY)
}

export function setSessionId(id: string): void {
  storageSet(SESSION_KEY, id)
}

export function clearSessionCookies(): void {
  storageRemove(TOKEN_KEY)
  storageRemove(REFRESH_KEY)
  storageRemove(SESSION_KEY)
}

export function getClientFingerprint(): string {
  let fp = storageGet(FINGERPRINT_KEY)
  if (!fp) {
    fp = crypto.randomUUID()
    storageSet(FINGERPRINT_KEY, fp)
  }
  return fp
}

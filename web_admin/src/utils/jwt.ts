export interface ConsoleJwtClaims {
  actor_id?: number
  tenant_id?: number
  name?: string
  roles?: string[]
}

export function parseConsoleJwt(token: string): ConsoleJwtClaims | null {
  const parts = token.split('.')
  if (parts.length !== 3) {
    return null
  }
  const payload = parts[1]
  if (!payload) {
    return null
  }
  try {
    const base64 = payload.replace(/-/g, '+').replace(/_/g, '/')
    const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4)
    const binary = atob(padded)
    const bytes = Uint8Array.from(binary, (ch) => ch.charCodeAt(0))
    const json = new TextDecoder().decode(bytes)
    return JSON.parse(json) as ConsoleJwtClaims
  } catch {
    return null
  }
}

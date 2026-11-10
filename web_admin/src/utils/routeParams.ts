import type { RouteLocationNormalizedLoaded } from 'vue-router'

export function routeParam(route: RouteLocationNormalizedLoaded, key: string): string {
  const value = route.params[key]
  if (Array.isArray(value)) {
    return value[0] ?? ''
  }
  return value ?? ''
}

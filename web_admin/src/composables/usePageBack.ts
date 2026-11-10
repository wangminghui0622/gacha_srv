import { useRouter, useRoute } from 'vue-router'

export function usePageBack() {
  const router = useRouter()
  const route = useRoute()

  function resolveFallback(path: string): string {
    return path.replace(/:([A-Za-z_]+)/g, (_, key: string) => String(route.params[key] ?? ''))
  }

  function goBack() {
    const raw =
      (typeof route.meta.backFallback === 'string' && route.meta.backFallback) || '/dashboard'
    const fallback = resolveFallback(raw)
    if (window.history.length > 1) {
      router.back()
      return
    }
    router.push(fallback)
  }

  return { goBack, backFallback: route.meta.backFallback }
}

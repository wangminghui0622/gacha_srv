import axios, { type AxiosError } from 'axios'
import { message } from 'ant-design-vue'
import router from '@/router'
import { clearSessionCookies, getAccessToken } from '@/utils/cache'
import { newRequestId } from '@/utils/requestId'

declare module 'axios' {
  interface AxiosRequestConfig {
    // 可选资源缺失是正常空态时才开启（如 GET 任务计划审批 404 = 未提请）。
    silentNotFound?: boolean
  }
}

export const http = axios.create({
  timeout: 15000,
  headers: {
    'Content-Type': 'application/json',
  },
})

http.interceptors.request.use((config) => {
  // X-Request-Id 对所有控制台请求都必须携带（含登录/刷新等免鉴权请求，
  // 网关 RequestId 中间件无条件校验），不能只跟在 token 后面
  if (!config.headers['X-Request-Id']) config.headers['X-Request-Id'] = newRequestId()
  const token = getAccessToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

http.interceptors.response.use(
  (response) => response.data,
  async (error: AxiosError<{ message?: string; reason?: string }>) => {
    const status = error.response?.status
    const msg = error.response?.data?.message ?? error.message
    if (status === 401) {
      clearSessionCookies()
      message.error(msg || '会话已失效，请重新登录')
      await router.push({ path: '/login', query: { redirect: router.currentRoute.value.fullPath } })
    } else if (msg && !(status === 404 && error.config?.silentNotFound)) {
      message.error(msg)
    }
    return Promise.reject(error)
  },
)

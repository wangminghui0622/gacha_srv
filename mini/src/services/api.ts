import { tokenStorage } from '@/utils/storage'

import type { ApiResponse } from './types'

// uni.request Promise 适配器（规范 05-api-layer）：services/ 是唯一发起请求的位置
// 语义与 web 版 api.ts 逐条对齐：信封判定（code===0 返回 data）、401 清令牌跳登录、HTTP 状态码中文文案

// 小程序/H5 的 baseURL 差异：mp-weixin 必须绝对地址，H5 用相对路径走 vite 代理
// 生产环境在 .env.production 配 VITE_API_BASE_URL=https 域名（微信后台 request 合法域名白名单）
// 写法用 let+双赋值：条件编译只留一个赋值，且 vue-tsc 双分支语法合法
let DEFAULT_BASE_URL: string
// #ifdef H5
DEFAULT_BASE_URL = '/gacha/api/v1'
// #endif
// #ifndef H5
DEFAULT_BASE_URL = 'http://127.0.0.1:9310/gacha/api/v1'
// #endif

const BASE_URL = import.meta.env.VITE_API_BASE_URL || DEFAULT_BASE_URL

// HTTP 状态码中文文案表（业务码 msg 优先；网关恒 200，此表兜底代理层错误）
const httpStatusMessages: Record<number, string> = {
  400: '请求参数有误',
  401: '未登录或登录已过期',
  403: '没有权限执行此操作',
  404: '接口不存在',
  408: '请求超时，请重试',
  429: '操作太频繁，请稍后再试',
  500: '服务器开小差了，请稍后再试',
  502: '网关异常，请稍后再试',
  503: '服务暂不可用，请稍后再试',
}

// GET 参数序列化为 query string（axios params 同款行为：空值剔除）
function buildQuery(params?: unknown): string {
  if (!params || typeof params !== 'object') return ''
  const qs = Object.entries(params as Record<string, unknown>)
    .filter(([, v]) => v !== undefined && v !== null && v !== '')
    .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`)
    .join('&')
  return qs ? `?${qs}` : ''
}

// 401 重跳登录防抖：并发请求同批 401 只跳一次
let redirecting = false
function redirectToLogin() {
  if (redirecting) return
  redirecting = true
  tokenStorage.remove()
  uni.reLaunch({ url: '/pages/login/index' })
  setTimeout(() => {
    redirecting = false
  }, 1000)
}

export interface RequestConfig {
  headers?: Record<string, string>
}

// 核心请求：GET 参数进 query string，POST/PUT 进 body；信封解包（code===0 → data）
function request<T>(method: 'GET' | 'POST' | 'PUT', url: string, params?: unknown, config?: RequestConfig): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const token = tokenStorage.get()
    const options: UniApp.RequestOptions = {
      url: BASE_URL + url + (method === 'GET' ? buildQuery(params) : ''),
      method,
      timeout: 30000,
      header: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...(config?.headers ?? {}),
      },
      success: (res) => {
        const body = res.data as ApiResponse<unknown>
        if (body && typeof body === 'object' && 'code' in body) {
          if (body.code === 0) {
            resolve(body.data as T)
            return
          }
          // 401：令牌缺失/失效 → 清令牌跳登录
          if (body.code === 401) {
            redirectToLogin()
            reject(new Error('未登录或登录已过期'))
            return
          }
          reject(new Error(body.msg || '请求失败'))
          return
        }
        // 非信封（代理层错误页/域名未配置等）
        reject(new Error(httpStatusMessages[res.statusCode] ?? `请求失败（${res.statusCode}）`))
      },
      fail: () => {
        reject(new Error('网络异常，请检查网络连接'))
      },
    }
    if (method !== 'GET') {
      options.data = params as Record<string, unknown>
    }
    uni.request(options)
  })
}

// 类型化便捷方法：业务代码只用 get/post/put（返回已解包的 data）
export function get<T>(url: string, params?: unknown): Promise<T> {
  return request<T>('GET', url, params)
}

export function post<T>(url: string, data?: unknown, config?: RequestConfig): Promise<T> {
  return request<T>('POST', url, data, config)
}

export function put<T>(url: string, data?: unknown, config?: RequestConfig): Promise<T> {
  return request<T>('PUT', url, data, config)
}

# Services 层（API 对接）规范

## axios 核心（services/api.ts）

1. 唯一 axios 实例，集中配置：

```ts
export const api = axios.create({
  baseURL: `${import.meta.env.VITE_API_BASE_URL || 'http://localhost:8888'}/<项目名>`,
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
});
```

2. **请求拦截器**：从 tokenStorage 取令牌，自动附加 `Authorization: Bearer <token>`。
3. **响应拦截器**统一处理信封 `ApiResponse<T> = { code, msg, data }`：
   - `code === 0` → 返回 `data`
   - 业务码 401/令牌失效 → 清 token → 跳转登录页
   - 其余错误 → 抛 `new Error(中文提示)`（HTTP 状态码映射一张 switch 中文文案表）
4. 暴露类型化便捷方法，业务代码只用它们：

```ts
export const get = <T>(url: string, params?: unknown) => request<T>('GET', url, params);
export const post = <T>(url: string, data?: unknown) => request<T>('POST', url, data);
```

## 域服务文件（services/<domain>.ts）

5. 每个域一个文件，只导出具名函数，返回类型化 Promise，每个函数一行中文 JSDoc：

```ts
/** 用户登录 */
export function login(data: LoginRequest): Promise<AuthResponse> {
  return post<AuthResponse>('/api/v1/auth/login', data);
}
```

6. 端点路径与后端 `.api` 定义一致：`/api/v1/<域>/<动作>`（kebab-case）。
7. 列表响应的解包收敛在服务函数内：

```ts
export function getChannels(): Promise<PaymentChannel[]> {
  return get<{ channels: PaymentChannel[] }>('/api/v1/payment/channels').then((r) => r.channels);
}
```

8. 页面侧调用可用命名空间导入：`import * as authApi from '@/services/auth'` → `authApi.login(form)`。

## 类型（services/types.ts）

9. 共享 API 类型集中在 `types.ts`：`export interface`、无 `I` 前缀，按域分组并注明与后端对齐（`// 与后端 SpinResp 对齐`）。
10. 仅一个域使用的私有小接口可放该域服务文件内；两个域共用必须上移 `types.ts`。
11. 类型导入一律 `import type`。

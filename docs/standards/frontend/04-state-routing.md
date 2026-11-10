# 状态管理与路由规范

## Pinia 状态

1. 每个业务域一个 store 文件：`store/<domain>Store.ts` → `use<Domain>Store`（authStore、userStore、gameStore……）。
2. 结构统一：Options 式（state/actions 分区）或 Setup 式（`defineStore('auth', () => {...})`），**项目内统一用一种**：

```ts
// Options 式示例
export const useAuthStore = defineStore('auth', {
  state: () => ({ user: null as UserInfo | null, isAuthenticated: false }),
  actions: {
    setAuth(user: UserInfo) {
      this.user = user;
      this.isAuthenticated = true;
    },
    clearAuth() {
      this.user = null;
      this.isAuthenticated = false;
    },
  },
});
```

3. 认证类数据的持久化统一走 `utils/storage.ts` 的 tokenStorage 门面（基于 js-cookie，统一 key 与存取逻辑），store 内同步读写；不引入持久化插件（确需 pinia-plugin-persistedstate 时 PR 说明理由）。
4. 组件内取 state 用 `storeToRefs(useAuthStore())` 保持响应性，actions 直接解构；store 实例只在 `setup`/composable 内创建。
5. 请求的 loading/error 状态放组件 `ref`，不进 store；store 只放业务数据与动作。

## Vue Router 路由

6. 路由集中 `router/index.ts`：`createRouter` + `createWebHistory`，路由对象具名导出 `export const router`。
7. 所有页面路由组件懒加载 `() => import('@/pages/Home')`；加载进度用 nprogress（在 `beforeEach` 启动、`afterEach` 结束，已在基线内）。
8. 受保护页面用全局守卫 + 路由 meta 声明：

```ts
router.beforeEach((to) => {
  const auth = useAuthStore();
  if (!to.meta.public && !auth.isAuthenticated) {
    return { path: '/login', query: { redirect: to.fullPath } };
  }
});
// 路由声明处：{ path: '/login', component: ..., meta: { public: true, title: '登录' } }
```

9. 登录/注册页挂 `AuthLayout`（已登录用户进入时重定向回首页）。
10. 兜底路由 `{ path: '/:pathMatch(.*)*', redirect: '/' }`；废弃路径用 `redirect` 兼容跳转。
11. **`base` 三方一致**：router history `base` = `vite.config.ts` 的 `base` = nginx 部署子路径（如 `/gacha`）。

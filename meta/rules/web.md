---
paths:
  - "**/*.vue"
  - "**/*.ts"
  - "**/*.scss"
  - "**/*.css"
---

# 前端规则（Vue 3 + TypeScript + Vite + Ant Design Vue）

## 命名与文件

1. 组件/页面文件 PascalCase 且与组件同名（`GameHeader.vue` → `GameHeader`）；页面入口为 `pages/<Feature>/index.vue`，组件名与目录一致。
2. 接口命名 PascalCase、不加 `I` 前缀；props 接口命名 `<组件名>Props`；字符串字面量联合用 `type`（`type GameState = 'idle' | ...`），对象结构用 `interface`。
3. 每个 SFC 的 script 首行为中文 JSDoc 头注释（`/** 登录页面 */`）；每个导出的 service/util 函数带一行中文 JSDoc。
4. SFC（组件/页面/布局）由 script setup 自动导出；composable、store、services、utils、引擎类一律具名导出。

## 组件

5. 只用组合式 API（`<script setup lang="ts">`），禁止 Options API 与 mixin；props/emits 用类型声明式（`defineProps<XxxProps>()`、`defineEmits<{...}>()`）；需要暴露命令式 API 用 `defineExpose`。
6. 组件遵循单一职责；跨组件复用逻辑抽成 composable（`useXxx`）。
7. 模板内组件引用 PascalCase（`<GameHeader :title="t('game.title')" />`）；事件绑定用 `@click`。

## 状态与请求

8. Pinia 按域拆 store（`<domain>Store.ts` → `use<Domain>Store`），Options 式或 Setup 式项目内统一；认证持久化走 `utils/storage.ts` 的 tokenStorage 门面（js-cookie），不引入持久化插件。
9. 组件内取 state 用 `storeToRefs`，禁止整体解构 store 实例丢响应性；store 实例只在 setup/composable 内创建。
10. 加载/错误状态放组件 `ref`，不进 store；异步处理一律 `try/catch/finally`，`catch` 收窄类型：`err instanceof Error ? err.message : '默认中文错误'`。
11. 请求只从 `services/` 发起：域服务文件导出具名函数返回 typed Promise，每个函数带中文 JSDoc；公共类型集中到 `services/types.ts`。
12. 类型导入必须 `import type`（`verbatimModuleSyntax` 开启）。

## 路由与 i18n

13. 路由集中 `router/index.ts`（`createRouter` + 懒加载）；受保护页面靠 `beforeEach` 守卫 + `meta.public` 声明；`base` 必须与 `vite.config.ts` 的 `base`、nginx 部署路径三者一致。
14. 所有面向用户的文案走 `t('域.键')`，禁止硬编码；新增文案同步补 `i18n/locales/<lang>.json` 的点分命名空间 key。

## 样式

15. 主题色统一定义在 `src/styles/tokens.scss`（SCSS 变量 + CSS 变量）；Antdv 主题收敛在 `ConfigProvider`；禁止散落任意十六进制值，覆盖 Antdv 样式收敛到全局样式文件。
16. 后台用 Antdv Layout 骨架；移动端页面移动优先（`max-width` 居中、fixed Header/TabBar）；静态资源放 `public/`，按 feature 建子目录；组件私有样式用 `<style scoped lang="scss">`。

## 工程质量

17. 路径别名用 `@/`（tsconfig 与 vite.config 都要配）；禁止 `console.log` 留在业务代码（排查用 `console.error/warn`）。
18. 环境变量只暴露 `VITE_` 前缀；必须有 `.env.example` 供新环境复制。
19. 提交前跑 `npm run lint` 与 `npm run build`（build 含 vue-tsc 类型检查）。

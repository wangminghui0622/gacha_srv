# 工程质量规范（配置与工具链）

## TypeScript

1. 开启严格检查（`tsconfig.app.json`）：`strict`、`noUnusedLocals`、`noUnusedParameters`、`noFallthroughCasesInSwitch`、`erasableSyntaxOnly`、`verbatimModuleSyntax`（→ 类型导入必须 `import type`）、`moduleResolution: "bundler"`、`noEmit`；app 侧 extends `@vue/tsconfig/tsconfig.dom.json`，node 侧（vite.config 等）单列 `tsconfig.node.json`。
2. 路径别名 `@/` → `./src`：tsconfig 与 `vite.config.ts` 两处都要配置（vite 用 `fileURLToPath(new URL('./src', import.meta.url))`）；业务代码统一 `@/` 导入（引擎/同目录内允许相对路径）。
3. 类型放最近的合适位置：组件私有放组件文件、域共享放 `services/types.ts`；禁止 `any` 逃逸（确需时写注释说明）。

## Lint

4. oxlint 配置 `.oxlintrc.json` 最小化：`plugins: ["typescript", "oxc", "vue"]`；eslint 侧用 `@vue/eslint-config-typescript` + `eslint-plugin-vue`（script setup 语法）；`npm run lint` 串联 oxlint 与 eslint（`run-s lint:oxlint lint:eslint`）。
5. `npm run lint` 零告警才能提交；禁止在代码里用 disable 注释绕过（确需时 PR 说明）。

## 构建与环境

6. 构建脚本固定 `run-p type-check "build-only"`（`vue-tsc --build` + `vite build`）：类型检查不过则构建失败。
7. `vite.config.ts` 设置 `base` 与部署子路径一致（nginx + router 三方一致，见路由规范第 11 条）。
8. 环境文件：`.env.development`（本地）、`.env.production`（生产）；**必须有 `.env.example`** 列出全部变量与示例值；只暴露 `VITE_` 前缀变量；开发代理 `/api` 转发到 `VITE_DEV_PROXY_TARGET`（默认 `http://127.0.0.1:8888`）。
9. `.gitignore` 至少覆盖：`node_modules/`、`dist/`、`.env`（不含 `.env.example`）、日志、编辑器目录。

## 依赖与性能

10. 依赖变更在 PR 中说明动机；生产依赖与开发依赖严格分区。
11. 大图/资源压缩后进 `public/`；按需加载（路由 lazy 已强制）；Antdv 组件与图标按需 import，禁止全量引入图标库。
12. 列表页必须考虑分页/无限滚动，禁止一次性渲染全量数据。

## 测试（当前基线）

13. 当前基线无单测框架；引入测试时优先 Vitest + @vue/test-utils，覆盖 utils 纯函数与关键组件交互，与后端 e2e 场景形成分工。

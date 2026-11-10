# 前端架构规范

## 技术栈（固定基线）

1. Vite + Vue 3 + TypeScript + Vue Router + Pinia + Ant Design Vue + Axios + vue-i18n，工具库 dayjs、echarts、js-cookie、nprogress、qrcode 已在基线内：

| 类别 | 模块 | 版本基线 |
|------|------|---------|
| 框架 / 构建 | vue / vite | ^3.5 / ^8.2 |
| 路由 / 状态 | vue-router / pinia | ^5.3 / ^4.0 |
| UI 组件 | ant-design-vue / @ant-design/icons-vue | ^4.2 / ^7.0 |
| 请求 / 工具 | axios / dayjs / echarts / js-cookie / nprogress / qrcode | ^1.8 / ^1.11 / ^5.6 / ^3.0 / ^0.2 / ^1.5 |
| 国际化 | vue-i18n | ^11（本包硬规则要求 i18n，属本包在参考基线之外的唯一新增） |
| 样式 | sass | ^1.86 |
| 类型 / 检查 | typescript / vue-tsc / eslint+eslint-plugin-vue / oxlint / prettier | ~6.0 / ^3.3 / ^10 / ~1.82 / 3.9 |
| 运行时 | node | ^22.18 \|\| ^23 \|\| >=24.12 |

2. 新增依赖前先确认现有栈是否已覆盖；引入新库需在 PR 描述中说明理由。

## 分层

3. 四层单向依赖，禁止跨层调用：

```
pages/（页面编排：状态、交互、组合组件）
  → components/（可复用 UI，无业务请求）
  → services/（唯一 HTTP 出口）+ store/（域状态）
  → utils/（纯函数）
```

4. `services/` 是唯一发起请求的位置，组件/页面内禁止直接 import axios。
5. 纯逻辑引擎（游戏、编辑器、图表内核等）与 Vue 隔离：目录独立（如 `src/game/`）、零 vue import、用 `new`/生命周期方法（`destroy()/reset()`）管理，Vue 侧用薄封装组件 + `ref`/`defineExpose` 桥接。
6. 业务数据流：页面 `onMounted` 调 service → 结果写 store 或本地 `ref`；加载/错误态用组件内 `ref`，与 store 分离。

## 全局装配

7. `main.ts` 只做装配：`createApp(App).use(pinia).use(router).use(i18n)` + 全局样式副作用导入（`import '@/styles/main.scss'`）+ 挂载；Ant Design Vue 组件与图标按需 import 到使用处（或统一在装配区注册，二选一，项目内一致）。
8. 环境相关差异只通过 `import.meta.env.VITE_*` 进入代码，见工程质量规范第 8 条。

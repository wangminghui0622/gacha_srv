# 组件规范

## 定义

1. 只用**组合式 API**：SFC 一律 `<script setup lang="ts">`，禁止 Options API、禁止 mixin。
2. 文件 PascalCase 且与组件同名；页面入口 `index.vue` 与目录同名。
3. 每个 SFC 的 script 首行为中文 JSDoc 头注释：

```vue
<script setup lang="ts">
/**
 * 登录页面
 */
</script>
```

4. 组件/页面/布局为 SFC（script setup 无需显式导出）；composable、store、services、utils 一律具名导出。

## Props、Emits 与暴露

5. props 用类型声明式 `defineProps<XxxProps>()`，接口写在 script 顶部：`interface XxxProps { ... }`，不加 `I` 前缀；对象结构用 `interface`，字符串联合用 `type`。
6. emits 用类型声明式：

```ts
defineEmits<{ (e: 'change', v: string): void }>();
```

7. 需要暴露命令式方法时用 `defineExpose`，只暴露必要 API；引擎桥接见架构规范第 5 条。
8. 类型导入一律 `import type`（`verbatimModuleSyntax` 强制）。

## 职责与复用

9. 单一职责：一个组件做一件事；超过 ~200 行拆分子组件或 composable。
10. 跨组件复用逻辑抽 composable（`useXxx.ts`，放组件目录内或 `composables/`）；命名 `useXxx`。
11. 不传无意义 props 链（prop drilling 超过两层改用 store 或组合）。
12. 禁止组件内直接请求（见架构规范第 4 条）；数据通过 props 或 store 进入。

## 事件与错误

13. 异步处理一律 `try/catch/finally`；`catch (err)` 中收窄：`err instanceof Error ? err.message : '默认中文提示'`；`finally` 复位 loading。
14. 禁止 `console.log` 残留；排查用 `console.error`/`console.warn`。

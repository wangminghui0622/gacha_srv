---
name: vue-page
description: Scaffolds frontend features in this repo — new page, component, domain service, Pinia store, or i18n keys — following the project's Vue 3 standard (script setup SFCs, Ant Design Vue, typed services, namespaced vue-i18n). Use when the user asks to create a page, add a screen, build a component, or connect a new API on the frontend.
---

# vue-page：按规范脚手架前端页面/组件

目标：产出符合本项目前端规范的代码。规范细节见 `../../../docs/standards/frontend/`（02 布局、03 组件、04 状态路由、05 api 层、06 i18n 样式）。

## 第一步：明确范围

| 任务 | 产出 |
|------|------|
| 新页面 | `pages/<Feature>/index.vue` + 路由注册 + 服务函数 + i18n key |
| 子页面 | `pages/<Feature>/<Name>.vue` |
| 可复用组件 | `components/<领域>/<Name>.vue` |
| 新域服务 | `services/<域>.ts` + `services/types.ts` 补充 |
| 新域状态 | `store/<域>Store.ts` |

## 第二步：按模板生成

页面模板（script 首行中文 JSDoc、script setup、loading/error 在组件内）：

```vue
<script setup lang="ts">
/**
 * 活动列表页
 */
import { onMounted, ref } from 'vue';
import type { Activity } from '@/services/types';
import { getActivities } from '@/services/activity';

const list = ref<Activity[]>([]);
const loading = ref(false);
const error = ref('');

onMounted(() => {
  loading.value = true;
  getActivities()
    .then((data) => {
      list.value = data;
    })
    .catch((err: unknown) => {
      error.value = err instanceof Error ? err.message : '加载失败，请稍后重试';
    })
    .finally(() => {
      loading.value = false;
    });
});
</script>

<template>
  <a-spin :spinning="loading">
    <a-alert v-if="error" type="error" :message="error" />
    <!-- 列表渲染 -->
  </a-spin>
</template>
```

服务函数模板（具名导出、类型化、中文 JSDoc）：

```ts
/** 获取活动列表 */
export function getActivities(): Promise<Activity[]> {
  return get<{ activities: Activity[] }>('/api/v1/activity/list').then((r) => r.activities);
}
```

组件模板（props 接口在 script 顶部、无 I 前缀、类型声明式）：

```vue
<script setup lang="ts">
/**
 * 游戏头部信息条
 */
interface GameHeaderProps {
  title: string;
  balance: string;
}

defineProps<GameHeaderProps>();
</script>

<template>
  <div class="game-header">
    <span>{{ title }}</span>
    <span>{{ balance }}</span>
  </div>
</template>
```

## 第三步：接线（必守规则）

1. 路由：`router/index.ts` 中懒加载 `() => import('@/pages/<Feature>')` 注册；公开页面声明 `meta: { public: true }`，其余靠 `beforeEach` 守卫保护。
2. 类型：`import type`；共享类型进 `services/types.ts`（`export interface`、无 I 前缀、注明与后端对齐）。
3. i18n：所有文案走 `t('域.键')`，同步补齐 `i18n/locales/<语言>.json` 的点分命名空间 key。
4. 样式：色值引用 `tokens.scss` 令牌；需要新色先加令牌；组件私有样式用 `<style scoped lang="scss">`。
5. 状态：新域数据建 `store/<域>Store.ts`（Pinia，`storeToRefs` 取用）；不需要全局共享的放组件 `ref`。
6. 路径别名 `@/`；禁止组件内直接 axios；Antdv 组件与图标按需 import。

## 第四步：验证

- `npm run lint` 零告警；`npm run build`（含 vue-tsc）通过。
- 对照 `../../../docs/standards/common/review-checklist.md` 前端部分自查一遍。

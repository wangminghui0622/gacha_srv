<template>
  <nav :aria-label="$t('nav.ariaLabel')">
    <a-menu
      theme="light"
      mode="inline"
      :selected-keys="[selectedKey]"
      @click="onClick"
    >
      <a-menu-item v-for="item in routes" :key="menuPath(item)">
        <template #icon><component :is="item.icon" /></template>
        {{ $t(item.title) }}
      </a-menu-item>
    </a-menu>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { PermissionNode } from '@/access/grants'
import { menuPath, menuRoutes } from './routeCatalog'

const props = defineProps<{ nodes: PermissionNode[]; selectedKey: string }>()
const emit = defineEmits<{ navigate: [path: string] }>()
const routes = computed(() => menuRoutes(props.nodes))

function onClick(info: { key: string | number }) {
  const path = String(info.key)
  if (routes.value.some((item) => menuPath(item) === path)) emit('navigate', path)
}
</script>

<style scoped lang="scss">
nav {
  padding-top: 12px;
}

:deep(.ant-menu-light) {
  border-inline-end: 0;
  background: transparent;
}

:deep(.ant-menu-item) {
  height: 44px;
  line-height: 44px;
  margin: 4px 8px;
  width: calc(100% - 16px);
  color: rgba(0, 0, 0, 0.75);
  font-size: 14px;
}

:deep(.ant-menu-item .anticon) {
  font-size: 17px;
  color: rgba(0, 0, 0, 0.55);
}

:deep(.ant-menu-item-selected) {
  background: #fff;
  color: #1677ff;
  font-weight: 500;
}

:deep(.ant-menu-item-selected .anticon) {
  color: #1677ff;
}

:deep(.ant-menu-item:hover) {
  color: #1677ff;
}
</style>

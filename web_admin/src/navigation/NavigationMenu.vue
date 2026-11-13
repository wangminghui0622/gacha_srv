<template>
  <nav :aria-label="$t('nav.ariaLabel')">
    <a-menu
      theme="light"
      mode="vertical"
      :selected-keys="[selectedKey]"
      :open-keys="openKeys"
      :trigger-sub-menu-action="'click'"
      @openChange="onOpenChange"
      @click="onClick"
    >
      <template v-for="item in tree" :key="item.key">
        <a-sub-menu v-if="item.children" :key="item.key" popup-class-name="side-menu-popup">
          <template #icon><component :is="item.icon" /></template>
          <template #title>{{ $t(item.title) }}</template>
          <a-menu-item v-for="child in item.children" :key="child.path!">
            {{ $t(child.title) }}
          </a-menu-item>
        </a-sub-menu>
        <a-menu-item v-else :key="item.path!">
          <template #icon><component :is="item.icon" /></template>
          {{ $t(item.title) }}
        </a-menu-item>
      </template>
    </a-menu>
  </nav>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { PermissionNode } from '@/access/grants'
import { menuPath, menuRoutes, menuTree } from './routeCatalog'

const props = defineProps<{ nodes: PermissionNode[]; selectedKey: string }>()
const emit = defineEmits<{ navigate: [path: string] }>()
const tree = computed(() => menuTree(props.nodes))
const paths = computed(() => menuRoutes(props.nodes).map(menuPath))
const openKeys = ref<string[]>([])

function activeParent(): string | undefined {
  return tree.value.find((node) => node.children?.some((child) => props.selectedKey === child.path || props.selectedKey.startsWith(`${child.path}/`)))?.key
}

watch(
  () => [props.selectedKey, tree.value] as const,
  () => {
    const parent = activeParent()
    if (parent) openKeys.value = [parent]
  },
  { immediate: true },
)

function onOpenChange(keys: string[]) {
  // 商品子菜单在选中子项后保持展开（与设计稿一致）；切到其他一级菜单时自然关闭
  if (keys.length === 0 && activeParent()) {
    openKeys.value = [activeParent()!]
    return
  }
  openKeys.value = keys
}

function onClick(info: { key: string | number }) {
  const path = String(info.key)
  if (paths.value.includes(path)) emit('navigate', path)
}
</script>

<style scoped lang="scss">
nav {
  padding-top: 8px;
}

:deep(.ant-menu-light) {
  border-inline-end: 0;
  background: transparent;
}

:deep(.ant-menu-item),
:deep(.ant-menu-submenu-title) {
  height: 44px;
  line-height: 44px;
  margin: 4px 0 4px 8px;
  width: calc(100% - 8px);
  padding-left: 16px !important;
  padding-right: 7px !important;
  color: rgba(0, 0, 0, 0.75);
  font-size: 14px;
}

:deep(.ant-menu-item .anticon),
:deep(.ant-menu-submenu-title .anticon) {
  font-size: 17px;
  color: rgba(0, 0, 0, 0.55);
}

:deep(.ant-menu-item-selected) {
  background: #fff;
  color: #1677ff;
  font-weight: 500;
}

:deep(.ant-menu-item-selected .anticon),
:deep(.ant-menu-submenu-selected > .ant-menu-submenu-title),
:deep(.ant-menu-submenu-selected > .ant-menu-submenu-title .anticon) {
  color: #1677ff;
}

:deep(.ant-menu-item:hover),
:deep(.ant-menu-submenu-title:hover) {
  color: #1677ff;
}

:deep(.ant-menu-submenu-arrow) {
  inset-inline-end: 6px !important;
  right: 6px !important;
}
</style>

<style lang="scss">
.side-menu-popup .ant-menu {
  padding: 4px 0;
  min-width: 150px;
  border-radius: 4px;
}

.side-menu-popup .ant-menu-vertical {
  padding: 8px 0 !important;
}

.side-menu-popup .ant-menu-item {
  height: 44px !important;
  line-height: 44px !important;
  margin: 0 !important;
  width: 100% !important;
  border-radius: 0;
  color: rgba(0, 0, 0, 0.75);
}

.side-menu-popup .ant-menu-item-selected {
  background: #e6f4ff;
  color: #1677ff;
  font-weight: 500;
}
</style>

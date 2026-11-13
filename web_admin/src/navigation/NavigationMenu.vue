<template>
  <nav :aria-label="$t('nav.ariaLabel')">
    <a-menu
      theme="dark"
      mode="vertical"
      :selected-keys="selectedKeys"
      @click="onClick"
    >
      <a-menu-item
        v-for="item in tree"
        :key="item.key"
        :class="{ 'group-active': isGroupActive(item), 'group-open': item.children && item.key === activeGroupKey }"
      >
        <template #icon><component :is="item.icon" /></template>
        <span class="item-title">{{ $t(item.title) }}</span>
        <RightOutlined v-if="item.children" class="group-arrow" />
      </a-menu-item>
    </a-menu>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { RightOutlined } from '@ant-design/icons-vue'
import type { PermissionNode } from '@/access/grants'
import { menuTree, type MenuNode } from './routeCatalog'

const props = defineProps<{ nodes: PermissionNode[]; selectedKey: string; activeGroupKey?: string | null }>()
const emit = defineEmits<{ navigate: [path: string]; 'toggle-group': [key: string] }>()
const tree = computed(() => menuTree(props.nodes))
const selectedKeys = computed(() =>
  tree.value.filter((item) => !item.children && item.path === props.selectedKey).map((item) => item.key),
)

function isGroupActive(item: MenuNode): boolean {
  if (!item.children) return false
  return item.children.some((child) => props.selectedKey === child.path || props.selectedKey.startsWith(`${child.path}/`))
}

function onClick(info: { key: string | number }) {
  const key = String(info.key)
  const item = tree.value.find((node) => node.key === key)
  if (!item) return
  if (item.children) {
    emit('toggle-group', key)
  } else if (item.path) {
    emit('navigate', item.path)
  }
}
</script>

<style scoped lang="scss">
nav {
  padding-top: 8px;
}

:deep(.ant-menu-dark) {
  border-inline-end: 0;
  background: transparent;
}

:deep(.ant-menu-item) {
  position: relative;
  height: 44px;
  line-height: 44px;
  margin: 4px 0 4px 8px;
  width: calc(100% - 8px);
  padding-left: 16px !important;
  padding-right: 26px !important;
  color: #bfcbd9;
  font-size: 14px;
}

:deep(.ant-menu-item .anticon) {
  font-size: 17px;
  color: #bfcbd9;
}

.item-title {
  display: inline-block;
  vertical-align: middle;
}

.group-arrow {
  position: absolute;
  top: 50%;
  right: 8px;
  transform: translateY(-50%);
  font-size: 10px;
  color: #bfcbd9;
}

:deep(.ant-menu-item-selected) {
  background: #263445;
  color: #409eff;
  font-weight: 500;
}

:deep(.ant-menu-item-selected .anticon),
:deep(.ant-menu-item.group-active),
:deep(.ant-menu-item.group-active .anticon),
:deep(.ant-menu-item.group-active .group-arrow) {
  color: #409eff;
}

:deep(.ant-menu-item.group-open::after) {
  content: '';
  position: absolute;
  top: 50%;
  right: 0;
  width: 8px;
  height: 2px;
  background: #409eff;
  border-radius: 1px;
  transform: translateY(-50%);
  z-index: 2;
}

:deep(.ant-menu-item:hover) {
  background: #263445;
  color: #409eff;
}
</style>

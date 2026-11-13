<template>
  <div class="submenu-list">
    <router-link
      v-for="child in group.children ?? []"
      :key="child.path"
      :to="child.path!"
      class="submenu-item"
      :class="{ active: isActive(child.path!) }"
      @click="$emit('navigate')"
    >
      {{ $t(child.title) }}
    </router-link>
  </div>
</template>

<script setup lang="ts">
import { useRoute } from 'vue-router'
import type { MenuNode } from './routeCatalog'

defineProps<{ group: MenuNode }>()
defineEmits<{ navigate: [] }>()

const route = useRoute()

function isActive(path: string): boolean {
  return route.path === path
}
</script>

<style scoped lang="scss">
.submenu-list {
  width: 100%;
  padding: 8px 0;
}

.submenu-item {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 38px;
  padding: 9px 16px;
  line-height: 20px;
  font-size: 14px;
  color: #606266;
  text-align: center;
  white-space: normal;
  word-break: break-word;
}

.submenu-item:hover {
  background: #e6f4ff;
  color: #409eff;
}

.submenu-item.active {
  background: #e6f4ff;
  color: #409eff;
  font-weight: 500;
}
</style>

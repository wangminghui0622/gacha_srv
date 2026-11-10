<template>
  <router-link v-if="linkTarget" :to="linkTarget" class="entity-link">{{ labelText }}</router-link>
  <span v-else class="entity-link-muted">{{ labelText }}</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
import { accessibleRoutes } from '@/navigation/routeCatalog'
import { useSessionStore } from '@/stores/session'

const props = defineProps<{
  to?: RouteLocationRaw | null
  label?: string
  id?: string | number
  kind?: string
}>()

const session = useSessionStore()
// 未授权的实体路由不会注册，避免 RouterLink 解析不存在的命名路由。
const linkTarget = computed(() => {
  const target = props.to
  if (target && typeof target === 'object' && 'name' in target && target.name) {
    if (!accessibleRoutes(session.permissionNodes).some((item) => item.name === target.name)) return null
  }
  return target
})

const labelText = computed(() => {
  if (props.label) {
    return props.label
  }
  const prefix = props.kind ? `${props.kind} ` : ''
  return `${prefix}${props.id ?? '—'}`
})
</script>

<style scoped>
.entity-link {
  color: #1677ff;
}
.entity-link-muted {
  color: rgba(0, 0, 0, 0.45);
}
</style>

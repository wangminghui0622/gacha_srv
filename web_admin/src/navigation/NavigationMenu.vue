<template>
  <nav :aria-label="$t('nav.ariaLabel')">
    <a-menu theme="dark" mode="inline" :selected-keys="[selectedKey]" :open-keys="openKeys" @openChange="onOpenChange" @click="onClick">
      <a-menu-item v-for="item in standalone" :key="menuPath(item)">
        <template #icon><component :is="item.icon" /></template>{{ $t(item.title) }}
      </a-menu-item>
      <a-sub-menu v-for="group in groups" :key="group.key">
        <template #title>{{ $t(group.title) }}</template>
        <a-menu-item v-for="item in group.items" :key="menuPath(item)">
          <template #icon><component :is="item.icon" /></template>{{ $t(item.title) }}
        </a-menu-item>
      </a-sub-menu>
    </a-menu>
  </nav>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { PermissionNode } from '@/access/grants'
import { menuGroups, menuPath, menuRoutes } from './routeCatalog'

const props = defineProps<{ nodes: PermissionNode[]; selectedKey: string }>()
const emit = defineEmits<{ navigate: [path: string] }>()
const routes = computed(() => menuRoutes(props.nodes))
const groups = computed(() => menuGroups(props.nodes))
const standalone = computed(() => routes.value.filter((item) => !item.menuGroup))
const openKeys = ref<string[]>([])

watch(
  () => [props.selectedKey, groups.value] as const,
  () => {
    const active = groups.value.find((group) => group.items.some((item) => menuPath(item) === props.selectedKey))
    if (active && !openKeys.value.includes(active.key)) openKeys.value = [...openKeys.value, active.key]
    openKeys.value = openKeys.value.filter((key) => groups.value.some((group) => group.key === key))
  },
  { immediate: true },
)

function onOpenChange(keys: string[]) { openKeys.value = keys }
function onClick(info: { key: string | number }) {
  const path = String(info.key)
  if (routes.value.some((item) => menuPath(item) === path)) emit('navigate', path)
}
</script>

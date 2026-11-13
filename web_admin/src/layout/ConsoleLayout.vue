<template>
  <a-layout class="console-layout">
    <SecurityWatermark v-if="session.accessToken" />
    <a-layout-sider class="desktop-sider" theme="light" width="144">
      <NavigationMenu
        :nodes="session.permissionNodes"
        :selected-key="selectedKey"
        :active-group-key="activeGroupKey"
        @toggle-group="toggleGroup"
        @navigate="navigate"
      />
    </a-layout-sider>
    <aside class="secondary-sider" :class="{ open: !!activeGroup }" :style="{ width: secondaryWidth }">
      <div class="secondary-body" :style="{ paddingTop: `${submenuTop}px` }">
        <div ref="submenuMeasure" class="submenu-card">
          <SubmenuList v-if="activeGroup" :group="activeGroup" @navigate="mobileOpen = false" />
        </div>
      </div>
    </aside>
    <a-drawer v-model:open="mobileOpen" placement="left" :width="144" :closable="false" root-class-name="console-mobile-drawer">
      <NavigationMenu
        :nodes="session.permissionNodes"
        :selected-key="selectedKey"
        :active-group-key="activeGroupKey"
        @toggle-group="toggleGroup"
        @navigate="navigate"
      />
      <div v-if="activeGroup" class="mobile-submenu-panel">
        <SubmenuList :group="activeGroup" @navigate="mobileOpen = false" />
      </div>
    </a-drawer>
    <a-layout>
      <a-layout-header class="header">
        <div class="header-heading">
          <a-button class="mobile-trigger" type="text" :aria-label="$t('layout.openMenu')" @click="mobileOpen = true"><MenuOutlined /></a-button>
          <router-link to="/dashboard" class="crumb-home">{{ $t('nav.dashboard') }}</router-link>
          <span class="crumb-sep">/</span>
          <span class="header-title">{{ headerTitle }}</span>
        </div>
        <a-space>
          <span class="user-label">{{ session.displayName }}</span>
          <a-button type="text" :loading="session.permissionsLoading" @click="refreshPermissions">{{ $t('layout.refreshPermissions') }}</a-button>
          <a-button type="link" @click="onLogout">{{ $t('common.logout') }}</a-button>
        </a-space>
      </a-layout-header>
      <div class="tab-container">
        <div class="custom-tabs">
          <div
            v-for="tab in tabs"
            :key="tab.path"
            class="custom-tab"
            :class="{ active: activeTab === tab.path }"
            @click="handleTabChange(tab.path)"
          >
            <span class="tab-label">{{ $t(tab.titleKey) }}</span>
            <CloseOutlined v-if="tabs.length > 1" class="tab-close" @click.stop="handleTabRemove(tab.path)" />
          </div>
        </div>
        <button v-if="tabs.length > 1" type="button" class="tab-close-all" @click="closeAllTabs">
          <CloseOutlined />
          <span>{{ $t('layout.closeAllTabs') }}</span>
        </button>
      </div>
      <a-layout-content class="content"><router-view /></a-layout-content>
    </a-layout>
  </a-layout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { CloseOutlined, MenuOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import SecurityWatermark from '@/components/SecurityWatermark.vue'
import NavigationMenu from '@/navigation/NavigationMenu.vue'
import SubmenuList from '@/navigation/SubmenuList.vue'
import { menuPath, menuRoutes, menuTree } from '@/navigation/routeCatalog'
import { useSessionStore } from '@/stores/session'

const route = useRoute()
const router = useRouter()
const session = useSessionStore()
const { t } = useI18n()
const mobileOpen = ref(false)
interface TabItem {
  path: string
  titleKey: string
}
const tabs = ref<TabItem[]>([])
const activeTab = computed(() => route.path)
const menus = computed(() => menuRoutes(session.permissionNodes))
const tree = computed(() => menuTree(session.permissionNodes))
const activeGroupKey = ref<string | null>(null)
const activeGroup = computed(() => tree.value.find((node) => node.key === activeGroupKey.value && node.children) ?? null)
const secondaryWidth = computed(() => (activeGroup.value ? '144px' : '0px'))
const submenuMeasure = ref<HTMLElement | null>(null)
const submenuTop = ref(0)
let submenuObserver: ResizeObserver | null = null

function updateSubmenuTop() {
  if (!activeGroup.value || !submenuMeasure.value) {
    submenuTop.value = 0
    return
  }
  const index = tree.value.findIndex((node) => node.key === activeGroupKey.value)
  // 一级菜单首项中心 34px，之后每项间距 48px（44px 高度 + 4px 外边距折叠）
  const parentCenter = 34 + Math.max(index, 0) * 48
  const height = submenuMeasure.value.offsetHeight
  const maxTop = Math.max(0, window.innerHeight - height - 8)
  submenuTop.value = Math.max(0, Math.min(parentCenter - height / 2, maxTop))
}

watch(
  () => [activeGroupKey.value, submenuMeasure.value] as const,
  async () => {
    await nextTick()
    submenuObserver?.disconnect()
    submenuObserver = null
    if (submenuMeasure.value) {
      submenuObserver = new ResizeObserver(updateSubmenuTop)
      submenuObserver.observe(submenuMeasure.value)
    }
    updateSubmenuTop()
  },
  { immediate: true },
)

onMounted(() => window.addEventListener('resize', updateSubmenuTop))
onBeforeUnmount(() => {
  window.removeEventListener('resize', updateSubmenuTop)
  submenuObserver?.disconnect()
})
const headerTitle = computed(() => t((route.meta.title as string) ?? 'nav.dashboard'))
const selectedKey = computed(() => {
  const hit = menus.value
    .filter((item) => route.path === menuPath(item) || route.path.startsWith(`${menuPath(item)}/`))
    .sort((a, b) => b.path.length - a.path.length)[0]
  return hit ? menuPath(hit) : route.path
})

watch(
  () => [route.path, tree.value] as const,
  ([path]) => {
    const group = tree.value.find((node) =>
      node.children?.some((child) => path === child.path || path.startsWith(`${child.path}/`)),
    )
    activeGroupKey.value = group?.key ?? null
  },
  { immediate: true },
)

function toggleGroup(key: string) {
  activeGroupKey.value = activeGroupKey.value === key ? null : key
}

function navigate(path: string) {
  mobileOpen.value = false
  if (path !== route.path) router.push(path)
}

watch(
  () => route.path,
  (path) => {
    if (path === '/login') return
    const titleKey = (route.meta.title as string | undefined) ?? 'nav.dashboard'
    if (!tabs.value.some((tab) => tab.path === path)) {
      tabs.value.push({ path, titleKey })
    }
  },
  { immediate: true },
)

function handleTabChange(path: string) {
  if (path !== route.path) router.push(path)
}

function handleTabRemove(path: string) {
  const index = tabs.value.findIndex((tab) => tab.path === path)
  if (index < 0) return
  const wasActive = path === route.path
  tabs.value.splice(index, 1)
  if (!wasActive) return
  const next = tabs.value[index] ?? tabs.value[index - 1] ?? tabs.value[0]
  router.push(next?.path ?? '/dashboard')
}

function closeAllTabs() {
  tabs.value = [{ path: '/dashboard', titleKey: 'nav.dashboard' }]
  activeGroupKey.value = null
  if (route.path !== '/dashboard') router.push('/dashboard')
}

async function refreshPermissions() {
  try {
    await session.loadPermissions()
    message.success(t('layout.permissionsUpdated'))
  } catch {
    // 请求拦截器已展示失败信息；失败时保留当前菜单，服务端继续校验权限。
  }
}

async function onLogout() {
  await session.logout()
  router.push('/login')
}
</script>

<style scoped lang="scss">
.console-layout { min-height: 100vh; }
.desktop-sider {
  border-right: none;
  background: #304156;
}
.secondary-sider {
  position: relative;
  flex: 0 0 auto;
  align-self: stretch;
  width: 0;
  overflow: hidden;
  background: #f0f2f5;
}
.secondary-sider.open {
  width: 144px;
  overflow-y: auto;
  overflow-x: hidden;
}
.secondary-sider.open::after {
  content: '';
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 1px;
  background: #e4e7ed;
  pointer-events: none;
  z-index: 2;
}
.secondary-body { min-height: 100%; }
.submenu-card {
  position: relative;
  width: 144px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
  overflow: hidden;
}
.mobile-submenu-panel {
  margin: 8px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  overflow: hidden;
}
.header {
  height: 48px;
  line-height: normal;
  background: #fff;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 0 20px;
  border-bottom: 1px solid #e4e7ed;
}
.header-heading { display: flex; align-items: center; gap: 6px; min-width: 0; }
.crumb-home { color: #606266; font-size: 14px; }
.crumb-home:hover { color: #409eff; }
.crumb-sep { color: #c0c4cc; }
.header-title { font-size: 14px; font-weight: 500; color: #303133; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.user-label { color: #606266; white-space: nowrap; }
.tab-container {
  background: #fff;
  border-bottom: 1px solid #e4e7ed;
  padding: 6px 20px 0;
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
}
.custom-tabs {
  flex: 1;
  min-width: 0;
  display: flex;
  gap: 4px;
  overflow-x: auto;
  overflow-y: hidden;
}
.custom-tabs::-webkit-scrollbar { height: 0; }
.custom-tab {
  position: relative;
  height: 32px;
  line-height: 32px;
  padding: 0 28px 0 16px;
  border: 1px solid #dcdfe6;
  border-bottom: none;
  border-radius: 4px 4px 0 0;
  background: #e4e7ed;
  color: #606266;
  cursor: pointer;
  white-space: nowrap;
  flex-shrink: 0;
  font-size: 14px;
  user-select: none;
}
.custom-tab:hover { color: #409eff; }
.custom-tab.active {
  background: #fff;
  border-bottom: 1px solid #fff;
  color: #409eff;
  font-weight: 500;
  margin-bottom: -1px;
}
.tab-label {
  display: inline-block;
  max-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: middle;
}
.tab-close {
  position: absolute;
  right: 8px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 12px;
  color: #909399;
}
.tab-close:hover { color: #409eff; }
.tab-close-all {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 32px;
  padding: 0 8px;
  border: none;
  background: transparent;
  color: #909399;
  font-size: 12px;
  cursor: pointer;
  white-space: nowrap;
  margin-bottom: 1px;
}
.tab-close-all:hover { color: #409eff; }
.content { padding: 20px; background: #f0f2f5; min-width: 0; min-height: calc(100vh - 88px); }
.mobile-trigger { display: none; }
@media (max-width: 767px) {
  .desktop-sider { display: none; }
  .secondary-sider { display: none; }
  .mobile-trigger { display: inline-flex; }
  .header { padding: 0 8px; }
  .user-label { display: none; }
  .tab-container { padding: 6px 8px 0; }
  .content { padding: 12px; }
}
</style>

<style lang="scss">
.console-mobile-drawer .ant-drawer-body { padding: 0; background: #304156; }
</style>

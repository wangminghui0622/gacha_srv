<template>
  <a-layout class="console-layout">
    <SecurityWatermark v-if="session.accessToken" />
    <a-layout-sider class="desktop-sider" theme="light" width="144">
      <NavigationMenu :nodes="session.permissionNodes" :selected-key="selectedKey" @navigate="navigate" />
    </a-layout-sider>
    <a-drawer v-model:open="mobileOpen" placement="left" :width="144" :closable="false" root-class-name="console-mobile-drawer">
      <NavigationMenu :nodes="session.permissionNodes" :selected-key="selectedKey" @navigate="navigate" />
    </a-drawer>
    <a-layout>
      <a-layout-header class="header">
        <div class="header-heading">
          <a-button class="mobile-trigger" type="text" :aria-label="$t('layout.openMenu')" @click="mobileOpen = true"><MenuOutlined /></a-button>
          <span class="header-title">{{ headerTitle }}</span>
        </div>
        <a-space>
          <span class="user-label">{{ session.displayName }}</span>
          <a-button type="text" :loading="session.permissionsLoading" @click="refreshPermissions">{{ $t('layout.refreshPermissions') }}</a-button>
          <a-button type="link" @click="onLogout">{{ $t('common.logout') }}</a-button>
        </a-space>
      </a-layout-header>
      <a-layout-content class="content"><router-view /></a-layout-content>
    </a-layout>
  </a-layout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { MenuOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import SecurityWatermark from '@/components/SecurityWatermark.vue'
import NavigationMenu from '@/navigation/NavigationMenu.vue'
import { menuPath, menuRoutes } from '@/navigation/routeCatalog'
import { useSessionStore } from '@/stores/session'

const route = useRoute()
const router = useRouter()
const session = useSessionStore()
const { t } = useI18n()
const mobileOpen = ref(false)
const menus = computed(() => menuRoutes(session.permissionNodes))
const headerTitle = computed(() => t((route.meta.title as string) ?? 'nav.dashboard'))
const selectedKey = computed(() => {
  const hit = menus.value
    .filter((item) => route.path === menuPath(item) || route.path.startsWith(`${menuPath(item)}/`))
    .sort((a, b) => b.path.length - a.path.length)[0]
  return hit ? menuPath(hit) : route.path
})

function navigate(path: string) {
  mobileOpen.value = false
  if (path !== route.path) router.push(path)
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
  border-right: 1px solid #f0f0f0;
  background: #fff;
}
.header { background: #fff; display: flex; align-items: center; justify-content: space-between; gap: 12px; padding-inline: 24px; border-bottom: 1px solid #f0f0f0; }
.header-heading { display: flex; align-items: center; gap: 8px; min-width: 0; }
.header-title { font-size: 16px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.user-label { color: rgba(0, 0, 0, 0.65); white-space: nowrap; }
.content { margin: 16px; padding: 16px; background: #fff; min-width: 0; min-height: 280px; }
.mobile-trigger { display: none; }
@media (max-width: 767px) {
  .desktop-sider { display: none; }
  .mobile-trigger { display: inline-flex; }
  .header { padding-inline: 8px; }
  .user-label { display: none; }
  .content { margin: 8px; padding: 12px; }
}
</style>

<style lang="scss">
.console-mobile-drawer .ant-drawer-body { padding: 0; background: #fff; }
</style>

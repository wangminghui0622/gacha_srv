<template>
  <div>
    <PageHeader :title="$t('nav.dashboard')" />
    <a-row :gutter="16">
      <a-col :xs="24" :md="12">
        <a-card :title="$t('dashboard.gatewayHealth')" :loading="healthLoading">
          <a-descriptions v-if="health" :column="1" size="small" bordered>
            <a-descriptions-item :label="$t('common.status')">{{ health.status }}</a-descriptions-item>
            <a-descriptions-item :label="$t('dashboard.service')">{{ $t('common.none') }}</a-descriptions-item>
          </a-descriptions>
          <a-alert v-else-if="healthError" type="warning" :message="healthError" show-icon />
        </a-card>
      </a-col>
      <a-col :xs="24" :md="12">
        <a-card :title="$t('dashboard.currentSession')">
          <a-descriptions :column="1" size="small" bordered>
            <a-descriptions-item :label="$t('dashboard.operator')">{{ session.displayName }}</a-descriptions-item>
            <a-descriptions-item :label="$t('dashboard.tenantId')">{{ session.tenantId ?? $t('common.none') }}</a-descriptions-item>
            <a-descriptions-item :label="$t('dashboard.roles')">
              <a-space wrap>
                <a-tag v-for="role in session.roles" :key="role">{{ role }}</a-tag>
                <span v-if="session.roles.length === 0">{{ $t('common.none') }}</span>
              </a-space>
            </a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
    </a-row>
    <a-card :title="$t('dashboard.shortcuts')" style="margin-top: 16px">
      <a-space v-if="shortcuts.length" wrap>
        <router-link v-for="item in shortcuts" :key="item.name" :to="menuPath(item)">{{ $t(item.title) }}</router-link>
      </a-space>
      <a-empty v-else :description="$t('dashboard.noAccess')" :image="Empty.PRESENTED_IMAGE_SIMPLE" />
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Empty } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { fetchConsoleHealth } from '@/api/console'
import PageHeader from '@/components/PageHeader.vue'
import { menuPath, menuRoutes } from '@/navigation/routeCatalog'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const { t } = useI18n()
const shortcuts = computed(() => menuRoutes(session.permissionNodes).filter((item) => item.name !== 'dashboard'))
const health = ref<{ status?: string } | null>(null)
const healthLoading = ref(false)
const healthError = ref('')

onMounted(async () => {
  healthLoading.value = true
  try {
    health.value = await fetchConsoleHealth()
  } catch {
    // 本仓 health 只有 {status}，错误文案指引到 /console/v2/api/health 与 9321 代理。
    healthError.value = t('dashboard.healthError')
  } finally {
    healthLoading.value = false
  }
})
</script>

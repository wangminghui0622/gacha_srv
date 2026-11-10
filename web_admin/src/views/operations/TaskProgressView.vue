<template>
  <div>
    <PageHeader :title="$t('nav.taskProgress')" />
    <a-alert
      v-if="!canRead"
      type="error"
      :message="$t('taskProgress.noPermission')"
      show-icon
      style="margin-bottom: 16px"
    />
    <a-form layout="inline" :model="query" @finish="loadTasks">
      <a-form-item :label="$t('taskProgress.tenantId')" name="tenantId" :rules="[{ required: true }]">
        <a-input-number v-model:value="query.tenantId" :min="1" />
      </a-form-item>
      <a-form-item :label="$t('taskProgress.userId')" name="userId" :rules="[{ required: true }]">
        <a-input-number v-model:value="query.userId" :min="1" />
      </a-form-item>
      <a-form-item>
        <a-button type="primary" html-type="submit" :loading="loading" :disabled="!canRead">{{ $t('common.query') }}</a-button>
      </a-form-item>
    </a-form>
    <a-table
      style="margin-top: 16px"
      :row-key="rowKey"
      :loading="loading"
      :columns="columns"
      :data-source="rows"
      :pagination="false"
    >
      <template #bodyCell="{ column }">
        <template v-if="column.key === 'userLink'">
          <EntityLink :kind="$t('entity.user')" :id="query.userId" :to="entityRoutes.userOverview(query.userId)" />
          <span> · </span>
          <router-link :to="entityRoutes.userProfit(query.userId)">{{ $t('taskProgress.profit') }}</router-link>
        </template>
      </template>
    </a-table>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { listUserTasks, type TaskView } from '@/api/console'
import PageHeader from '@/components/PageHeader.vue'
import EntityLink from '@/components/entity/EntityLink.vue'
import { allows } from '@/access/grants'
import { entityRoutes } from '@/navigation/entityRoutes'
import { useSessionStore } from '@/stores/session'

const route = useRoute()
const session = useSessionStore()
const { t } = useI18n()
const canRead = computed(() => allows(session.permissionNodes, 'task.progress', 'read'))

const initialUserId = Number(route.query.userId ?? 1)

const query = reactive({
  tenantId: session.tenantId ?? Number(import.meta.env.VITE_DEFAULT_TENANT_ID || 1),
  userId: Number.isFinite(initialUserId) && initialUserId > 0 ? initialUserId : 1,
})

const loading = ref(false)
const rows = ref<TaskView[]>([])

const columns = [
  { title: t('taskProgress.user'), key: 'userLink' },
  { title: t('taskProgress.taskId'), dataIndex: ['definition', 'taskId'], key: 'taskId' },
  { title: t('taskProgress.progress'), dataIndex: 'progress', key: 'progress' },
  { title: t('taskProgress.claimed'), dataIndex: 'claimed', key: 'claimed' },
  { title: t('taskProgress.taskState'), dataIndex: 'state', key: 'state' },
]

// 本仓 TaskView 嵌套 definition（wire 差异见 api/console.ts）。
const rowKey = (record: TaskView) => record.definition?.taskId ?? ''

async function loadTasks() {
  if (!canRead.value) {
    return
  }
  loading.value = true
  try {
    const reply = await listUserTasks({
      tenantId: query.tenantId,
      userId: query.userId,
    })
    rows.value = (reply.tasks ?? []).map((row) => ({ ...row, definition: row.definition ?? {} }))
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (route.query.userId && canRead.value) {
    void loadTasks()
  }
})
</script>

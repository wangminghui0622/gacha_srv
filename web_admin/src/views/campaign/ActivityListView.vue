<template>
  <div class="activity-page console-list-page">
    <div class="activity-toolbar">
      <span class="toolbar-label">活动key:</span>
      <a-input
        v-model:value="keyword"
        class="toolbar-input"
        placeholder="活动key"
        allow-clear
        @press-enter="queryList"
      />
      <a-button class="btn-query" type="primary" :loading="loading" @click="queryList">查询</a-button>
      <a-button v-if="canWrite" @click="openCreate">创建</a-button>
    </div>

    <a-table
      :data-source="records"
      :loading="loading"
      :pagination="false"
      :row-selection="rowSelection"
      :row-key="(record: Activity) => String(record.id)"
      :scroll="{ x: 1700 }"
      size="middle"
      class="activity-table"
    >
      <a-table-column title="ID" width="70" align="center">
        <template #default="{ record }">{{ record.id }}</template>
      </a-table-column>
      <a-table-column title="Key" width="150" align="center">
        <template #default="{ record }">{{ record.activityKey || '-' }}</template>
      </a-table-column>
      <a-table-column title="活动名称" min-width="200" align="center">
        <template #default="{ record }">{{ record.name || '-' }}</template>
      </a-table-column>
      <a-table-column title="状态" width="90" align="center">
        <template #default="{ record }">
          <span :class="['status-text', record.enabled ? 'status-on' : 'status-off']">
            {{ record.enabled ? '开启' : '关闭' }}
          </span>
        </template>
      </a-table-column>
      <a-table-column title="描述" min-width="260">
        <template #default="{ record }">
          <a-tooltip :title="record.description || '-'">
            <span class="ellipsis-text">{{ record.description || '-' }}</span>
          </a-tooltip>
        </template>
      </a-table-column>
      <a-table-column title="协议" width="120" align="center">
        <template #default="{ record }">{{ record.protocol || '-' }}</template>
      </a-table-column>
      <a-table-column title="时间范围" width="330" align="center">
        <template #default="{ record }">
          <div v-if="hasMainRange(record)" class="time-cell">
            <span>{{ formatTime(record.startAtUnixMillis) }}</span>
            <span class="time-sep">~</span>
            <span>{{ formatTime(record.endAtUnixMillis) }}</span>
          </div>
          <span v-else>-</span>
        </template>
      </a-table-column>
      <a-table-column title="日活动时间" width="170" align="center">
        <template #default="{ record }">{{ dailyRangeText(record) }}</template>
      </a-table-column>
      <a-table-column title="创建时间" width="180" align="center">
        <template #default="{ record }">{{ formatTime(record.createdAtUnixMillis) }}</template>
      </a-table-column>
      <a-table-column title="操作" width="190" align="center" fixed="right">
        <template #default="{ record }">
          <a-space v-if="canWrite">
            <a-button size="small" @click="openCoupons(record)">优惠券</a-button>
            <a-button size="small" type="primary" @click="openEdit(record)">编辑</a-button>
          </a-space>
          <span v-else>-</span>
        </template>
      </a-table-column>
    </a-table>

    <div class="activity-footer console-list-footer">
      <a-pagination
        v-model:current="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-size-options="['10', '20', '50', '100']"
        :show-total="showTotal"
        show-size-changer
        show-less-items
        @change="load"
      />
    </div>

    <a-modal
      v-model:open="editorOpen"
      title="活动"
      :footer="null"
      :width="1100"
      :style="{ maxWidth: '94vw' }"
      :body-style="{ maxHeight: 'calc(100vh - 160px)', overflowY: 'auto' }"
      :mask-closable="false"
      centered
      destroy-on-close
    >
      <ActivityForm :key="editorKey" :record="editingRecord" @success="onSaved" @cancel="editorOpen = false" />
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import dayjs from 'dayjs'
import ActivityForm from '@/views/campaign/ActivityForm.vue'
import { activityClient } from '@/api/activity'
import type { Activity } from '@/api/activity'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'

const router = useRouter()
const session = useSessionStore()
const canWrite = computed(() => allows(session.permissionNodes, 'campaign.definition', 'write'))

const loading = ref(false)
const records = ref<Activity[]>([])
const total = ref(0)
const keyword = ref('')
const page = ref(1)
const pageSize = ref(15)
const selectedRowKeys = ref<(string | number)[]>([])
const editorOpen = ref(false)
const editingRecord = ref<Activity | null>(null)
const editorKey = ref(0)

const rowSelection = computed(() => ({
  selectedRowKeys: selectedRowKeys.value,
  onChange: (keys: (string | number)[]) => {
    selectedRowKeys.value = keys
  },
}))

function formatTime(value: string | number | undefined) {
  const ms = Number(value ?? 0)
  return ms > 0 ? dayjs(ms).format('YYYY-MM-DD HH:mm:ss') : '-'
}

function hasMainRange(record: Activity) {
  return Number(record.startAtUnixMillis ?? 0) > 0 || Number(record.endAtUnixMillis ?? 0) > 0
}

function dailyRangeText(record: Activity) {
  if (!record.dailyStart && !record.dailyEnd) return '-'
  return `${record.dailyStart || '-'} ~ ${record.dailyEnd || '-'}`
}

async function load() {
  loading.value = true
  try {
    const result = await activityClient().List({
      keyword: keyword.value.trim(),
      page: page.value,
      pageSize: pageSize.value,
    })
    records.value = result.items ?? []
    total.value = Number(result.total ?? 0)
  } catch {
    records.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function showTotal(value: number) {
  return `共 ${value} 条`
}

function queryList() {
  page.value = 1
  load()
}

function openCreate() {
  editingRecord.value = null
  editorKey.value += 1
  editorOpen.value = true
}

function openEdit(record: Activity) {
  editingRecord.value = record
  editorKey.value += 1
  editorOpen.value = true
}

function onSaved() {
  editorOpen.value = false
  load()
}

function openCoupons(record: Activity) {
  router.push({ path: '/promotion', query: { activityId: String(record.id ?? '') } })
}

onMounted(load)
</script>

<style scoped lang="scss">
.activity-page {
  padding: 20px 24px 28px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}
.activity-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}
.toolbar-label {
  font-size: 14px;
  color: #303133;
  white-space: nowrap;
}
.toolbar-input {
  width: 220px;
}
.btn-query {
  background: #52c41a !important;
  border-color: #52c41a !important;
}
.btn-query:hover {
  background: #73d13d !important;
  border-color: #73d13d !important;
}
.activity-table :deep(.ant-table-thead > tr > th) {
  background: #fafafa;
  color: #303133;
  font-weight: 600;
}
.activity-table :deep(.ant-table-tbody > tr > td) {
  padding-top: 14px;
  padding-bottom: 14px;
}
.ellipsis-text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.status-text {
  font-weight: 500;
}
.status-on {
  color: #52c41a;
}
.status-off {
  color: #8c8c8c;
}
.time-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
  line-height: 1.5;
}
.time-sep {
  color: #bfbfbf;
  font-size: 12px;
}
.activity-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
}
</style>

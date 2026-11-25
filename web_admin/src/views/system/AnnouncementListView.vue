<template>
  <div class="announcement-page">
    <div class="announcement-toolbar">
      <a-button class="btn-query" type="primary" :loading="loading" @click="queryList">查询</a-button>
      <a-button v-if="canWrite" @click="openCreate">创建</a-button>
    </div>

    <a-table
      :data-source="records"
      :loading="loading"
      :pagination="false"
      :row-selection="rowSelection"
      :row-key="(record: Announcement) => String(record.id)"
      :scroll="{ x: 1300 }"
      size="middle"
      class="announcement-table"
    >
      <a-table-column title="ID" width="70" align="center">
        <template #default="{ record }">{{ record.id }}</template>
      </a-table-column>
      <a-table-column title="公告标题" min-width="260" align="center">
        <template #default="{ record }">{{ record.title || '-' }}</template>
      </a-table-column>
      <a-table-column title="公告类型" width="110" align="center">
        <template #default="{ record }">{{ record.type ?? 0 }}</template>
      </a-table-column>
      <a-table-column title="发布时间" width="180" align="center">
        <template #default="{ record }">{{ formatTime(record.publishAtUnixMillis) }}</template>
      </a-table-column>
      <a-table-column title="是否展示" width="100" align="center">
        <template #default="{ record }">
          <a-tag :color="record.visible ? 'green' : 'default'">{{ record.visible ? '是' : '否' }}</a-tag>
        </template>
      </a-table-column>
      <a-table-column title="创建时间" width="180" align="center">
        <template #default="{ record }">{{ formatTime(record.createdAtUnixMillis) }}</template>
      </a-table-column>
      <a-table-column title="跳转类型" width="110" align="center">
        <template #default="{ record }">{{ jumpTypeText(record.jumpType) }}</template>
      </a-table-column>
      <a-table-column title="跳转id" width="150" align="center">
        <template #default="{ record }">{{ record.jumpTarget || '-' }}</template>
      </a-table-column>
      <a-table-column title="操作" width="110" align="center" fixed="right">
        <template #default="{ record }">
          <a-button v-if="canWrite" size="small" type="primary" @click="openEdit(record)">编辑</a-button>
          <span v-else>-</span>
        </template>
      </a-table-column>
    </a-table>

    <div class="announcement-footer">
      <a-pagination
        v-model:current="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-size-options="['15', '30', '50']"
        :show-total="showTotal"
        show-size-changer
        show-less-items
        @change="load"
      />
    </div>

    <a-modal
      v-model:open="editorOpen"
      title="公告"
      :footer="null"
      :width="1100"
      :style="{ maxWidth: '94vw' }"
      :body-style="{ maxHeight: 'calc(100vh - 160px)', overflowY: 'auto' }"
      :mask-closable="false"
      centered
      destroy-on-close
    >
      <AnnouncementForm :key="editorKey" :announcement="editingRecord" @success="onSaved" @cancel="editorOpen = false" />
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import dayjs from 'dayjs'
import AnnouncementForm from '@/views/system/AnnouncementForm.vue'
import { announcementClient } from '@/api/announcement'
import type { Announcement } from '@/api/announcement'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const canWrite = computed(() => allows(session.permissionNodes, 'system.announcement', 'write'))

const loading = ref(false)
const records = ref<Announcement[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(15)
const selectedRowKeys = ref<(string | number)[]>([])
const editorOpen = ref(false)
const editorKey = ref(0)
const editingRecord = ref<Announcement | null>(null)

const jumpTypeLabels: Record<string, string> = {
  none: '无',
  item: '商品',
  activity: '活动',
  url: '链接',
  page: '页面',
}

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

function jumpTypeText(value: string | undefined) {
  if (!value) return '-'
  return jumpTypeLabels[value] ?? value
}

function showTotal(value: number) {
  return `共 ${value} 条`
}

async function load() {
  loading.value = true
  try {
    const result = await announcementClient().List({ page: page.value, pageSize: pageSize.value })
    records.value = result.items ?? []
    total.value = Number(result.total ?? 0)
  } catch {
    records.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
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

function openEdit(record: Announcement) {
  editingRecord.value = record
  editorKey.value += 1
  editorOpen.value = true
}

function onSaved() {
  editorOpen.value = false
  load()
}

onMounted(load)
</script>

<style scoped lang="scss">
.announcement-page {
  padding: 20px 24px 28px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}
.announcement-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}
.btn-query {
  background: #52c41a !important;
  border-color: #52c41a !important;
}
.btn-query:hover {
  background: #73d13d !important;
  border-color: #73d13d !important;
}
.announcement-table :deep(.ant-table-thead > tr > th) {
  background: #fafafa;
  color: #303133;
  font-weight: 600;
}
.announcement-table :deep(.ant-table-tbody > tr > td) {
  padding-top: 14px;
  padding-bottom: 14px;
}
.announcement-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
}
</style>

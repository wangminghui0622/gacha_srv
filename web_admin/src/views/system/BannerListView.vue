<template>
  <div class="banner-page console-list-page">
    <div class="banner-toolbar">
      <a-button class="btn-query" type="primary" :loading="loading" @click="queryList">查询</a-button>
      <a-button v-if="canWrite" @click="openCreate">创建</a-button>
    </div>

    <a-table
      :data-source="records"
      :loading="loading"
      :pagination="false"
      :row-selection="rowSelection"
      :row-key="(record: Banner) => String(record.id)"
      :scroll="{ x: 1500 }"
      size="middle"
      class="banner-table"
    >
      <a-table-column title="位置" width="80" align="center">
        <template #default="{ record }">{{ record.position ?? 0 }}</template>
      </a-table-column>
      <a-table-column title="类型" width="180" align="center">
        <template #default="{ record }">{{ record.type || '-' }}</template>
      </a-table-column>
      <a-table-column title="Banner名称" min-width="180" align="center">
        <template #default="{ record }">{{ record.name || '-' }}</template>
      </a-table-column>
      <a-table-column title="图片" width="180" align="center">
        <template #default="{ record }">
          <a-image v-if="record.imageUrl" :src="record.imageUrl" :width="80" :height="45" class="banner-thumb" />
          <span v-else>-</span>
        </template>
      </a-table-column>
      <a-table-column title="是否展示" width="100" align="center">
        <template #default="{ record }">
          <a-tag :color="record.visible ? 'green' : 'default'">{{ record.visible ? '展示' : '不展示' }}</a-tag>
        </template>
      </a-table-column>
      <a-table-column title="目标赏池ID" width="120" align="center">
        <template #default="{ record }">{{ record.targetPoolId ?? 0 }}</template>
      </a-table-column>
      <a-table-column title="弹窗类型" width="120" align="center">
        <template #default="{ record }">{{ record.popupType || '-' }}</template>
      </a-table-column>
      <a-table-column title="弹窗目标类型" width="130" align="center">
        <template #default="{ record }">{{ record.popupTargetType || '-' }}</template>
      </a-table-column>
      <a-table-column title="弹窗目标" min-width="150" align="center">
        <template #default="{ record }">{{ record.popupTargetId || '-' }}</template>
      </a-table-column>
      <a-table-column title="操作" width="110" align="center" fixed="right">
        <template #default="{ record }">
          <a-button v-if="canWrite" class="btn-edit" size="small" @click="openEdit(record)">编辑</a-button>
          <span v-else>-</span>
        </template>
      </a-table-column>
    </a-table>

    <div class="banner-footer console-list-footer">
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
      title="Banner"
      :footer="null"
      :width="1100"
      :style="{ maxWidth: '94vw' }"
      :body-style="{ maxHeight: 'calc(100vh - 160px)', overflowY: 'auto' }"
      :mask-closable="false"
      centered
      destroy-on-close
    >
      <BannerForm :key="editorKey" :banner="editingRecord" @success="onSaved" @cancel="editorOpen = false" />
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import BannerForm from '@/views/system/BannerForm.vue'
import { bannerClient } from '@/api/banner'
import type { Banner } from '@/api/banner'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const canWrite = computed(() => allows(session.permissionNodes, 'system.banner', 'write'))

const loading = ref(false)
const records = ref<Banner[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(15)
const selectedRowKeys = ref<(string | number)[]>([])
const editorOpen = ref(false)
const editorKey = ref(0)
const editingRecord = ref<Banner | null>(null)

const rowSelection = computed(() => ({
  selectedRowKeys: selectedRowKeys.value,
  onChange: (keys: (string | number)[]) => {
    selectedRowKeys.value = keys
  },
}))

function showTotal(value: number) {
  return `共 ${value} 条`
}

async function load() {
  loading.value = true
  try {
    const result = await bannerClient().List({ page: page.value, pageSize: pageSize.value })
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

function openEdit(record: Banner) {
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
.banner-page {
  padding: 20px 24px 28px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}
.banner-toolbar {
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
.btn-edit {
  background: #fa8c16 !important;
  border-color: #fa8c16 !important;
  color: #fff !important;
}
.btn-edit:hover {
  background: #ffa940 !important;
  border-color: #ffa940 !important;
  color: #fff !important;
}
.banner-table :deep(.ant-table-thead > tr > th) {
  background: #fafafa;
  color: #303133;
  font-weight: 600;
}
.banner-table :deep(.ant-table-tbody > tr > td) {
  padding-top: 12px;
  padding-bottom: 12px;
}
.banner-thumb {
  object-fit: cover;
  border-radius: 4px;
}
.banner-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
}
</style>

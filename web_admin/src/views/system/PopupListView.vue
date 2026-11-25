<template>
  <div class="popup-page">
    <div class="popup-toolbar">
      <a-button :loading="loading" @click="queryList">查询</a-button>
      <a-button v-if="canWrite" type="primary" @click="openCreate">创建</a-button>
    </div>

    <a-table
      :data-source="records"
      :loading="loading"
      :pagination="false"
      :row-selection="rowSelection"
      :row-key="(record: Popup) => String(record.id)"
      :scroll="{ x: 1400 }"
      size="middle"
      class="popup-table"
    >
      <a-table-column title="id" width="70" align="center">
        <template #default="{ record }">{{ record.id }}</template>
      </a-table-column>
      <a-table-column title="主动展示" width="100" align="center">
        <template #default="{ record }">{{ record.activeDisplay ? '是' : '否' }}</template>
      </a-table-column>
      <a-table-column title="状态" width="100" align="center">
        <template #default="{ record }">{{ record.status || '-' }}</template>
      </a-table-column>
      <a-table-column title="序号" width="80" align="center">
        <template #default="{ record }">{{ record.sortOrder ?? 0 }}</template>
      </a-table-column>
      <a-table-column title="名称" min-width="160" align="center">
        <template #default="{ record }">{{ record.name || '-' }}</template>
      </a-table-column>
      <a-table-column title="类型" width="180" align="center">
        <template #default="{ record }">{{ record.type || '-' }}</template>
      </a-table-column>
      <a-table-column title="条件" width="130" align="center">
        <template #default="{ record }">{{ record.condition || '-' }}</template>
      </a-table-column>
      <a-table-column title="跳转类型" width="120" align="center">
        <template #default="{ record }">{{ record.jumpType || '-' }}</template>
      </a-table-column>
      <a-table-column title="跳转目标" min-width="180" align="center">
        <template #default="{ record }">{{ record.jumpTarget || '-' }}</template>
      </a-table-column>
      <a-table-column title="弹窗图片" width="120" align="center">
        <template #default="{ record }">
          <a-image v-if="record.imageUrl" :src="record.imageUrl" :width="70" :height="70" class="popup-thumb" />
          <span v-else>-</span>
        </template>
      </a-table-column>
      <a-table-column title="操作" width="110" align="center" fixed="right">
        <template #default="{ record }">
          <a-button v-if="canWrite" class="btn-edit" size="small" @click="openEdit(record)">编辑</a-button>
          <span v-else>-</span>
        </template>
      </a-table-column>
    </a-table>

    <div class="popup-footer">
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
      :title="editingRecord ? '编辑弹窗' : '创建弹窗'"
      :footer="null"
      :width="1100"
      :style="{ maxWidth: '94vw' }"
      :body-style="{ maxHeight: 'calc(100vh - 160px)', overflowY: 'auto' }"
      :mask-closable="false"
      centered
      destroy-on-close
    >
      <PopupForm :key="editorKey" :popup="editingRecord" @success="onSaved" @cancel="editorOpen = false" />
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import PopupForm from '@/views/system/PopupForm.vue'
import { popupClient } from '@/api/popup'
import type { Popup } from '@/api/popup'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const canWrite = computed(() => allows(session.permissionNodes, 'system.popup', 'write'))

const loading = ref(false)
const records = ref<Popup[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(15)
const selectedRowKeys = ref<(string | number)[]>([])
const editorOpen = ref(false)
const editorKey = ref(0)
const editingRecord = ref<Popup | null>(null)

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
    const result = await popupClient().List({ page: page.value, pageSize: pageSize.value })
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

function openEdit(record: Popup) {
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
.popup-page {
  padding: 20px 24px 28px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}
.popup-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
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
.popup-table :deep(.ant-table-thead > tr > th) {
  background: #fafafa;
  color: #303133;
  font-weight: 600;
}
.popup-table :deep(.ant-table-tbody > tr > td) {
  padding-top: 12px;
  padding-bottom: 12px;
}
.popup-thumb {
  object-fit: cover;
  border-radius: 4px;
}
.popup-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
}
</style>

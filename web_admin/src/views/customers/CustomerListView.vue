<template>
  <div class="customer-page console-list-page">
    <div class="customer-toolbar">
      <a-input v-model:value="keyword" class="filter-input keyword-input" placeholder="昵称/数字ID/字符串ID" allow-clear @press-enter="queryList" />
      <a-input v-model:value="phone" class="filter-input phone-input" placeholder="手机号" allow-clear @press-enter="queryList" />
      <a-select v-model:value="identity" class="filter-select" :options="identityOptions" />
      <a-select v-model:value="status" class="filter-select" :options="statusOptions" />
      <a-button class="btn-query" type="primary" :loading="loading" @click="queryList">查询</a-button>
      <a-button v-if="canWrite" @click="openCreate">创建</a-button>
    </div>

    <a-table
      :data-source="records"
      :loading="loading"
      :pagination="false"
      :row-selection="rowSelection"
      :row-key="(record: ConsoleUser) => String(record.userId)"
      :scroll="{ x: 1600 }"
      size="middle"
      class="customer-table"
    >
      <a-table-column title="数字ID" width="110" align="center">
        <template #default="{ record }">
          <span class="user-id-num">{{ record.userId }}</span>
        </template>
      </a-table-column>
      <a-table-column title="字符串ID" width="140" align="center">
        <template #default="{ record }">{{ record.publicUserId || '-' }}</template>
      </a-table-column>
      <a-table-column title="昵称" min-width="150" align="center">
        <template #default="{ record }">{{ record.nickname || '-' }}</template>
      </a-table-column>
      <a-table-column title="手机号码" width="140" align="center">
        <template #default="{ record }">{{ record.phone || '-' }}</template>
      </a-table-column>
      <a-table-column title="状态" width="90" align="center">
        <template #default="{ record }">
          <span :class="['status-text', record.status === 2 ? 'status-off' : 'status-on']">
            {{ record.status === 2 ? '禁用' : '正常' }}
          </span>
        </template>
      </a-table-column>
      <a-table-column title="吉祥展示" width="110" align="center">
        <template #default="{ record }">{{ record.luckyDisplay || '初始化' }}</template>
      </a-table-column>
      <a-table-column title="注册IP" width="150" align="center">
        <template #default="{ record }">{{ record.registerIp || '-' }}</template>
      </a-table-column>
      <a-table-column title="注册设备码" min-width="260" align="center">
        <template #default="{ record }">{{ record.registerDevice || '-' }}</template>
      </a-table-column>
      <a-table-column title="头像" width="90" align="center">
        <template #default="{ record }">
          <a-avatar v-if="record.avatarUrl" :src="record.avatarUrl" :size="44" />
          <a-avatar v-else :size="44">
            <template #icon><UserOutlined /></template>
          </a-avatar>
        </template>
      </a-table-column>
      <a-table-column title="注册时间" width="180" align="center">
        <template #default="{ record }">{{ formatTime(record.createdAtUnixMillis) }}</template>
      </a-table-column>
      <a-table-column title="操作" width="120" align="center" fixed="right">
        <template #default="{ record }">
          <a-button type="primary" size="small" @click="manageUser(record)">
            <UserOutlined />
            管理
          </a-button>
        </template>
      </a-table-column>
    </a-table>

    <div class="customer-footer console-list-footer">
      <a-pagination
        v-model:current="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-size-options="['10', '15', '30', '50']"
        :show-total="showTotal"
        show-size-changer
        show-less-items
        @change="load"
      />
    </div>

    <a-modal
      v-model:open="editorOpen"
      title="用户"
      :footer="null"
      :width="760"
      :style="{ maxWidth: '92vw' }"
      :body-style="{ maxHeight: 'calc(100vh - 160px)', overflowY: 'auto' }"
      :mask-closable="false"
      centered
      destroy-on-close
    >
      <CustomerForm :key="editorKey" @success="onSaved" @cancel="editorOpen = false" />
    </a-modal>

    <a-modal
      v-model:open="manageOpen"
      title="用户管理"
      :footer="null"
      :width="760"
      :style="{ maxWidth: '92vw' }"
      :body-style="{ maxHeight: 'calc(100vh - 160px)', overflowY: 'auto' }"
      :mask-closable="false"
      centered
      destroy-on-close
    >
      <CustomerManageModal
        v-if="manageUserRecord"
        :key="manageKey"
        :user="manageUserRecord"
        @success="onManaged"
      />
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import dayjs from 'dayjs'
import { UserOutlined } from '@ant-design/icons-vue'
import CustomerForm from '@/views/customers/CustomerForm.vue'
import CustomerManageModal from '@/views/customers/CustomerManageModal.vue'
import { customerClient } from '@/api/customer'
import type { ConsoleUser } from '@/api/customer'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const canWrite = computed(() => allows(session.permissionNodes, 'identity.user', 'write'))

const loading = ref(false)
const records = ref<ConsoleUser[]>([])
const total = ref(0)
const keyword = ref('')
const phone = ref('')
const identity = ref('用户')
const status = ref(1)
const page = ref(1)
const pageSize = ref(15)
const selectedRowKeys = ref<(string | number)[]>([])
const editorOpen = ref(false)
const editorKey = ref(0)
const manageOpen = ref(false)
const manageKey = ref(0)
const manageUserRecord = ref<ConsoleUser | null>(null)

const identityOptions = [
  { value: '', label: '全部身份' },
  { value: '用户', label: '用户' },
  { value: '代理', label: '代理' },
  { value: '达人', label: '达人' },
]
const statusOptions = [
  { value: 1, label: '正常' },
  { value: 2, label: '禁用' },
  { value: 0, label: '全部状态' },
]

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

function showTotal(value: number) {
  return `共 ${value} 条`
}

async function load() {
  loading.value = true
  try {
    const result = await customerClient().List({
      keyword: keyword.value.trim(),
      phone: phone.value.trim(),
      identity: identity.value,
      status: status.value,
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

function queryList() {
  page.value = 1
  load()
}

function openCreate() {
  editorKey.value += 1
  editorOpen.value = true
}

function onSaved() {
  editorOpen.value = false
  load()
}

function manageUser(record: ConsoleUser) {
  if (!record.userId) return
  manageUserRecord.value = record
  manageKey.value += 1
  manageOpen.value = true
}

function onManaged() {
  manageOpen.value = false
  load()
}

onMounted(load)
</script>

<style scoped lang="scss">
.customer-page {
  padding: 20px 24px 28px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}
.customer-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 16px;
}
.filter-input,
.filter-select {
  width: 160px;
}
.keyword-input {
  width: 180px;
}
.btn-query {
  background: #52c41a !important;
  border-color: #52c41a !important;
}
.btn-query:hover {
  background: #73d13d !important;
  border-color: #73d13d !important;
}
.customer-table :deep(.ant-table-thead > tr > th) {
  background: #fafafa;
  color: #303133;
  font-weight: 600;
}
.customer-table :deep(.ant-table-tbody > tr > td) {
  padding-top: 14px;
  padding-bottom: 14px;
}
.user-id-num {
  color: #303133;
  font-weight: 600;
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
.customer-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
}
</style>

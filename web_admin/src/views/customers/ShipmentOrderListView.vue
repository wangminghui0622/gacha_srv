<template>
  <div class="shipment-page console-list-page">
    <div class="shipment-toolbar">
      <a-input
        v-model:value="filters.keyword"
        class="filter-keyword"
        placeholder="UserID或订单ID"
        allow-clear
        @press-enter="search"
      />
      <a-select
        v-model:value="filters.status"
        class="filter-status"
        placeholder="订单状态"
        :options="statusOptions"
        allow-clear
      />
      <a-button class="btn-query" type="primary" :loading="loading" @click="search">查 询</a-button>
      <a-button class="btn-plain" :loading="loading" @click="exportCsv">导 出</a-button>
      <a-button class="btn-plain" :loading="loading" @click="load">刷新</a-button>
    </div>

    <a-table
      :data-source="items"
      :loading="loading"
      :pagination="false"
      :row-selection="rowSelection"
      :row-key="(record: ShipmentOrder) => String(record.id ?? '')"
      :scroll="{ x: 1650 }"
      size="middle"
      class="shipment-table"
    >
      <a-table-column title="ID" width="210" align="center">
        <template #default="{ record }">{{ record.orderNo || '-' }}</template>
      </a-table-column>
      <a-table-column title="用户ID" width="90" align="center">
        <template #default="{ record }">{{ record.userId }}</template>
      </a-table-column>
      <a-table-column title="状态" width="110" align="center">
        <template #default="{ record }">
          <a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag>
        </template>
      </a-table-column>
      <a-table-column title="创建时间" width="180" align="center">
        <template #default="{ record }">{{ formatTime(record.createdAtUnixMillis) }}</template>
      </a-table-column>
      <a-table-column title="支付时间" width="180" align="center">
        <template #default="{ record }">{{ formatTime(record.paidAtUnixMillis) }}</template>
      </a-table-column>
      <a-table-column title="已提交天数" width="110" align="center">
        <template #default="{ record }">{{ submittedDaysText(record) }}</template>
      </a-table-column>
      <a-table-column title="发货时间" width="180" align="center">
        <template #default="{ record }">{{ formatTime(record.shippedAtUnixMillis) }}</template>
      </a-table-column>
      <a-table-column title="完成时间" width="180" align="center">
        <template #default="{ record }">{{ formatTime(record.completedAtUnixMillis) }}</template>
      </a-table-column>
      <a-table-column title="备注" min-width="160" align="center">
        <template #default="{ record }">{{ record.remark || '-' }}</template>
      </a-table-column>
      <a-table-column title="操作" width="170" align="center" fixed="right">
        <template #default="{ record }">
          <a-space v-if="canWrite" :size="6">
            <a-button class="btn-edit" size="small" @click="openEdit(record)">编辑</a-button>
            <a-button
              v-if="canClose(record)"
              class="btn-close"
              size="small"
              danger
              :loading="closingId === String(record.id)"
              @click="closeOrder(record)"
            >
              关闭
            </a-button>
          </a-space>
          <span v-else>-</span>
        </template>
      </a-table-column>

      <template #expandedRowRender="{ record }">
        <div class="shipment-detail">
          <div class="detail-row">
            <span>收件人：{{ record.receiverName || '-' }}</span>
            <span>手机号：{{ record.receiverPhone || '-' }}</span>
            <span>物流：{{ record.logisticsCompany || '-' }} {{ record.trackingNo || '' }}</span>
          </div>
          <div class="detail-row">
            <span>收货地址：{{ record.receiverAddress || '-' }}</span>
          </div>
          <a-table
            :data-source="record.items ?? []"
            :pagination="false"
            :row-key="(item: ShipmentOrderItem) => String(item.id ?? '')"
            size="small"
            :scroll="{ x: 760 }"
            class="item-table"
          >
            <a-table-column title="图片" width="90" align="center">
              <template #default="{ record: item }">
                <a-image
                  v-if="item.imageUrl"
                  :src="item.imageUrl"
                  :width="44"
                  :height="44"
                  :preview="false"
                />
                <span v-else>-</span>
              </template>
            </a-table-column>
            <a-table-column title="物品名称" min-width="260" align="center">
              <template #default="{ record: item }">{{ item.itemName || '-' }}</template>
            </a-table-column>
            <a-table-column title="来源宝箱ID" width="130" align="center">
              <template #default="{ record: item }">{{ item.boxId || '-' }}</template>
            </a-table-column>
            <a-table-column title="数量" width="80" align="center">
              <template #default="{ record: item }">{{ item.quantity ?? 1 }}</template>
            </a-table-column>
            <a-table-column title="入仓时间" width="180" align="center">
              <template #default="{ record: item }">{{ formatTime(item.createdAtUnixMillis) }}</template>
            </a-table-column>
          </a-table>
        </div>
      </template>
    </a-table>

    <div class="shipment-footer console-list-footer">
      <a-pagination
        :current="page"
        :page-size="pageSize"
        :total="total"
        :page-size-options="pageSizeOptions"
        :locale="{ items_per_page: '条/页' }"
        show-size-changer
        show-less-items
        @change="onPageChange"
        @show-size-change="onPageSizeChange"
      />
    </div>

    <a-modal
      v-if="editOpen"
      v-model:open="editOpen"
      title="编辑发货订单"
      centered
      :width="680"
      :footer="null"
      :mask-closable="false"
      :destroy-on-close="true"
      :body-style="{ maxHeight: 'calc(100vh - 160px)', overflowY: 'auto' }"
    >
      <a-form :label-col="{ span: 6 }" :wrapper-col="{ span: 17 }">
        <a-form-item label="订单ID">
          <a-input :value="editing?.orderNo" disabled />
        </a-form-item>
        <a-form-item label="用户ID">
          <a-input :value="editing?.userId" disabled />
        </a-form-item>
        <a-form-item label="状态" required>
          <a-select v-model:value="editForm.status" :options="statusOptions" />
        </a-form-item>
        <a-form-item label="收件人">
          <a-input v-model:value="editForm.receiverName" placeholder="收件人姓名" />
        </a-form-item>
        <a-form-item label="手机号">
          <a-input v-model:value="editForm.receiverPhone" placeholder="收件人手机号" />
        </a-form-item>
        <a-form-item label="收货地址">
          <a-textarea v-model:value="editForm.receiverAddress" :rows="2" placeholder="收货地址" />
        </a-form-item>
        <a-form-item label="物流公司">
          <a-input v-model:value="editForm.logisticsCompany" placeholder="物流公司" />
        </a-form-item>
        <a-form-item label="物流单号">
          <a-input v-model:value="editForm.trackingNo" placeholder="物流单号" />
        </a-form-item>
        <a-form-item label="发货时间">
          <a-date-picker v-model:value="editForm.shippedAt" show-time style="width: 100%" placeholder="发货时间" />
        </a-form-item>
        <a-form-item label="完成时间">
          <a-date-picker v-model:value="editForm.completedAt" show-time style="width: 100%" placeholder="完成时间" />
        </a-form-item>
        <a-form-item label="备注">
          <a-textarea v-model:value="editForm.remark" :rows="3" placeholder="备注" />
        </a-form-item>
      </a-form>
      <div class="edit-actions">
        <a-button type="primary" :loading="saving" @click="saveEdit">保 存</a-button>
        <a-button @click="editOpen = false">取 消</a-button>
      </div>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message, Modal } from 'ant-design-vue'
import dayjs, { type Dayjs } from 'dayjs'

import { allows } from '@/access/grants'
import { shipmentClient, type ShipmentOrder, type ShipmentOrderItem, type UpdateShipmentOrderPayload } from '@/api/shipment'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()

const canRead = computed(() => allows(session.permissionNodes, 'identity.user', 'read'))
const canWrite = computed(() => allows(session.permissionNodes, 'identity.user', 'write'))

const filters = reactive({
  keyword: '',
  status: undefined as number | undefined,
})

const statusOptions = [
  { label: '待付款', value: 1 },
  { label: '待支付尾款', value: 2 },
  { label: '待发货', value: 3 },
  { label: '待收货', value: 4 },
  { label: '已完成', value: 5 },
  { label: '已关闭', value: 6 },
  { label: '用户关闭', value: 7 },
]

const items = ref<ShipmentOrder[]>([])
const page = ref(1)
const pageSize = ref(15)
const pageSizeOptions = ['15', '30', '50', '100']
const total = ref(0)
const loading = ref(false)
const selectedRowKeys = ref<string[]>([])

const rowSelection = computed(() => ({
  selectedRowKeys: selectedRowKeys.value,
  onChange: (keys: (string | number)[]) => {
    selectedRowKeys.value = keys.map((key) => String(key))
  },
}))

const editOpen = ref(false)
const editing = ref<ShipmentOrder | null>(null)
const saving = ref(false)
const closingId = ref('')
const editForm = reactive({
  status: 3,
  receiverName: '',
  receiverPhone: '',
  receiverAddress: '',
  logisticsCompany: '',
  trackingNo: '',
  shippedAt: null as Dayjs | null,
  completedAt: null as Dayjs | null,
  remark: '',
})

async function load() {
  if (!canRead.value) return
  loading.value = true
  try {
    const reply = await shipmentClient().List({
      keyword: filters.keyword.trim(),
      status: filters.status ?? 0,
      page: page.value,
      pageSize: pageSize.value,
    })
    items.value = reply.items ?? []
    total.value = Number(reply.total ?? 0)
    selectedRowKeys.value = []
  } catch {
    items.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  void load()
}

function onPageChange(nextPage: number) {
  page.value = nextPage
  void load()
}

function onPageSizeChange(_current: number, nextPageSize: number) {
  pageSize.value = nextPageSize
  page.value = 1
  void load()
}

function statusCode(status?: string): number {
  switch (status) {
    case 'SHIPMENT_ORDER_STATUS_PENDING_PAYMENT': return 1
    case 'SHIPMENT_ORDER_STATUS_PENDING_FINAL': return 2
    case 'SHIPMENT_ORDER_STATUS_PENDING_SHIPMENT': return 3
    case 'SHIPMENT_ORDER_STATUS_PENDING_RECEIPT': return 4
    case 'SHIPMENT_ORDER_STATUS_COMPLETED': return 5
    case 'SHIPMENT_ORDER_STATUS_CLOSED': return 6
    case 'SHIPMENT_ORDER_STATUS_USER_CLOSED': return 7
    default: return 0
  }
}

function statusText(status?: string): string {
  return statusOptions.find((item) => item.value === statusCode(status))?.label ?? '-'
}

function statusColor(status?: string): string {
  switch (statusCode(status)) {
    case 1: return 'orange'
    case 2: return 'gold'
    case 3: return 'blue'
    case 4: return 'cyan'
    case 5: return 'green'
    case 6: return 'red'
    case 7: return 'default'
    default: return 'default'
  }
}

function submittedDaysText(record: ShipmentOrder): string {
  const code = statusCode(record.status)
  if (code !== 3 && code !== 4) return '--'
  return String(record.submittedDays ?? 0)
}

function canClose(record: ShipmentOrder): boolean {
  const code = statusCode(record.status)
  return code >= 1 && code <= 4
}

function formatTime(value?: string | number): string {
  const n = Number(value ?? 0)
  if (!n) return '-'
  return dayjs(n).format('YYYY-MM-DD HH:mm:ss')
}

function openEdit(record: ShipmentOrder) {
  editing.value = record
  editForm.status = statusCode(record.status) || 3
  editForm.receiverName = record.receiverName ?? ''
  editForm.receiverPhone = record.receiverPhone ?? ''
  editForm.receiverAddress = record.receiverAddress ?? ''
  editForm.logisticsCompany = record.logisticsCompany ?? ''
  editForm.trackingNo = record.trackingNo ?? ''
  editForm.shippedAt = Number(record.shippedAtUnixMillis ?? 0) ? dayjs(Number(record.shippedAtUnixMillis)) : null
  editForm.completedAt = Number(record.completedAtUnixMillis ?? 0) ? dayjs(Number(record.completedAtUnixMillis)) : null
  editForm.remark = record.remark ?? ''
  editOpen.value = true
}

async function saveEdit() {
  if (!editing.value?.id) return
  const payload: UpdateShipmentOrderPayload = {
    status: editForm.status,
    remark: editForm.remark.trim(),
    receiverName: editForm.receiverName.trim(),
    receiverPhone: editForm.receiverPhone.trim(),
    receiverAddress: editForm.receiverAddress.trim(),
    logisticsCompany: editForm.logisticsCompany.trim(),
    trackingNo: editForm.trackingNo.trim(),
    shippedAtUnixMillis: editForm.shippedAt ? editForm.shippedAt.valueOf() : 0,
    completedAtUnixMillis: editForm.completedAt ? editForm.completedAt.valueOf() : 0,
  }
  saving.value = true
  try {
    await shipmentClient().Update(editing.value.id, payload)
    message.success('发货订单已保存')
    editOpen.value = false
    void load()
  } catch {
    message.error('保存失败')
  } finally {
    saving.value = false
  }
}

function closeOrder(record: ShipmentOrder) {
  Modal.confirm({
    title: '关闭发货订单',
    content: `确认关闭订单「${record.orderNo || record.id}」？关闭后不能继续发货。`,
    okText: '关闭',
    cancelText: '取消',
    okType: 'danger',
    onOk: async () => {
      closingId.value = String(record.id)
      try {
        await shipmentClient().Close(String(record.id), 6)
        message.success('订单已关闭')
        void load()
      } finally {
        closingId.value = ''
      }
    },
  })
}

function csvEscape(value: unknown): string {
  const text = String(value ?? '')
  if (/[",\n]/.test(text)) return `"${text.replace(/"/g, '""')}"`
  return text
}

function exportCsv() {
  const source = selectedRowKeys.value.length
    ? items.value.filter((item) => selectedRowKeys.value.includes(String(item.id)))
    : items.value
  if (!source.length) {
    message.warning('暂无可导出的数据')
    return
  }
  const header = ['订单ID', '用户ID', '状态', '创建时间', '支付时间', '已提交天数', '发货时间', '完成时间', '备注']
  const rows = source.map((record) => [
    record.orderNo ?? '',
    record.userId ?? '',
    statusText(record.status),
    formatTime(record.createdAtUnixMillis),
    formatTime(record.paidAtUnixMillis),
    submittedDaysText(record),
    formatTime(record.shippedAtUnixMillis),
    formatTime(record.completedAtUnixMillis),
    record.remark ?? '',
  ])
  const csv = '\uFEFF' + [header, ...rows].map((row) => row.map(csvEscape).join(',')).join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `发货订单_${dayjs().format('YYYYMMDD_HHmmss')}.csv`
  link.click()
  URL.revokeObjectURL(url)
}

onMounted(load)
</script>

<style scoped lang="scss">
.shipment-page {
  padding: 20px 24px 32px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}

.shipment-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
}

.filter-keyword {
  width: 220px;
}

.filter-status {
  width: 150px;
}

.btn-query {
  background: #42b983;
  border-color: #42b983;
}

.btn-query:hover {
  background: #36a374 !important;
  border-color: #36a374 !important;
}

.btn-plain {
  background: #fff;
  border-color: #d9d9d9;
}

.shipment-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.shipment-detail {
  padding: 8px 16px 12px;
  background: #fafafa;
}

.detail-row {
  display: flex;
  flex-wrap: wrap;
  gap: 24px;
  margin-bottom: 8px;
  color: #606266;
  font-size: 13px;
}

.item-table {
  margin-top: 8px;
}

.btn-edit {
  color: #fff;
  background: #ff9f1c;
  border-color: #ff9f1c;
}

.btn-edit:hover {
  color: #fff !important;
  background: #f08c00 !important;
  border-color: #f08c00 !important;
}

.btn-close {
  background: #ff4d4f;
  border-color: #ff4d4f;
  color: #fff;
}

.edit-actions {
  display: flex;
  justify-content: center;
  gap: 10px;
  margin-top: 8px;
}
</style>

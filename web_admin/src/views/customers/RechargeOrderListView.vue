<template>
  <div class="order-page">
    <div class="order-toolbar">
      <a-input v-model:value="filters.keyword" class="w-180" placeholder="用户ID或订单ID" allow-clear @press-enter="search" />
      <a-select v-model:value="filters.status" class="w-140" placeholder="订单状态" :options="statusOptions" allow-clear />
      <a-select v-model:value="filters.rechargeType" class="w-140" placeholder="充值类型" :options="typeOptions" allow-clear />
      <a-select v-model:value="filters.platform" class="w-140" placeholder="充值平台" :options="platformOptions" allow-clear />
      <a-input v-model:value="filters.payKeyword" class="w-200" placeholder="支付平台订单ID/OpenID" allow-clear @press-enter="search" />
      <a-input-number v-model:value="filters.minAmountYuan" class="w-110" :min="0" :precision="2" placeholder="最小金额" />
      <a-input-number v-model:value="filters.maxAmountYuan" class="w-110" :min="0" :precision="2" placeholder="最大金额" />
      <a-select v-model:value="filters.timeField" class="w-130" :options="timeFieldOptions" />
      <a-range-picker v-model:value="filters.range" class="w-260" />
      <a-button class="btn-query" type="primary" :loading="loading" @click="search">查 询</a-button>
      <a-button class="btn-plain" :loading="loading" @click="exportCsv">导 出</a-button>
      <a-button
        v-if="canWrite"
        class="btn-refund"
        danger
        :loading="refunding"
        :disabled="!selectedRowKeys.length"
        @click="refundSelected"
      >
        选中订单直接退款
      </a-button>
    </div>

    <a-table
      :data-source="items"
      :loading="loading"
      :pagination="false"
      :row-selection="rowSelection"
      :row-key="(record: RechargeOrder) => String(record.dbId ?? '')"
      :scroll="{ x: 2600 }"
      size="middle"
    >
      <a-table-column title="ID" width="200" align="center">
        <template #default="{ record }">{{ record.orderNo || '-' }}</template>
      </a-table-column>
      <a-table-column title="用户ID" width="90" align="center">
        <template #default="{ record }">{{ record.userId }}</template>
      </a-table-column>
      <a-table-column title="用户昵称" width="130" align="center">
        <template #default="{ record }">{{ record.userNickname || '-' }}</template>
      </a-table-column>
      <a-table-column title="充值金额" width="100" align="center">
        <template #default="{ record }">{{ money(record.amountFen) }}</template>
      </a-table-column>
      <a-table-column title="到账金额" width="100" align="center">
        <template #default="{ record }">{{ money(record.creditedFen) }}</template>
      </a-table-column>
      <a-table-column title="折扣金额" width="100" align="center">
        <template #default="{ record }">{{ money(record.discountFen) }}</template>
      </a-table-column>
      <a-table-column title="充值平台" width="100" align="center">
        <template #default="{ record }">{{ record.platform || '-' }}</template>
      </a-table-column>
      <a-table-column title="充值状态" width="110" align="center">
        <template #default="{ record }">
          <a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag>
        </template>
      </a-table-column>
      <a-table-column title="充值类型" width="110" align="center">
        <template #default="{ record }">{{ record.rechargeType || '-' }}</template>
      </a-table-column>
      <a-table-column title="创建时间" width="180" align="center">
        <template #default="{ record }">{{ formatTime(record.createdAtUnixMillis) }}</template>
      </a-table-column>
      <a-table-column title="完成时间" width="180" align="center">
        <template #default="{ record }">{{ formatTime(record.completedAtUnixMillis) }}</template>
      </a-table-column>
      <a-table-column title="退款时间" width="180" align="center">
        <template #default="{ record }">{{ formatTime(record.refundedAtUnixMillis) }}</template>
      </a-table-column>
      <a-table-column title="退款金额" width="100" align="center">
        <template #default="{ record }">{{ money(record.refundAmountFen) }}</template>
      </a-table-column>
      <a-table-column title="退款订单ID" width="200" align="center">
        <template #default="{ record }">{{ record.refundOrderNo || '-' }}</template>
      </a-table-column>
      <a-table-column title="支付平台订单ID" width="260" align="center">
        <template #default="{ record }">{{ record.payPlatformOrderId || '-' }}</template>
      </a-table-column>
      <a-table-column title="支付平台OpenID" width="200" align="center">
        <template #default="{ record }">{{ record.payPlatformOpenid || '-' }}</template>
      </a-table-column>
      <a-table-column title="错误信息" min-width="220" align="center">
        <template #default="{ record }">{{ record.errorMessage || '-' }}</template>
      </a-table-column>
      <a-table-column title="操作" width="110" align="center" fixed="right">
        <template #default="{ record }">
          <a-button
            v-if="canWrite && canRefund(record)"
            class="btn-refund"
            danger
            size="small"
            @click="refundOne(record)"
          >
            退款
          </a-button>
          <span v-else>-</span>
        </template>
      </a-table-column>
    </a-table>

    <div class="order-footer">
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
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message, Modal } from 'ant-design-vue'
import dayjs, { type Dayjs } from 'dayjs'

import { allows } from '@/access/grants'
import { rechargeOrderClient, type RechargeOrder } from '@/api/recharge'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const canRead = computed(() => allows(session.permissionNodes, 'payment.order', 'read'))
const canWrite = computed(() => allows(session.permissionNodes, 'payment.order', 'write'))

const statusOptions = [
  { label: '初始化', value: 1 },
  { label: '异常', value: 2 },
  { label: '已取消', value: 3 },
  { label: '已完成', value: 4 },
  { label: '已关闭', value: 5 },
  { label: '失败', value: 6 },
  { label: '退款中', value: 7 },
  { label: '退款异常', value: 8 },
  { label: '退款完成', value: 9 },
  { label: '退款失败', value: 10 },
  { label: '被迫退款中', value: 11 },
  { label: '被迫退款异常', value: 12 },
  { label: '被迫退款完成', value: 13 },
  { label: '被迫退款失败', value: 14 },
]
const typeOptions = ['金币充值', '玩法', '对对碰', '串串赏', '挖宝赏', '拼团赏', '无限赏'].map((value) => ({ label: value, value }))
const platformOptions = ['支付宝', '汇付', '拉卡拉', '微信'].map((value) => ({ label: value, value }))
const timeFieldOptions = [
  { label: '创建时间', value: 'created' },
  { label: '完成时间', value: 'completed' },
  { label: '退款时间', value: 'refunded' },
]

const filters = reactive({
  keyword: '',
  status: undefined as number | undefined,
  rechargeType: undefined as string | undefined,
  platform: undefined as string | undefined,
  payKeyword: '',
  minAmountYuan: undefined as number | undefined,
  maxAmountYuan: undefined as number | undefined,
  timeField: 'created',
  range: null as [Dayjs, Dayjs] | null,
})

const items = ref<RechargeOrder[]>([])
const page = ref(1)
const pageSize = ref(15)
const pageSizeOptions = ['15', '30', '50', '100']
const total = ref(0)
const loading = ref(false)
const refunding = ref(false)
const selectedRowKeys = ref<string[]>([])

const rowSelection = computed(() => ({
  selectedRowKeys: selectedRowKeys.value,
  onChange: (keys: (string | number)[]) => {
    selectedRowKeys.value = keys.map((key) => String(key))
  },
}))

async function load() {
  if (!canRead.value) return
  loading.value = true
  try {
    const reply = await rechargeOrderClient().List({
      keyword: filters.keyword.trim(),
      status: filters.status ?? 0,
      rechargeType: filters.rechargeType ?? '',
      platform: filters.platform ?? '',
      payKeyword: filters.payKeyword.trim(),
      minAmountFen: filters.minAmountYuan ? Math.round(filters.minAmountYuan * 100) : 0,
      maxAmountFen: filters.maxAmountYuan ? Math.round(filters.maxAmountYuan * 100) : 0,
      timeField: filters.timeField,
      startAtUnixMillis: filters.range ? filters.range[0].startOf('day').valueOf() : 0,
      endAtUnixMillis: filters.range ? filters.range[1].endOf('day').valueOf() : 0,
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
    case 'RECHARGE_ORDER_STATUS_INIT': return 1
    case 'RECHARGE_ORDER_STATUS_EXCEPTION': return 2
    case 'RECHARGE_ORDER_STATUS_CANCELED': return 3
    case 'RECHARGE_ORDER_STATUS_COMPLETED': return 4
    case 'RECHARGE_ORDER_STATUS_CLOSED': return 5
    case 'RECHARGE_ORDER_STATUS_FAILED': return 6
    case 'RECHARGE_ORDER_STATUS_REFUNDING': return 7
    case 'RECHARGE_ORDER_STATUS_REFUND_EXCEPTION': return 8
    case 'RECHARGE_ORDER_STATUS_REFUNDED': return 9
    case 'RECHARGE_ORDER_STATUS_REFUND_FAILED': return 10
    case 'RECHARGE_ORDER_STATUS_FORCE_REFUNDING': return 11
    case 'RECHARGE_ORDER_STATUS_FORCE_REFUND_EXCEPTION': return 12
    case 'RECHARGE_ORDER_STATUS_FORCE_REFUNDED': return 13
    case 'RECHARGE_ORDER_STATUS_FORCE_REFUND_FAILED': return 14
    default: return 0
  }
}

function statusText(status?: string): string {
  return statusOptions.find((item) => item.value === statusCode(status))?.label ?? '-'
}

function statusColor(status?: string): string {
  switch (statusCode(status)) {
    case 1: return 'default'
    case 2: return 'red'
    case 3: return 'default'
    case 4: return 'green'
    case 5: return 'default'
    case 6: return 'red'
    case 7: return 'orange'
    case 8: return 'red'
    case 9: return 'purple'
    case 10: return 'red'
    case 11: return 'orange'
    case 12: return 'red'
    case 13: return 'purple'
    case 14: return 'red'
    default: return 'default'
  }
}

function money(value?: string | number): string {
  return `¥${(Number(value ?? 0) / 100).toFixed(2)}`
}

function formatTime(value?: string | number): string {
  const n = Number(value ?? 0)
  if (!n) return '--'
  return dayjs(n).format('YYYY-MM-DD HH:mm:ss')
}

function canRefund(record: RechargeOrder): boolean {
  const code = statusCode(record.status)
  return code === 4 || code === 10 || code === 14
}

function refundOne(record: RechargeOrder) {
  if (!record.dbId) return
  Modal.confirm({
    title: '充值订单退款',
    content: `确认退款订单「${record.orderNo}」？退款金额 ${money(record.amountFen)}。`,
    okText: '退款',
    cancelText: '取消',
    okType: 'danger',
    onOk: async () => {
      const reply = await rechargeOrderClient().Refund([record.dbId as string])
      if (reply.failedOrderNos?.length) {
        message.warning(`退款失败：${reply.failedOrderNos.join('、')}`)
      } else {
        message.success('退款完成')
      }
      void load()
    },
  })
}

function refundSelected() {
  if (!selectedRowKeys.value.length) return
  Modal.confirm({
    title: '批量退款',
    content: `确认对已选中的 ${selectedRowKeys.value.length} 笔充值订单执行退款？`,
    okText: '退款',
    cancelText: '取消',
    okType: 'danger',
    onOk: async () => {
      refunding.value = true
      try {
        const reply = await rechargeOrderClient().Refund(selectedRowKeys.value)
        if (reply.failedOrderNos?.length) {
          message.warning(`部分订单退款失败：${reply.failedOrderNos.join('、')}`)
        } else {
          message.success(`已退款 ${reply.refunded ?? 0} 笔订单`)
        }
        void load()
      } finally {
        refunding.value = false
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
    ? items.value.filter((item) => selectedRowKeys.value.includes(String(item.dbId)))
    : items.value
  if (!source.length) {
    message.warning('暂无可导出的数据')
    return
  }
  const header = ['ID', '用户ID', '用户昵称', '充值金额', '到账金额', '折扣金额', '充值平台', '充值状态', '充值类型', '创建时间', '完成时间', '退款时间', '退款金额', '退款订单ID', '支付平台订单ID', '支付平台OpenID', '错误信息']
  const rows = source.map((record) => [
    record.orderNo ?? '',
    record.userId ?? '',
    record.userNickname ?? '',
    money(record.amountFen),
    money(record.creditedFen),
    money(record.discountFen),
    record.platform ?? '',
    statusText(record.status),
    record.rechargeType ?? '',
    formatTime(record.createdAtUnixMillis),
    formatTime(record.completedAtUnixMillis),
    formatTime(record.refundedAtUnixMillis),
    money(record.refundAmountFen),
    record.refundOrderNo ?? '',
    record.payPlatformOrderId ?? '',
    record.payPlatformOpenid ?? '',
    record.errorMessage ?? '',
  ])
  const csv = '\uFEFF' + [header, ...rows].map((row) => row.map(csvEscape).join(',')).join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `充值订单_${dayjs().format('YYYYMMDD_HHmmss')}.csv`
  link.click()
  URL.revokeObjectURL(url)
}

onMounted(load)
</script>

<style scoped>
.order-page {
  padding: 20px 24px 32px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}

.order-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}

.w-110 { width: 110px; }
.w-130 { width: 130px; }
.w-140 { width: 140px; }
.w-180 { width: 180px; }
.w-200 { width: 200px; }
.w-260 { width: 260px; }

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

.order-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>

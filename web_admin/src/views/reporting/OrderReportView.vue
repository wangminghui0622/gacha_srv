<template>
  <div class="order-report console-list-page">
    <div class="report-toolbar">
      <span class="filter-label">时间</span>
      <a-range-picker
        v-model:value="range"
        class="date-range"
        :allow-clear="false"
        :placeholder="['开始日期', '结束日期']"
      />
      <span class="filter-label">用户ID</span>
      <a-input
        v-model:value="userId"
        class="filter-input"
        placeholder="请输入用户ID"
        allow-clear
        @press-enter="load"
      />
      <span class="filter-label">用户昵称</span>
      <a-input
        v-model:value="nickname"
        class="filter-input"
        placeholder="请输入用户昵称"
        allow-clear
        @press-enter="load"
      />
      <span class="filter-label">手机号</span>
      <a-input
        v-model:value="phone"
        class="filter-input"
        placeholder="请输入手机号"
        :maxlength="11"
        allow-clear
        @press-enter="load"
      />
      <a-button class="btn-query" type="primary" :loading="loading" @click="load">查 询</a-button>
      <a-button class="btn-export" :loading="loading" :disabled="!rows.length" @click="exportCsv">导 出</a-button>
    </div>

    <div class="table-fill">
      <a-table
        :data-source="pageRows"
        :columns="columns"
        :loading="loading"
        :pagination="false"
        :row-key="rowKey"
        size="middle"
        :scroll="{ x: 1280 }"
        :row-class-name="rowClassName"
        class="report-table"
      />
    </div>

    <div class="pagination-bar">
      <a-pagination
        v-model:current="current"
        v-model:page-size="pageSize"
        :total="rows.length"
        :show-size-changer="true"
        :page-size-options="['10', '20', '50', '100']"
        :show-quick-jumper="true"
        :show-total="showTotal"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import dayjs, { type Dayjs } from 'dayjs'
import { message } from 'ant-design-vue'

import { getOrderReport, type OrderReportRow } from '@/api/orderReport'

interface OrderTableRow extends OrderReportRow {
  isTotal?: boolean
}

const range = ref<[Dayjs, Dayjs]>([dayjs().subtract(31, 'day'), dayjs()])
const userId = ref('')
const nickname = ref('')
const phone = ref('')
const loading = ref(false)
const rows = ref<OrderReportRow[]>([])
const current = ref(1)
const pageSize = ref(50)

function money(fen: unknown): string {
  return (Number(fen ?? 0) / 100).toFixed(2)
}

const totalRow = computed<OrderTableRow>(() => {
  const costFen = rows.value.reduce((total, row) => total + Number(row.costFen ?? 0), 0)
  return {
    date: 'Total',
    userId: 0,
    nickname: '',
    costFen,
    isTotal: true,
  }
})

const pageRows = computed<OrderTableRow[]>(() => {
  const start = (current.value - 1) * pageSize.value
  const list: OrderTableRow[] = rows.value.slice(start, start + pageSize.value)
  if (rows.value.length > 0) {
    list.push(totalRow.value)
  }
  return list
})

const columns = [
  { title: '日期', dataIndex: 'date', key: 'date', width: 280, align: 'center' as const },
  { title: '用户ID', dataIndex: 'userId', key: 'userId', width: 320, align: 'center' as const,
    customRender: ({ record }: { record: OrderTableRow }) => (record.isTotal ? '' : String(record.userId ?? '')) },
  { title: '用户昵称', dataIndex: 'nickname', key: 'nickname', width: 360, align: 'center' as const,
    customRender: ({ text, record }: { text: string; record: OrderTableRow }) => (record.isTotal ? '' : (text || '-')) },
  { title: '成本价', dataIndex: 'costFen', key: 'costFen', width: 320, align: 'center' as const,
    customRender: ({ text }: { text: number }) => money(text) },
]

function rowKey(record: OrderTableRow): string {
  return record.isTotal ? 'total' : `${record.date}-${record.userId}`
}

function rowClassName(record: OrderTableRow, index: number): string {
  if (record.isTotal) {
    return 'order-total-row'
  }
  return index % 2 === 1 ? 'order-even-row' : ''
}

function showTotal(total: number): string {
  return `共 ${total} 条`
}

async function load() {
  const [start, end] = range.value
  if (!start || !end) {
    return
  }
  loading.value = true
  try {
    const reply = await getOrderReport({
      startDate: start.format('YYYY-MM-DD'),
      endDate: end.format('YYYY-MM-DD'),
      userId: userId.value.trim(),
      nickname: nickname.value.trim(),
      phone: phone.value.trim(),
    })
    rows.value = reply.rows ?? []
    current.value = 1
  } catch {
    rows.value = []
    message.error('订单报表查询失败')
  } finally {
    loading.value = false
  }
}

function csvEscape(value: unknown): string {
  const text = String(value ?? '')
  return /[",\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text
}

function exportCsv() {
  const header = ['日期', '用户ID', '用户昵称', '成本价']
  const data = rows.value.map((row) => [
    row.date,
    row.userId,
    row.nickname || '-',
    money(row.costFen),
  ])
  data.push(['Total', '', '', money(totalRow.value.costFen)])
  const csv = '\uFEFF' + [header, ...data].map((line) => line.map(csvEscape).join(',')).join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `订单报表_${dayjs().format('YYYYMMDD_HHmmss')}.csv`
  link.click()
  URL.revokeObjectURL(url)
}

onMounted(load)
</script>

<style scoped lang="scss">
.order-report {
  display: flex;
  flex-direction: column;
}

.report-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px 10px;
  margin-bottom: 12px;
}

.filter-label {
  color: #303133;
  font-size: 14px;
  white-space: nowrap;
}

.date-range {
  width: 280px;
}

.filter-input {
  width: 160px;
}

.btn-query {
  background: #1677ff !important;
  border-color: #1677ff !important;
}

.btn-query:hover {
  background: #4096ff !important;
  border-color: #4096ff !important;
}

.btn-export {
  background: #faad14 !important;
  border-color: #faad14 !important;
  color: #fff !important;
}

.btn-export:hover {
  background: #ffc53d !important;
  border-color: #ffc53d !important;
  color: #fff !important;
}

.report-table {
  :deep(.ant-table-thead > tr > th) {
    background: #fafafa;
    color: #303133;
    font-weight: 500;
    font-size: 14px;
  }

  :deep(.ant-table-tbody > tr > td) {
    font-size: 14px;
    color: #595959;
    padding-top: 10px;
    padding-bottom: 10px;
  }

  :deep(.order-even-row > td) {
    background: #f7fbff;
  }

  :deep(.order-total-row > td) {
    background: #fafafa;
    font-weight: 600;
  }
}

.pagination-bar {
  display: flex;
  justify-content: flex-start;
  margin-top: 12px;
}
</style>

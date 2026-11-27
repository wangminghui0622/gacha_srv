<template>
  <div class="market-report console-list-page">
    <div class="report-toolbar">
      <a-range-picker
        v-model:value="range"
        class="date-range"
        :allow-clear="false"
        :placeholder="['开始日期', '结束日期']"
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
        row-key="date"
        size="middle"
        :scroll="{ x: 1220 }"
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

import { getMarketReport, type MarketReportRow } from '@/api/marketReport'

interface MarketTableRow extends MarketReportRow {
  isTotal?: boolean
}

const range = ref<[Dayjs, Dayjs]>([dayjs().subtract(30, 'day'), dayjs().subtract(1, 'day')])
const loading = ref(false)
const rows = ref<MarketReportRow[]>([])
const current = ref(1)
const pageSize = ref(50)

function money(fen: unknown): string {
  return (Number(fen ?? 0) / 100).toFixed(2)
}

const totalRow = computed<MarketTableRow>(() => {
  const total: MarketTableRow = {
    date: 'Total',
    participants: 0,
    newOrders: 0,
    amountTotalFen: 0,
    amountNormalFen: 0,
    amountSampleFen: 0,
    isTotal: true,
  }
  for (const row of rows.value) {
    total.participants += Number(row.participants ?? 0)
    total.newOrders += Number(row.newOrders ?? 0)
    total.amountTotalFen += Number(row.amountTotalFen ?? 0)
    total.amountNormalFen += Number(row.amountNormalFen ?? 0)
    total.amountSampleFen += Number(row.amountSampleFen ?? 0)
  }
  return total
})

const pageRows = computed<MarketTableRow[]>(() => {
  const start = (current.value - 1) * pageSize.value
  const list: MarketTableRow[] = rows.value.slice(start, start + pageSize.value)
  const lastPage = Math.max(1, Math.ceil(rows.value.length / pageSize.value))
  if (rows.value.length > 0 && current.value === lastPage) {
    list.push(totalRow.value)
  }
  return list
})

const columns = [
  { title: '日期', dataIndex: 'date', key: 'date', width: 150, align: 'center' as const, fixed: 'left' as const },
  { title: '参与用户数', dataIndex: 'participants', key: 'participants', width: 200, align: 'center' as const },
  { title: '新增订单数', dataIndex: 'newOrders', key: 'newOrders', width: 200, align: 'center' as const },
  { title: '成交金额(总)', dataIndex: 'amountTotalFen', key: 'amountTotalFen', width: 220, align: 'center' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '成交金额(普通用户)', dataIndex: 'amountNormalFen', key: 'amountNormalFen', width: 230, align: 'center' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '成交金额(样子)', dataIndex: 'amountSampleFen', key: 'amountSampleFen', width: 220, align: 'center' as const, customRender: ({ text }: { text: number }) => money(text) },
]

function rowClassName(record: MarketTableRow, index: number): string {
  if (record.isTotal) {
    return 'market-total-row'
  }
  return index % 2 === 1 ? 'market-even-row' : ''
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
    const reply = await getMarketReport({
      startDate: start.format('YYYY-MM-DD'),
      endDate: end.format('YYYY-MM-DD'),
    })
    rows.value = reply.rows ?? []
    current.value = 1
  } catch {
    rows.value = []
    message.error('市场报表查询失败')
  } finally {
    loading.value = false
  }
}

function csvEscape(value: unknown): string {
  const text = String(value ?? '')
  return /[",\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text
}

function exportCsv() {
  const header = ['日期', '参与用户数', '新增订单数', '成交金额(总)', '成交金额(普通用户)', '成交金额(样子)']
  const data = [...rows.value, totalRow.value].map((row) => [
    row.date,
    row.participants,
    row.newOrders,
    money(row.amountTotalFen),
    money(row.amountNormalFen),
    money(row.amountSampleFen),
  ])
  const csv = '\uFEFF' + [header, ...data].map((line) => line.map(csvEscape).join(',')).join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `市场报表_${dayjs().format('YYYYMMDD_HHmmss')}.csv`
  link.click()
  URL.revokeObjectURL(url)
}

onMounted(load)
</script>

<style scoped lang="scss">
.market-report {
  display: flex;
  flex-direction: column;
}

.report-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 12px;
}

.date-range {
  width: 280px;
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

  :deep(.market-even-row > td) {
    background: #f7fbff;
  }

  :deep(.market-total-row > td) {
    background: #fafafa;
    font-weight: 600;
  }
}

.pagination-bar {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}
</style>

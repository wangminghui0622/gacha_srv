<template>
  <div class="revenue-report console-list-page">
    <div class="report-toolbar">
      <a-range-picker
        v-model:value="range"
        class="date-range"
        :allow-clear="false"
        :presets="rangePresets"
        :placeholder="['开始日期', '结束日期']"
      />
      <a-button class="btn-query" type="primary" :loading="loading" @click="load">查 询</a-button>
      <a-button :loading="loading" :disabled="!rows.length" @click="exportCsv">导 出</a-button>
    </div>

    <div class="report-title">统一大盘趋势图</div>
    <RevenueTrendChart :rows="rows" />
    <a-table
      :data-source="rows"
      :columns="columns"
      :loading="loading"
      :pagination="false"
      row-key="date"
      size="middle"
      :scroll="{ x: 1200 }"
      class="report-table"
    />
    <div class="console-list-footer">
      <span>共 {{ rows.length }} 天</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import dayjs, { type Dayjs } from 'dayjs'
import { message } from 'ant-design-vue'

import RevenueTrendChart from '@/components/charts/RevenueTrendChart.vue'
import { getRevenueReport, type RevenueReportRow } from '@/api/revenueReport'

const rangePresets: { label: string; value: [Dayjs, Dayjs] }[] = [
  { label: '最近7天', value: [dayjs().subtract(6, 'day'), dayjs()] },
  { label: '最近30天', value: [dayjs().subtract(29, 'day'), dayjs()] },
  { label: '最近90天', value: [dayjs().subtract(89, 'day'), dayjs()] },
  { label: '最近180天', value: [dayjs().subtract(179, 'day'), dayjs()] },
]
const range = ref<[Dayjs, Dayjs]>([dayjs().subtract(30, 'day'), dayjs()])
const loading = ref(false)
const rows = ref<RevenueReportRow[]>([])
let requestSeq = 0

function money(value: unknown): string {
  return (Number(value ?? 0) / 100).toFixed(2)
}

function rate(value: unknown): string {
  return `${(Number(value ?? 0) / 100).toFixed(2)}%`
}

const columns = [
  { title: '日期', dataIndex: 'date', key: 'date', width: 120, align: 'center' as const, fixed: 'left' as const },
  { title: '日活', dataIndex: 'activeUsers', key: 'activeUsers', width: 100, align: 'center' as const },
  { title: '总付费', dataIndex: 'payAmountFen', key: 'payAmountFen', width: 130, align: 'center' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '退款金额', dataIndex: 'refundAmountFen', key: 'refundAmountFen', width: 130, align: 'center' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '付费用户数', dataIndex: 'payingUsers', key: 'payingUsers', width: 110, align: 'center' as const },
  { title: 'arpu', dataIndex: 'arpuFen', key: 'arpuFen', width: 110, align: 'center' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '渗透率', dataIndex: 'penetrationBps', key: 'penetrationBps', width: 110, align: 'center' as const, customRender: ({ text }: { text: number }) => rate(text) },
  { title: '总营收', dataIndex: 'netRevenueFen', key: 'netRevenueFen', width: 130, align: 'center' as const, customRender: ({ text }: { text: number }) => money(text) },
]

async function load() {
  const [start, end] = range.value
  if (!start || !end) {
    return
  }
  const seq = ++requestSeq
  loading.value = true
  rows.value = []
  try {
    const reply = await getRevenueReport({
      startDate: start.format('YYYY-MM-DD'),
      endDate: end.format('YYYY-MM-DD'),
    })
    if (seq !== requestSeq) {
      return
    }
    rows.value = reply.rows ?? []
  } catch {
    if (seq !== requestSeq) {
      return
    }
    rows.value = []
    message.error('营收报表查询失败')
  } finally {
    if (seq === requestSeq) {
      loading.value = false
    }
  }
}

function csvEscape(value: unknown): string {
  const text = String(value ?? '')
  return /[",\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text
}

function exportCsv() {
  const header = ['日期', '日活', '总付费', '退款金额', '付费用户数', 'arpu', '渗透率', '总营收']
  const data = rows.value.map((row) => [
    row.date,
    row.activeUsers,
    money(row.payAmountFen),
    money(row.refundAmountFen),
    row.payingUsers,
    money(row.arpuFen),
    rate(row.penetrationBps),
    money(row.netRevenueFen),
  ])
  const csv = '\uFEFF' + [header, ...data].map((line) => line.map(csvEscape).join(',')).join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `营收报表_${dayjs().format('YYYYMMDD_HHmmss')}.csv`
  link.click()
  URL.revokeObjectURL(url)
}

onMounted(load)
</script>

<style scoped lang="scss">
.revenue-report {
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
  background: #52c41a !important;
  border-color: #52c41a !important;
}

.btn-query:hover {
  background: #73d13d !important;
  border-color: #73d13d !important;
}

.report-title {
  margin: 0 0 12px;
  color: #262626;
  font-size: 16px;
  font-weight: 600;
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
    padding-top: 12px;
    padding-bottom: 12px;
  }
}

.console-list-footer {
  display: flex;
  justify-content: flex-end;
  color: #8c8c8c;
  font-size: 13px;
  margin-top: 8px;
}
</style>

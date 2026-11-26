<template>
  <div class="daily-report console-list-page">
    <div class="report-toolbar">
      <a-range-picker
        v-model:value="range"
        class="date-range"
        :allow-clear="false"
        :placeholder="['开始日期', '结束日期']"
      />
      <a-button class="btn-query" type="primary" :loading="loading" @click="load">查 询</a-button>
      <a-button :loading="loading" :disabled="!rows.length" @click="exportCsv">导 出</a-button>
    </div>

    <div class="summary-scroll">
      <table class="summary-table">
        <thead>
          <tr>
            <th v-for="metric in summaryMetrics" :key="`c-${metric.key}`">{{ cumulativeLabel(metric) }}</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td v-for="metric in summaryMetrics" :key="`cv-${metric.key}`">{{ summaryValue(summary, metric) }}</td>
          </tr>
        </tbody>
      </table>
      <table class="summary-table summary-table-month">
        <thead>
          <tr>
            <th v-for="metric in summaryMetrics" :key="`m-${metric.key}`">{{ monthLabel(metric) }}</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td v-for="metric in summaryMetrics" :key="`mv-${metric.key}`">{{ summaryValue(month, metric) }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="table-fill">
      <a-table
        :data-source="rows"
        :columns="columns"
        :loading="loading"
        :pagination="false"
        row-key="date"
        size="small"
        :scroll="{ x: 2500 }"
        class="report-table"
      />
    </div>

    <div class="console-list-footer">
      <span>共 {{ rows.length }} 天</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import dayjs, { type Dayjs } from 'dayjs'
import { message } from 'ant-design-vue'

import { getDailyReport, type DailyReportResponse, type DailyReportRow, type DailyReportSummary } from '@/api/dailyReport'

type MetricType = 'count' | 'money' | 'rate'
interface SummaryMetric {
  key: keyof DailyReportSummary
  label: string
  type: MetricType
}

const summaryMetrics: SummaryMetric[] = [
  { key: 'registrations', label: '注册', type: 'count' },
  { key: 'payingUsers', label: '付费', type: 'count' },
  { key: 'drawCount', label: '抽赏', type: 'count' },
  { key: 'prizeAmountFen', label: '奖品金额', type: 'money' },
  { key: 'grossProfitFen', label: '总毛利', type: 'money' },
  { key: 'grossMarginBps', label: '总毛利率', type: 'rate' },
  { key: 'totalReceivedFen', label: '总到账', type: 'money' },
  { key: 'totalRechargeFen', label: '总充值', type: 'money' },
  { key: 'totalRefundFen', label: '总退款', type: 'money' },
  { key: 'alipayRechargeFen', label: '支付宝充值', type: 'money' },
  { key: 'huifuRechargeFen', label: '汇付充值', type: 'money' },
  { key: 'alipayRefundFen', label: '支付宝退款', type: 'money' },
  { key: 'huifuRefundFen', label: '汇付退款', type: 'money' },
  { key: 'rechargeOriginalRefundFen', label: '充值原路退款', type: 'money' },
  { key: 'userRefundFen', label: '用户退款', type: 'money' },
  { key: 'rechargeMinusRefundFen', label: '充值减退款', type: 'money' },
  { key: 'refundRateBps', label: '平均退款率', type: 'rate' },
]

const emptySummary = (): DailyReportSummary => ({
  registrations: 0,
  payingUsers: 0,
  drawCount: 0,
  drawAmountFen: 0,
  prizeAmountFen: 0,
  grossProfitFen: 0,
  grossMarginBps: 0,
  totalReceivedFen: 0,
  totalRechargeFen: 0,
  totalRefundFen: 0,
  alipayRechargeFen: 0,
  huifuRechargeFen: 0,
  alipayRefundFen: 0,
  huifuRefundFen: 0,
  rechargeOriginalRefundFen: 0,
  userRefundFen: 0,
  rechargeMinusRefundFen: 0,
  refundRateBps: 0,
})

const range = ref<[Dayjs, Dayjs]>([dayjs().subtract(29, 'day'), dayjs()])
const loading = ref(false)
const summary = ref<DailyReportSummary>(emptySummary())
const month = ref<DailyReportSummary>(emptySummary())
const rows = ref<DailyReportRow[]>([])

function money(value: unknown): string {
  return (Number(value ?? 0) / 100).toFixed(2)
}

function rate(value: unknown): string {
  return `${(Number(value ?? 0) / 100).toFixed(2)}%`
}

function summaryValue(data: DailyReportSummary, metric: SummaryMetric): string {
  const value = data[metric.key]
  if (metric.type === 'money') return money(value)
  if (metric.type === 'rate') return rate(value)
  return String(value ?? 0)
}

function cumulativeLabel(metric: SummaryMetric): string {
  return `累计${metric.label}`
}

function monthLabel(metric: SummaryMetric): string {
  if (metric.key === 'grossMarginBps') return '本月毛利率'
  return `本月${metric.label}`
}

const columns = [
  { title: '时间', dataIndex: 'date', key: 'date', width: 100, fixed: 'left' as const },
  { title: '注册用户', dataIndex: 'registrations', key: 'registrations', width: 82, align: 'right' as const },
  { title: '活跃用户', dataIndex: 'activeUsers', key: 'activeUsers', width: 82, align: 'right' as const },
  { title: '付费用户', dataIndex: 'payingUsers', key: 'payingUsers', width: 82, align: 'right' as const },
  { title: '用户可退款金额', dataIndex: 'refundableFen', key: 'refundableFen', width: 122, align: 'right' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '金额剩余(0点)', dataIndex: 'balanceFen', key: 'balanceFen', width: 122, align: 'right' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '库存价值', dataIndex: 'inventoryValueFen', key: 'inventoryValueFen', width: 112, align: 'right' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '总抽赏金额', dataIndex: 'drawAmountFen', key: 'drawAmountFen', width: 112, align: 'right' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '奖品金额', dataIndex: 'prizeAmountFen', key: 'prizeAmountFen', width: 112, align: 'right' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '毛利', dataIndex: 'grossProfitFen', key: 'grossProfitFen', width: 112, align: 'right' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '毛利率', dataIndex: 'grossMarginBps', key: 'grossMarginBps', width: 86, align: 'right' as const, customRender: ({ text }: { text: number }) => rate(text) },
  { title: '总到账', dataIndex: 'totalReceivedFen', key: 'totalReceivedFen', width: 100, align: 'right' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '总充值', dataIndex: 'totalRechargeFen', key: 'totalRechargeFen', width: 100, align: 'right' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '总退款', dataIndex: 'totalRefundFen', key: 'totalRefundFen', width: 100, align: 'right' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '支付宝充值', dataIndex: 'alipayRechargeFen', key: 'alipayRechargeFen', width: 106, align: 'right' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '汇付充值', dataIndex: 'huifuRechargeFen', key: 'huifuRechargeFen', width: 100, align: 'right' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '支付宝退款', dataIndex: 'alipayRefundFen', key: 'alipayRefundFen', width: 106, align: 'right' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '汇付退款', dataIndex: 'huifuRefundFen', key: 'huifuRefundFen', width: 100, align: 'right' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '充值原路退款', dataIndex: 'rechargeOriginalRefundFen', key: 'rechargeOriginalRefundFen', width: 122, align: 'right' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '用户退款', dataIndex: 'userRefundFen', key: 'userRefundFen', width: 100, align: 'right' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '充值减退款', dataIndex: 'rechargeMinusRefundFen', key: 'rechargeMinusRefundFen', width: 112, align: 'right' as const, customRender: ({ text }: { text: number }) => money(text) },
  { title: '退款率', dataIndex: 'refundRateBps', key: 'refundRateBps', width: 86, align: 'right' as const, customRender: ({ text }: { text: number }) => rate(text) },
]

async function load() {
  const [start, end] = range.value
  if (!start || !end) return
  loading.value = true
  try {
    const reply: DailyReportResponse = await getDailyReport({
      startDate: start.format('YYYY-MM-DD'),
      endDate: end.format('YYYY-MM-DD'),
    })
    summary.value = reply.summary ?? emptySummary()
    month.value = reply.month ?? emptySummary()
    rows.value = reply.rows ?? []
  } catch {
    summary.value = emptySummary()
    month.value = emptySummary()
    rows.value = []
    message.error('经营日报查询失败')
  } finally {
    loading.value = false
  }
}

function csvEscape(value: unknown): string {
  const text = String(value ?? '')
  return /[",\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text
}

function exportCsv() {
  const summaryHeader = summaryMetrics.map((metric) => `累计${metric.label}`)
  const summaryRow = summaryMetrics.map((metric) => summaryValue(summary.value, metric))
  const monthHeader = summaryMetrics.map((metric) => monthLabel(metric))
  const monthRow = summaryMetrics.map((metric) => summaryValue(month.value, metric))
  const header = ['时间', '注册用户', '活跃用户', '付费用户', '用户可退款金额', '金额剩余(0点)', '库存价值', '总抽赏金额', '奖品金额', '毛利', '毛利率', '总到账', '总充值', '总退款', '支付宝充值', '汇付充值', '支付宝退款', '汇付退款', '充值原路退款', '用户退款', '充值减退款', '退款率']
  const data = rows.value.map((row) => [
    row.date,
    row.registrations,
    row.activeUsers,
    row.payingUsers,
    money(row.refundableFen),
    money(row.balanceFen),
    money(row.inventoryValueFen),
    money(row.drawAmountFen),
    money(row.prizeAmountFen),
    money(row.grossProfitFen),
    rate(row.grossMarginBps),
    money(row.totalReceivedFen),
    money(row.totalRechargeFen),
    money(row.totalRefundFen),
    money(row.alipayRechargeFen),
    money(row.huifuRechargeFen),
    money(row.alipayRefundFen),
    money(row.huifuRefundFen),
    money(row.rechargeOriginalRefundFen),
    money(row.userRefundFen),
    money(row.rechargeMinusRefundFen),
    rate(row.refundRateBps),
  ])
  const lines = [summaryHeader, summaryRow, monthHeader, monthRow, [], header, ...data]
  const csv = '\uFEFF' + lines.map((line) => line.map(csvEscape).join(',')).join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `经营日报_${dayjs().format('YYYYMMDD_HHmmss')}.csv`
  link.click()
  URL.revokeObjectURL(url)
}

onMounted(load)
</script>

<style scoped lang="scss">
.daily-report {
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

.summary-scroll {
  overflow-x: auto;
  margin-bottom: 12px;
}

.summary-table {
  width: 100%;
  min-width: 1900px;
  border-collapse: collapse;
  table-layout: fixed;
  font-size: 14px;

  th,
  td {
    border: 1px solid #f0f0f0;
    padding: 8px 10px;
    text-align: center;
    white-space: nowrap;
  }

  th {
    background: #fafafa;
    color: #303133;
    font-weight: 500;
  }

  td {
    color: #595959;
  }
}

.summary-table-month {
  margin-top: 8px;
}

.table-fill {
  flex: 1;
  min-height: 0;
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

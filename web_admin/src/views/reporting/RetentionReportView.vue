<template>
  <div class="retention-report console-list-page">
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

    <a-tabs v-model:activeKey="activeTab" class="retention-tabs" @change="load">
      <a-tab-pane v-for="tab in tabs" :key="tab.key">
        <template #tab>
          <span>{{ tab.label }}</span>
        </template>
        <div v-if="activeTab === tab.key" class="retention-pane">
          <div class="report-title">{{ activeTabLabel }}</div>
          <RetentionCurveChart :rows="rows" :day-nos="dayNos" />
          <a-table
            :data-source="tableRows"
            :columns="columns"
            :loading="loading"
            :pagination="false"
            row-key="cohortDate"
            size="small"
            :scroll="{ x: 1500 }"
            :row-class-name="rowClassName"
            class="report-table"
          />
          <div class="console-list-footer">
            <span>共 {{ rows.length }} 天</span>
          </div>
        </div>
      </a-tab-pane>
    </a-tabs>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import dayjs, { type Dayjs } from 'dayjs'
import { message } from 'ant-design-vue'

import RetentionCurveChart from '@/components/charts/RetentionCurveChart.vue'
import { getRetentionReport, type RetentionRow, type RetentionType } from '@/api/retentionReport'

interface RetentionTableRow extends RetentionRow {
  isTotal?: boolean
}

const tabs: { key: RetentionType; label: string }[] = [
  { key: 'new', label: '新增留存' },
  { key: 'active', label: '有效用户' },
  { key: 'new_pay', label: '新增消费' },
  { key: 'join', label: '参与留存' },
  { key: 'pay', label: '消费留存' },
]

const emptyTotal = (): RetentionRow => ({ cohortDate: 'TOTAL', totalUsers: 0, cells: [] })

const activeTab = ref<RetentionType>('new')
const activeTabLabel = computed(() => tabs.find((tab) => tab.key === activeTab.value)?.label ?? '留存报表')
const rangePresets: { label: string; value: [Dayjs, Dayjs] }[] = [
  { label: '最近7天', value: [dayjs().subtract(6, 'day'), dayjs()] },
  { label: '最近30天', value: [dayjs().subtract(29, 'day'), dayjs()] },
  { label: '最近90天', value: [dayjs().subtract(89, 'day'), dayjs()] },
  { label: '最近180天', value: [dayjs().subtract(179, 'day'), dayjs()] },
]
const range = ref<[Dayjs, Dayjs]>([dayjs().subtract(29, 'day'), dayjs()])
const loading = ref(false)
const rows = ref<RetentionRow[]>([])
const total = ref<RetentionRow>(emptyTotal())
const dayNos = ref<number[]>([1, 2, 3, 4, 5, 6, 7, 14, 30, 60, 90, 180])
let requestSeq = 0

const tableRows = computed<RetentionTableRow[]>(() => {
  const list: RetentionTableRow[] = rows.value.map((row) => ({ ...row, isTotal: false }))
  if (rows.value.length > 0) {
    list.push({ ...total.value, isTotal: true })
  }
  return list
})

function renderCell(row: RetentionRow, dayNo: number) {
  const cell = row.cells?.find((item) => item.dayNo === dayNo)
  const rate = cell?.available ? (cell.rateBps / 100).toFixed(2) : '0.00'
  const count = cell?.available ? cell.retainedUsers : 0
  return h('div', { class: 'ret-cell' }, [
    h('span', { class: 'ret-rate' }, `${rate}%`),
    h('span', { class: 'ret-count' }, `${count}`),
  ])
}

const columns = computed(() => [
  {
    title: '日期',
    dataIndex: 'cohortDate',
    key: 'cohortDate',
    width: 110,
    align: 'center' as const,
    fixed: 'left' as const,
  },
  {
    title: '总数',
    dataIndex: 'totalUsers',
    key: 'totalUsers',
    width: 80,
    align: 'center' as const,
  },
  ...dayNos.value.map((dayNo) => ({
    title: `第${dayNo}天`,
    key: `d${dayNo}`,
    width: 90,
    align: 'center' as const,
    customRender: ({ record }: { record: RetentionRow }) => renderCell(record, dayNo),
  })),
])

function rowClassName(record: RetentionTableRow): string {
  return record.isTotal ? 'retention-total-row' : ''
}

async function load() {
  const [start, end] = range.value
  if (!start || !end) {
    return
  }
  const seq = ++requestSeq
  loading.value = true
  rows.value = []
  total.value = emptyTotal()
  try {
    const reply = await getRetentionReport({
      type: activeTab.value,
      startDate: start.format('YYYY-MM-DD'),
      endDate: end.format('YYYY-MM-DD'),
    })
    if (seq !== requestSeq) {
      return
    }
    rows.value = reply.rows ?? []
    total.value = reply.total ?? emptyTotal()
    if (reply.dayNos?.length) {
      dayNos.value = reply.dayNos
    }
  } catch {
    if (seq !== requestSeq) {
      return
    }
    rows.value = []
    total.value = emptyTotal()
    message.error('留存报表查询失败')
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

function cellText(row: RetentionRow, dayNo: number): string {
  const cell = row.cells?.find((item) => item.dayNo === dayNo)
  if (!cell?.available) {
    return '0.00% (0)'
  }
  return `${(cell.rateBps / 100).toFixed(2)}% (${cell.retainedUsers})`
}

function exportCsv() {
  const header = ['日期', '总数', ...dayNos.value.map((dayNo) => `第${dayNo}天`)]
  const data = tableRows.value.map((row) => [
    row.cohortDate,
    row.totalUsers,
    ...dayNos.value.map((dayNo) => cellText(row, dayNo)),
  ])
  const csv = '\uFEFF' + [header, ...data].map((line) => line.map(csvEscape).join(',')).join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `留存报表_${tabs.find((tab) => tab.key === activeTab.value)?.label ?? ''}_${dayjs().format('YYYYMMDD_HHmmss')}.csv`
  link.click()
  URL.revokeObjectURL(url)
}

onMounted(load)
</script>

<style scoped lang="scss">
.retention-report {
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

.retention-tabs {
  :deep(.ant-tabs-nav) {
    margin-bottom: 12px;
  }
}

.retention-pane {
  display: flex;
  flex-direction: column;
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
  }

  :deep(.retention-total-row > td) {
    background: #fafafa;
    font-weight: 600;
  }
}

.ret-cell {
  display: flex;
  flex-direction: column;
  align-items: center;
  line-height: 1.35;
}

.ret-rate {
  color: #262626;
}

.ret-count {
  color: #8c8c8c;
  font-size: 12px;
}

.console-list-footer {
  display: flex;
  justify-content: flex-end;
  color: #8c8c8c;
  font-size: 13px;
  margin-top: 8px;
}
</style>

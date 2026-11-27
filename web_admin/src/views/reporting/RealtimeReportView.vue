<template>
  <div class="realtime-report">
    <div class="report-toolbar">
      <a-button type="primary" :loading="loading" @click="load">查 询</a-button>
      <span v-if="updatedAt" class="update-time">更新于 {{ updatedAt }}</span>
    </div>

    <div class="summary-grid">
      <div v-for="card in summaryCards" :key="card.label" class="summary-card">
        <div class="summary-label">{{ card.label }}</div>
        <div class="summary-value">{{ card.value }}</div>
      </div>
    </div>

    <div v-for="chart in charts" :key="chart.title" class="chart-card">
      <div class="chart-title">{{ chart.title }}</div>
      <RealtimeChart :labels="labels" :series="chart.series" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import dayjs from 'dayjs'
import { message } from 'ant-design-vue'

import RealtimeChart from '@/components/charts/RealtimeChart.vue'
import {
  getRealtimeReport,
  type RealtimeChart as RealtimeChartData,
  type RealtimeSummary,
} from '@/api/realtimeReport'

const loading = ref(false)
const updatedAt = ref('')
const labels = ref<string[]>([])
const charts = ref<RealtimeChartData[]>([])
const summary = ref<RealtimeSummary>({
  totalRegistrations: 0,
  totalPayingUsers: 0,
  totalPayFen: 0,
  totalRechargeUsers: 0,
  totalRechargeFen: 0,
  totalRefundFen: 0,
  arppuFen: 0,
  todayRegistrations: 0,
  todayPayFen: 0,
  todayRechargeFen: 0,
})

function money(fen: number): string {
  return (Number(fen ?? 0) / 100).toFixed(2)
}

const summaryCards = computed(() => [
  { label: '累计注册', value: String(summary.value.totalRegistrations ?? 0) },
  { label: '累计付费用户数', value: String(summary.value.totalPayingUsers ?? 0) },
  { label: '累计付费总额', value: money(summary.value.totalPayFen) },
  { label: '累计充值用户数', value: String(summary.value.totalRechargeUsers ?? 0) },
  { label: '累计充值总额', value: money(summary.value.totalRechargeFen) },
  { label: '累计退款(¥)总额', value: money(summary.value.totalRefundFen) },
  { label: '付费ARPPU(总付费)', value: money(summary.value.arppuFen) },
  { label: '今日注册', value: String(summary.value.todayRegistrations ?? 0) },
  { label: '今日付费', value: money(summary.value.todayPayFen) },
  { label: '今日充值', value: money(summary.value.todayRechargeFen) },
])

async function load() {
  loading.value = true
  try {
    const reply = await getRealtimeReport()
    summary.value = reply.summary
    labels.value = reply.labels ?? []
    charts.value = reply.charts ?? []
    updatedAt.value = dayjs().format('HH:mm:ss')
  } catch {
    message.error('实时报表查询失败')
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped lang="scss">
.realtime-report {
  padding-bottom: 16px;
}

.report-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}

.update-time {
  color: #8c8c8c;
  font-size: 12px;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(10, minmax(0, 1fr));
  overflow-x: auto;
  border-top: 1px solid #f0f0f0;
  border-left: 1px solid #f0f0f0;
}

.summary-card {
  min-width: 0;
  padding: 10px 12px;
  text-align: center;
  background: #fafafa;
  border-right: 1px solid #f0f0f0;
  border-bottom: 1px solid #f0f0f0;
}

.summary-label {
  color: #8c8c8c;
  font-size: 13px;
  white-space: nowrap;
}

.summary-value {
  margin-top: 6px;
  color: #262626;
  font-size: 18px;
  line-height: 1.3;
  word-break: break-all;
}

.chart-card {
  margin-top: 22px;
}

.chart-title {
  margin-bottom: 6px;
  color: #262626;
  font-size: 15px;
  font-weight: 600;
}
</style>

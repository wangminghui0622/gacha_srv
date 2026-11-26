<template>
  <div class="revenue-chart-wrap">
    <div ref="chartEl" class="revenue-chart" />
    <div v-if="empty" class="chart-empty">暂无可展示的营收数据</div>
  </div>
</template>

<script setup lang="ts">
import * as echarts from 'echarts'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { RevenueReportRow } from '@/api/revenueReport'

const props = defineProps<{
  rows: RevenueReportRow[]
}>()

const chartEl = ref<HTMLDivElement | null>(null)
const empty = ref(true)
let chart: echarts.ECharts | null = null

interface SeriesDef {
  name: string
  axis: 0 | 1
  kind: 'count' | 'money' | 'rate'
  value: (row: RevenueReportRow) => number
}

const defs: SeriesDef[] = [
  { name: '注册数', axis: 0, kind: 'count', value: (r) => r.registrations },
  { name: '日活', axis: 0, kind: 'count', value: (r) => r.activeUsers },
  { name: '日活(新)', axis: 0, kind: 'count', value: (r) => r.newActiveUsers },
  { name: '日活(旧)', axis: 0, kind: 'count', value: (r) => r.oldActiveUsers },
  { name: '有效用户数(7天)', axis: 0, kind: 'count', value: (r) => r.activeUsers7d },
  { name: '有效用户数(15天)', axis: 0, kind: 'count', value: (r) => r.activeUsers15d },
  { name: '前1天流失率', axis: 1, kind: 'rate', value: (r) => r.churn1dBps / 100 },
  { name: '前3天流失率', axis: 1, kind: 'rate', value: (r) => r.churn3dBps / 100 },
  { name: '总付费', axis: 0, kind: 'money', value: (r) => r.payAmountFen / 100 },
  { name: '总付费(新)', axis: 0, kind: 'money', value: (r) => r.newPayAmountFen / 100 },
  { name: '总付费(旧)', axis: 0, kind: 'money', value: (r) => r.oldPayAmountFen / 100 },
  { name: '退款金额', axis: 0, kind: 'money', value: (r) => r.refundAmountFen / 100 },
  { name: '退款人数', axis: 0, kind: 'count', value: (r) => r.refundUsers },
  { name: '付费用户数', axis: 0, kind: 'count', value: (r) => r.payingUsers },
  { name: '付费用户数(新)', axis: 0, kind: 'count', value: (r) => r.newPayingUsers },
  { name: '付费用户数(旧)', axis: 0, kind: 'count', value: (r) => r.oldPayingUsers },
  { name: 'arpu', axis: 0, kind: 'money', value: (r) => r.arpuFen / 100 },
  { name: 'arppu', axis: 0, kind: 'money', value: (r) => r.arppuFen / 100 },
  { name: '渗透率', axis: 1, kind: 'rate', value: (r) => r.penetrationBps / 100 },
  { name: '总营收', axis: 0, kind: 'money', value: (r) => r.netRevenueFen / 100 },
]

const colors = [
  '#d4a017',
  '#1677ff',
  '#52c41a',
  '#fa8c16',
  '#13c2c2',
  '#eb2f96',
  '#722ed1',
  '#fa541c',
  '#2f54eb',
  '#a0d911',
  '#f5222d',
  '#8c8c8c',
  '#d46b08',
  '#69b1ff',
  '#95de64',
  '#ffc53d',
  '#ff85c0',
  '#5cdbd3',
  '#b37feb',
  '#ff9c6e',
]

function render() {
  if (!chartEl.value) {
    return
  }
  if (!chart) {
    chart = echarts.init(chartEl.value)
  }

  const data = [...props.rows].sort((a, b) => a.date.localeCompare(b.date))
  const labels = data.map((row) => row.date)
  empty.value = data.length === 0

  chart.setOption(
    {
      tooltip: {
        trigger: 'axis',
        formatter: (params: any) => {
          if (!Array.isArray(params) || params.length === 0) {
            return ''
          }
          const title = params[0].axisValue
          const lines = params.map((item: any) => {
            const def = defs.find((entry) => entry.name === item.seriesName)
            let text = `${item.value}`
            if (def?.kind === 'money') {
              text = `¥${Number(item.value).toFixed(2)}`
            } else if (def?.kind === 'rate') {
              text = `${Number(item.value).toFixed(2)}%`
            }
            return `${item.marker}${item.seriesName}: ${text}`
          })
          return [title, ...lines].join('<br/>')
        },
      },
      legend: {
        type: 'scroll',
        top: 0,
        data: defs.map((def) => def.name),
        icon: 'circle',
        itemWidth: 8,
        itemHeight: 8,
        textStyle: { fontSize: 12, color: '#595959' },
      },
      grid: { left: 64, right: 64, top: 54, bottom: labels.length > 15 ? 84 : 56 },
      xAxis: {
        type: 'category',
        data: labels,
        axisLabel: {
          fontSize: 11,
          rotate: labels.length > 15 ? 45 : 0,
          hideOverlap: true,
        },
      },
      yAxis: [
        {
          type: 'value',
          min: 0,
          axisLabel: { fontSize: 11 },
        },
        {
          type: 'value',
          min: 0,
          axisLabel: { fontSize: 11, formatter: '{value}%' },
          splitLine: { show: false },
        },
      ],
      dataZoom:
        labels.length > 60
          ? [
              { type: 'inside' },
              { type: 'slider', height: 14, bottom: 8 },
            ]
          : undefined,
      series: defs.map((def, index) => ({
        name: def.name,
        type: 'line',
        smooth: 0.35,
        symbol: 'none',
        data: data.map(def.value),
        yAxisIndex: def.axis,
        lineStyle: { width: 1.3 },
        itemStyle: { color: colors[index % colors.length] },
        emphasis: { focus: 'series' },
      })),
    },
    true,
  )
  chart.resize()
}

watch(() => props.rows, () => render(), { deep: true })

onMounted(() => {
  render()
  window.addEventListener('resize', onResize)
})

function onResize() {
  chart?.resize()
}

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  chart?.dispose()
  chart = null
})
</script>

<style scoped>
.revenue-chart-wrap {
  position: relative;
  width: 100%;
  height: 360px;
}

.revenue-chart {
  width: 100%;
  height: 100%;
}

.chart-empty {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #bfbfbf;
  font-size: 14px;
  pointer-events: none;
}
</style>

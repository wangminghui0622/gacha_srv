<template>
  <div class="retention-chart-wrap">
    <div ref="chartEl" class="retention-chart" />
    <div v-if="empty" class="chart-empty">暂无可展示的留存数据（留存天数尚未到期）</div>
  </div>
</template>

<script setup lang="ts">
import * as echarts from 'echarts'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { RetentionRow } from '@/api/retentionReport'

const props = defineProps<{
  rows: RetentionRow[]
  dayNos: number[]
}>()

const chartEl = ref<HTMLDivElement | null>(null)
const empty = ref(true)
let chart: echarts.ECharts | null = null

const colors = [
  '#1677ff',
  '#52c41a',
  '#faad14',
  '#eb2f96',
  '#13c2c2',
  '#722ed1',
  '#fa541c',
  '#2f54eb',
  '#a0d911',
  '#f5222d',
  '#8c8c8c',
  '#d46b08',
]

function render() {
  if (!chartEl.value) {
    return
  }
  if (!chart) {
    chart = echarts.init(chartEl.value)
  }

  const data = [...props.rows]
    .filter((row) => row.cohortDate !== 'TOTAL')
    .sort((a, b) => a.cohortDate.localeCompare(b.cohortDate))
  const labels = data.map((row) => row.cohortDate)
  const series: any[] = []
  let colorIndex = 0

  for (const dayNo of props.dayNos) {
    const values = data.map((row) => {
      const cell = row.cells.find((item) => item.dayNo === dayNo)
      if (!cell?.available) {
        return null
      }
      return {
        value: cell.rateBps / 100,
        retained: cell.retainedUsers,
        total: cell.totalUsers,
      }
    })
    if (!values.some((value) => value !== null)) {
      continue
    }
    series.push({
      name: `D${dayNo}`,
      type: 'line',
      smooth: true,
      connectNulls: false,
      symbol: 'circle',
      symbolSize: 5,
      data: values,
      itemStyle: { color: colors[colorIndex % colors.length] },
      lineStyle: { width: 1.6 },
    })
    colorIndex += 1
  }
  empty.value = series.length === 0

  chart.setOption(
    {
      tooltip: {
        trigger: 'axis',
        formatter: (params: any) => {
          if (!Array.isArray(params) || params.length === 0) {
            return ''
          }
          const title = params[0].axisValue
          const lines = params
            .filter((item: any) => item.value !== null && item.value !== undefined)
            .map((item: any) => {
              const value = item.data as { value: number; retained: number; total: number }
              return `${item.marker}${item.seriesName}: ${value.value.toFixed(2)}%（${value.retained}/${value.total}）`
            })
          return [title, ...lines].join('<br/>')
        },
      },
      legend: { type: 'scroll', top: 0 },
      grid: { left: 56, right: 24, top: 48, bottom: labels.length > 30 ? 72 : 48 },
      xAxis: {
        type: 'category',
        data: labels,
        axisLabel: { rotate: labels.length > 15 ? 45 : 0 },
      },
      yAxis: {
        type: 'value',
        name: '%',
        min: 0,
        axisLabel: { formatter: '{value}%' },
      },
      dataZoom:
        labels.length > 30
          ? [
              { type: 'inside' },
              { type: 'slider', height: 16, bottom: 8 },
            ]
          : undefined,
      series,
    },
    true,
  )
  chart.resize()
}

watch(() => [props.rows, props.dayNos], () => render(), { deep: true })

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
.retention-chart-wrap {
  position: relative;
  width: 100%;
  height: 340px;
}

.retention-chart {
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

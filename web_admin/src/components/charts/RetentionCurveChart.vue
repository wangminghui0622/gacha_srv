<template>
  <div class="retention-chart-wrap">
    <div ref="chartEl" class="retention-chart" />
    <div v-if="empty" class="chart-empty">暂无可展示的留存数据</div>
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

const countColors = [
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
const rateColors = [
  '#69b1ff',
  '#95de64',
  '#ffd666',
  '#ff85c0',
  '#5cdbd3',
  '#b37feb',
  '#ff9c6e',
  '#85a5ff',
  '#d3f261',
  '#ff7875',
  '#bfbfbf',
  '#ffc069',
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
  const totals = data.map((row) => row.totalUsers)
  const series: any[] = [
    {
      name: '总数',
      type: 'line',
      smooth: false,
      symbol: 'circle',
      symbolSize: 5,
      data: totals,
      lineStyle: { width: 1.2 },
      itemStyle: { color: '#1677ff' },
      emphasis: { focus: 'series' },
    },
  ]
  const countSelected: Record<string, boolean> = {}
  const rateSelected: Record<string, boolean> = {}

  props.dayNos.forEach((dayNo, index) => {
    const countName = `第${dayNo}天`
    const rateName = `第${dayNo}天占比`
    countSelected[countName] = false
    rateSelected[rateName] = false
    const counts = data.map((row) => {
      const cell = row.cells.find((item) => item.dayNo === dayNo)
      return cell?.available ? cell.retainedUsers : null
    })
    const rates = data.map((row) => {
      const cell = row.cells.find((item) => item.dayNo === dayNo)
      return cell?.available ? cell.rateBps / 100 : null
    })
    series.push({
      name: countName,
      type: 'line',
      smooth: false,
      symbol: 'circle',
      symbolSize: 4,
      data: counts,
      lineStyle: { width: 1 },
      itemStyle: { color: countColors[index % countColors.length] },
      emphasis: { focus: 'series' },
    })
    series.push({
      name: rateName,
      type: 'line',
      smooth: false,
      symbol: 'circle',
      symbolSize: 4,
      data: rates,
      lineStyle: { width: 1 },
      itemStyle: { color: rateColors[index % rateColors.length] },
      emphasis: { focus: 'series' },
    })
  })

  empty.value = data.length === 0 || data.every((row) => row.totalUsers === 0)

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
              const isRate = String(item.seriesName).endsWith('占比')
              const text = isRate ? `${Number(item.value).toFixed(2)}%` : `${item.value}`
              return `${item.marker}${item.seriesName}: ${text}`
            })
          return [title, ...lines].join('<br/>')
        },
      },
      legend: [
        {
          data: props.dayNos.map((dayNo) => `第${dayNo}天`),
          orient: 'vertical',
          type: 'scroll',
          left: 8,
          top: 48,
          bottom: 48,
          itemWidth: 8,
          itemHeight: 8,
          itemGap: 8,
          icon: 'circle',
          textStyle: { fontSize: 12, color: '#595959' },
          selected: countSelected,
        },
        {
          data: props.dayNos.map((dayNo) => `第${dayNo}天占比`),
          orient: 'vertical',
          type: 'scroll',
          right: 8,
          top: 48,
          bottom: 48,
          itemWidth: 8,
          itemHeight: 8,
          itemGap: 8,
          icon: 'circle',
          textStyle: { fontSize: 12, color: '#595959' },
          selected: rateSelected,
        },
        {
          data: ['总数'],
          left: 'center',
          top: 8,
          itemWidth: 8,
          itemHeight: 8,
          icon: 'circle',
          textStyle: { fontSize: 12, color: '#595959' },
          selected: { 总数: true },
        },
      ],
      grid: {
        left: 120,
        right: 120,
        top: 52,
        bottom: labels.length > 15 ? 84 : 56,
      },
      xAxis: {
        type: 'category',
        data: labels,
        axisLabel: {
          fontSize: 11,
          rotate: labels.length > 15 ? 45 : 0,
          hideOverlap: true,
        },
      },
      yAxis: {
        type: 'value',
        min: 0,
      },
      dataZoom:
        labels.length > 30
          ? [
              { type: 'inside' },
              { type: 'slider', height: 14, bottom: 8 },
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

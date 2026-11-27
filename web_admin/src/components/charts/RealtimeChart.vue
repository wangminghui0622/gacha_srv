<template>
  <div class="realtime-chart-wrap">
    <div ref="chartEl" class="realtime-chart" />
    <div v-if="empty" class="chart-empty">暂无数据</div>
  </div>
</template>

<script setup lang="ts">
import * as echarts from 'echarts'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { RealtimeSeries } from '@/api/realtimeReport'

const props = defineProps<{
  labels: string[]
  series: RealtimeSeries[]
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
  '#69b1ff',
  '#95de64',
]

function render() {
  if (!chartEl.value) {
    return
  }
  if (!chart) {
    chart = echarts.init(chartEl.value)
  }
  const isMoney = props.series.length > 0 && props.series.every((item) => item.kind === 'money')
  const data = props.series.map((item) => ({
    name: item.name,
    values: item.values.map((value) => (isMoney ? value / 100 : value)),
  }))
  empty.value = data.length === 0 || data.every((item) => item.values.every((value) => value === 0))

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
            const value = isMoney ? `¥${Number(item.value).toFixed(2)}` : `${item.value}`
            return `${item.marker}${item.seriesName}: ${value}`
          })
          return [title, ...lines].join('<br/>')
        },
      },
      legend: {
        type: props.series.length > 6 ? 'scroll' : 'plain',
        top: 0,
        left: 0,
        right: 0,
        data: data.map((item) => item.name),
        icon: 'circle',
        itemWidth: 8,
        itemHeight: 8,
        textStyle: { fontSize: 12, color: '#595959' },
      },
      grid: {
        left: 56,
        right: 24,
        top: props.series.length > 6 ? 48 : 34,
        bottom: 36,
      },
      xAxis: {
        type: 'category',
        data: props.labels,
        axisLabel: {
          fontSize: 11,
          // 00:10、01:00、01:50 ... 每 50 分钟一个标签，和实时报表参考图一致。
          interval: (index: number) => index % 5 === 1,
        },
      },
      yAxis: {
        type: 'value',
        min: 0,
        axisLabel: { fontSize: 11 },
      },
      series: data.map((item, index) => ({
        name: item.name,
        type: 'line',
        smooth: false,
        symbol: 'none',
        data: item.values,
        lineStyle: { width: 1.3 },
        itemStyle: { color: colors[index % colors.length] },
        emphasis: { focus: 'series' },
      })),
    },
    true,
  )
  chart.resize()
}

watch(() => [props.labels, props.series], () => render(), { deep: true })

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
.realtime-chart-wrap {
  position: relative;
  width: 100%;
  height: 260px;
}

.realtime-chart {
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
  font-size: 13px;
  pointer-events: none;
}
</style>

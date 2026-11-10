<template>
  <div ref="chartEl" class="profit-chart" />
</template>

<script setup lang="ts">
import * as echarts from 'echarts'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserProfitCurvePoint } from '@/api/analytics'

const props = defineProps<{
  points: UserProfitCurvePoint[]
  loading?: boolean
}>()

const { t } = useI18n()
const chartEl = ref<HTMLDivElement | null>(null)
let chart: echarts.ECharts | null = null

function render() {
  if (!chartEl.value) {
    return
  }
  if (!chart) {
    chart = echarts.init(chartEl.value)
  }
  const labels = props.points.map((p) => p.bucketStart ?? '')
  const profit = props.points.map((p) => Number(p.profitMinor ?? 0) / 100)
  const spend = props.points.map((p) => Number(p.spendMinor ?? 0) / 100)
  chart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { data: [t('userProfit.profitLegend'), t('userProfit.spendLegend')] },
    grid: { left: 48, right: 24, top: 48, bottom: 48 },
    xAxis: { type: 'category', data: labels },
    yAxis: { type: 'value', name: 'CNY' },
    series: [
      { name: t('userProfit.profitLegend'), type: 'line', smooth: true, data: profit },
      { name: t('userProfit.spendLegend'), type: 'line', smooth: true, data: spend },
    ],
  })
  chart.resize()
}

watch(
  () => props.points,
  () => render(),
  { deep: true },
)

// 语言切换后图例需重绘（双语是刻意差异）。
watch(
  () => t('userProfit.profitLegend'),
  () => render(),
)

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
.profit-chart {
  width: 100%;
  height: 360px;
}
</style>

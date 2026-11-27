<template>
  <div class="revenue-chart-wrap">
    <div class="custom-legend">
      <div class="legend-scroll">
        <div class="legend-grid">
          <button
            v-for="item in legendItems"
            :key="item.name"
            type="button"
            class="legend-item"
            :class="{ 'is-selected': selectedMap[item.name] }"
            @click="toggle(item.name)"
          >
            <span
              class="legend-dot"
              :style="{ backgroundColor: selectedMap[item.name] ? seriesColor(item.index) : '#d9d9d9' }"
            />
            <span class="legend-text">{{ item.name }}</span>
          </button>
        </div>
      </div>
    </div>
    <div class="chart-box">
      <div ref="chartEl" class="revenue-chart" />
      <div v-if="empty" class="chart-empty">暂无可展示的营收数据</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import * as echarts from 'echarts'
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
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
  // 第 1 行：12 个核心指标
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
  // 第 2 行：10 个付费/渠道指标
  { name: '退款人数', axis: 0, kind: 'count', value: (r) => r.refundUsers },
  { name: '付费用户数', axis: 0, kind: 'count', value: (r) => r.payingUsers },
  { name: '付费用户数(新)', axis: 0, kind: 'count', value: (r) => r.newPayingUsers },
  { name: '付费用户数(旧)', axis: 0, kind: 'count', value: (r) => r.oldPayingUsers },
  { name: 'arpu', axis: 0, kind: 'money', value: (r) => r.arpuFen / 100 },
  { name: 'arppu', axis: 0, kind: 'money', value: (r) => r.arppuFen / 100 },
  { name: 'arpu(新)', axis: 0, kind: 'money', value: (r) => r.arpuNewFen / 100 },
  { name: 'arpu(旧)', axis: 0, kind: 'money', value: (r) => r.arpuOldFen / 100 },
  { name: '渗透率', axis: 1, kind: 'rate', value: (r) => r.penetrationBps / 100 },
  { name: '渗透率(新)', axis: 1, kind: 'rate', value: (r) => r.penetrationNewBps / 100 },
  // 第 3 行：剩余指标
  { name: '渗透率(旧)', axis: 1, kind: 'rate', value: (r) => r.penetrationOldBps / 100 },
  { name: '微信充值', axis: 0, kind: 'money', value: (r) => r.wechatPayFen / 100 },
  { name: '支付宝充值', axis: 0, kind: 'money', value: (r) => r.alipayPayFen / 100 },
  { name: '汇付充值', axis: 0, kind: 'money', value: (r) => r.huifuPayFen / 100 },
  { name: '退微信充值', axis: 0, kind: 'money', value: (r) => r.wechatRefundFen / 100 },
  { name: '退支付宝充值', axis: 0, kind: 'money', value: (r) => r.alipayRefundFen / 100 },
  { name: '退汇付充值', axis: 0, kind: 'money', value: (r) => r.huifuRefundFen / 100 },
  { name: '微信赠送', axis: 0, kind: 'money', value: (r) => r.wechatGiftFen / 100 },
  { name: '支付宝赠送', axis: 0, kind: 'money', value: (r) => r.alipayGiftFen / 100 },
  { name: '汇付赠送', axis: 0, kind: 'money', value: (r) => r.huifuGiftFen / 100 },
  { name: '退微信赠送', axis: 0, kind: 'money', value: (r) => r.wechatGiftRefundFen / 100 },
  { name: '退支付宝赠送', axis: 0, kind: 'money', value: (r) => r.alipayGiftRefundFen / 100 },
  { name: '退汇付赠送', axis: 0, kind: 'money', value: (r) => r.huifuGiftRefundFen / 100 },
  { name: '总退款(¥)', axis: 0, kind: 'money', value: (r) => r.refundAmountFen / 100 },
  { name: '退款(¥)用户数', axis: 0, kind: 'count', value: (r) => r.refundUsers },
  { name: '退款(¥)率', axis: 1, kind: 'rate', value: (r) => r.refundRateBps / 100 },
  { name: '总税收', axis: 0, kind: 'money', value: (r) => r.taxFen / 100 },
  { name: '税收(新)', axis: 0, kind: 'money', value: (r) => r.newTaxFen / 100 },
  { name: '税收(旧)', axis: 0, kind: 'money', value: (r) => r.oldTaxFen / 100 },
  { name: '总营收', axis: 0, kind: 'money', value: (r) => r.netRevenueFen / 100 },
  { name: '营收率', axis: 1, kind: 'rate', value: (r) => r.revenueRateBps / 100 },
  { name: '营收ARPU', axis: 0, kind: 'money', value: (r) => r.revenueArpuFen / 100 },
  { name: 'WalletBalance', axis: 0, kind: 'money', value: (r) => r.walletBalanceFen / 100 },
  { name: '余额余额', axis: 0, kind: 'money', value: (r) => r.walletBalanceFen / 100 },
  { name: '吉豆余额', axis: 0, kind: 'count', value: (r) => r.bonusBalanceFen },
  { name: '积分余额', axis: 0, kind: 'count', value: (r) => r.loyaltyPointBalance },
  { name: '吉祥值余额', axis: 0, kind: 'count', value: (r) => r.redeemPointBalance },
  { name: '总吉祥币付费', axis: 0, kind: 'money', value: (r) => r.platformCurrencyPayFen / 100 },
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

const defaultSelected: Record<string, boolean> = Object.fromEntries(
  defs.map((def) => [def.name, false]),
)
for (const name of ['注册数', '日活', '总付费', '付费用户数', '渗透率', '前1天流失率', 'arpu', '总营收']) {
  if (name in defaultSelected) {
    defaultSelected[name] = true
  }
}

const selectedMap = reactive<Record<string, boolean>>({ ...defaultSelected })

const legendItems = computed(() => defs.map((def, index) => ({ name: def.name, index })))

function seriesColor(index: number): string {
  if (index < colors.length) {
    return colors[index] ?? '#1677ff'
  }
  return `hsl(${(index * 47) % 360}, 65%, 50%)`
}

function toggle(name: string) {
  selectedMap[name] = !selectedMap[name]
  chart?.setOption({ legend: { selected: { ...selectedMap } } })
}

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
        show: false,
        data: defs.map((def) => def.name),
        selected: { ...selectedMap },
      },
      grid: { left: 64, right: 64, top: 48, bottom: labels.length > 15 ? 84 : 56 },
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
        itemStyle: { color: seriesColor(index) },
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
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.custom-legend {
  width: 100%;
}

.legend-scroll {
  width: 100%;
  overflow: visible;
}

.legend-grid {
  display: grid;
  grid-template-columns: repeat(10, minmax(0, 1fr));
  column-gap: 6px;
  row-gap: 6px;
  width: 100%;
}

.legend-item {
  display: inline-flex;
  align-items: flex-start;
  gap: 4px;
  width: 100%;
  min-width: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: #8c8c8c;
  font-size: 14px;
  line-height: 20px;
  text-align: left;
  white-space: nowrap;
  cursor: pointer;
}

.legend-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.legend-item.is-selected {
  color: #262626;
}

.legend-dot {
  flex: 0 0 auto;
  width: 8px;
  height: 8px;
  margin-top: 6px;
  border-radius: 50%;
}

.chart-box {
  position: relative;
  width: 100%;
  height: 340px;
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

<template>
  <div class="dashboard-page">
    <div class="metric-grid cols-7">
      <div v-for="m in row1" :key="m.label" class="metric-card" :class="`tone-${m.tone}`">
        <div class="metric-label">{{ m.label }}</div>
        <div class="metric-value">{{ m.value }}<span v-if="m.extra" class="metric-extra" :class="m.extraClass">{{ m.extra }}</span></div>
        <div class="metric-foot"><span>{{ m.footLeft }}</span><span>{{ m.footRight }}</span></div>
      </div>
    </div>

    <div class="metric-grid cols-5">
      <div v-for="m in row2" :key="m.label" class="metric-card" :class="`tone-${m.tone}`">
        <div class="metric-label">{{ m.label }}</div>
        <div class="metric-value">{{ m.value }}<span v-if="m.extra" class="metric-extra" :class="m.extraClass">{{ m.extra }}</span></div>
        <div class="metric-foot"><span>{{ m.footLeft }}</span><span>{{ m.footRight }}</span></div>
      </div>
    </div>

    <div class="metric-grid cols-5">
      <div v-for="m in row3" :key="m.label" class="metric-card" :class="`tone-${m.tone}`">
        <div class="metric-label">{{ m.label }}</div>
        <div class="metric-value">{{ m.value }}<span v-if="m.extra" class="metric-extra" :class="m.extraClass">{{ m.extra }}</span></div>
        <div class="metric-foot"><span>{{ m.footLeft }}</span><span>{{ m.footRight }}</span></div>
      </div>
    </div>

    <a-table
      class="daily-table"
      :data-source="rows"
      :columns="columns"
      :pagination="false"
      row-key="date"
      size="small"
      :scroll="{ x: 1600 }"
    />
  </div>
</template>

<script setup lang="ts">
interface MetricCard {
  label: string
  value: string
  extra?: string
  extraClass?: string
  footLeft?: string
  footRight?: string
  tone: 'blue' | 'green' | 'yellow' | 'gray'
}

const row1: MetricCard[] = [
  { label: '总充值', value: '0', footRight: '昨天 0', tone: 'blue' },
  { label: '总充值(汇付)', value: '0', footRight: '昨天 0', tone: 'blue' },
  { label: '总充值(支付宝)', value: '0', footRight: '昨天 0', tone: 'blue' },
  { label: '退款(汇付)', value: '0', footRight: '昨天 0', tone: 'gray' },
  { label: '退款(支付宝)', value: '0', footRight: '昨天 0', tone: 'gray' },
  { label: '总退款(¥)', value: '0', footRight: '昨天 0', tone: 'yellow' },
  { label: '注册数', value: '0', footRight: '昨天 0', tone: 'green' },
]

const row2: MetricCard[] = [
  { label: '日活', value: '1', extra: '-2', extraClass: 'is-negative', footRight: '昨天 3', tone: 'green' },
  { label: '充值用户数', value: '0', footLeft: '新 0 / 旧 0', footRight: '昨天 0', tone: 'blue' },
  { label: '参与率(新)', value: '0%', extra: '0%', footRight: '昨天 0%', tone: 'blue' },
  { label: '累计注册', value: '4', footRight: '昨天 0', tone: 'green' },
  { label: '累计充值用户数', value: '6', footRight: '昨天 0', tone: 'blue' },
]

const row3: MetricCard[] = [
  { label: '新增订单数', value: '0', footRight: '昨天 0', tone: 'gray' },
  { label: '累计订单数', value: '29', footRight: '昨天 0', tone: 'gray' },
  { label: '成交金额(普通用户)', value: '0', footRight: '昨天 0', tone: 'gray' },
  { label: '成交金额(祥子)', value: '0', footRight: '昨天 0', tone: 'gray' },
  { label: '成交金额(客服)', value: '0', footRight: '昨天 0', tone: 'gray' },
]

interface DailyRow {
  date: string
  totalRecharge: string
  rechargeHuifu: string
  rechargeAlipay: string
  refundAlipay: string
  refundHuifu: string
  totalRefund: string
  registrations: number
  activeUsers: number
  rechargeUsers: number
  newOrders: number
  gmvNormal: string
  gmvXiangzi: string
  gmvService: string
}

const rows: DailyRow[] = [
  { date: '2026-11-13', totalRecharge: '0.00', rechargeHuifu: '0.00', rechargeAlipay: '0.00', refundAlipay: '0.00', refundHuifu: '0.00', totalRefund: '0.00', registrations: 0, activeUsers: 1, rechargeUsers: 0, newOrders: 0, gmvNormal: '0.00', gmvXiangzi: '0.00', gmvService: '0.00' },
  { date: '2026-11-12', totalRecharge: '0.00', rechargeHuifu: '0.00', rechargeAlipay: '0.00', refundAlipay: '0.00', refundHuifu: '0.00', totalRefund: '0.00', registrations: 0, activeUsers: 3, rechargeUsers: 0, newOrders: 0, gmvNormal: '0.00', gmvXiangzi: '0.00', gmvService: '0.00' },
  { date: '2026-11-11', totalRecharge: '0.00', rechargeHuifu: '0.00', rechargeAlipay: '0.00', refundAlipay: '0.00', refundHuifu: '0.00', totalRefund: '0.00', registrations: 1, activeUsers: 3, rechargeUsers: 0, newOrders: 0, gmvNormal: '0.00', gmvXiangzi: '0.00', gmvService: '0.00' },
  { date: '2026-11-10', totalRecharge: '0.00', rechargeHuifu: '0.00', rechargeAlipay: '0.00', refundAlipay: '0.00', refundHuifu: '0.00', totalRefund: '0.00', registrations: 0, activeUsers: 2, rechargeUsers: 0, newOrders: 0, gmvNormal: '0.00', gmvXiangzi: '0.00', gmvService: '0.00' },
  { date: '2026-11-09', totalRecharge: '0.00', rechargeHuifu: '0.00', rechargeAlipay: '0.00', refundAlipay: '0.00', refundHuifu: '0.00', totalRefund: '0.00', registrations: 0, activeUsers: 3, rechargeUsers: 0, newOrders: 0, gmvNormal: '0.00', gmvXiangzi: '0.00', gmvService: '0.00' },
  { date: '2026-11-08', totalRecharge: '0.00', rechargeHuifu: '0.00', rechargeAlipay: '0.00', refundAlipay: '0.00', refundHuifu: '0.00', totalRefund: '0.00', registrations: 0, activeUsers: 0, rechargeUsers: 0, newOrders: 0, gmvNormal: '0.00', gmvXiangzi: '0.00', gmvService: '0.00' },
  { date: '2026-11-07', totalRecharge: '0.00', rechargeHuifu: '0.00', rechargeAlipay: '0.00', refundAlipay: '0.00', refundHuifu: '0.00', totalRefund: '0.00', registrations: 0, activeUsers: 0, rechargeUsers: 0, newOrders: 0, gmvNormal: '0.00', gmvXiangzi: '0.00', gmvService: '0.00' },
  { date: '2026-11-06', totalRecharge: '0.00', rechargeHuifu: '0.00', rechargeAlipay: '0.00', refundAlipay: '0.00', refundHuifu: '0.00', totalRefund: '0.00', registrations: 0, activeUsers: 1, rechargeUsers: 0, newOrders: 0, gmvNormal: '0.00', gmvXiangzi: '0.00', gmvService: '0.00' },
  { date: '2026-11-05', totalRecharge: '0.00', rechargeHuifu: '0.00', rechargeAlipay: '0.00', refundAlipay: '0.00', refundHuifu: '0.00', totalRefund: '0.00', registrations: 0, activeUsers: 3, rechargeUsers: 0, newOrders: 0, gmvNormal: '0.00', gmvXiangzi: '0.00', gmvService: '0.00' },
  { date: '2026-11-04', totalRecharge: '0.00', rechargeHuifu: '0.00', rechargeAlipay: '0.00', refundAlipay: '0.00', refundHuifu: '0.00', totalRefund: '0.00', registrations: 0, activeUsers: 2, rechargeUsers: 0, newOrders: 0, gmvNormal: '0.00', gmvXiangzi: '0.00', gmvService: '0.00' },
]

const columns = [
  { title: '日期', dataIndex: 'date', key: 'date', width: 110, fixed: 'left' },
  { title: '总充值', dataIndex: 'totalRecharge', key: 'totalRecharge', align: 'right' },
  { title: '总充值(汇付)', dataIndex: 'rechargeHuifu', key: 'rechargeHuifu', align: 'right' },
  { title: '总充值(支付宝)', dataIndex: 'rechargeAlipay', key: 'rechargeAlipay', align: 'right' },
  { title: '退款(支付宝)', dataIndex: 'refundAlipay', key: 'refundAlipay', align: 'right' },
  { title: '退款(汇付)', dataIndex: 'refundHuifu', key: 'refundHuifu', align: 'right' },
  { title: '总退款(¥)', dataIndex: 'totalRefund', key: 'totalRefund', align: 'right' },
  { title: '注册数', dataIndex: 'registrations', key: 'registrations', align: 'right' },
  { title: '日活', dataIndex: 'activeUsers', key: 'activeUsers', align: 'right' },
  { title: '充值用户数', dataIndex: 'rechargeUsers', key: 'rechargeUsers', align: 'right' },
  { title: '新增订单数', dataIndex: 'newOrders', key: 'newOrders', align: 'right' },
  { title: '成交金额(普通用户)', dataIndex: 'gmvNormal', key: 'gmvNormal', align: 'right' },
  { title: '成交金额(祥子)', dataIndex: 'gmvXiangzi', key: 'gmvXiangzi', align: 'right' },
  { title: '成交金额(客服)', dataIndex: 'gmvService', key: 'gmvService', align: 'right' },
]
</script>

<style scoped lang="scss">
.dashboard-page {
  padding-bottom: 16px;
}

.metric-grid {
  display: grid;
  gap: 16px;
  margin-bottom: 16px;
}

.cols-7 {
  grid-template-columns: repeat(7, minmax(0, 1fr));
}

.cols-5 {
  grid-template-columns: repeat(5, minmax(0, 1fr));
}

.metric-card {
  position: relative;
  background: #fff;
  border: 1px solid #ebedf0;
  border-bottom: 3px solid #d9d9d9;
  border-radius: 4px;
  padding: 14px 16px 10px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03);
  min-height: 100px;
}

.metric-card.tone-blue { border-bottom-color: #1677ff; }
.metric-card.tone-green { border-bottom-color: #52c41a; }
.metric-card.tone-yellow { border-bottom-color: #faad14; }
.metric-card.tone-gray { border-bottom-color: #d9d9d9; }

.metric-label {
  color: rgba(0, 0, 0, 0.55);
  font-size: 13px;
  line-height: 20px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.metric-value {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-top: 4px;
  font-size: 32px;
  font-weight: 600;
  line-height: 40px;
  color: rgba(0, 0, 0, 0.88);
}

.metric-extra {
  font-size: 14px;
  font-weight: 400;
  color: rgba(0, 0, 0, 0.45);
}

.metric-extra.is-negative {
  color: #ff4d4f;
}

.metric-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  border-top: 1px solid #f2f3f5;
  margin-top: 6px;
  padding-top: 6px;
  color: rgba(0, 0, 0, 0.45);
  font-size: 12px;
  white-space: nowrap;
}

.daily-table {
  margin-top: 8px;
}

:deep(.daily-table .ant-table-thead > tr > th) {
  background: #fafafa;
  color: rgba(0, 0, 0, 0.65);
  font-weight: 500;
  font-size: 12px;
}

:deep(.daily-table .ant-table-tbody > tr > td) {
  font-size: 12px;
  color: rgba(0, 0, 0, 0.65);
}

@media (max-width: 1500px) {
  .cols-7 { grid-template-columns: repeat(4, minmax(0, 1fr)); }
  .cols-5 { grid-template-columns: repeat(3, minmax(0, 1fr)); }
}

@media (max-width: 900px) {
  .cols-7,
  .cols-5 { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (max-width: 600px) {
  .cols-7,
  .cols-5 { grid-template-columns: 1fr; }
}
</style>

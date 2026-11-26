<template>
  <div class="refund-page console-list-page">
    <div class="auto-bar">
      <div class="auto-item">
        <span>自动退款剩余金额：</span><b>{{ money(autoConfig.remainingAmountFen) }}</b>
        <a-button class="auto-edit" type="link" size="small" @click="openConfig">修改</a-button>
      </div>
      <div class="auto-item">
        <span>自动退款审核金额上限(含)：</span><b>{{ money(autoConfig.auditLimitFen) }}</b>
        <a-button class="auto-edit" type="link" size="small" @click="openConfig">修改</a-button>
      </div>
      <div class="auto-item">
        <span>自动退款延迟时间(秒)：</span><b>{{ Number(autoConfig.delaySeconds ?? 0) }}</b>
        <a-button class="auto-edit" type="link" size="small" @click="openConfig">修改</a-button>
      </div>
      <div class="auto-item">
        <span>自动交替退款次数：</span><b>{{ Number(autoConfig.alternateCount ?? 0) }}</b>
        <a-button class="auto-edit" type="link" size="small" @click="openConfig">修改</a-button>
      </div>
      <div class="auto-item">
        <span>自动交替退款金额(≤)：</span><b>{{ money(autoConfig.alternateAmountFen) }}</b>
        <a-button class="auto-edit" type="link" size="small" @click="openConfig">修改</a-button>
      </div>
    </div>

    <div class="refund-toolbar">
      <a-input v-model:value="filters.keyword" class="w-220" placeholder="用户ID/退款账号/退款..." allow-clear @press-enter="search" />
      <a-input v-model:value="filters.auditRemark" class="w-200" placeholder="审核备注" allow-clear @press-enter="search" />
      <a-select v-model:value="filters.status" class="w-140" placeholder="订单状态" :options="statusOptions" allow-clear />
      <a-input-number v-model:value="filters.minAmountYuan" class="w-110" :min="0" :precision="2" placeholder="最小金额" />
      <a-input-number v-model:value="filters.maxAmountYuan" class="w-110" :min="0" :precision="2" placeholder="最大金额" />
      <a-select v-model:value="filters.timeField" class="w-130" :options="timeFieldOptions" />
      <a-range-picker v-model:value="filters.range" class="w-260" />
      <a-button class="btn-query" type="primary" :loading="loading" @click="search">查 询</a-button>
      <a-button class="btn-plain" :loading="loading" @click="exportCsv">导 出</a-button>
    </div>

    <div class="table-fill">
      <a-table
      :data-source="items"
      :loading="loading"
      :pagination="false"
      :row-selection="rowSelection"
      :row-key="(record: RefundOrder) => String(record.dbId ?? '')"
      :scroll="{ x: 1586 }"
      size="small"
    >
      <a-table-column title="ID" :width="140" align="center">
        <template #default="{ record }">{{ record.orderNo || '-' }}</template>
      </a-table-column>
      <a-table-column title="用户ID" :width="45" align="center">
        <template #default="{ record }">{{ record.userId }}</template>
      </a-table-column>
      <a-table-column title="用户昵称" :width="62" ellipsis align="center">
        <template #default="{ record }">{{ record.userNickname || '-' }}</template>
      </a-table-column>
      <a-table-column title="真实姓名" :width="64" ellipsis align="center">
        <template #default="{ record }">{{ record.realName || '-' }}</template>
      </a-table-column>
      <a-table-column title="支付宝账号" :width="95" ellipsis align="center">
        <template #default="{ record }"><span :title="record.alipayAccount || ''">{{ record.alipayAccount || '-' }}</span></template>
      </a-table-column>
      <a-table-column title="退款金额" :width="75" align="center">
        <template #default="{ record }">{{ money(record.amountFen) }}</template>
      </a-table-column>
      <a-table-column title="状态" :width="70" align="center">
        <template #default="{ record }">
          <a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag>
        </template>
      </a-table-column>
      <a-table-column title="创建时间" :width="136" align="center">
        <template #default="{ record }">{{ formatTime(record.createdAtUnixMillis) }}</template>
      </a-table-column>
      <a-table-column title="完成时间" :width="136" align="center">
        <template #default="{ record }">{{ formatTime(record.completedAtUnixMillis) }}</template>
      </a-table-column>
      <a-table-column title="第三方订单ID" :width="125" ellipsis align="center">
        <template #default="{ record }"><span :title="record.thirdPartyOrderId || ''">{{ record.thirdPartyOrderId || '--' }}</span></template>
      </a-table-column>
      <a-table-column title="退款商户ID" :width="110" ellipsis align="center">
        <template #default="{ record }"><span :title="record.refundMerchantId || ''">{{ record.refundMerchantId || '--' }}</span></template>
      </a-table-column>
      <a-table-column title="审核人" :width="80" ellipsis align="center">
        <template #default="{ record }">{{ record.auditor || '--' }}</template>
      </a-table-column>
      <a-table-column title="财务审核人" :width="80" ellipsis align="center">
        <template #default="{ record }">{{ record.financeAuditor || '--' }}</template>
      </a-table-column>
      <a-table-column title="审核备注" :width="90" ellipsis align="center">
        <template #default="{ record }"><span :title="record.auditRemark || ''">{{ record.auditRemark || '--' }}</span></template>
      </a-table-column>
      <a-table-column title="错误信息" :width="90" ellipsis align="center">
        <template #default="{ record }"><span :title="record.errorMessage || ''">{{ record.errorMessage || '--' }}</span></template>
      </a-table-column>
      <a-table-column title="操作" :width="156" align="center" fixed="right">
        <template #default="{ record }">
          <a-space v-if="canWrite" :size="4" wrap>
            <a-button v-if="statusCode(record.status) === 1" class="btn-audit" size="small" @click="openAudit(record)">审核</a-button>
            <a-button
              v-if="statusCode(record.status) === 1 || statusCode(record.status) === 5"
              class="btn-force"
              size="small"
              @click="operate(record, 2, '被迫退款')"
            >
              被迫退款
            </a-button>
            <a-button v-if="statusCode(record.status) === 5" size="small" @click="operate(record, 3, '人工处理')">人工处理</a-button>
            <a-button v-if="statusCode(record.status) === 5" class="btn-retry" size="small" @click="operate(record, 4, '重试退款')">重试退款</a-button>
            <span v-if="![1, 5].includes(statusCode(record.status))">-</span>
          </a-space>
          <span v-else>-</span>
        </template>
      </a-table-column>
    </a-table>
    </div>

    <div class="refund-footer console-list-footer">
      <a-pagination
        :current="page"
        :page-size="pageSize"
        :total="total"
        :page-size-options="pageSizeOptions"
        :locale="{ items_per_page: '条/页' }"
        show-size-changer
        show-less-items
        @change="onPageChange"
        @show-size-change="onPageSizeChange"
      />
    </div>

    <a-modal
      v-if="auditOpen"
      v-model:open="auditOpen"
      title="审核退款订单"
      centered
      :width="520"
      :confirm-loading="saving"
      ok-text="通过"
      cancel-text="取消"
      @ok="submitAudit"
    >
      <a-descriptions :column="1" size="small" bordered>
        <a-descriptions-item label="退款订单">{{ editing?.orderNo }}</a-descriptions-item>
        <a-descriptions-item label="用户">{{ editing?.userNickname || '-' }}（{{ editing?.userId }}）</a-descriptions-item>
        <a-descriptions-item label="退款金额">{{ money(editing?.amountFen) }}</a-descriptions-item>
        <a-descriptions-item label="支付宝账号">{{ editing?.alipayAccount || '-' }}</a-descriptions-item>
      </a-descriptions>
      <a-form layout="vertical" style="margin-top: 16px">
        <a-form-item label="审核备注">
          <a-textarea v-model:value="auditRemark" :rows="3" placeholder="审核备注" />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal
      v-if="configOpen"
      v-model:open="configOpen"
      title="自动退款配置"
      centered
      :width="560"
      :confirm-loading="saving"
      ok-text="保存"
      cancel-text="取消"
      @ok="saveConfig"
    >
      <a-form :label-col="{ span: 9 }" :wrapper-col="{ span: 14 }">
        <a-form-item label="自动退款剩余金额">
          <a-input-number v-model:value="configForm.remainingAmountYuan" :min="0" :precision="2" style="width: 100%" />
        </a-form-item>
        <a-form-item label="自动退款审核金额上限(含)">
          <a-input-number v-model:value="configForm.auditLimitYuan" :min="0" :precision="2" style="width: 100%" />
        </a-form-item>
        <a-form-item label="自动退款延迟时间(秒)">
          <a-input-number v-model:value="configForm.delaySeconds" :min="0" :precision="0" style="width: 100%" />
        </a-form-item>
        <a-form-item label="自动交替退款次数">
          <a-input-number v-model:value="configForm.alternateCount" :min="0" :precision="0" style="width: 100%" />
        </a-form-item>
        <a-form-item label="自动交替退款金额(≤)">
          <a-input-number v-model:value="configForm.alternateAmountYuan" :min="0" :precision="2" style="width: 100%" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message, Modal } from 'ant-design-vue'
import dayjs, { type Dayjs } from 'dayjs'

import { allows } from '@/access/grants'
import {
  refundOrderClient,
  REFUND_ORDER_ACTIONS,
  type RefundAutoConfig,
  type RefundOrder,
} from '@/api/refund'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const canRead = computed(() => allows(session.permissionNodes, 'payment.order', 'read'))
const canWrite = computed(() => allows(session.permissionNodes, 'payment.order', 'write'))

const statusOptions = [
  { label: '待审核', value: 1 },
  { label: '审核通过', value: 2 },
  { label: '已成功', value: 3 },
  { label: '已驳回', value: 4 },
  { label: '退款异常', value: 5 },
  { label: '人工处理', value: 6 },
]
const timeFieldOptions = [
  { label: '创建时间', value: 'created' },
  { label: '完成时间', value: 'completed' },
]

const filters = reactive({
  keyword: '',
  auditRemark: '',
  status: undefined as number | undefined,
  minAmountYuan: undefined as number | undefined,
  maxAmountYuan: undefined as number | undefined,
  timeField: 'created',
  range: null as [Dayjs, Dayjs] | null,
})

const items = ref<RefundOrder[]>([])
const page = ref(1)
const pageSize = ref(15)
const pageSizeOptions = ['15', '30', '50', '100']
const total = ref(0)
const loading = ref(false)
const saving = ref(false)
const selectedRowKeys = ref<string[]>([])

const rowSelection = computed(() => ({
  selectedRowKeys: selectedRowKeys.value,
  onChange: (keys: (string | number)[]) => {
    selectedRowKeys.value = keys.map((key) => String(key))
  },
}))

const autoConfig = ref<RefundAutoConfig>({})
const configOpen = ref(false)
const configForm = reactive({
  remainingAmountYuan: 0,
  auditLimitYuan: 0,
  delaySeconds: 0,
  alternateCount: 0,
  alternateAmountYuan: 0,
})

const auditOpen = ref(false)
const editing = ref<RefundOrder | null>(null)
const auditRemark = ref('')

async function load() {
  if (!canRead.value) return
  loading.value = true
  try {
    const reply = await refundOrderClient().List({
      keyword: filters.keyword.trim(),
      auditRemark: filters.auditRemark.trim(),
      status: filters.status ?? 0,
      minAmountFen: filters.minAmountYuan ? Math.round(filters.minAmountYuan * 100) : 0,
      maxAmountFen: filters.maxAmountYuan ? Math.round(filters.maxAmountYuan * 100) : 0,
      timeField: filters.timeField,
      startAtUnixMillis: filters.range ? filters.range[0].startOf('day').valueOf() : 0,
      endAtUnixMillis: filters.range ? filters.range[1].endOf('day').valueOf() : 0,
      page: page.value,
      pageSize: pageSize.value,
    })
    items.value = reply.items ?? []
    total.value = Number(reply.total ?? 0)
    selectedRowKeys.value = []
  } catch {
    items.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

async function loadConfig() {
  try {
    const reply = await refundOrderClient().GetAutoConfig()
    autoConfig.value = reply.config ?? {}
  } catch {
    autoConfig.value = {}
  }
}

function search() {
  page.value = 1
  void load()
}

function onPageChange(nextPage: number) {
  page.value = nextPage
  void load()
}

function onPageSizeChange(_current: number, nextPageSize: number) {
  pageSize.value = nextPageSize
  page.value = 1
  void load()
}

function statusCode(status?: string): number {
  switch (status) {
    case 'REFUND_ORDER_STATUS_PENDING': return 1
    case 'REFUND_ORDER_STATUS_APPROVED': return 2
    case 'REFUND_ORDER_STATUS_SUCCESS': return 3
    case 'REFUND_ORDER_STATUS_REJECTED': return 4
    case 'REFUND_ORDER_STATUS_EXCEPTION': return 5
    case 'REFUND_ORDER_STATUS_MANUAL': return 6
    default: return 0
  }
}

function statusText(status?: string): string {
  return statusOptions.find((item) => item.value === statusCode(status))?.label ?? '-'
}

function statusColor(status?: string): string {
  switch (statusCode(status)) {
    case 1: return 'orange'
    case 2: return 'blue'
    case 3: return 'green'
    case 4: return 'default'
    case 5: return 'red'
    case 6: return 'purple'
    default: return 'default'
  }
}

function money(value?: string | number): string {
  return `¥${(Number(value ?? 0) / 100).toFixed(2)}`
}

function formatTime(value?: string | number): string {
  const n = Number(value ?? 0)
  if (!n) return '--'
  return dayjs(n).format('YYYY-MM-DD HH:mm:ss')
}

function openAudit(record: RefundOrder) {
  editing.value = record
  auditRemark.value = record.auditRemark ?? ''
  auditOpen.value = true
}

async function submitAudit() {
  if (!editing.value?.dbId) return
  saving.value = true
  try {
    await refundOrderClient().Operate(editing.value.dbId, REFUND_ORDER_ACTIONS.APPROVE, auditRemark.value.trim())
    message.success('审核通过')
    auditOpen.value = false
    void load()
  } catch {
    message.error('审核失败')
  } finally {
    saving.value = false
  }
}

function operate(record: RefundOrder, action: number, title: string) {
  if (!record.dbId) return
  Modal.confirm({
    title,
    content: `确认对退款订单「${record.orderNo}」执行${title}？`,
    okText: '确认',
    cancelText: '取消',
    okType: action === REFUND_ORDER_ACTIONS.FORCE_REFUND ? 'danger' : 'primary',
    onOk: async () => {
      await refundOrderClient().Operate(record.dbId as string, action)
      message.success(`${title}完成`)
      void load()
    },
  })
}

function openConfig() {
  configForm.remainingAmountYuan = Number(autoConfig.value.remainingAmountFen ?? 0) / 100
  configForm.auditLimitYuan = Number(autoConfig.value.auditLimitFen ?? 0) / 100
  configForm.delaySeconds = Number(autoConfig.value.delaySeconds ?? 0)
  configForm.alternateCount = Number(autoConfig.value.alternateCount ?? 0)
  configForm.alternateAmountYuan = Number(autoConfig.value.alternateAmountFen ?? 0) / 100
  configOpen.value = true
}

async function saveConfig() {
  saving.value = true
  try {
    const reply = await refundOrderClient().UpdateAutoConfig({
      remainingAmountFen: String(Math.round(configForm.remainingAmountYuan * 100)),
      auditLimitFen: String(Math.round(configForm.auditLimitYuan * 100)),
      delaySeconds: String(Math.round(configForm.delaySeconds)),
      alternateCount: Math.round(configForm.alternateCount),
      alternateAmountFen: String(Math.round(configForm.alternateAmountYuan * 100)),
    })
    autoConfig.value = reply.config ?? autoConfig.value
    message.success('自动退款配置已保存')
    configOpen.value = false
  } catch {
    message.error('保存失败')
  } finally {
    saving.value = false
  }
}

function csvEscape(value: unknown): string {
  const text = String(value ?? '')
  if (/[",\n]/.test(text)) return `"${text.replace(/"/g, '""')}"`
  return text
}

function exportCsv() {
  const source = selectedRowKeys.value.length
    ? items.value.filter((item) => selectedRowKeys.value.includes(String(item.dbId)))
    : items.value
  if (!source.length) {
    message.warning('暂无可导出的数据')
    return
  }
  const header = ['ID', '用户ID', '用户昵称', '真实姓名', '支付宝账号', '退款金额', '状态', '创建时间', '完成时间', '第三方订单ID', '退款商户ID', '审核人', '财务审核人', '审核备注', '错误信息']
  const rows = source.map((record) => [
    record.orderNo ?? '',
    record.userId ?? '',
    record.userNickname ?? '',
    record.realName ?? '',
    record.alipayAccount ?? '',
    money(record.amountFen),
    statusText(record.status),
    formatTime(record.createdAtUnixMillis),
    formatTime(record.completedAtUnixMillis),
    record.thirdPartyOrderId ?? '',
    record.refundMerchantId ?? '',
    record.auditor ?? '',
    record.financeAuditor ?? '',
    record.auditRemark ?? '',
    record.errorMessage ?? '',
  ])
  const csv = '\uFEFF' + [header, ...rows].map((row) => row.map(csvEscape).join(',')).join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `退款订单_${dayjs().format('YYYYMMDD_HHmmss')}.csv`
  link.click()
  URL.revokeObjectURL(url)
}

onMounted(() => {
  void load()
  void loadConfig()
})
</script>

<style scoped>
.refund-page {
  height: calc(100vh - 132px);
  overflow: hidden;
  padding: 14px 16px 18px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}

.refund-page :deep(.ant-table-small .ant-table-thead > tr > th),
.refund-page :deep(.ant-table-small .ant-table-tbody > tr > td) {
  padding: 6px 4px;
  white-space: nowrap;
}

.refund-page :deep(.ant-tag) {
  margin-inline-end: 0;
  padding-inline: 4px;
  line-height: 18px;
}

.auto-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  padding: 8px 0 14px;
  font-size: 13px;
  color: #303133;
}

.auto-item {
  display: inline-flex;
  align-items: center;
  height: 34px;
  overflow: hidden;
  padding-left: 12px;
  background: #fff;
  border: 1px solid #d9d9d9;
  border-radius: 6px;
}

.auto-item b {
  margin-right: 4px;
  color: #cf1322;
}

.auto-item :deep(.auto-edit) {
  height: 32px;
  margin-left: 4px;
  padding: 0 12px;
  border-left: 1px solid #f0f0f0;
  border-radius: 0;
}

.refund-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
}

.w-110 { width: 110px; }
.w-130 { width: 130px; }
.w-140 { width: 140px; }
.w-200 { width: 200px; }
.w-220 { width: 220px; }
.w-260 { width: 260px; }

.btn-query {
  background: #42b983;
  border-color: #42b983;
}

.btn-query:hover {
  background: #36a374 !important;
  border-color: #36a374 !important;
}

.btn-plain {
  background: #fff;
  border-color: #d9d9d9;
}

.btn-audit {
  color: #fff;
  background: #ff9f1c;
  border-color: #ff9f1c;
}

.btn-force {
  color: #4096ff;
  border-color: #4096ff;
}

.btn-retry {
  color: #ff4d4f;
  border-color: #ff4d4f;
}

.refund-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>

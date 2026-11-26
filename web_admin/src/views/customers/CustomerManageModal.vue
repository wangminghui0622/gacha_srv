<template>
  <div class="customer-manage-body">
    <a-spin :spinning="loading">
      <a-tabs v-model:activeKey="activeTab">
        <a-tab-pane key="basic" tab="基本信息">
          <a-form :label-col="{ span: 4 }" :wrapper-col="{ span: 20 }" label-align="right">
            <a-form-item label="对内ID">
              <a-input class="user-id-input" :value="internalUserId" disabled />
            </a-form-item>
            <a-form-item label="对外ID">
              <a-input class="user-id-input" :value="publicUserId" disabled />
            </a-form-item>
            <a-form-item label="昵称" required>
              <a-input v-model:value="form.nickname" :maxlength="32" placeholder="请输入昵称" allow-clear />
            </a-form-item>
            <a-form-item label="状态">
              <a-select v-model:value="form.status" :options="statusOptions" />
            </a-form-item>
            <a-form-item label="集市展示状态">
              <a-select v-model:value="form.luckyDisplay" :options="luckyDisplayOptions" />
            </a-form-item>
            <a-form-item label="上级用户对内ID">
              <a-input-number
                v-model:value="form.parentUserId"
                :min="0"
                :precision="0"
                :controls="false"
                placeholder="请输入上级用户对内ID"
                style="width: 100%"
              />
            </a-form-item>
            <a-form-item label="是否补发欧气值">
              <a-select v-model:value="form.reissue" :options="reissueOptions" />
            </a-form-item>
            <a-form-item label="头像" required>
              <div class="avatar-upload-wrap">
                <a-upload :show-upload-list="false" accept="image/*" :before-upload="handleAvatarUpload">
                  <div class="avatar-upload-box">
                    <img v-if="form.avatarUrl" :src="form.avatarUrl" alt="" />
                    <template v-else>
                      <span class="upload-plus">+</span>
                      <span>Upload</span>
                    </template>
                  </div>
                </a-upload>
                <button
                  v-if="form.avatarUrl"
                  type="button"
                  class="avatar-remove"
                  title="移除头像"
                  aria-label="移除头像"
                  @click.stop.prevent="form.avatarUrl = ''"
                >
                  <CloseOutlined />
                </button>
              </div>
            </a-form-item>
            <a-form-item label="返佣比例">
              <div class="rebate-row">
                <a-input-number
                  v-model:value="form.rebateRate"
                  :min="0"
                  :max="100"
                  :precision="0"
                  :controls="false"
                  style="width: 120px"
                />
                <span class="percent">%</span>
              </div>
            </a-form-item>
            <a-form-item label="身份">
              <a-select v-model:value="form.identity" :options="identityOptions" />
            </a-form-item>
          </a-form>
          <div class="manage-actions">
            <a-button type="primary" :loading="saving" @click="save">保存基本信息</a-button>
          </div>
        </a-tab-pane>

        <a-tab-pane key="finance" tab="财务管理">
          <a-spin :spinning="financeLoading">
            <div class="finance-card">
              <div class="finance-card-title">钱包余额</div>
              <a-table
                :data-source="walletRows"
                :pagination="false"
                :row-key="(record: WalletRow) => String(record.kind)"
                size="middle"
                bordered
              >
                <a-table-column title="钱包" data-index="label" align="center" />
                <a-table-column title="余额" data-index="balance" align="center" />
                <a-table-column title="状态" align="center" width="120">
                  <template #default="{ record }">
                    <a-tag :color="record.enabled ? 'green' : 'default'">{{ record.enabled ? '开启' : '关闭' }}</a-tag>
                  </template>
                </a-table-column>
                <a-table-column title="操作" align="center" width="110">
                  <template #default="{ record }">
                    <a-button v-if="record.enabled" size="small" @click="toggleWallet(record)">关闭</a-button>
                    <a-button v-else size="small" type="primary" @click="toggleWallet(record)">开启</a-button>
                  </template>
                </a-table-column>
              </a-table>
            </div>

            <div class="finance-card">
              <div class="finance-card-title">充值 / 加币</div>
              <a-form layout="vertical">
                <a-row :gutter="24">
                  <a-col :xs="24" :md="12">
                    <a-form-item label="金额">
                      <a-input v-model:value="rechargeAmount" placeholder="0" />
                    </a-form-item>
                  </a-col>
                  <a-col :xs="24" :md="12">
                    <a-form-item label="类型">
                      <a-select v-model:value="rechargeKind" :options="rechargeTypeOptions" />
                    </a-form-item>
                  </a-col>
                </a-row>
                <a-form-item label="备注">
                  <a-input v-model:value="rechargeRemark" placeholder="请输入加币备注" allow-clear />
                </a-form-item>
                <div class="finance-actions">
                  <a-button type="primary" :loading="recharging" @click="confirmRecharge">确认充值</a-button>
                </div>
              </a-form>
            </div>

            <div class="finance-card">
              <div class="finance-card-title">退还吉豆</div>
              <a-form layout="vertical">
                <a-form-item label="退还金额（吉豆）">
                  <a-input v-model:value="refundAmount" placeholder="0" />
                </a-form-item>
                <div class="finance-actions">
                  <a-button type="primary" danger :loading="refunding" @click="confirmRefund">确认退还</a-button>
                </div>
              </a-form>
            </div>

            <div class="finance-card">
              <div class="finance-card-title">退款上限设置</div>
              <a-row :gutter="24">
                <a-col :xs="24" :md="12">
                  <a-form-item label="当前生效限制(¥)">
                    <a-input :value="effectiveLimitYuan" disabled />
                  </a-form-item>
                </a-col>
                <a-col :xs="24" :md="12">
                  <a-form-item label="管理员设置限制(¥)">
                    <a-input-number
                      v-model:value="adminLimitYuan"
                      :min="0"
                      :precision="2"
                      :controls="false"
                      placeholder="请输入限制金额"
                      style="width: 100%"
                    />
                  </a-form-item>
                </a-col>
                <a-col :xs="24" :md="12">
                  <a-form-item label="当前生效次数">
                    <a-input :value="String(effectiveLimitCount)" disabled />
                  </a-form-item>
                </a-col>
                <a-col :xs="24" :md="12">
                  <a-form-item label="管理员设置次数">
                    <a-input-number
                      v-model:value="adminLimitCount"
                      :min="0"
                      :precision="0"
                      :controls="false"
                      placeholder="请输入次数"
                      style="width: 100%"
                    />
                  </a-form-item>
                </a-col>
              </a-row>
              <div class="finance-actions">
                <a-button type="primary" :loading="savingLimits" @click="saveRefundLimits">保存设置</a-button>
              </div>
            </div>
          </a-spin>
        </a-tab-pane>

        <a-tab-pane key="binding" tab="绑定信息">
          <div class="binding-toolbar">
            <a-button danger :loading="resettingSms" @click="resetBinding('sms')">重置短信限制</a-button>
            <a-button danger :loading="resettingRealname" @click="resetBinding('realname')">重置实名认证次数</a-button>
          </div>
          <a-spin :spinning="bindingsLoading">
            <div class="binding-section-title">支付绑定信息</div>
            <a-table
              :data-source="bindings.payments"
              :pagination="false"
              :row-key="(record: PaymentBinding) => String(record.id)"
              :locale="{ emptyText: '暂无数据' }"
              size="middle"
              bordered
            >
              <a-table-column title="ID" data-index="id" align="center" width="80" />
              <a-table-column title="创建时间" align="center" width="180">
                <template #default="{ record }">{{ formatTime(record.createdAtUnixMillis) }}</template>
              </a-table-column>
              <a-table-column title="对外ID" align="center" width="140">
                <template #default>{{ publicUserId }}</template>
              </a-table-column>
              <a-table-column title="姓名" data-index="name" align="center" />
              <a-table-column title="支付账号" data-index="paymentAccount" align="center" />
              <a-table-column title="操作" align="center" width="100">
                <template #default>-</template>
              </a-table-column>
            </a-table>

            <div class="binding-section-title">实名认证信息</div>
            <a-table
              :data-source="bindings.realnames"
              :pagination="false"
              :row-key="(record: RealnameBinding) => String(record.id)"
              :locale="{ emptyText: '暂无数据' }"
              size="middle"
              bordered
            >
              <a-table-column title="ID" data-index="id" align="center" width="80" />
              <a-table-column title="创建时间" align="center" width="180">
                <template #default="{ record }">{{ formatTime(record.createdAtUnixMillis) }}</template>
              </a-table-column>
              <a-table-column title="对外ID" align="center" width="140">
                <template #default>{{ publicUserId }}</template>
              </a-table-column>
              <a-table-column title="姓名" data-index="name" align="center" />
              <a-table-column title="身份证" data-index="idCard" align="center" />
              <a-table-column title="手机号" data-index="phone" align="center" />
              <a-table-column title="操作" align="center" width="100">
                <template #default>-</template>
              </a-table-column>
            </a-table>
          </a-spin>
        </a-tab-pane>
      </a-tabs>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { CloseOutlined } from '@ant-design/icons-vue'
import dayjs from 'dayjs'
import { customerClient } from '@/api/customer'
import type { ConsoleUser, PaymentBinding, RealnameBinding, UserBindings, UserFinance } from '@/api/customer'
import { uploadFileToOss } from '@/api/upload'
import { compressImage } from '@/utils/imageUpload'
import { newRequestId } from '@/utils/requestId'

const props = defineProps<{ user: ConsoleUser }>()
const emit = defineEmits<{ (e: 'success'): void }>()

const internalUserId = computed(() => props.user.userId || '-')
const publicUserId = computed(() => props.user.publicUserId || '-')

const activeTab = ref('basic')
const loading = ref(false)
const saving = ref(false)

const statusOptions = [
  { value: 1, label: '正常' },
  { value: 2, label: '禁用' },
]
const luckyDisplayOptions = ['初始化', '达标', '开放'].map((value) => ({ value, label: value }))
const reissueOptions = [
  { value: 'no', label: '否' },
  { value: 'yes', label: '是' },
]
const identityOptions = ['用户', '代理', '达人'].map((value) => ({ value, label: value }))

const form = reactive({
  userId: props.user.userId ?? '',
  username: props.user.username ?? '',
  phone: props.user.phone ?? '',
  nickname: props.user.nickname ?? '',
  status: props.user.status ?? 1,
  luckyDisplay: props.user.luckyDisplay || '初始化',
  parentUserId: Number(props.user.parentUserId ?? 0) || null,
  reissue: props.user.reissueLuckPoints ? 'yes' : 'no',
  avatarUrl: props.user.avatarUrl ?? '',
  rebateRate: props.user.rebateRate ?? 10,
  identity: props.user.identity || '用户',
})

function applyUser(user: ConsoleUser) {
  form.userId = user.userId ?? form.userId
  form.username = user.username ?? ''
  form.phone = user.phone ?? ''
  form.nickname = user.nickname ?? ''
  form.status = user.status ?? 1
  form.luckyDisplay = user.luckyDisplay || '初始化'
  form.parentUserId = Number(user.parentUserId ?? 0) || null
  form.reissue = user.reissueLuckPoints ? 'yes' : 'no'
  form.avatarUrl = user.avatarUrl ?? ''
  form.rebateRate = user.rebateRate ?? 10
  form.identity = user.identity || '用户'
}

async function load() {
  if (!props.user.userId) return
  loading.value = true
  try {
    applyUser(await customerClient().Get(props.user.userId))
  } catch {
    applyUser(props.user)
  } finally {
    loading.value = false
  }
}

async function handleAvatarUpload(file: File) {
  try {
    const uploadFile = await compressImage(file)
    const result = await uploadFileToOss(uploadFile, 'avatar')
    if (result.url) form.avatarUrl = result.url
    message.success('头像上传成功')
  } catch {
    // 错误提示由请求层统一处理
  }
  return false
}

async function save() {
  if (saving.value) return
  const nickname = form.nickname.trim()
  if (!nickname) {
    message.warning('请输入昵称')
    return
  }
  if (!form.avatarUrl) {
    message.warning('请上传头像')
    return
  }
  if (form.rebateRate < 0 || form.rebateRate > 100) {
    message.warning('返佣比例须为 0-100')
    return
  }
  saving.value = true
  try {
    await customerClient(newRequestId()).Update(props.user.userId ?? form.userId, {
      nickname,
      status: form.status,
      luckyDisplay: form.luckyDisplay,
      parentUserId: form.parentUserId ?? 0,
      reissueLuckPoints: form.reissue === 'yes',
      avatarUrl: form.avatarUrl,
      rebateRate: Math.trunc(form.rebateRate),
      identity: form.identity,
    })
    message.success('保存成功')
    emit('success')
  } catch {
    // 错误提示由请求层统一处理
  } finally {
    saving.value = false
  }
}

// ── 财务管理 ──
interface WalletRow {
  kind: number
  label: string
  enabled: boolean
  balance: string
}

const displayWallets = [
  { kind: 5, label: '吉祥币' },
  { kind: 1, label: '余额' },
  { kind: 2, label: '吉豆' },
  { kind: 3, label: '积分' },
  { kind: 4, label: '吉祥值' },
]

const finance = ref<UserFinance | null>(null)
const financeLoading = ref(false)
const recharging = ref(false)
const refunding = ref(false)
const savingLimits = ref(false)
const rechargeAmount = ref('0')
const rechargeKind = ref(2)
const rechargeRemark = ref('')
const refundAmount = ref('0')
const adminLimitYuan = ref<number | null>(null)
const adminLimitCount = ref<number | null>(null)

const walletRows = computed<WalletRow[]>(() => {
  const wallets = finance.value?.wallets ?? []
  return displayWallets.map((item) => {
    const wallet = wallets.find((value) => value.kind === item.kind)
    return {
      kind: item.kind,
      label: item.label,
      enabled: wallet?.enabled !== false,
      balance: formatBalance(wallet?.balance),
    }
  })
})

const rechargeTypeOptions = displayWallets.map((item) => ({
  value: item.kind,
  label: item.kind === 5 ? '吉祥币（暂不支持）' : item.label,
  disabled: item.kind === 5,
}))

const effectiveLimitYuan = computed(() => {
  const fen = Number(finance.value?.effectiveLimitFen ?? 500000)
  return formatNumber(fen / 100)
})

const effectiveLimitCount = computed(() => Number(finance.value?.effectiveLimitCount ?? 10))

function formatNumber(value: number) {
  if (!Number.isFinite(value)) return '0'
  return Number.isInteger(value) ? String(value) : value.toFixed(2).replace(/0+$/, '').replace(/\.$/, '')
}

function formatBalance(value: string | undefined) {
  const amount = Number(value ?? 0)
  if (!Number.isFinite(amount)) return '0'
  return formatNumber(amount)
}

function formatTime(value: string | number | undefined) {
  const ms = Number(value ?? 0)
  return ms > 0 ? dayjs(ms).format('YYYY-MM-DD HH:mm:ss') : '-'
}

async function loadFinance() {
  if (!props.user.userId) return
  financeLoading.value = true
  try {
    const result = await customerClient().GetFinance(props.user.userId)
    finance.value = result
    const limitFen = Number(result.refundLimitFen ?? 0)
    adminLimitYuan.value = limitFen > 0 ? limitFen / 100 : null
    const limitCount = Number(result.refundLimitCount ?? 0)
    adminLimitCount.value = limitCount > 0 ? limitCount : null
  } catch {
    finance.value = null
  } finally {
    financeLoading.value = false
  }
}

async function updateWalletSettings(wallets: Array<{ kind: number; enabled: boolean }>) {
  await customerClient(newRequestId()).UpdateFinance(props.user.userId ?? form.userId, {
    wallets,
    refundLimitFen: finance.value?.refundLimitFen ?? 0,
    refundLimitCount: Number(finance.value?.refundLimitCount ?? 0),
  })
}

async function toggleWallet(row: WalletRow) {
  try {
    await updateWalletSettings(
      displayWallets.map((item) => ({
        kind: item.kind,
        enabled: item.kind === row.kind ? !row.enabled : walletRows.value.find((value) => value.kind === item.kind)?.enabled !== false,
      })),
    )
    message.success(row.enabled ? '已关闭' : '已开启')
    await loadFinance()
  } catch {
    // 错误提示由请求层统一处理
  }
}

async function confirmRecharge() {
  if (rechargeKind.value === 5) {
    message.warning('吉祥币暂不支持充值')
    return
  }
  const amount = Number(rechargeAmount.value)
  if (!Number.isFinite(amount) || amount <= 0) {
    message.warning('请输入大于 0 的充值金额')
    return
  }
  recharging.value = true
  try {
    await customerClient(newRequestId()).Recharge(props.user.userId ?? form.userId, {
      amount: rechargeAmount.value,
      kind: rechargeKind.value,
      remark: rechargeRemark.value,
    })
    message.success('充值成功')
    rechargeAmount.value = '0'
    rechargeRemark.value = ''
    await loadFinance()
  } catch {
    // 错误提示由请求层统一处理
  } finally {
    recharging.value = false
  }
}

async function confirmRefund() {
  const amount = Number(refundAmount.value)
  if (!Number.isFinite(amount) || amount <= 0) {
    message.warning('请输入大于 0 的退还金额')
    return
  }
  refunding.value = true
  try {
    await customerClient(newRequestId()).RefundBeans(props.user.userId ?? form.userId, {
      amount: refundAmount.value,
    })
    message.success('退还成功')
    refundAmount.value = '0'
    await loadFinance()
  } catch {
    // 错误提示由请求层统一处理
  } finally {
    refunding.value = false
  }
}

async function saveRefundLimits() {
  savingLimits.value = true
  try {
    await customerClient(newRequestId()).UpdateFinance(props.user.userId ?? form.userId, {
      wallets: walletRows.value.map((item) => ({ kind: item.kind, enabled: item.enabled })),
      refundLimitFen: adminLimitYuan.value ? Math.round(adminLimitYuan.value * 100) : 0,
      refundLimitCount: adminLimitCount.value ? Math.trunc(adminLimitCount.value) : 0,
    })
    message.success('保存成功')
    await loadFinance()
  } catch {
    // 错误提示由请求层统一处理
  } finally {
    savingLimits.value = false
  }
}

// ── 绑定信息 ──
const bindingsLoading = ref(false)
const resettingSms = ref(false)
const resettingRealname = ref(false)
const bindings = ref<UserBindings>({ payments: [], realnames: [] })

async function loadBindings() {
  if (!props.user.userId) return
  bindingsLoading.value = true
  try {
    const result = await customerClient().GetBindings(props.user.userId)
    bindings.value = {
      payments: result.payments ?? [],
      realnames: result.realnames ?? [],
    }
  } catch {
    bindings.value = { payments: [], realnames: [] }
  } finally {
    bindingsLoading.value = false
  }
}

async function resetBinding(kind: 'sms' | 'realname') {
  if (kind === 'sms') resettingSms.value = true
  else resettingRealname.value = true
  try {
    await customerClient(newRequestId()).ResetBindingLimit(props.user.userId ?? form.userId, kind)
    message.success('重置成功')
  } catch {
    // 错误提示由请求层统一处理
  } finally {
    if (kind === 'sms') resettingSms.value = false
    else resettingRealname.value = false
  }
}

onMounted(async () => {
  await Promise.all([load(), loadFinance(), loadBindings()])
})
</script>

<style scoped lang="scss">
.customer-manage-body {
  padding-top: 4px;
}
.user-id-input:disabled {
  background: #f5f5f5;
  color: #595959;
  cursor: not-allowed;
}
.customer-manage-body :deep(.ant-form-item) {
  margin-bottom: 22px;
}
.customer-manage-body :deep(.ant-form-item-label > label) {
  font-size: 15px;
}
.avatar-upload-wrap {
  position: relative;
  display: inline-flex;
  width: 96px;
  height: 96px;
  line-height: 0;
}
.avatar-upload-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 96px;
  height: 96px;
  overflow: hidden;
  color: #8c8c8c;
  background: #fafafa;
  border: 1px dashed #d9d9d9;
  border-radius: 6px;
}
.avatar-upload-box img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.avatar-remove {
  position: absolute;
  top: 4px;
  right: 4px;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  padding: 0;
  font-size: 12px;
  color: #ff4d4f;
  background: transparent;
  border: 1px solid currentColor;
  border-radius: 50%;
  cursor: pointer;
  opacity: 0;
  pointer-events: none;
}
.avatar-upload-wrap:hover .avatar-remove {
  opacity: 1;
  pointer-events: auto;
}
.avatar-remove:hover {
  color: #cf1322;
}
.upload-plus {
  margin-bottom: 4px;
  font-size: 22px;
  line-height: 1;
}
.rebate-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.percent {
  color: #8c8c8c;
}
.manage-actions {
  display: flex;
  justify-content: flex-end;
  padding-top: 8px;
  border-top: 1px solid #f0f0f0;
}
.manage-actions button {
  min-width: 110px;
}
.finance-card {
  margin-bottom: 16px;
  padding: 16px;
  border: 1px solid #f0f0f0;
  border-radius: 4px;
}
.finance-card-title {
  margin-bottom: 14px;
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}
.finance-actions {
  display: flex;
  justify-content: flex-end;
  padding-top: 4px;
}
.finance-actions button {
  min-width: 100px;
}
.binding-toolbar {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-bottom: 16px;
}
.binding-section-title {
  margin: 18px 0 10px;
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}
</style>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { onLoad, onUnload } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as couponApi from '@/services/coupon'
import * as drawApi from '@/services/draw'
import * as userApi from '@/services/user'
import { useAuthStore } from '@/store/authStore'
import { couponValueText } from '@/utils/coupon'
import { formatMoney } from '@/utils/format'
import { randomUUID } from '@/utils/id'

import type { BoxDetailResponse, BoxQuoteResponse, CouponItem, DrawRecordItem } from '@/services/types'

// 购买页：确认（报价/券/积分）→ 提交 SAGA → 轮询记录 → 开奖动画 → 揭晓
// 与 web Buy 页同流程；弹窗改 uni.showModal，券选择改 picker，开关改原生 switch

const { t } = useI18n()
const auth = useAuthStore()

const boxId = ref(0)

// 阶段：idle 确认购买 / submitting 提交中 / polling 轮询 / drawing 开奖动画 / done 结果
const phase = ref<'idle' | 'submitting' | 'polling' | 'drawing' | 'done'>('idle')
const detail = ref<BoxDetailResponse | null>(null)
const loadError = ref('')
const record = ref<DrawRecordItem | null>(null)
const failed = ref(false)

// M10 报价：服务端算价（原价/券/积分抵扣），仅预览；购买时后端按同参数重算
const quote = ref<BoxQuoteResponse | null>(null)
const quoteLoading = ref(false)
const quoteError = ref('')
const coupons = ref<CouponItem[]>([]) // 仅可用券（usable=true）
const selectedCouponIndex = ref(0) // picker 下标；0 = 不使用券
const useLoyalty = ref(false)

// 幂等键：单次购买会话内复用（网络错误重试走同一 SAGA 单号，后端幂等）
const idempotencyKey = randomUUID()
let pollTimer: ReturnType<typeof setInterval> | null = null

const resultIsPrize = computed(() => record.value?.isPrize ?? false)
// 券选择器选项：不使用 + 可用券列表（picker 用 string 数组，下标映射 coupon id）
const couponOptions = computed(() => [
  { id: 0, label: t('buy.noCoupon') },
  ...coupons.value.map((c) => ({
    id: c.id,
    label: `${c.name}（${couponValueText(c.kind, c.value, t)}）`,
  })),
])
const couponLabels = computed(() => couponOptions.value.map((o) => o.label))
const selectedCouponId = computed(() => couponOptions.value[selectedCouponIndex.value]?.id ?? 0)
const hasCouponDiscount = computed(() => Number(quote.value?.couponDiscount ?? 0) > 0)
const hasLoyaltyDiscount = computed(() => Number(quote.value?.loyalty ?? 0) > 0)
// 积分余额：后端返回元等价（如 "10.00"），按每元消耗积分换算成点数展示
const loyaltyPoints = computed(() =>
  Math.round(Number(quote.value?.loyaltyBalance ?? 0) * Number(quote.value?.loyaltyRate ?? 0)),
)

async function loadDetail() {
  try {
    detail.value = await drawApi.boxDetail(boxId.value)
  } catch (err) {
    loadError.value = (err as Error).message
  }
}

async function loadCoupons() {
  try {
    const res = await couponApi.listUserCoupons()
    coupons.value = res.list.filter((c) => c.usable)
  } catch {
    // 券列表失败不阻塞购买：选择器留空即可
  }
}

// 报价：券/积分抵扣均为服务端算价，客户端只传选择项
async function loadQuote() {
  quoteLoading.value = true
  quoteError.value = ''
  try {
    quote.value = await drawApi.getQuote(boxId.value, {
      userCouponId: selectedCouponId.value || undefined,
      useLoyaltyOffset: useLoyalty.value,
    })
  } catch (err) {
    quoteError.value = (err as Error).message
    quote.value = null
  } finally {
    quoteLoading.value = false
  }
}

// 券与积分互斥（后端 3002 兜底）：选券关积分，开积分清券，然后重新报价
watch(selectedCouponId, (id) => {
  if (id > 0) useLoyalty.value = false
  void loadQuote()
})
watch(useLoyalty, (checked) => {
  if (checked) selectedCouponIndex.value = 0
  void loadQuote()
})

function onCouponChange(e: { detail: { value: string | number } }) {
  const idx = Number(e.detail.value)
  if (idx === selectedCouponIndex.value) return
  selectedCouponIndex.value = idx
}

function onLoyaltyChange(e: Event) {
  // uni 组件事件模板类型是 Event，运行时为 { detail: { value } }
  useLoyalty.value = Boolean((e as unknown as { detail: { value: boolean } }).detail.value)
}

// 模板导航（uni 全局不在模板类型检查内，统一收敛为方法）
function goWarehouse() {
  uni.navigateTo({ url: '/pages/warehouse/index' })
}

function goRecords() {
  uni.navigateTo({ url: '/pages/records/index' })
}

function goRecharge() {
  uni.navigateTo({ url: '/pages/recharge/index' })
}

function goBackBoxes() {
  uni.switchTab({ url: '/pages/index/index' })
}

function goBackDetail() {
  uni.navigateBack()
}

// 刷新余额（开奖成功后余额已变化）
async function refreshProfile() {
  try {
    auth.setUser(await userApi.getProfile())
  } catch {
    // 忽略：401 由适配器统一处理
  }
}

function stopPoll() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

// 轮询抽赏记录：后端未产生记录时返回 2004，继续轮询直到超时
function startPoll(requestId: string) {
  phase.value = 'polling'
  let tries = 0
  pollTimer = setInterval(async () => {
    tries += 1
    try {
      const res = await drawApi.getRecordByRequestId(requestId)
      if (res.list.length > 0) {
        stopPoll()
        phase.value = 'drawing'
        // 开奖动画 2.5s 后揭晓（体验节奏）
        setTimeout(() => {
          record.value = res.list[0] ?? null
          failed.value = false
          phase.value = 'done'
          void refreshProfile()
        }, 2500)
      }
    } catch {
      // 2004（暂无记录）或网络抖动：继续轮询
    }
    if (tries >= 30) {
      stopPoll()
      record.value = null
      failed.value = true
      phase.value = 'done'
      void refreshProfile()
    }
  }, 1000)
}

async function buy() {
  if (!detail.value) return
  phase.value = 'submitting'
  try {
    const res = await drawApi.buyBox(boxId.value, idempotencyKey, {
      userCouponId: selectedCouponId.value || undefined,
      useLoyaltyOffset: useLoyalty.value,
    })
    // status=pending：SAGA 已提交异步开奖；success：同 Key 重试命中已有单号
    if (res.status === 'success') {
      const existing = await drawApi.getRecordByRequestId(res.requestId)
      if (existing.list.length > 0) {
        record.value = existing.list[0] ?? null
        failed.value = false
        phase.value = 'done'
        void refreshProfile()
        return
      }
    }
    startPoll(res.requestId)
  } catch (err) {
    phase.value = 'idle'
    uni.showModal({ title: t('buy.failTitle'), content: (err as Error).message, showCancel: false })
  }
}

function onBuyClick() {
  if (!quote.value) {
    uni.showToast({ title: t('buy.quotePending'), icon: 'none' })
    return
  }
  uni.showModal({
    title: t('buy.confirmTitle'),
    content: t('buy.confirmText', { price: formatMoney(quote.value.payable), box: detail.value?.boxName ?? '' }),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    success: (res) => {
      if (res.confirm) void buy()
    },
  })
}

function drawAgain() {
  // 同 Key 重试：后端幂等返回同一单号与结果；新抽盒请回详情页
  void buy()
}

onLoad((query) => {
  boxId.value = Number(query?.boxId ?? 0)
  void loadDetail()
  void loadCoupons()
  void loadQuote()
})
onUnload(stopPoll)
</script>

<template>
  <view class="buy-page">
    <view v-if="loadError" class="error-text">
      {{ loadError }}
    </view>

    <template v-else-if="detail">
      <!-- 确认购买 -->
      <view v-if="phase === 'idle'" class="gacha-card buy-card">
        <view class="buy-box-name">{{ detail.boxName }}</view>
        <view class="buy-line">{{ t('buy.balance') }}：<text class="money">{{ formatMoney(auth.balance) }}</text></view>

        <!-- M10 报价面板：原价 + 券/积分抵扣 + 实付（全部服务端算价） -->
        <view class="quote-card">
          <template v-if="quote">
            <view class="quote-row">
              <text>{{ t('buy.gross') }}</text>
              <text>{{ formatMoney(quote.gross) }}</text>
            </view>
            <view v-if="hasCouponDiscount" class="quote-row discount">
              <text>{{ t('buy.couponDiscount') }}</text>
              <text>-{{ formatMoney(quote.couponDiscount) }}</text>
            </view>
            <view v-if="hasLoyaltyDiscount" class="quote-row discount">
              <text>{{ t('buy.loyaltyDiscount') }}（{{ t('buy.loyaltyPoints', { n: quote.loyaltyPointsDebit }) }}）</text>
              <text>-{{ formatMoney(quote.loyalty) }}</text>
            </view>
            <view class="quote-row payable">
              <text>{{ t('buy.payable') }}</text>
              <text class="money payable-value">{{ formatMoney(quote.payable) }}</text>
            </view>
            <view v-if="hasLoyaltyDiscount" class="quote-row sub">
              <text>{{ t('buy.cash') }}</text>
              <text>{{ formatMoney(quote.cash) }}</text>
            </view>
            <view class="quote-row sub">
              <text>{{ t('buy.loyaltyBalance') }}</text>
              <text>{{ loyaltyPoints }} {{ t('buy.loyaltyMaxHint', { max: formatMoney(quote.loyaltyMax) }) }}</text>
            </view>
          </template>
          <view v-else-if="quoteError" class="error-text quote-error">{{ quoteError }}</view>
          <view v-else class="quote-empty">{{ t('buy.quoteLoading') }}</view>
        </view>

        <!-- 优惠选择：券与积分互斥 -->
        <view class="discount-options">
          <picker
            :range="couponLabels"
            :value="selectedCouponIndex"
            :disabled="useLoyalty"
            @change="onCouponChange"
          >
            <view class="coupon-picker" :class="{ disabled: useLoyalty }">
              {{ couponLabels[selectedCouponIndex] }} ▾
            </view>
          </picker>
          <view class="loyalty-row">
            <text>{{ t('buy.loyaltySwitch') }}</text>
            <switch
              :checked="useLoyalty"
              :disabled="selectedCouponId > 0"
              color="#6c4cf1"
              style="transform: scale(0.8)"
              @change="onLoyaltyChange"
            />
          </view>
        </view>

        <view class="btn-primary" :class="{ disabled: !quote }" @click="onBuyClick">
          {{ t('box.buy') }}
        </view>
        <view class="btn-plain" @click="goBackDetail">{{ t('box.probability') }}</view>
      </view>

      <!-- 提交/轮询/开奖动画 -->
      <view
        v-else-if="phase === 'submitting' || phase === 'polling' || phase === 'drawing'"
        class="gacha-card buy-card center"
      >
        <view class="box-emoji" :class="{ shake: phase !== 'submitting' }">
          {{ phase === 'submitting' ? '📦' : '🎁' }}
        </view>
        <view class="buy-status">
          {{
            phase === 'submitting' ? t('buy.submitting') : phase === 'polling' ? t('buy.submitted') : t('buy.drawing')
          }}
        </view>
        <view v-if="phase !== 'drawing'" class="spinner"></view>
      </view>

      <!-- 结果揭晓 -->
      <view v-else class="gacha-card buy-card center">
        <template v-if="record">
          <view class="result-emoji">{{ resultIsPrize ? '🎉' : '🍀' }}</view>
          <view class="result-title">{{ resultIsPrize ? t('buy.winTitle') : t('buy.thanksTitle') }}</view>
          <view v-if="resultIsPrize" class="result-prize">{{ record.entryName }}</view>
          <view v-else class="result-thanks">{{ t('buy.thanksText') }}</view>
          <view class="result-actions">
            <view v-if="resultIsPrize" class="btn-primary" @click="goWarehouse">
              {{ t('buy.goWarehouse') }}
            </view>
            <view class="btn-plain" @click="goRecords">{{ t('buy.goRecords') }}</view>
            <view class="btn-plain" @click="goBackDetail">{{ t('buy.drawAgain') }}</view>
          </view>
        </template>
        <template v-else-if="failed">
          <view class="result-emoji">😢</view>
          <view class="result-title">{{ t('buy.failTitle') }}</view>
          <view class="result-thanks">{{ t('buy.failText') }}</view>
          <view class="result-actions">
            <view class="btn-plain" @click="goRecharge">{{ t('user.recharge') }}</view>
            <view class="btn-primary" @click="drawAgain">{{ t('buy.drawAgain') }}</view>
            <view class="btn-plain" @click="goBackBoxes">{{ t('buy.backBoxes') }}</view>
          </view>
        </template>
      </view>
    </template>
  </view>
</template>

<style scoped lang="scss">
.buy-card {
  .buy-box-name {
    font-size: 40rpx;
    font-weight: 700;
    margin-bottom: 16rpx;
  }

  .buy-line {
    margin-bottom: 12rpx;
    font-size: 28rpx;
  }
}

// 报价面板（M10）
.quote-card {
  background: var(--color-bg);
  border-radius: 12rpx;
  padding: 20rpx 24rpx;
  margin: 20rpx 0;

  .quote-row {
    display: flex;
    justify-content: space-between;
    font-size: 26rpx;
    padding: 4rpx 0;

    &.discount {
      color: var(--color-danger);
    }

    &.payable {
      font-size: 30rpx;
      font-weight: 600;
      border-top: 1rpx dashed var(--color-border);
      margin-top: 8rpx;
      padding-top: 12rpx;
    }

    &.sub {
      color: var(--color-text-secondary);
      font-size: 24rpx;
    }
  }

  .payable-value {
    color: var(--color-accent);
    font-size: 36rpx;
  }

  .quote-empty {
    text-align: center;
    color: var(--color-text-secondary);
    font-size: 24rpx;
  }

  .quote-error {
    margin: 8rpx 0;
    font-size: 24rpx;
  }
}

.discount-options {
  margin-bottom: 20rpx;

  .coupon-picker {
    border: 1rpx solid var(--color-border);
    border-radius: 12rpx;
    padding: 16rpx 20rpx;
    font-size: 28rpx;
    color: var(--color-text);

    &.disabled {
      color: var(--color-text-secondary);
      background: var(--color-bg);
    }
  }

  .loyalty-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 26rpx;
    margin-top: 16rpx;
  }
}

.center {
  text-align: center;

  .box-emoji {
    font-size: 128rpx;
    margin-bottom: 24rpx;
  }

  // 开奖抖动动画（纯 CSS，规范 06 第 9 条）
  .shake {
    animation: shake 0.5s infinite;
  }

  .buy-status {
    font-size: 30rpx;
    margin-bottom: 16rpx;
  }

  .spinner {
    width: 48rpx;
    height: 48rpx;
    margin: 0 auto;
    border: 4rpx solid var(--color-border);
    border-top-color: var(--color-primary);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
}

@keyframes shake {
  0% { transform: rotate(-8deg); }
  50% { transform: rotate(8deg); }
  100% { transform: rotate(-8deg); }
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.result-emoji {
  font-size: 128rpx;
  margin-bottom: 16rpx;
}

.result-title {
  font-size: 44rpx;
  font-weight: 700;
  margin-bottom: 16rpx;
}

.result-prize {
  font-size: 36rpx;
  color: var(--color-accent);
  font-weight: 600;
  margin-bottom: 32rpx;
}

.result-thanks {
  font-size: 26rpx;
  color: var(--color-text-secondary);
  margin-bottom: 32rpx;
}

.result-actions {
  display: flex;
  flex-direction: column;
  gap: 16rpx;

  .btn-plain {
    margin-top: 0;
  }
}
</style>

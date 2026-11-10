<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onUnload } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as groupBuyApi from '@/services/groupBuy'
import * as userApi from '@/services/user'
import { useAuthStore } from '@/store/authStore'
import { formatMoney, formatUnix } from '@/utils/format'
import { randomUUID } from '@/utils/id'

import type { DigPrizeItem, GroupBuyDetailResponse, GroupBuyRecordItem } from '@/services/types'

// 拼团抽：幸运奖进度 + 下注数量（1/2/3/包尾）→ 提交 SAGA → 轮询 → 揭晓（web GroupBuy 页同流程）
const { t } = useI18n()
const auth = useAuthStore()

// 拼团抽无列表端点（与后端一致只有详情端点），种子仅 1 号团，前端固定取之
const CONFIG_ID = 1

// 阶段：idle 选数 / submitting 提交中 / polling 轮询 / revealing 揭晓 / done 结果
const phase = ref<'idle' | 'submitting' | 'polling' | 'revealing' | 'done'>('idle')
const detail = ref<GroupBuyDetailResponse | null>(null)
const selectedCount = ref<number | null>(null)
const record = ref<GroupBuyRecordItem | null>(null)
const records = ref<GroupBuyRecordItem[]>([])
const failed = ref(false)

let pollTimer: ReturnType<typeof setInterval> | null = null

const isSettled = computed(() => detail.value?.status === 2)
const remaining = computed(() => detail.value?.remainingBets ?? 0)

// 下注选项：1/2/3 + 包尾（余位≤100 时后端校验允许）
const countOptions = computed(() => {
  const opts: { label: string; value: number }[] = [1, 2, 3].map((n) => ({ label: String(n), value: n }))
  if (remaining.value > 3 && remaining.value <= 100) {
    opts.push({ label: t('groupBuy.buyRest', { n: remaining.value }), value: remaining.value })
  }
  return opts
})

function prizeText(p: DigPrizeItem): string {
  if (p.kind === 1) return t('groupBuy.prizeCash', { amount: formatMoney(p.amount) })
  if (p.kind === 2) return t('groupBuy.prizePoints', { n: p.points })
  return t('groupBuy.prizeCoupon', { name: p.couponName || p.couponTemplateNo })
}

// 幸运奖进度（a-progress → 自绘进度条）
function luckyPercent(p: { sent: number; quantity: number }): number {
  return p.quantity > 0 ? Math.round((p.sent / p.quantity) * 100) : 0
}

async function loadDetail() {
  try {
    detail.value = await groupBuyApi.getGroupBuy(CONFIG_ID)
  } catch (err) {
    detail.value = null
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  }
}

async function loadRecords() {
  try {
    records.value = (await groupBuyApi.listGroupBuyRecords({ page: 1, pageSize: 5 })).items
  } catch {
    // 记录列表失败不阻塞主流程
  }
}

async function refreshProfile() {
  try {
    auth.setUser(await userApi.getProfile())
  } catch {
    // 401 由适配器统一处理
  }
}

async function refreshAll() {
  await Promise.allSettled([loadDetail(), loadRecords(), refreshProfile()])
  selectedCount.value = null
}

function stopPoll() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

// 轮询拼团记录：出记录后揭晓动画再展示结果
function startPoll(requestId: string) {
  phase.value = 'polling'
  let tries = 0
  pollTimer = setInterval(async () => {
    tries += 1
    try {
      const res = await groupBuyApi.listGroupBuyRecords({ requestId })
      if (res.items.length > 0) {
        stopPoll()
        phase.value = 'revealing'
        setTimeout(async () => {
          record.value = res.items[0] ?? null
          failed.value = false
          phase.value = 'done'
          await refreshAll()
        }, 1600)
      }
    } catch {
      // 3032（暂无记录）或网络抖动：继续轮询
    }
    if (tries >= 30) {
      stopPoll()
      record.value = null
      failed.value = true
      phase.value = 'done'
      void refreshAll()
    }
  }, 1000)
}

function selectCount(n: number) {
  // web:214 单选项 disabled = phase !== 'idle' || isSettled || opt.value > remaining；mini 类名仅置灰，行为层镜像同条件
  if (phase.value !== 'idle' || isSettled.value || n > remaining.value) return
  selectedCount.value = selectedCount.value === n ? null : n
}

async function buy() {
  if (selectedCount.value === null) return
  phase.value = 'submitting'
  try {
    // 每次下注生成新幂等键：同一次下注重试复用、再次下注是新事务
    const key = randomUUID()
    const res = await groupBuyApi.groupBuyBuy(CONFIG_ID, selectedCount.value, key)
    // status=success：同 Key 重试命中已有单号，直接取结果
    if (res.status === 'success') {
      const existing = await groupBuyApi.listGroupBuyRecords({ requestId: res.requestId })
      if (existing.items.length > 0) {
        record.value = existing.items[0] ?? null
        failed.value = false
        phase.value = 'done'
        await refreshAll()
        return
      }
    }
    startPoll(res.requestId)
  } catch (err) {
    phase.value = 'idle'
    uni.showModal({ title: t('groupBuy.failTitle'), content: (err as Error).message, showCancel: false })
  }
}

function onBuyClick() {
  if (selectedCount.value === null) {
    uni.showToast({ title: t('groupBuy.selectCount'), icon: 'none' })
    return
  }
  // web:232 disabled = selectedCount === null || isSettled || remaining <= 0 || phase !== 'idle'（真正阻断点击）；mini 类名仅置灰，行为层镜像同条件早退
  if (isSettled.value || remaining.value <= 0 || phase.value !== 'idle') return
  uni.showModal({
    title: t('groupBuy.confirmTitle'),
    content: t('groupBuy.confirmText', {
      count: selectedCount.value,
      price: formatMoney(detail.value?.price ?? '0.00'),
    }),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    success: (res) => {
      if (res.confirm) void buy()
    },
  })
}

function backToDraw() {
  phase.value = 'idle'
  void refreshAll()
}

onLoad(async () => {
  await loadDetail()
  await Promise.allSettled([loadRecords(), refreshProfile()])
})
onUnload(stopPoll)
</script>

<template>
  <view class="group-page">
    <template v-if="detail">
      <!-- 选数 / 提交 / 轮询 / 揭晓 -->
      <view v-if="phase !== 'done'" class="gacha-card group-card">
        <view class="group-name">{{ detail.name }}</view>
        <view class="group-line">
          <text>{{ t('groupBuy.remaining', { n: detail.remainingBets }) }}</text>
          <text>{{ t('groupBuy.target', { n: detail.targetBetCount }) }}</text>
          <text>{{ t('groupBuy.myBets', { n: detail.myBets }) }}</text>
          <text>{{ t('groupBuy.participants', { n: detail.participants }) }}</text>
          <text v-if="isSettled" class="settled-tag">{{ t('groupBuy.settled') }}</text>
        </view>

        <!-- 幸运奖进度卡 -->
        <view v-if="detail.luckyPrizes.length > 0" class="lucky-list">
          <view class="section-title">🍀 {{ t('groupBuy.luckyPrizes') }}</view>
          <view v-for="p in detail.luckyPrizes" :key="p.prizeIndex" class="lucky-card">
            <text class="lucky-badge">🍀</text>
            <view class="lucky-body">
              <view class="lucky-line">{{ prizeText(p.prize) }}</view>
              <view class="progress-track" :class="{ done: p.sent >= p.quantity }">
                <view class="progress-fill" :style="{ width: luckyPercent(p) + '%' }"></view>
              </view>
              <view class="lucky-sent">{{ t('groupBuy.luckySent', { sent: p.sent, quantity: p.quantity }) }}</view>
            </view>
          </view>
        </view>

        <!-- 下注数量选择 -->
        <view class="count-select">
          <text class="count-label">{{ t('groupBuy.countLabel') }}</text>
          <view
            v-for="opt in countOptions"
            :key="opt.value"
            class="count-btn"
            :class="{ selected: selectedCount === opt.value, disabled: phase !== 'idle' || isSettled || opt.value > remaining }"
            @click="selectCount(opt.value)"
          >
            {{ opt.label }}
          </view>
        </view>

        <!-- 单价 + 下注 -->
        <view class="quote-card">
          <view class="quote-row">
            <text>{{ t('groupBuy.unitPrice') }}</text>
            <text>{{ formatMoney(detail.price) }}</text>
          </view>
        </view>
        <view
          class="btn-primary"
          :class="{ disabled: selectedCount === null || isSettled || remaining <= 0 || phase !== 'idle' }"
          @click="onBuyClick"
        >
          {{ t('groupBuy.buy') }}
        </view>

        <!-- 提交/轮询/揭晓状态 -->
        <view v-if="phase === 'submitting' || phase === 'polling' || phase === 'revealing'" class="group-status">
          <view class="group-emoji" :class="{ shake: phase !== 'submitting' }">🎲</view>
          <view class="status-text">
            {{
              phase === 'submitting' ? t('groupBuy.submitting') : phase === 'polling' ? t('groupBuy.submitted') : t('groupBuy.revealing')
            }}
          </view>
          <view v-if="phase !== 'revealing'" class="spinner"></view>
        </view>
      </view>

      <!-- 结果 -->
      <view v-else class="gacha-card group-card center">
        <template v-if="record">
          <view class="result-emoji">{{ record.awards.length > 0 ? '🎉' : '🎲' }}</view>
          <view class="result-title">{{ t('groupBuy.resultTitle') }}</view>
          <view v-if="record.awards.length > 0" class="prize-list">
            <view class="awards-title">✨ {{ t('groupBuy.awardsTitle') }}</view>
            <view v-for="(p, i) in record.awards" :key="i" class="prize-row">
              <text>🎁</text>
              <text>{{ prizeText(p) }}</text>
            </view>
          </view>
          <view v-else class="result-thanks">{{ t('groupBuy.noAward') }}</view>
          <view v-if="isSettled" class="settled-tip">{{ t('groupBuy.settledTip') }}</view>
          <view class="result-actions">
            <view v-if="!isSettled" class="btn-plain" @click="backToDraw">{{ t('groupBuy.backDraw') }}</view>
            <view v-else class="btn-plain" @click="backToDraw">{{ t('groupBuy.records') }}</view>
          </view>
        </template>
        <template v-else-if="failed">
          <view class="result-emoji">😢</view>
          <view class="result-title">{{ t('groupBuy.failTitle') }}</view>
          <view class="result-thanks">{{ t('groupBuy.failText') }}</view>
          <view class="result-actions">
            <view class="btn-plain" @click="backToDraw">{{ t('groupBuy.retry') }}</view>
          </view>
        </template>
      </view>

      <!-- 下注记录 -->
      <view class="gacha-card records-card">
        <view class="records-title">{{ t('groupBuy.records') }}</view>
        <template v-if="records.length > 0">
          <view v-for="r in records" :key="r.recordId" class="record-row">
            <text class="record-awards">{{ r.awards.map((p) => prizeText(p)).join('、') || '—' }}</text>
            <text class="record-count">{{ t('groupBuy.recordCount', { n: r.count }) }}</text>
            <text class="record-price money">{{ formatMoney(r.price) }}</text>
            <text class="record-time">{{ formatUnix(r.createdAt) }}</text>
          </view>
        </template>
        <view v-else class="records-empty">{{ t('groupBuy.recordEmpty') }}</view>
      </view>
    </template>

    <view v-else class="gacha-card group-card center">
      <view class="records-empty">{{ t('groupBuy.empty') }}</view>
    </view>
  </view>
</template>

<style scoped lang="scss">
.group-card {
  text-align: left;

  .group-name {
    font-size: 40rpx;
    font-weight: 700;
    margin-bottom: 8rpx;
  }

  .group-line {
    font-size: 26rpx;
    color: var(--color-text-secondary);
    margin-bottom: 20rpx;
    display: flex;
    gap: 20rpx;
    flex-wrap: wrap;

    .settled-tag {
      color: var(--color-accent);
    }
  }
}

// 幸运奖进度卡
.lucky-list {
  display: flex;
  flex-direction: column;
  gap: 16rpx;
  margin: 16rpx 0;

  .lucky-card {
    display: flex;
    gap: 20rpx;
    align-items: flex-start;
    border: 1rpx solid var(--color-border);
    border-radius: 12rpx;
    padding: 16rpx 20rpx;
    background: var(--color-bg);

    .lucky-badge {
      font-size: 44rpx;
      line-height: 1.2;
    }

    .lucky-body {
      flex: 1;
      min-width: 0;

      .lucky-line {
        font-size: 26rpx;
        margin-bottom: 8rpx;
      }

      .lucky-sent {
        font-size: 22rpx;
        color: var(--color-text-secondary);
        margin-top: 4rpx;
      }
    }
  }
}

// 下注数量选择
.count-select {
  display: flex;
  gap: 16rpx;
  align-items: center;
  margin: 20rpx 0;

  .count-label {
    font-size: 26rpx;
    color: var(--color-text-secondary);
  }

  .count-btn {
    flex: 1;
    border: 1rpx solid var(--color-border);
    border-radius: 12rpx;
    background: var(--color-bg);
    padding: 16rpx 0;
    font-size: 28rpx;
    text-align: center;

    &.selected {
      border-color: var(--color-accent);
      box-shadow: 0 0 0 3rpx rgba(255, 165, 0, 0.25);
    }

    &.disabled {
      opacity: 0.5;
    }
  }
}

.quote-card {
  background: var(--color-bg);
  border-radius: 12rpx;
  padding: 20rpx 24rpx;
  margin: 16rpx 0 24rpx;

  .quote-row {
    display: flex;
    justify-content: space-between;
    font-size: 26rpx;
    padding: 4rpx 0;
  }
}

.group-status {
  text-align: center;
  margin-top: 24rpx;

  .group-emoji {
    font-size: 96rpx;
    margin-bottom: 16rpx;
  }

  .shake {
    animation: shake 0.5s infinite;
  }

  .status-text {
    font-size: 28rpx;
    margin-bottom: 12rpx;
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

.center {
  text-align: center;

  .result-emoji {
    font-size: 128rpx;
    margin-bottom: 16rpx;
  }

  .result-title {
    font-size: 44rpx;
    font-weight: 700;
    margin-bottom: 16rpx;
  }

  .settled-tip {
    font-size: 26rpx;
    color: var(--color-accent);
    margin-bottom: 24rpx;
  }

  .result-thanks {
    font-size: 26rpx;
    color: var(--color-text-secondary);
    margin-bottom: 32rpx;
  }
}

.prize-list {
  text-align: left;
  margin-bottom: 24rpx;

  .awards-title {
    text-align: center;
    font-size: 26rpx;
    font-weight: 600;
    color: var(--color-accent);
    margin-bottom: 8rpx;
  }

  .prize-row {
    display: flex;
    gap: 16rpx;
    padding: 8rpx 0;
    font-size: 28rpx;
    justify-content: center;
  }
}

.result-actions {
  display: flex;
  flex-direction: column;
  gap: 16rpx;

  .btn-plain {
    margin-top: 0;
  }
}

.records-card {
  text-align: left;

  .records-title {
    font-size: 30rpx;
    font-weight: 600;
    margin-bottom: 16rpx;
  }

  .record-row {
    display: flex;
    align-items: center;
    gap: 16rpx;
    font-size: 24rpx;
    padding: 12rpx 0;
    border-top: 1rpx solid var(--color-border);

    .record-awards {
      flex: 1;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .record-count {
      color: var(--color-text-secondary);
    }

    .record-price {
      color: var(--color-accent);
      font-weight: 600;
    }

    .record-time {
      color: var(--color-text-secondary);
    }
  }

  .records-empty {
    color: var(--color-text-secondary);
    text-align: center;
    font-size: 26rpx;
  }
}

.records-empty {
  color: var(--color-text-secondary);
  text-align: center;
}
</style>

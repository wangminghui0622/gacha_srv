<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onUnload } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as gashaponApi from '@/services/gashapon'
import * as userApi from '@/services/user'
import { useAuthStore } from '@/store/authStore'
import { formatMoney, formatUnix } from '@/utils/format'
import { randomUUID } from '@/utils/id'

import type { DigPrizeItem, GashaponDetailResponse, GashaponRecordItem, GashaponSpecialPrizeItem } from '@/services/types'

// 扭蛋：特别奖进度 + 抽数选择（1/2/3/全包）→ 提交 SAGA → 轮询 → 揭晓（web Gashapon 页同流程）
const { t } = useI18n()
const auth = useAuthStore()

// 扭蛋无列表端点（与后端一致只有详情端点），种子仅 1 号许愿机，前端固定取之
const CONFIG_ID = 1

// 阶段：idle 选数 / submitting 提交中 / polling 轮询 / revealing 揭晓 / done 结果
const phase = ref<'idle' | 'submitting' | 'polling' | 'revealing' | 'done'>('idle')
const detail = ref<GashaponDetailResponse | null>(null)
const selectedCount = ref<number | null>(null)
const record = ref<GashaponRecordItem | null>(null)
const records = ref<GashaponRecordItem[]>([])
const failed = ref(false)
// 抽前特别奖 sent 快照：揭晓时 diff 出本次开出的特别奖
const sentBefore = ref(new Map<number, number>())

let pollTimer: ReturnType<typeof setInterval> | null = null

const isSettled = computed(() => detail.value?.status === 2)
const remaining = computed(() => detail.value?.remainingTickets ?? 0)

// 抽数选项：1/2/3 + 全包（余票≤100 时后端校验允许）
const countOptions = computed(() => {
  const opts: { label: string; value: number }[] = [1, 2, 3].map((n) => ({ label: String(n), value: n }))
  if (remaining.value > 3 && remaining.value <= 100) {
    opts.push({ label: t('gashapon.buyAll', { n: remaining.value }), value: remaining.value })
  }
  return opts
})

// 本次开出的特别奖（sent 快照 diff）
const specialWon = computed(() =>
  (detail.value?.specialPrizes ?? []).filter((p) => (sentBefore.value.get(p.prizeIndex) ?? 0) < p.sent),
)

function kindBadge(kind: string): string {
  if (kind === 'LAST') return '🏆'
  if (kind === 'MVP') return '⭐'
  return '🍀'
}

function kindText(kind: string): string {
  if (kind === 'LAST') return t('gashapon.kindLast')
  if (kind === 'MVP') return t('gashapon.kindMvp')
  return t('gashapon.kindLucky')
}

function prizeText(p: DigPrizeItem): string {
  if (p.kind === 1) return t('gashapon.prizeCash', { amount: formatMoney(p.amount) })
  if (p.kind === 2) return t('gashapon.prizePoints', { n: p.points })
  return t('gashapon.prizeCoupon', { name: p.couponName || p.couponTemplateNo })
}

// 特别奖卡片文案：种类 + 阶段 + 奖品 + 进度
function specialLine(p: GashaponSpecialPrizeItem): string {
  const stage = p.specialKind === 'LUCKY' && p.stageOrder > 0 ? ` · ${t('gashapon.stageText', { n: p.stageOrder })}` : ''
  return `${kindText(p.specialKind)}${stage} · ${prizeText(p.prize)}`
}

// 特别奖进度（a-progress → 自绘进度条）
function specialPercent(p: GashaponSpecialPrizeItem): number {
  return p.quantity > 0 ? Math.round((p.sent / p.quantity) * 100) : 0
}

async function loadDetail() {
  try {
    detail.value = await gashaponApi.getGashapon(CONFIG_ID)
  } catch (err) {
    detail.value = null
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  }
}

async function loadRecords() {
  try {
    records.value = (await gashaponApi.listGashaponRecords({ page: 1, pageSize: 5 })).items
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

// 轮询扭蛋记录：出记录后揭晓动画再展示结果
function startPoll(requestId: string) {
  phase.value = 'polling'
  let tries = 0
  pollTimer = setInterval(async () => {
    tries += 1
    try {
      const res = await gashaponApi.listGashaponRecords({ requestId })
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
      // 3025（暂无记录）或网络抖动：继续轮询
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
  // web:239 单选项 disabled = opt.value > remaining；mini 类名仅降透明，行为层镜像同条件
  if (phase.value !== 'idle' || isSettled.value || n > remaining.value) return
  selectedCount.value = selectedCount.value === n ? null : n
}

async function buy() {
  if (selectedCount.value === null) return
  // 抽前快照特别奖 sent（揭晓时 diff 出本次开出的特别奖）
  sentBefore.value = new Map((detail.value?.specialPrizes ?? []).map((p) => [p.prizeIndex, p.sent]))
  phase.value = 'submitting'
  try {
    // 每次抽取生成新幂等键：同一次抽重试复用、再次抽取是新事务
    const key = randomUUID()
    const res = await gashaponApi.gashaponBuy(CONFIG_ID, selectedCount.value, key)
    // status=success：同 Key 重试命中已有单号，直接取结果
    if (res.status === 'success') {
      const existing = await gashaponApi.listGashaponRecords({ requestId: res.requestId })
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
    uni.showModal({ title: t('gashapon.failTitle'), content: (err as Error).message, showCancel: false })
  }
}

function onBuyClick() {
  // web:257 disabled 含 phase !== 'idle'（轮询/揭晓期间阻断重复下单）；mini 类名仅降透明，行为层镜像同条件早退
  if (selectedCount.value === null) {
    uni.showToast({ title: t('gashapon.selectCount'), icon: 'none' })
    return
  }
  if (phase.value !== 'idle' || isSettled.value || remaining.value <= 0) return
  uni.showModal({
    title: t('gashapon.confirmTitle'),
    content: t('gashapon.confirmText', {
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
  <view class="gashapon-page">
    <template v-if="detail">
      <!-- 选数 / 提交 / 轮询 / 揭晓 -->
      <view v-if="phase !== 'done'" class="gacha-card gas-card">
        <view class="gas-name">{{ detail.name }}</view>
        <view class="gas-line">
          <text>{{ t('gashapon.remaining', { n: detail.remainingTickets }) }}</text>
          <text>{{ t('gashapon.totalTickets', { n: detail.totalTickets }) }}</text>
          <text>{{ t('gashapon.participants', { n: detail.participants }) }}</text>
          <text v-if="isSettled" class="settled-tag">{{ t('gashapon.settled') }}</text>
        </view>

        <!-- 特别奖进度卡 -->
        <view class="special-list">
          <view v-for="p in detail.specialPrizes" :key="p.prizeIndex" class="special-card">
            <text class="special-badge">{{ kindBadge(p.specialKind) }}</text>
            <view class="special-body">
              <view class="special-line">{{ specialLine(p) }}</view>
              <view class="progress-track" :class="{ done: p.sent >= p.quantity }">
                <view class="progress-fill" :style="{ width: specialPercent(p) + '%' }"></view>
              </view>
              <view class="special-sent">{{ t('gashapon.specialSent', { sent: p.sent, quantity: p.quantity }) }}</view>
            </view>
          </view>
        </view>

        <!-- 抽数选择 -->
        <view class="count-select">
          <text class="count-label">{{ t('gashapon.countLabel') }}</text>
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

        <!-- 单价 + 购买 -->
        <view class="quote-card">
          <view class="quote-row">
            <text>{{ t('gashapon.unitPrice') }}</text>
            <text>{{ formatMoney(detail.price) }}</text>
          </view>
        </view>
        <view
          class="btn-primary"
          :class="{ disabled: selectedCount === null || isSettled || remaining <= 0 || phase !== 'idle' }"
          @click="onBuyClick"
        >
          {{ t('gashapon.buy') }}
        </view>

        <!-- 提交/轮询/揭晓状态 -->
        <view v-if="phase === 'submitting' || phase === 'polling' || phase === 'revealing'" class="gas-status">
          <view class="gas-emoji" :class="{ shake: phase !== 'submitting' }">🥚</view>
          <view class="status-text">
            {{
              phase === 'submitting' ? t('gashapon.submitting') : phase === 'polling' ? t('gashapon.submitted') : t('gashapon.revealing')
            }}
          </view>
          <view v-if="phase !== 'revealing'" class="spinner"></view>
        </view>
      </view>

      <!-- 结果 -->
      <view v-else class="gacha-card gas-card center">
        <template v-if="record">
          <view class="result-emoji">{{ record.entries.some((e) => e.isPrize) ? '🎉' : '🥚' }}</view>
          <view class="result-title">{{ t('gashapon.resultTitle') }}</view>
          <view class="prize-list">
            <view v-for="(e, i) in record.entries" :key="i" class="prize-row">
              <text>{{ e.isPrize ? '🎁' : '🥚' }}</text>
              <text>{{ e.name }}</text>
            </view>
          </view>
          <view v-if="specialWon.length > 0" class="special-won">
            <view class="special-won-title">✨ {{ t('gashapon.specialTriggered') }}</view>
            <view v-for="p in specialWon" :key="p.prizeIndex" class="special-won-row">
              <text>{{ kindBadge(p.specialKind) }}</text>
              <text>{{ specialLine(p) }}</text>
            </view>
          </view>
          <view v-if="isSettled" class="settled-tip">{{ t('gashapon.settledTip') }}</view>
          <view class="result-actions">
            <view v-if="!isSettled" class="btn-plain" @click="backToDraw">{{ t('gashapon.backDraw') }}</view>
            <view v-else class="btn-plain" @click="backToDraw">{{ t('gashapon.records') }}</view>
          </view>
        </template>
        <template v-else-if="failed">
          <view class="result-emoji">😢</view>
          <view class="result-title">{{ t('gashapon.failTitle') }}</view>
          <view class="result-thanks">{{ t('gashapon.failText') }}</view>
          <view class="result-actions">
            <view class="btn-plain" @click="backToDraw">{{ t('gashapon.retry') }}</view>
          </view>
        </template>
      </view>

      <!-- 扭蛋记录 -->
      <view class="gacha-card records-card">
        <view class="records-title">{{ t('gashapon.records') }}</view>
        <template v-if="records.length > 0">
          <view v-for="r in records" :key="r.recordId" class="record-row">
            <text class="record-entries">{{ r.entries.map((e) => e.name).join('、') }}</text>
            <text class="record-count">{{ t('gashapon.recordCount', { n: r.count }) }}</text>
            <text class="record-price money">{{ formatMoney(r.price) }}</text>
            <text class="record-time">{{ formatUnix(r.createdAt) }}</text>
          </view>
        </template>
        <view v-else class="records-empty">{{ t('gashapon.recordEmpty') }}</view>
      </view>
    </template>

    <view v-else class="gacha-card gas-card center">
      <view class="records-empty">{{ t('gashapon.empty') }}</view>
    </view>
  </view>
</template>

<style scoped lang="scss">
.gas-card {
  text-align: left;

  .gas-name {
    font-size: 40rpx;
    font-weight: 700;
    margin-bottom: 8rpx;
  }

  .gas-line {
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

// 特别奖进度卡
.special-list {
  display: flex;
  flex-direction: column;
  gap: 16rpx;
  margin: 20rpx 0;

  .special-card {
    display: flex;
    gap: 20rpx;
    align-items: flex-start;
    border: 1rpx solid var(--color-border);
    border-radius: 12rpx;
    padding: 16rpx 20rpx;
    background: var(--color-bg);

    .special-badge {
      font-size: 44rpx;
      line-height: 1.2;
    }

    .special-body {
      flex: 1;
      min-width: 0;

      .special-line {
        font-size: 26rpx;
        margin-bottom: 8rpx;
      }

      .special-sent {
        font-size: 22rpx;
        color: var(--color-text-secondary);
        margin-top: 4rpx;
      }
    }
  }
}

// 抽数选择
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

.gas-status {
  text-align: center;
  margin-top: 24rpx;

  .gas-emoji {
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

  .prize-row {
    display: flex;
    gap: 16rpx;
    padding: 8rpx 0;
    font-size: 28rpx;
    justify-content: center;
  }
}

.special-won {
  text-align: left;
  border: 1rpx dashed var(--color-accent);
  border-radius: 12rpx;
  padding: 16rpx 20rpx;
  margin-bottom: 24rpx;

  .special-won-title {
    font-size: 26rpx;
    font-weight: 600;
    color: var(--color-accent);
    margin-bottom: 8rpx;
  }

  .special-won-row {
    display: flex;
    gap: 16rpx;
    font-size: 26rpx;
    padding: 4rpx 0;
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

    .record-entries {
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

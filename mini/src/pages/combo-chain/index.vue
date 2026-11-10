<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onUnload } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as comboChainApi from '@/services/comboChain'
import * as userApi from '@/services/user'
import { useAuthStore } from '@/store/authStore'
import { formatMoney, formatUnix } from '@/utils/format'
import { randomUUID } from '@/utils/id'

import type { ComboChainDetailResponse, ComboChainRecordItem, DigPrizeItem } from '@/services/types'

// 连击链：全局奖槽 + 共享奖池 + 我的连击 → 购票 SAGA → 轮询 → 揭晓（web ComboChain 页同流程）
const { t } = useI18n()
const auth = useAuthStore()

// 连击链无列表端点（与后端一致只有详情端点），种子仅 1 号配置，前端固定取之
const CONFIG_ID = 1

// 阶段：idle 待购票 / submitting 提交中 / polling 轮询 / revealing 揭晓 / done 结果
const phase = ref<'idle' | 'submitting' | 'polling' | 'revealing' | 'done'>('idle')
const detail = ref<ComboChainDetailResponse | null>(null)
const record = ref<ComboChainRecordItem | null>(null)
const records = ref<ComboChainRecordItem[]>([])
const failed = ref(false)

let pollTimer: ReturnType<typeof setInterval> | null = null

const isSettled = computed(() => detail.value?.status === 2)
const remaining = computed(() => detail.value?.remainingTickets ?? 0)
// 当前车是否已购票（记录里出现当前车号的正常记录即已上车；一车一票）
const alreadyOnBoard = computed(
  () =>
    !!detail.value &&
    records.value.some((r) => r.carIndex === detail.value!.carIndex && r.status === 1),
)

function kindBadge(kind: string): string {
  if (kind === 'COMBO_TIER') return '🎯'
  if (kind === 'EMPTY_STREAK') return '🔥'
  return '💫'
}

function kindText(kind: string): string {
  if (kind === 'COMBO_TIER') return t('comboChain.kindComboTier')
  if (kind === 'EMPTY_STREAK') return t('comboChain.kindEmptyStreak')
  return t('comboChain.kindWinStreak')
}

function prizeText(p: DigPrizeItem): string {
  if (p.kind === 1) return t('comboChain.prizeCash', { amount: formatMoney(p.amount) })
  if (p.kind === 2) return t('comboChain.prizePoints', { n: p.points })
  return t('comboChain.prizeCoupon', { name: p.couponName || p.couponTemplateNo })
}

async function loadDetail() {
  try {
    detail.value = await comboChainApi.getComboChain(CONFIG_ID)
  } catch (err) {
    detail.value = null
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  }
}

async function loadRecords() {
  try {
    records.value = (await comboChainApi.listComboChainRecords({ page: 1, pageSize: 10 })).items
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
}

function stopPoll() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

// 轮询连击链记录：出记录后揭晓动画再展示结果
function startPoll(requestId: string) {
  phase.value = 'polling'
  let tries = 0
  pollTimer = setInterval(async () => {
    tries += 1
    try {
      const res = await comboChainApi.listComboChainRecords({ requestId })
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
      // 3029（暂无记录）或网络抖动：继续轮询
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

async function buy() {
  phase.value = 'submitting'
  try {
    // 每次购票生成新幂等键：同一次购票重试复用、再次购票是新事务
    const key = randomUUID()
    const res = await comboChainApi.comboChainBuy(CONFIG_ID, key)
    // status=success：同 Key 重试命中已有单号，直接取结果
    if (res.status === 'success') {
      const existing = await comboChainApi.listComboChainRecords({ requestId: res.requestId })
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
    uni.showModal({ title: t('comboChain.failTitle'), content: (err as Error).message, showCancel: false })
  }
}

function onBuyClick() {
  // web:235 disabled = isSettled || remaining <= 0 || alreadyOnBoard || phase !== 'idle'（真正阻断点击）；mini 类名仅置灰，行为层镜像同条件早退
  if (phase.value !== 'idle' || isSettled.value || remaining.value <= 0 || alreadyOnBoard.value) return
  uni.showModal({
    title: t('comboChain.confirmTitle'),
    content: t('comboChain.confirmText', { price: formatMoney(detail.value?.price ?? '0.00') }),
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
  <view class="combo-page">
    <template v-if="detail">
      <!-- 待购票 / 提交 / 轮询 / 揭晓 -->
      <view v-if="phase !== 'done'" class="gacha-card combo-card">
        <view class="combo-name">{{ detail.name }}</view>
        <view class="combo-line">
          <text>{{ t('comboChain.carNo', { index: detail.carIndex, total: detail.totalCars }) }}</text>
          <text>{{ t('comboChain.remaining', { n: detail.remainingTickets }) }}</text>
          <text>{{ t('comboChain.participants', { n: detail.participants }) }}</text>
          <text v-if="isSettled" class="settled-tag">{{ t('comboChain.settled') }}</text>
        </view>

        <!-- 全局奖槽 -->
        <view v-if="detail.globalSlots.length > 0" class="global-slots">
          <view class="section-title">🎁 {{ t('comboChain.globalSlots') }}</view>
          <view class="slot-chips">
            <text v-for="(s, i) in detail.globalSlots" :key="i" class="slot-chip">{{ prizeText(s) }}</text>
          </view>
        </view>

        <!-- 共享奖池 -->
        <view v-if="detail.sharedPrizes.length > 0" class="shared-list">
          <view class="section-title">✨ {{ t('comboChain.sharedPrizes') }}</view>
          <view v-for="(p, i) in detail.sharedPrizes" :key="i" class="shared-card">
            <text class="shared-badge">{{ kindBadge(p.kind) }}</text>
            <view class="shared-body">
              <view class="shared-line">
                <text>{{ kindText(p.kind) }}</text>
                <text>{{ prizeText(p.prize) }}</text>
              </view>
              <view class="shared-meta">
                <text v-if="p.kind === 'COMBO_TIER' && p.quantity > 0">
                  {{ t('comboChain.tierRemaining', { remaining: p.remaining, quantity: p.quantity }) }}
                </text>
                <text v-if="p.tierWeight > 0">{{ t('comboChain.tierWeight', { w: p.tierWeight }) }}</text>
                <text v-if="p.concentrateThreshold > 0">{{ t('comboChain.threshold', { n: p.concentrateThreshold }) }}</text>
                <text v-if="p.streakThreshold > 0">{{ t('comboChain.streakNeed', { n: p.streakThreshold }) }}</text>
              </view>
              <view
                v-if="p.kind === 'COMBO_TIER' && p.quantity > 0"
                class="progress-track"
                :class="{ done: p.remaining <= 0 }"
              >
                <view
                  class="progress-fill"
                  :style="{ width: (p.remaining >= 0 && p.quantity > 0 ? Math.round(((p.quantity - p.remaining) / p.quantity) * 100) : 0) + '%' }"
                ></view>
              </view>
            </view>
          </view>
        </view>

        <!-- 我的连击 -->
        <view class="streak-row">
          <text>🔥 {{ t('comboChain.emptyStreak', { n: detail.emptyStreak }) }}</text>
          <text>💫 {{ t('comboChain.winStreak', { n: detail.winStreak }) }}</text>
        </view>

        <!-- 单价 + 购票 -->
        <view class="quote-card">
          <view class="quote-row">
            <text>{{ t('comboChain.unitPrice') }}</text>
            <text>{{ formatMoney(detail.price) }}</text>
          </view>
        </view>
        <view
          class="btn-primary"
          :class="{ disabled: isSettled || remaining <= 0 || alreadyOnBoard || phase !== 'idle' }"
          @click="onBuyClick"
        >
          {{
            alreadyOnBoard
              ? t('comboChain.onBoard')
              : isSettled
                ? t('comboChain.settled')
                : t('comboChain.buy')
          }}
        </view>

        <!-- 提交/轮询/揭晓状态 -->
        <view v-if="phase === 'submitting' || phase === 'polling' || phase === 'revealing'" class="combo-status">
          <view class="combo-emoji" :class="{ shake: phase !== 'submitting' }">🚌</view>
          <view class="status-text">
            {{
              phase === 'submitting' ? t('comboChain.submitting') : phase === 'polling' ? t('comboChain.submitted') : t('comboChain.revealing')
            }}
          </view>
          <view v-if="phase !== 'revealing'" class="spinner"></view>
        </view>
      </view>

      <!-- 结果 -->
      <view v-else class="gacha-card combo-card center">
        <template v-if="record">
          <view class="result-emoji">{{ record.awards.length > 0 ? '🎉' : '🚌' }}</view>
          <view class="result-title">{{ t('comboChain.resultTitle') }}</view>
          <view v-if="record.awards.length > 0" class="prize-list">
            <view class="awards-title">✨ {{ t('comboChain.awardsTitle') }}</view>
            <view v-for="(p, i) in record.awards" :key="i" class="prize-row">
              <text>🎁</text>
              <text>{{ prizeText(p) }}</text>
            </view>
          </view>
          <view v-else class="result-thanks">{{ t('comboChain.noAward') }}</view>
          <view v-if="isSettled" class="settled-tip">{{ t('comboChain.settledTip') }}</view>
          <view class="result-actions">
            <view v-if="!isSettled" class="btn-plain" @click="backToDraw">{{ t('comboChain.backDraw') }}</view>
            <view v-else class="btn-plain" @click="backToDraw">{{ t('comboChain.records') }}</view>
          </view>
        </template>
        <template v-else-if="failed">
          <view class="result-emoji">😢</view>
          <view class="result-title">{{ t('comboChain.failTitle') }}</view>
          <view class="result-thanks">{{ t('comboChain.failText') }}</view>
          <view class="result-actions">
            <view class="btn-plain" @click="backToDraw">{{ t('comboChain.retry') }}</view>
          </view>
        </template>
      </view>

      <!-- 购票记录 -->
      <view class="gacha-card records-card">
        <view class="records-title">{{ t('comboChain.records') }}</view>
        <template v-if="records.length > 0">
          <view v-for="r in records" :key="r.recordId" class="record-row">
            <text class="record-car">{{ t('comboChain.recordCar', { n: r.carIndex }) }}</text>
            <text class="record-awards">{{ r.awards.map((p) => prizeText(p)).join('、') || '—' }}</text>
            <text class="record-price money">{{ formatMoney(r.price) }}</text>
            <text class="record-time">{{ formatUnix(r.createdAt) }}</text>
          </view>
        </template>
        <view v-else class="records-empty">{{ t('comboChain.recordEmpty') }}</view>
      </view>
    </template>

    <view v-else class="gacha-card combo-card center">
      <view class="records-empty">{{ t('comboChain.empty') }}</view>
    </view>
  </view>
</template>

<style scoped lang="scss">
.combo-card {
  text-align: left;

  .combo-name {
    font-size: 40rpx;
    font-weight: 700;
    margin-bottom: 8rpx;
  }

  .combo-line {
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

// 全局奖槽
.global-slots {
  margin-bottom: 8rpx;

  .slot-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 12rpx;

    .slot-chip {
      font-size: 24rpx;
      border: 1rpx solid var(--color-border);
      border-radius: 999rpx;
      padding: 6rpx 20rpx;
      background: var(--color-bg);
    }
  }
}

// 共享奖池
.shared-list {
  display: flex;
  flex-direction: column;
  gap: 16rpx;
  margin: 16rpx 0;

  .shared-card {
    display: flex;
    gap: 20rpx;
    align-items: flex-start;
    border: 1rpx solid var(--color-border);
    border-radius: 12rpx;
    padding: 16rpx 20rpx;
    background: var(--color-bg);

    .shared-badge {
      font-size: 44rpx;
      line-height: 1.2;
    }

    .shared-body {
      flex: 1;
      min-width: 0;

      .shared-line {
        display: flex;
        justify-content: space-between;
        gap: 16rpx;
        font-size: 26rpx;
        margin-bottom: 4rpx;
      }

      .shared-meta {
        display: flex;
        gap: 16rpx;
        flex-wrap: wrap;
        font-size: 22rpx;
        color: var(--color-text-secondary);
        margin-bottom: 8rpx;
      }
    }
  }
}

.streak-row {
  display: flex;
  gap: 32rpx;
  font-size: 26rpx;
  color: var(--color-accent);
  margin: 16rpx 0;
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

.combo-status {
  text-align: center;
  margin-top: 24rpx;

  .combo-emoji {
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

    .record-car {
      color: var(--color-accent);
      font-weight: 600;
    }

    .record-awards {
      flex: 1;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
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

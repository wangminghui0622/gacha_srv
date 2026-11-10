<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onUnload } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as fullCarApi from '@/services/fullCar'
import * as userApi from '@/services/user'
import { useAuthStore } from '@/store/authStore'
import { formatMoney, formatUnix } from '@/utils/format'
import { randomUUID } from '@/utils/id'

import type { FullCarAwardItem, FullCarDetailResponse, FullCarRecordItem, DigPrizeItem } from '@/services/types'

// 满车：选座 → 确认 → 提交 SAGA → 轮询记录 → 揭晓（web FullCar 页同流程）
const { t } = useI18n()
const auth = useAuthStore()

// 满车无列表端点（与后端一致只有详情端点），种子仅 1 号专列，前端固定取之
const CONFIG_ID = 1

// 阶段：idle 选座 / submitting 提交中 / polling 轮询 / revealing 揭晓 / done 结果
const phase = ref<'idle' | 'submitting' | 'polling' | 'revealing' | 'done'>('idle')
const detail = ref<FullCarDetailResponse | null>(null)
const selectedSeat = ref<number | null>(null)
const record = ref<FullCarRecordItem | null>(null)
const records = ref<FullCarRecordItem[]>([])
const failed = ref(false)

// 幂等键：单次购买会话内复用（网络错误重试走同一 SAGA 单号）
const idempotencyKey = randomUUID()
let pollTimer: ReturnType<typeof setInterval> | null = null

const isDeparted = computed(() => detail.value?.status === 2)

// 座位列表：detail.seats 补齐到 seatCount（防御状态行长度异常）
const seats = computed(() => {
  const d = detail.value
  if (!d) return []
  const list: { seatIndex: number; userId: number }[] = []
  for (let i = 0; i < d.seatCount; i += 1) {
    list.push(d.seats[i] ?? { seatIndex: i, userId: 0 })
  }
  return list
})

// 整车奖按座索引（DEPARTED 后展示）
const awardBySeat = computed(() => {
  const map = new Map<number, FullCarAwardItem>()
  for (const a of detail.value?.awards ?? []) map.set(a.seatIndex, a)
  return map
})

// 座位网格列数：seatCount 上限 4 列（web 同款 repeat(min(seatCount,4))）
const gridCols = computed(() => Math.min(detail.value?.seatCount ?? 1, 4))
const seatWidth = computed(() => `calc(${100 / gridCols.value}% - 12rpx)`)

function seatMask(userId: number): string {
  return t('fullCar.seatMask', { suffix: String(userId).slice(-4) })
}

function prizeText(p: DigPrizeItem): string {
  if (p.kind === 1) return t('fullCar.prizeCash', { amount: formatMoney(p.amount) })
  if (p.kind === 2) return t('fullCar.prizePoints', { n: p.points })
  return t('fullCar.prizeCoupon', { name: p.couponName || p.couponTemplateNo })
}

function awardText(a: FullCarAwardItem): string {
  const tier = a.tier === 1 ? t('fullCar.tierLucky') : t('fullCar.tierNormal')
  return `${tier} · ${prizeText(a.prize)}`
}

async function loadDetail() {
  try {
    detail.value = await fullCarApi.getFullCar(CONFIG_ID)
  } catch (err) {
    detail.value = null
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  }
}

async function loadRecords() {
  try {
    records.value = (await fullCarApi.listFullCarRecords({ page: 1, pageSize: 5 })).items
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
  selectedSeat.value = null
}

function stopPoll() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

// 轮询满车记录：出记录后揭晓动画再展示结果
function startPoll(requestId: string) {
  phase.value = 'polling'
  let tries = 0
  pollTimer = setInterval(async () => {
    tries += 1
    try {
      const res = await fullCarApi.listFullCarRecords({ requestId })
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
      // 3024（暂无记录）或网络抖动：继续轮询
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

function selectSeat(i: number) {
  const s = seats.value[i]
  if (!s || phase.value !== 'idle' || s.userId !== 0 || isDeparted.value) return
  selectedSeat.value = selectedSeat.value === i ? null : i
}

async function buy() {
  if (selectedSeat.value === null) return
  phase.value = 'submitting'
  try {
    const res = await fullCarApi.fullCarBuy(CONFIG_ID, idempotencyKey, selectedSeat.value)
    // status=success：同 Key 重试命中已有单号，直接取结果
    if (res.status === 'success') {
      const existing = await fullCarApi.listFullCarRecords({ requestId: res.requestId })
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
    uni.showModal({ title: t('fullCar.failTitle'), content: (err as Error).message, showCancel: false })
  }
}

function onBuyClick() {
  // web:243 disabled = selectedSeat === null || isDeparted || remainingTickets <= 0（真正阻断点击）；mini 类名仅降透明，行为层镜像同条件早退
  if (selectedSeat.value === null) {
    uni.showToast({ title: t('fullCar.selectSeat'), icon: 'none' })
    return
  }
  if (isDeparted.value || (detail.value?.remainingTickets ?? 0) <= 0) return
  uni.showModal({
    title: t('fullCar.confirmTitle'),
    content: t('fullCar.confirmText', { seat: selectedSeat.value + 1, price: formatMoney(detail.value?.price ?? '0.00') }),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    success: (res) => {
      if (res.confirm) void buy()
    },
  })
}

function backToSeats() {
  selectedSeat.value = null
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
  <view class="full-car-page">
    <template v-if="detail">
      <!-- 选座 / 提交 / 轮询 / 揭晓 -->
      <view v-if="phase !== 'done'" class="gacha-card car-card">
        <view class="car-name">{{ detail.name }}</view>
        <view class="car-line">
          <text>{{ t('fullCar.remaining', { n: detail.remainingTickets }) }}</text>
          <text>{{ t('fullCar.seatCount', { n: detail.seatCount }) }}</text>
          <text v-if="isDeparted" class="departed-tag">{{ t('fullCar.departed') }}</text>
        </view>

        <!-- 座位网格：空位可点、已占显示掩码、DEPARTED 显示整车奖 -->
        <view class="seat-grid">
          <view
            v-for="s in seats"
            :key="s.seatIndex"
            class="seat"
            :class="{
              occupied: s.userId !== 0,
              selected: selectedSeat === s.seatIndex,
              revealed: phase === 'revealing' && record?.seatIndex === s.seatIndex,
              departed: isDeparted,
            }"
            :style="{ width: seatWidth }"
            @click="selectSeat(s.seatIndex)"
          >
            <template v-if="isDeparted && awardBySeat.has(s.seatIndex)">
              <text class="seat-award">{{ awardBySeat.get(s.seatIndex)!.tier === 1 ? '👑' : '🎁' }}</text>
              <text class="seat-award-text">{{ awardText(awardBySeat.get(s.seatIndex)!) }}</text>
            </template>
            <template v-else-if="s.userId !== 0">
              <text>🧑‍✈️</text>
              <text class="seat-mask">{{ seatMask(s.userId) }}</text>
            </template>
            <template v-else>
              <text v-if="selectedSeat === s.seatIndex">✦</text>
              <text v-else class="seat-idx">{{ s.seatIndex + 1 }}</text>
            </template>
          </view>
        </view>

        <!-- 单价 + 购买 -->
        <view class="quote-card">
          <view class="quote-row">
            <text>{{ t('fullCar.unitPrice') }}</text>
            <text>{{ formatMoney(detail.price) }}</text>
          </view>
        </view>
        <view
          class="btn-primary"
          :class="{ disabled: selectedSeat === null || isDeparted || detail.remainingTickets <= 0 }"
          @click="onBuyClick"
        >
          {{ t('fullCar.buy') }}
        </view>

        <!-- 提交/轮询/揭晓状态 -->
        <view v-if="phase === 'submitting' || phase === 'polling' || phase === 'revealing'" class="car-status">
          <view class="car-emoji" :class="{ shake: phase !== 'submitting' }">🚂</view>
          <view class="status-text">
            {{
              phase === 'submitting' ? t('fullCar.submitting') : phase === 'polling' ? t('fullCar.submitted') : t('fullCar.revealing')
            }}
          </view>
          <view v-if="phase !== 'revealing'" class="spinner"></view>
        </view>
      </view>

      <!-- 结果 -->
      <view v-else class="gacha-card car-card center">
        <template v-if="record">
          <view class="result-emoji">{{ record.isPrize ? '🎉' : '🚂' }}</view>
          <view class="result-title">{{ t('fullCar.resultTitle') }}</view>
          <view class="result-seat">{{ t('fullCar.seatNo', { seat: record.seatIndex + 1 }) }}</view>
          <view class="prize-list">
            <view class="prize-row">
              <text>🎟️</text>
              <text>{{ record.entryName }}</text>
            </view>
          </view>
          <view v-if="isDeparted" class="departed-tip">{{ t('fullCar.carDepartedTip') }}</view>
          <view class="result-actions">
            <view class="btn-plain" @click="backToSeats">{{ t('fullCar.backSeats') }}</view>
          </view>
        </template>
        <template v-else-if="failed">
          <view class="result-emoji">😢</view>
          <view class="result-title">{{ t('fullCar.failTitle') }}</view>
          <view class="result-thanks">{{ t('fullCar.failText') }}</view>
          <view class="result-actions">
            <view class="btn-plain" @click="backToSeats">{{ t('fullCar.retry') }}</view>
          </view>
        </template>
      </view>

      <!-- 乘车记录 -->
      <view class="gacha-card records-card">
        <view class="records-title">{{ t('fullCar.records') }}</view>
        <template v-if="records.length > 0">
          <view v-for="r in records" :key="r.recordId" class="record-row">
            <text class="record-seat">{{ t('fullCar.seatNo', { seat: r.seatIndex + 1 }) }}</text>
            <text class="record-entry">{{ r.entryName }}</text>
            <text class="record-price money">{{ formatMoney(r.price) }}</text>
            <text class="record-time">{{ formatUnix(r.createdAt) }}</text>
          </view>
        </template>
        <view v-else class="records-empty">{{ t('fullCar.recordEmpty') }}</view>
      </view>
    </template>

    <view v-else class="gacha-card car-card center">
      <view class="records-empty">{{ t('fullCar.empty') }}</view>
    </view>
  </view>
</template>

<style scoped lang="scss">
.car-card {
  text-align: left;

  .car-name {
    font-size: 40rpx;
    font-weight: 700;
    margin-bottom: 8rpx;
  }

  .car-line {
    font-size: 26rpx;
    color: var(--color-text-secondary);
    margin-bottom: 20rpx;
    display: flex;
    gap: 20rpx;

    .departed-tag {
      color: var(--color-accent);
    }
  }
}

// 座位网格：flex-wrap（web 为 CSS grid repeat(min(seatCount,4))）
.seat-grid {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  margin: 20rpx -6rpx;

  .seat {
    box-sizing: border-box;
    margin: 6rpx;
    height: 140rpx;
    border: 1rpx solid var(--color-border);
    border-radius: 12rpx;
    background: var(--color-bg);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 4rpx;
    font-size: 32rpx;

    .seat-idx {
      color: var(--color-text-secondary);
      font-size: 26rpx;
    }

    .seat-mask {
      font-size: 20rpx;
      color: var(--color-text-secondary);
    }

    .seat-award {
      font-size: 40rpx;
    }

    .seat-award-text {
      font-size: 18rpx;
      color: var(--color-accent);
      text-align: center;
      line-height: 1.2;
    }

    &.occupied {
      background: #f0f0f0;
      opacity: 0.8;
    }

    &.departed {
      background: #fff7e6;
      opacity: 1;
    }

    &.selected {
      border-color: var(--color-accent);
      box-shadow: 0 0 0 3rpx rgba(255, 165, 0, 0.25);
    }

    &.revealed {
      animation: pop 0.5s ease;
      border-color: var(--color-accent);
      background: #fff7e6;
    }
  }
}

@keyframes pop {
  0% { transform: scale(0.6) rotate(-20deg); opacity: 0.3; }
  60% { transform: scale(1.25) rotate(6deg); }
  100% { transform: scale(1) rotate(0); opacity: 1; }
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

.car-status {
  text-align: center;
  margin-top: 24rpx;

  .car-emoji {
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

  .result-seat {
    font-size: 28rpx;
    margin-bottom: 16rpx;
  }

  .departed-tip {
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
  margin-bottom: 32rpx;

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

    .record-seat {
      color: var(--color-accent);
      font-weight: 600;
    }

    .record-entry {
      max-width: 35%;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .record-price {
      color: var(--color-accent);
      font-weight: 600;
    }

    .record-time {
      margin-left: auto;
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

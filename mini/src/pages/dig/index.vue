<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { onLoad, onUnload } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as digApi from '@/services/dig'
import * as userApi from '@/services/user'
import { useAuthStore } from '@/store/authStore'
import { formatMoney, formatUnix } from '@/utils/format'
import { randomUUID } from '@/utils/id'

import type { DigBoxItem, DigBoxSessionResponse, DigPrizeItem, DigQuoteResponse, DigRecordItem } from '@/services/types'

// 挖宝箱：选格 → 报价 → 提交 SAGA → 轮询记录 → 揭晓（web DigBox 页同流程，grid 换 flex-wrap）
const { t } = useI18n()
const auth = useAuthStore()

// 阶段：idle 选格 / submitting 提交中 / polling 轮询 / revealing 揭晓 / done 结果
const phase = ref<'idle' | 'submitting' | 'polling' | 'revealing' | 'done'>('idle')
const boxes = ref<DigBoxItem[]>([])
const box = ref<DigBoxItem | null>(null)
const session = ref<DigBoxSessionResponse | null>(null)
const quote = ref<DigQuoteResponse | null>(null)
const quoteLoading = ref(false)
const selected = ref<Set<number>>(new Set())
const takeAll = ref(false)
const record = ref<DigRecordItem | null>(null)
const records = ref<DigRecordItem[]>([])
const failed = ref(false)

// 幂等键：单次购买会话内复用（网络错误重试走同一 SAGA 单号）
const idempotencyKey = randomUUID()
let pollTimer: ReturnType<typeof setInterval> | null = null

// 已挖格子集合（会话位图；无会话=全空格）
const dugCells = computed(() => {
  const set = new Set<number>()
  const s = session.value
  if (!s) return set
  for (let i = 0; i < s.cellCount; i += 1) {
    if (Number(s.occupiedBitmap) & (1 << i)) set.add(i)
  }
  return set
})

const isCompleted = computed(() => session.value?.status === 2)

// 格子棋盘列数：cellCount 上限 5 列（web 同款 repeat(min(cellCount,5))）
const gridCols = computed(() => Math.min(box.value?.cellCount ?? 1, 5))
const cellWidth = computed(() => `calc(${100 / gridCols.value}% - 12rpx)`)

function isDug(i: number): boolean {
  return dugCells.value.has(i)
}

function isSelected(i: number): boolean {
  return takeAll.value || selected.value.has(i)
}

async function loadBoxes() {
  try {
    boxes.value = (await digApi.listDigBoxes()).items
    box.value = boxes.value[0] ?? null
  } catch (err) {
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  }
}

async function loadSession() {
  if (!box.value) return
  try {
    session.value = await digApi.getDigSession(box.value.boxId)
  } catch (err) {
    session.value = null
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  }
}

async function loadRecords() {
  if (!box.value) return
  try {
    records.value = (await digApi.listDigRecords({ page: 1, pageSize: 5 })).items
  } catch {
    // 记录列表失败不阻塞主流程
  }
}

// 报价：选格或全开变化时重新试算（服务端单价）
async function loadQuote() {
  if (!box.value) return
  quoteLoading.value = true
  try {
    quote.value = await digApi.digQuote(
      box.value.boxId,
      takeAll.value ? undefined : [...selected.value].sort((a, b) => a - b),
      takeAll.value,
    )
  } catch (err) {
    quote.value = null
    if (phase.value === 'idle') uni.showToast({ title: (err as Error).message, icon: 'none' })
  } finally {
    quoteLoading.value = false
  }
}

function toggleCell(i: number) {
  if (phase.value !== 'idle' || isDug(i)) return
  takeAll.value = false
  const next = new Set(selected.value)
  if (next.has(i)) {
    next.delete(i)
  } else {
    next.add(i)
  }
  selected.value = next
  void loadQuote()
}

function onTakeAllChange(e: Event) {
  if (phase.value !== 'idle') return // web toggleTakeAll 同款守门（开关 disabled 是展示层，行为层再兜一道）
  // uni 组件事件模板类型是 Event，运行时为 { detail: { value } }
  takeAll.value = Boolean((e as unknown as { detail: { value: boolean } }).detail.value)
  if (takeAll.value) selected.value = new Set()
  void loadQuote()
}

function stopPoll() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

// 轮询挖宝记录：出记录后揭晓动画再展示结果
function startPoll(requestId: string) {
  phase.value = 'polling'
  let tries = 0
  pollTimer = setInterval(async () => {
    tries += 1
    try {
      const res = await digApi.listDigRecords({ requestId })
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
      // 3019（暂无记录）或网络抖动：继续轮询
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

async function refreshAll() {
  await Promise.allSettled([loadSession(), loadRecords(), refreshProfile()])
  selected.value = new Set()
  takeAll.value = false
}

// 结果页 → 回到选格：默认回到空选（用户重新选择），报价随之刷新
function backToGrid(takeAllNext: boolean) {
  takeAll.value = takeAllNext
  selected.value = new Set()
  phase.value = 'idle'
  void loadQuote()
}

async function refreshProfile() {
  try {
    auth.setUser(await userApi.getProfile())
  } catch {
    // 401 由适配器统一处理
  }
}

async function buy() {
  if (!box.value) return
  phase.value = 'submitting'
  try {
    const res = await digApi.digBuy(
      box.value.boxId,
      idempotencyKey,
      takeAll.value ? undefined : [...selected.value],
      takeAll.value,
    )
    // status=success：同 Key 重试命中已有单号，直接取结果
    if (res.status === 'success') {
      const existing = await digApi.listDigRecords({ requestId: res.requestId })
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
    uni.showModal({ title: t('dig.failTitle'), content: (err as Error).message, showCancel: false })
  }
}

function onBuyClick() {
  // web:309 disabled = !quote || quote.count === 0（真正阻断点击）；mini 类名仅降透明，行为层镜像同条件早退
  if (!quote.value) {
    uni.showToast({ title: t('dig.quotePending'), icon: 'none' })
    return
  }
  if (quote.value.count === 0) return
  const count = quote.value.count
  uni.showModal({
    title: t('dig.confirmTitle'),
    content: t('dig.confirmText', { n: count, price: formatMoney(quote.value.payable) }),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    success: (res) => {
      if (res.confirm) void buy()
    },
  })
}

// 记录奖品文案
function prizeText(p: DigPrizeItem): string {
  if (p.kind === 1) return t('dig.prizeCash', { amount: formatMoney(p.amount) })
  if (p.kind === 2) return t('dig.prizePoints', { n: p.points })
  return t('dig.prizeCoupon', { name: p.couponName || p.couponTemplateNo })
}

// 本次揭晓的格子（记录 cellsChosen）
const revealedCells = computed(() => {
  if (!record.value) return new Set<number>()
  return new Set(
    record.value.cellsChosen
      .split(',')
      .map((s) => Number(s))
      .filter((n) => Number.isInteger(n)),
  )
})

watch(box, () => {
  void refreshAll()
})

onLoad(async () => {
  await loadBoxes()
  await refreshAll()
  if (phase.value === 'idle') void loadQuote()
})
onUnload(stopPoll)
</script>

<template>
  <view class="dig-page">
    <template v-if="box">
      <!-- 选格 / 提交 / 轮询 / 揭晓 -->
      <view v-if="phase !== 'done'" class="gacha-card dig-card">
        <view class="dig-box-name">{{ box.name }}</view>
        <view class="dig-line">
          {{ t('dig.dug', { n: dugCells.size, total: box.cellCount }) }}
          <text v-if="isCompleted" class="completed-tag">{{ t('dig.completed') }}</text>
        </view>

        <!-- 格子棋盘：cellCount 格，已挖灰显，选中高亮 -->
        <view class="cell-grid">
          <view
            v-for="i in box.cellCount"
            :key="i - 1"
            class="cell"
            :class="{
              dug: isDug(i - 1),
              selected: isSelected(i - 1),
              revealed: phase === 'revealing' && revealedCells.has(i - 1),
            }"
            :style="{ width: cellWidth }"
            @click="toggleCell(i - 1)"
          >
            <text v-if="isDug(i - 1)">💎</text>
            <text v-else-if="isSelected(i - 1)">✦</text>
            <text v-else class="cell-idx">{{ i }}</text>
          </view>
        </view>

        <!-- 全开开关 + 报价 -->
        <view class="takeall-row">
          <text>{{ t('dig.takeAll') }}</text>
          <switch
            :checked="takeAll"
            :disabled="phase !== 'idle'"
            color="#6c4cf1"
            style="transform: scale(0.8)"
            @change="onTakeAllChange"
          />
        </view>
        <view class="quote-card">
          <view v-if="quoteLoading" class="quote-loading">{{ t('common.loading') }}</view>
          <template v-if="quote">
            <view class="quote-row">
              <text>{{ t('dig.quoteCount') }}</text>
              <text>{{ quote.count }}</text>
            </view>
            <view class="quote-row">
              <text>{{ t('dig.unitPrice') }}</text>
              <text>{{ formatMoney(quote.unitPrice) }}</text>
            </view>
            <view class="quote-row payable">
              <text>{{ t('dig.payable') }}</text>
              <text class="money payable-value">{{ formatMoney(quote.payable) }}</text>
            </view>
          </template>
          <view v-else class="quote-empty">{{ t('dig.quotePending') }}</view>
        </view>

        <view class="btn-primary" :class="{ disabled: !quote || quote.count === 0 }" @click="onBuyClick">
          {{ t('dig.buy') }}
        </view>

        <!-- 提交/轮询/揭晓状态 -->
        <view v-if="phase === 'submitting' || phase === 'polling' || phase === 'revealing'" class="dig-status">
          <view class="box-emoji" :class="{ shake: phase !== 'submitting' }">⛏️</view>
          <view class="status-text">
            {{
              phase === 'submitting' ? t('dig.submitting') : phase === 'polling' ? t('dig.submitted') : t('dig.revealing')
            }}
          </view>
          <view v-if="phase !== 'revealing'" class="spinner"></view>
        </view>
      </view>

      <!-- 结果 -->
      <view v-else class="gacha-card dig-card center">
        <template v-if="record">
          <view class="result-emoji">🎉</view>
          <view class="result-title">{{ t('dig.resultTitle') }}</view>
          <view class="prize-list">
            <view v-for="(p, idx) in record.prizes" :key="idx" class="prize-row">
              <text>🎁</text>
              <text>{{ prizeText(p) }}</text>
            </view>
          </view>
          <view class="result-actions">
            <view v-if="isCompleted" class="btn-primary" @click="backToGrid(true)">
              {{ t('dig.restart') }}
            </view>
            <view v-else class="btn-plain" @click="backToGrid(false)">
              {{ t('dig.continue') }}
            </view>
            <view class="btn-plain" @click="backToGrid(false)">{{ t('dig.backGrid') }}</view>
          </view>
        </template>
        <template v-else-if="failed">
          <view class="result-emoji">😢</view>
          <view class="result-title">{{ t('dig.failTitle') }}</view>
          <view class="result-thanks">{{ t('dig.failText') }}</view>
          <view class="result-actions">
            <view class="btn-plain" @click="backToGrid(false)">{{ t('dig.retry') }}</view>
          </view>
        </template>
      </view>

      <!-- 挖宝记录 -->
      <view class="gacha-card records-card">
        <view class="records-title">{{ t('dig.records') }}</view>
        <template v-if="records.length > 0">
          <view v-for="r in records" :key="r.recordId" class="record-row">
            <text class="record-cells">[{{ r.cellsChosen }}]</text>
            <text class="record-price money">{{ formatMoney(r.price) }}</text>
            <text class="record-time">{{ formatUnix(r.createdAt) }}</text>
          </view>
        </template>
        <view v-else class="records-empty">{{ t('dig.recordEmpty') }}</view>
      </view>
    </template>

    <view v-else class="gacha-card dig-card center">
      <view class="records-empty">{{ t('dig.emptyBox') }}</view>
    </view>
  </view>
</template>

<style scoped lang="scss">
.dig-card {
  text-align: left;

  .dig-box-name {
    font-size: 40rpx;
    font-weight: 700;
    margin-bottom: 8rpx;
  }

  .dig-line {
    font-size: 26rpx;
    color: var(--color-text-secondary);
    margin-bottom: 20rpx;

    .completed-tag {
      color: var(--color-accent);
      margin-left: 12rpx;
    }
  }
}

// 格子棋盘：flex-wrap 网格（web 为 CSS grid repeat(min(cellCount,5))）
.cell-grid {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  margin: 20rpx -6rpx;

  .cell {
    box-sizing: border-box;
    margin: 6rpx;
    height: 100rpx;
    border: 1rpx solid var(--color-border);
    border-radius: 12rpx;
    background: var(--color-bg);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 36rpx;

    .cell-idx {
      color: var(--color-text-secondary);
      font-size: 26rpx;
    }

    &.dug {
      background: #f0f0f0;
      opacity: 0.6;
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

.takeall-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 26rpx;
  margin: 8rpx 0 16rpx;
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

    &.payable {
      font-size: 30rpx;
      font-weight: 600;
      border-top: 1rpx dashed var(--color-border);
      margin-top: 8rpx;
      padding-top: 12rpx;
    }
  }

  .payable-value {
    color: var(--color-accent);
    font-size: 36rpx;
  }

  .quote-loading {
    text-align: center;
    color: var(--color-text-secondary);
    font-size: 24rpx;
    margin-bottom: 8rpx;
  }

  .quote-empty {
    text-align: center;
    color: var(--color-text-secondary);
    font-size: 24rpx;
  }
}

.dig-status {
  text-align: center;
  margin-top: 24rpx;

  .box-emoji {
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

    .record-cells {
      color: var(--color-text-secondary);
      max-width: 45%;
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

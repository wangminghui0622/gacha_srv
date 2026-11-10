<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as memoryMatchApi from '@/services/memoryMatch'
import * as userApi from '@/services/user'
import { useAuthStore } from '@/store/authStore'
import { formatMoney } from '@/utils/format'
import { randomUUID } from '@/utils/id'

import type {
  DigPrizeItem,
  MemoryMatchConfigResponse,
  MemoryMatchFacePreset,
  MemoryMatchRoundResponse,
  MemoryMatchSessionResponse,
  MemoryMatchSettlementView,
} from '@/services/types'

// 记忆翻牌：购买牌局 → 命运选择 → 配对牌局 → 终局结算（web MemoryMatch 页同流程；
// localStorage 断点续玩换 uni storage；a-modal 结算换弹层视图；img 换 image）
const { t } = useI18n()
const auth = useAuthStore()

// 记忆翻牌无列表端点（与后端一致），种子仅 1 号命运牌局，前端固定取之
const CONFIG_ID = 1
const STORAGE_KEY = `mm_session_${CONFIG_ID}`

// 会话状态：2 待选命运 / 3 待选第二命运 / 4 游玩中 / 6 已完成 / 7 已过期
const STATE_AWAITING_DESTINY = 2
const STATE_AWAITING_SECOND = 3
const STATE_PLAYING = 4
const STATE_COMPLETED = 6

const config = ref<MemoryMatchConfigResponse | null>(null)
const session = ref<MemoryMatchSessionResponse | null>(null)
const round = ref<MemoryMatchRoundResponse | null>(null)
const buying = ref(false)
const acting = ref(false)
// 手牌选中（最多两张）
const selectedCards = ref<number[]>([])
const settlement = ref<MemoryMatchSettlementView | null>(null)
const settlementOpen = ref(false)

// 外观选择（0=默认；购买前 PUT 持久化）
const facePresetId = ref(0)
const backPresetId = ref(0)

const inPlay = computed(
  () =>
    session.value !== null &&
    (session.value.state === STATE_AWAITING_DESTINY ||
      session.value.state === STATE_AWAITING_SECOND ||
      session.value.state === STATE_PLAYING),
)

// 当前生效的正面预设（会话携带 or 默认）
const activeFacePreset = computed<MemoryMatchFacePreset | null>(() => {
  const presets = config.value?.config.facePresets ?? []
  if (presets.length === 0) return null
  const sid = session.value?.facePresetId ?? config.value?.config.defaultFacePresetId ?? presets[0]?.presetId
  return presets.find((p) => p.presetId === sid) ?? presets[0] ?? null
})

const backPresetName = computed(() => {
  const presets = config.value?.config.backPresets ?? []
  const bid = session.value?.backPresetId ?? config.value?.config.defaultBackPresetId ?? 0
  return presets.find((p) => p.presetId === bid)?.name ?? ''
})

// 卡 1..54 → 图案 1..9（后端引擎 PatternOfCard 同款：9 图案 × 各 6 张）
function patternOfCard(card: number): number {
  return Math.floor((card - 1) / 6) + 1
}

function faceOfPattern(patternId: number) {
  return activeFacePreset.value?.faces.find((f) => f.patternId === patternId)
}

function patternOptions() {
  // 命运选择盘：按正面预设展示 9 图案
  if (!activeFacePreset.value) {
    return Array.from({ length: 9 }, (_, i) => ({ patternId: i + 1, name: String(i + 1), imageUrl: '' }))
  }
  return activeFacePreset.value.faces
}

function prizeText(p: DigPrizeItem): string {
  if (p.kind === 1) return t('memoryMatch.prizeCash', { amount: formatMoney(p.amount) })
  if (p.kind === 2) return t('memoryMatch.prizePoints', { n: p.points })
  return t('memoryMatch.prizeCoupon', { name: p.couponName || p.couponTemplateNo })
}

function entryPrizeText(p: DigPrizeItem): string {
  if (p.kind === 0) return t('memoryMatch.noPrize')
  return prizeText(p)
}

function championText(): string {
  const c = config.value?.champion
  if (!c || !c.exists) return t('memoryMatch.championNone')
  const mask = String(c.userId).slice(-4)
  return t('memoryMatch.championLine', { user: `***${mask}`, n: c.totalPairCount })
}

// ── 读取 ──

async function loadConfig() {
  try {
    config.value = await memoryMatchApi.getMemoryMatchConfig(CONFIG_ID)
    const app = config.value.appearance
    facePresetId.value = app.facePresetId
    backPresetId.value = app.backPresetId
  } catch (err) {
    config.value = null
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  }
}

async function refreshProfile() {
  try {
    auth.setUser(await userApi.getProfile())
  } catch {
    // 401 由适配器统一处理
  }
}

function saveSessionId(sessionId: number) {
  try {
    uni.setStorageSync(STORAGE_KEY, String(sessionId))
  } catch {
    // 存储不可用：忽略，仅失去断点续玩
  }
}

function clearSessionId() {
  try {
    uni.removeStorageSync(STORAGE_KEY)
  } catch {
    // 同上
  }
}

async function loadSession(sessionId: number): Promise<boolean> {
  try {
    const s = await memoryMatchApi.getMemoryMatchSession(sessionId)
    if (s.state === STATE_COMPLETED) {
      // 已完成：直接展示结算
      session.value = s
      settlement.value = s.settlement ?? null
      settlementOpen.value = true
      clearSessionId()
      return true
    }
    if (s.state !== STATE_AWAITING_DESTINY && s.state !== STATE_AWAITING_SECOND && s.state !== STATE_PLAYING) {
      // 过期/未知状态：当作无会话
      clearSessionId()
      return false
    }
    session.value = s
    saveSessionId(sessionId)
    if (s.state === STATE_PLAYING && s.currentRound > 0) {
      await loadRound(sessionId, s.currentRound)
    }
    return true
  } catch {
    clearSessionId()
    return false
  }
}

async function loadRound(sessionId: number, roundNo: number) {
  try {
    round.value = await memoryMatchApi.getMemoryMatchRound(sessionId, roundNo)
  } catch (err) {
    round.value = null
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  }
}

// ── 购买 ──

function selectFacePreset(id: number) {
  // web:521 锁定项 disabled = 非默认且未拥有；mini 类名仅置灰，行为层镜像
  const owned = config.value?.appearance.ownedFacePresetIds ?? []
  if (buying.value) return
  if (id !== config.value?.config.defaultFacePresetId && !owned.includes(id)) return
  facePresetId.value = id
}

function selectBackPreset(id: number) {
  if (buying.value) return
  backPresetId.value = id
}

function onBuyClick() {
  if (buying.value) return // web:554 :loading="buying" 时点击被吞，mini 行为层镜像
  const c = config.value?.config
  if (!c) return
  uni.showModal({
    title: t('memoryMatch.confirmTitle'),
    content: t('memoryMatch.confirmText', {
      rounds: c.roundsPerPurchase,
      price: formatMoney(c.purchasePrice),
    }),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    success: (res) => {
      if (res.confirm) void buy()
    },
  })
}

async function buy() {
  if (buying.value) return
  buying.value = true
  try {
    // 先持久化外观选择，再携带同值购买（后端 Lock 分支校验）
    await memoryMatchApi.setMemoryMatchAppearance(CONFIG_ID, facePresetId.value, backPresetId.value)
    // 每次购买生成新幂等键：同一次购买重试复用、再次购买是新事务
    const key = randomUUID()
    const res = await memoryMatchApi.purchaseMemoryMatch(
      { configId: CONFIG_ID, facePresetId: facePresetId.value, backPresetId: backPresetId.value },
      key,
    )
    await loadSession(res.sessionId)
    await Promise.allSettled([loadConfig(), refreshProfile()])
  } catch (err) {
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  } finally {
    buying.value = false
  }
}

// ── 命运 ──

async function pickDestiny(patternId: number, second: boolean) {
  if (acting.value || !session.value) return
  acting.value = true
  try {
    if (second) {
      await memoryMatchApi.selectMemoryMatchSecondDestiny(session.value.sessionId, patternId)
    } else {
      await memoryMatchApi.selectMemoryMatchDestiny(session.value.sessionId, patternId)
    }
    await loadSession(session.value.sessionId)
  } catch (err) {
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  } finally {
    acting.value = false
  }
}

// ── 牌局 ──

function onCardClick(card: number) {
  if (acting.value || !round.value || round.value.terminal) return
  if (selectedCards.value.includes(card)) {
    selectedCards.value = selectedCards.value.filter((c) => c !== card)
    return
  }
  if (selectedCards.value.length >= 2) return
  selectedCards.value = [...selectedCards.value, card]
  if (selectedCards.value.length === 2) {
    void applyPair(selectedCards.value[0]!, selectedCards.value[1]!)
  }
}

async function applyPair(first: number, second: number) {
  if (!session.value || !round.value) return
  acting.value = true
  try {
    const res = await memoryMatchApi.applyMemoryMatchPair(
      session.value.sessionId,
      round.value.roundNo,
      first,
      second,
    )
    round.value = res.round
  } catch (err) {
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  } finally {
    selectedCards.value = []
    acting.value = false
  }
}

async function onDestinyDraw() {
  if (acting.value || !session.value || !round.value) return
  if (round.value.availableDestiny <= 0) return // web:450 无命运资源时禁用
  acting.value = true
  try {
    const res = await memoryMatchApi.applyMemoryMatchDestinyDraw(session.value.sessionId, round.value.roundNo)
    round.value = res.round
  } catch (err) {
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  } finally {
    acting.value = false
  }
}

async function onComplete() {
  if (acting.value || !session.value || !round.value) return
  if (!round.value.terminal) return // web:458 非终局禁用完成
  acting.value = true
  try {
    const res = await memoryMatchApi.completeMemoryMatchRound(session.value.sessionId, round.value.roundNo)
    round.value = res.round
    if (res.settlement) {
      await finishWithSettlement(res.settlement)
      return
    }
    await loadSession(session.value.sessionId)
    if (session.value?.state === STATE_PLAYING && session.value.currentRound > 0) {
      await loadRound(session.value.sessionId, session.value.currentRound)
    }
  } catch (err) {
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  } finally {
    acting.value = false
  }
}

function onSkipClick() {
  if (!session.value || acting.value) return // web:463 请求进行中禁用跳过
  uni.showModal({
    title: t('memoryMatch.skipTitle'),
    content: t('memoryMatch.skipText'),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    success: async (res) => {
      if (!res.confirm || !session.value) return
      try {
        const r = await memoryMatchApi.skipMemoryMatch(session.value.sessionId)
        await finishWithSettlement(r.settlement ?? null)
      } catch (err) {
        uni.showToast({ title: (err as Error).message, icon: 'none' })
      }
    },
  })
}

async function finishWithSettlement(s: MemoryMatchSettlementView | null) {
  settlement.value = s
  settlementOpen.value = true
  session.value = null
  round.value = null
  clearSessionId()
  // 结算可能换擂主：刷新擂主展示
  await Promise.allSettled([loadConfig(), refreshProfile()])
}

function closeSettlement() {
  settlementOpen.value = false
  settlement.value = null
  void loadConfig()
}

// 遮罩关闭只收起弹层（web a-modal 点遮罩仅关弹窗，不清 settlement 不重拉配置；「再玩一次」才走 closeSettlement）
function onSettlementMask() {
  settlementOpen.value = false
}

// ── 生命周期 ──

onLoad(async () => {
  await loadConfig()
  await refreshProfile()
  // 断点续玩：上次会话未完成则直接回到牌局
  try {
    const saved = uni.getStorageSync(STORAGE_KEY)
    if (saved) {
      await loadSession(Number(saved))
    }
  } catch {
    // storage 不可用：忽略
  }
})
</script>

<template>
  <view class="memorymatch-page">
    <template v-if="config">
      <!-- ═══ 牌局（命运选择 / 第二命运 / 游玩中）═══ -->
      <template v-if="inPlay && session">
        <view class="gacha-card mm-card">
          <view class="mm-head">
            <text class="mm-name">{{ config.config.name }}</text>
            <text class="mm-round-progress">
              {{ t('memoryMatch.roundProgress', { cur: session.completedRounds, total: session.purchasedRounds }) }}
            </text>
          </view>
          <view class="mm-champion">{{ championText() }}</view>

          <!-- 命运选择（会话状态 2） -->
          <view v-if="session.state === STATE_AWAITING_DESTINY" class="destiny-panel">
            <view class="panel-title">🔮 {{ t('memoryMatch.pickDestiny') }}</view>
            <view class="pattern-grid">
              <view
                v-for="p in patternOptions()"
                :key="p.patternId"
                class="pattern-tile"
                :class="{ disabled: acting }"
                @click="pickDestiny(p.patternId, false)"
              >
                <image v-if="p.imageUrl" :src="p.imageUrl" class="pattern-img" mode="aspectFit" />
                <text class="pattern-name">{{ p.name }}</text>
              </view>
            </view>
          </view>

          <!-- 第二命运选择（会话状态 3） -->
          <view v-else-if="session.state === STATE_AWAITING_SECOND" class="destiny-panel">
            <view class="panel-title">🔮 {{ t('memoryMatch.pickSecondDestiny') }}</view>
            <view class="pattern-grid">
              <view
                v-for="p in patternOptions()"
                :key="p.patternId"
                class="pattern-tile"
                :class="{ disabled: acting }"
                @click="pickDestiny(p.patternId, true)"
              >
                <image v-if="p.imageUrl" :src="p.imageUrl" class="pattern-img" mode="aspectFit" />
                <text class="pattern-name">{{ p.name }}</text>
              </view>
            </view>
          </view>

          <!-- 牌局（会话状态 4） -->
          <view v-else-if="round" class="board-panel">
            <view class="board-meta">
              <text>{{ t('memoryMatch.roundNo', { n: round.roundNo }) }}</text>
              <text>{{ t('memoryMatch.pairProgress', { n: round.pairCount, max: round.maxPairCount }) }}</text>
              <text class="destiny-count">
                ✨ {{ t('memoryMatch.destinyLeft', { n: round.availableDestiny }) }}（{{ t('memoryMatch.destinyDrawn', { n: round.destinyDraws }) }}）
              </text>
            </view>

            <!-- 手牌：点两张同图案配对 -->
            <view class="hand-grid">
              <view
                v-for="card in round.handCardIds"
                :key="card"
                class="hand-tile"
                :class="{ selected: selectedCards.includes(card) }"
                @click="onCardClick(card)"
              >
                <image
                  v-if="faceOfPattern(patternOfCard(card))?.imageUrl"
                  :src="faceOfPattern(patternOfCard(card))!.imageUrl"
                  class="hand-img"
                  mode="aspectFit"
                />
                <text v-else class="hand-pattern">{{ patternOfCard(card) }}</text>
              </view>
              <view v-if="round.handCardIds.length === 0" class="hand-empty">{{ t('memoryMatch.handEmpty') }}</view>
            </view>
            <view class="board-hint">{{ round.terminal ? t('memoryMatch.roundTerminal') : t('memoryMatch.pairHint') }}</view>

            <!-- 回合操作 -->
            <view class="board-actions">
              <view class="btn-plain act-btn" :class="{ disabled: acting || round.availableDestiny <= 0 }" @click="onDestinyDraw">
                ✨ {{ t('memoryMatch.destinyDraw') }}
              </view>
              <view class="btn-primary act-btn" :class="{ disabled: acting || !round.terminal }" @click="onComplete">
                {{ t('memoryMatch.completeRound') }}
              </view>
              <view class="btn-plain act-btn danger" :class="{ disabled: acting }" @click="onSkipClick">
                {{ t('memoryMatch.skip') }}
              </view>
            </view>

            <!-- 回合进度 -->
            <view class="rounds-list">
              <view
                v-for="r in session.rounds"
                :key="r.roundNo"
                class="round-chip"
                :class="{ active: r.state === 2, done: r.state === 3, skipped: r.state === 4 }"
              >
                <text>{{ t('memoryMatch.roundNo', { n: r.roundNo }) }}</text>
                <text class="round-chip-state">
                  {{ r.state === 2 ? t('memoryMatch.roundPlaying') : r.state === 3 ? t('memoryMatch.roundResolved') : t('memoryMatch.roundSkipped') }}
                </text>
              </view>
            </view>
          </view>

          <view v-else class="board-panel">
            <view class="spinner"></view>
            <view class="board-hint">{{ t('common.loading') }}</view>
          </view>
        </view>
      </template>

      <!-- ═══ 购买页（无进行中会话）═══ -->
      <template v-else>
        <view class="gacha-card mm-card">
          <view class="mm-head">
            <text class="mm-name">{{ config.config.name }}</text>
            <text class="mm-price money">{{ formatMoney(config.config.purchasePrice) }}</text>
          </view>
          <view class="mm-line">
            {{ t('memoryMatch.roundsPerPurchase', { n: config.config.roundsPerPurchase }) }}
            · {{ t('memoryMatch.maxPairs', { n: config.config.maxPairCount }) }}
          </view>
          <view class="mm-champion">👑 {{ championText() }}</view>

          <!-- 奖档 -->
          <view class="entries-list">
            <view v-for="e in config.config.entries" :key="e.entryId" class="entry-row">
              <text class="entry-pairs">{{ t('memoryMatch.entryPairs', { n: e.totalPairCount }) }}</text>
              <text class="entry-name">{{ e.name }} · {{ entryPrizeText(e.prize) }}</text>
              <text class="entry-remain">{{ t('memoryMatch.entryRemain', { n: e.remaining }) }}</text>
            </view>
          </view>

          <!-- 正面预设选择（非默认需拥有权益） -->
          <view class="panel-title">🎴 {{ t('memoryMatch.facePresetLabel') }}</view>
          <view class="preset-grid">
            <view
              v-for="p in config.config.facePresets"
              :key="p.presetId"
              class="preset-tile"
              :class="{
                selected: facePresetId === p.presetId,
                disabled: buying || (p.presetId !== config.config.defaultFacePresetId && !config.appearance.ownedFacePresetIds.includes(p.presetId)),
              }"
              @click="selectFacePreset(p.presetId)"
            >
              <text class="preset-name">{{ p.name }}</text>
              <text
                v-if="p.presetId !== config.config.defaultFacePresetId && !config.appearance.ownedFacePresetIds.includes(p.presetId)"
                class="preset-lock"
              >🔒</text>
            </view>
          </view>

          <!-- 背面预设选择 -->
          <view class="panel-title">🃏 {{ t('memoryMatch.backPresetLabel') }}</view>
          <view class="preset-grid">
            <view
              v-for="p in config.config.backPresets"
              :key="p.presetId"
              class="preset-tile"
              :class="{ selected: backPresetId === p.presetId, disabled: buying }"
              @click="selectBackPreset(p.presetId)"
            >
              <text class="preset-name">{{ p.name }}</text>
            </view>
          </view>

          <!-- 购买 -->
          <view class="quote-card">
            <view class="quote-row">
              <text>{{ t('memoryMatch.unitPrice') }}</text>
              <text class="money">{{ formatMoney(config.config.purchasePrice) }}</text>
            </view>
          </view>
          <view class="btn-primary" :class="{ disabled: buying }" @click="onBuyClick">
            {{ t('memoryMatch.buy') }}
          </view>
          <view class="wallet-tip">{{ t('memoryMatch.walletOnly') }}</view>
        </view>
      </template>

      <!-- 结算弹层 -->
      <view v-if="settlementOpen" class="modal-mask" @click="onSettlementMask">
        <view class="modal-card" @click.stop>
          <view class="settle-title">{{ t('memoryMatch.settleTitle') }}</view>
          <view v-if="settlement" class="settle-body">
            <view class="settle-emoji">{{ settlement.prize.kind === 0 ? '🍂' : '🎉' }}</view>
            <view class="settle-row">
              <text>{{ t('memoryMatch.totalPairs') }}</text>
              <text class="settle-strong">{{ settlement.totalPairCount }}</text>
            </view>
            <view class="settle-row">
              <text>{{ t('memoryMatch.settlePrize') }}</text>
              <text class="settle-strong">{{ entryPrizeText(settlement.prize) }}</text>
            </view>
            <view v-if="settlement.finalHash" class="settle-hash">
              <text class="settle-hash-label">{{ t('memoryMatch.finalHash') }}</text>
              <text class="settle-hash-value">{{ settlement.finalHash.slice(0, 16) }}…</text>
            </view>
            <view class="mm-champion">👑 {{ championText() }}</view>
            <view class="btn-primary" @click="closeSettlement">{{ t('memoryMatch.playAgain') }}</view>
          </view>
          <view v-else class="settle-empty">{{ t('common.empty') }}</view>
        </view>
      </view>
    </template>

    <view v-else class="gacha-card mm-card center">
      <view class="mm-empty">{{ t('memoryMatch.empty') }}</view>
    </view>
  </view>
</template>

<style scoped lang="scss">
.mm-card {
  text-align: left;

  .mm-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 8rpx;

    .mm-name {
      font-size: 40rpx;
      font-weight: 700;
    }

    .mm-price {
      font-size: 36rpx;
      font-weight: 700;
      color: var(--color-accent);
    }

    .mm-round-progress {
      font-size: 26rpx;
      color: var(--color-text-secondary);
    }
  }

  .mm-line {
    font-size: 26rpx;
    color: var(--color-text-secondary);
    margin-bottom: 12rpx;
  }

  .mm-champion {
    font-size: 26rpx;
    color: var(--color-accent);
    margin-bottom: 20rpx;
  }
}

.money {
  color: var(--color-accent);
}

// 奖档列表
.entries-list {
  display: flex;
  flex-direction: column;
  gap: 12rpx;
  margin-bottom: 24rpx;

  .entry-row {
    display: flex;
    align-items: center;
    gap: 16rpx;
    border: 1rpx solid var(--color-border);
    border-radius: 12rpx;
    padding: 12rpx 20rpx;
    background: var(--color-bg);
    font-size: 26rpx;

    .entry-pairs {
      flex-shrink: 0;
      font-weight: 700;
      color: var(--color-primary);
    }

    .entry-name {
      flex: 1;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .entry-remain {
      flex-shrink: 0;
      color: var(--color-text-secondary);
      font-size: 24rpx;
    }
  }
}

.panel-title {
  font-size: 28rpx;
  font-weight: 600;
  margin: 20rpx 0 12rpx;
}

// 预设选择（web grid 3 列 → flex-wrap）
.preset-grid {
  display: flex;
  flex-wrap: wrap;
  margin: 0 -6rpx 8rpx;

  .preset-tile {
    position: relative;
    box-sizing: border-box;
    width: calc(33.33% - 12rpx);
    margin: 6rpx;
    border: 1rpx solid var(--color-border);
    border-radius: 12rpx;
    background: var(--color-bg);
    padding: 20rpx 8rpx;
    font-size: 26rpx;
    text-align: center;

    &.selected {
      border-color: var(--color-accent);
      box-shadow: 0 0 0 3rpx rgba(255, 165, 0, 0.25);
    }

    &.disabled {
      opacity: 0.45;
    }

    .preset-lock {
      position: absolute;
      top: 4rpx;
      right: 8rpx;
      font-size: 24rpx;
    }
  }
}

.quote-card {
  background: var(--color-bg);
  border-radius: 12rpx;
  padding: 20rpx 24rpx;
  margin: 20rpx 0 24rpx;

  .quote-row {
    display: flex;
    justify-content: space-between;
    font-size: 26rpx;
  }
}

.wallet-tip {
  margin-top: 16rpx;
  font-size: 24rpx;
  color: var(--color-text-secondary);
  text-align: center;
}

// 命运选择盘（web grid 3 列 → flex-wrap）
.destiny-panel {
  .pattern-grid {
    display: flex;
    flex-wrap: wrap;
    margin: 16rpx -8rpx 0;

    .pattern-tile {
      box-sizing: border-box;
      width: calc(33.33% - 16rpx);
      margin: 8rpx;
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 8rpx;
      border: 1rpx solid var(--color-border);
      border-radius: 12rpx;
      background: var(--color-bg);
      padding: 20rpx 8rpx;

      &.disabled {
        opacity: 0.5;
      }

      .pattern-img {
        width: 96rpx;
        height: 96rpx;
      }

      .pattern-name {
        font-size: 24rpx;
      }
    }
  }
}

// 牌局
.board-panel {
  .board-meta {
    display: flex;
    gap: 20rpx;
    flex-wrap: wrap;
    font-size: 26rpx;
    color: var(--color-text-secondary);
    margin-bottom: 20rpx;

    .destiny-count {
      color: var(--color-accent);
    }
  }

  .hand-grid {
    display: flex;
    flex-wrap: wrap;
    margin: 0 -6rpx 16rpx;

    .hand-tile {
      box-sizing: border-box;
      width: calc(33.33% - 12rpx);
      margin: 6rpx;
      height: 220rpx;
      display: flex;
      align-items: center;
      justify-content: center;
      border: 1rpx solid var(--color-border);
      border-radius: 12rpx;
      background: var(--color-bg);

      &.selected {
        border-color: var(--color-accent);
        box-shadow: 0 0 0 3rpx rgba(255, 165, 0, 0.25);
      }

      .hand-img {
        width: 100%;
        height: 100%;
        padding: 12rpx;
        box-sizing: border-box;
      }

      .hand-pattern {
        font-size: 48rpx;
        font-weight: 700;
        color: var(--color-primary);
      }
    }

    .hand-empty {
      width: 100%;
      text-align: center;
      font-size: 26rpx;
      color: var(--color-text-secondary);
      padding: 32rpx 0;
    }
  }

  .board-hint {
    font-size: 24rpx;
    color: var(--color-text-secondary);
    margin-bottom: 20rpx;
  }

  .board-actions {
    display: flex;
    gap: 16rpx;
    margin-bottom: 24rpx;

    .act-btn {
      flex: 1;
      font-size: 26rpx;
      line-height: 76rpx;
      margin-top: 0;

      &.danger {
        color: var(--color-danger);
        border-color: var(--color-danger);
      }
    }
  }

  .rounds-list {
    display: flex;
    gap: 16rpx;
    flex-wrap: wrap;

    .round-chip {
      display: flex;
      align-items: center;
      gap: 12rpx;
      border: 1rpx solid var(--color-border);
      border-radius: 999rpx;
      padding: 6rpx 20rpx;
      font-size: 24rpx;
      background: var(--color-bg);

      &.active {
        border-color: var(--color-primary);
        color: var(--color-primary);
      }

      &.done {
        border-color: var(--color-accent);
        color: var(--color-accent);
      }

      &.skipped {
        opacity: 0.5;
      }

      .round-chip-state {
        font-size: 22rpx;
      }
    }
  }

  .spinner {
    width: 48rpx;
    height: 48rpx;
    margin: 0 auto 12rpx;
    border: 4rpx solid var(--color-border);
    border-top-color: var(--color-primary);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

// 结算弹层（a-modal → 自绘弹层）
.settle-title {
  font-size: 34rpx;
  font-weight: 700;
  text-align: center;
  margin-bottom: 24rpx;
}

.settle-body {
  text-align: center;

  .settle-emoji {
    font-size: 112rpx;
    margin-bottom: 20rpx;
  }

  .settle-row {
    display: flex;
    justify-content: space-between;
    font-size: 28rpx;
    padding: 12rpx 8rpx;

    .settle-strong {
      font-weight: 700;
      color: var(--color-accent);
    }
  }

  .settle-hash {
    text-align: left;
    font-size: 24rpx;
    color: var(--color-text-secondary);
    padding: 12rpx 8rpx 20rpx;

    .settle-hash-label {
      margin-right: 12rpx;
    }

    .settle-hash-value {
      font-size: 22rpx;
      word-break: break-all;
    }
  }

  .mm-champion {
    font-size: 26rpx;
    color: var(--color-accent);
    margin-bottom: 24rpx;
  }
}

.settle-empty {
  text-align: center;
  color: var(--color-text-secondary);
  font-size: 26rpx;
}

.center {
  text-align: center;

  .mm-empty {
    color: var(--color-text-secondary);
    font-size: 26rpx;
  }
}
</style>

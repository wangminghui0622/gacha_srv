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
  MemoryMatchRoundActionResponse,
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
const autoPlaying = ref(false)
// 手牌选中（最多 pairSize 张）
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

// 当前玩法几张成碰（配置优先，回合快照兜底）
const pairSize = computed(() => Math.max(2, Number(config.value?.config.pairSize || round.value?.pairSize || 2)))

// 3x3 牌桌：手牌按顺序填入 9 个格子，空位保留牌桌底纹
const boardSlots = computed(() => {
  const hand = round.value?.handCardIds ?? []
  return Array.from({ length: 9 }, (_, i) => ({ key: `slot-${i}`, card: hand[i] ?? 0 }))
})

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

// 中断续玩开关（全局规则 resumeEnabled）；关闭时不落本地断点，也不恢复旧会话。
function resumeAllowed(): boolean {
  return config.value?.config.resumeEnabled !== false
}

function saveSessionId(sessionId: number) {
  if (!resumeAllowed()) return
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

// ── 页面跳转 ──

function openRules() {
  uni.navigateTo({ url: '/pages/memory-match/rules' })
}

function openBuff() {
  uni.navigateTo({ url: '/pages/memory-match/buff' })
}

function openRecords() {
  uni.navigateTo({ url: '/pages/records/index' })
}

// ── 牌局 ──

function findAutoMatch(hand: number[]): number[] | null {
  const size = pairSize.value
  const groups = new Map<number, number[]>()
  for (const card of hand) {
    const pattern = patternOfCard(card)
    const group = groups.get(pattern) ?? []
    group.push(card)
    if (group.length >= size) {
      return group.slice(0, size)
    }
    groups.set(pattern, group)
  }
  return null
}

async function onAutoPair() {
  if (acting.value || autoPlaying.value || !session.value || !round.value || round.value.terminal) return
  const activeSession = session.value
  autoPlaying.value = true
  acting.value = true
  try {
    let paired = 0
    for (let step = 0; step < 100; step++) {
      const current: MemoryMatchRoundResponse | null = round.value
      if (!current || current.terminal) break
      const match = findAutoMatch(current.handCardIds)
      if (!match) break
      const res: MemoryMatchRoundActionResponse = await memoryMatchApi.applyMemoryMatchPair(
        activeSession.sessionId,
        current.roundNo,
        match,
      )
      round.value = res.round
      paired++
    }
    if (paired === 0) {
      uni.showToast({ title: t('memoryMatch.autoPairNone'), icon: 'none' })
    }
    selectedCards.value = []
  } catch (err) {
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  } finally {
    acting.value = false
    autoPlaying.value = false
  }
}

function onCellClick(card: number) {
  if (card > 0) onCardClick(card)
}

function onCardClick(card: number) {
  if (acting.value || !round.value || round.value.terminal) return
  if (selectedCards.value.includes(card)) {
    selectedCards.value = selectedCards.value.filter((c) => c !== card)
    return
  }
  if (selectedCards.value.length >= pairSize.value) return
  selectedCards.value = [...selectedCards.value, card]
  if (selectedCards.value.length === pairSize.value) {
    void applyPair([...selectedCards.value])
  }
}

async function applyPair(cards: number[]) {
  if (!session.value || !round.value) return
  acting.value = true
  try {
    const res = await memoryMatchApi.applyMemoryMatchPair(
      session.value.sessionId,
      round.value.roundNo,
      cards,
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
  if (!resumeAllowed()) {
    clearSessionId()
    return
  }
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
        <view class="mm-shell">
          <view class="mm-top">
            <view class="mm-heading">
              <text class="mm-name">{{ config.config.name }}</text>
              <text class="mm-round-progress">
                {{ t('memoryMatch.roundProgress', { cur: session.completedRounds, total: session.purchasedRounds }) }}
              </text>
            </view>
            <view class="mm-skip" :class="{ disabled: acting }" @click="onSkipClick">
              {{ t('memoryMatch.skip') }}
            </view>
          </view>
          <view class="mm-champion">👑 {{ championText() }}</view>
          <view class="quick-links">
            <view class="quick-link" @click="openRules">
              <text class="quick-icon">📖</text>
              <text>{{ t('memoryMatch.ruleLink') }}</text>
            </view>
            <view class="quick-link buff" @click="openBuff">
              <text class="quick-icon">⚡</text>
              <text>{{ t('memoryMatch.buffLink') }}</text>
            </view>
            <view class="quick-link" @click="openRecords">
              <text class="quick-icon">🧾</text>
              <text>{{ t('memoryMatch.recordsLink') }}</text>
            </view>
          </view>

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
                <view class="pattern-face">
                  <image v-if="p.imageUrl" :src="p.imageUrl" class="pattern-img" mode="aspectFit" />
                  <text v-else class="pattern-glyph">{{ p.patternId }}</text>
                </view>
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
                <view class="pattern-face">
                  <image v-if="p.imageUrl" :src="p.imageUrl" class="pattern-img" mode="aspectFit" />
                  <text v-else class="pattern-glyph">{{ p.patternId }}</text>
                </view>
                <text class="pattern-name">{{ p.name }}</text>
              </view>
            </view>
          </view>

          <!-- 牌局（会话状态 4） -->
          <view v-else-if="round" class="board-panel">
            <view class="stat-row">
              <view class="stat-box">
                <text class="stat-label">{{ t('memoryMatch.destinyCount') }}</text>
                <text class="stat-value">{{ round.availableDestiny }}</text>
              </view>
              <view class="stat-box">
                <text class="stat-label">{{ t('memoryMatch.currentPairs') }}</text>
                <text class="stat-value">{{ round.pairCount }}</text>
              </view>
            </view>

            <view class="board-meta">
              <text>{{ t('memoryMatch.roundNo', { n: round.roundNo }) }}</text>
              <text>{{ t('memoryMatch.pairProgress', { n: round.pairCount, max: round.maxPairCount }) }}</text>
              <text class="destiny-count">✨ {{ t('memoryMatch.destinyDrawn', { n: round.destinyDraws }) }}</text>
            </view>

            <!-- 3x3 牌桌：点相同图案的牌完成一碰 -->
            <view class="board-stage">
              <view class="board-grid">
                <view
                  v-for="slot in boardSlots"
                  :key="slot.key"
                  class="board-cell"
                  :class="{ filled: slot.card > 0, selected: slot.card > 0 && selectedCards.includes(slot.card) }"
                  @click="onCellClick(slot.card)"
                >
                  <image
                    v-if="slot.card > 0 && faceOfPattern(patternOfCard(slot.card))?.imageUrl"
                    :src="faceOfPattern(patternOfCard(slot.card))!.imageUrl"
                    class="board-face"
                    mode="aspectFit"
                  />
                  <text v-else-if="slot.card > 0" class="board-glyph">{{ patternOfCard(slot.card) }}</text>
                  <view v-else class="board-empty">
                    <text class="board-empty-mark">✦</text>
                  </view>
                </view>
              </view>

              <view class="board-side">
                <view
                  class="rail-btn gold"
                  :class="{ disabled: acting || autoPlaying || round.terminal }"
                  @click="onAutoPair"
                >
                  <text class="rail-icon">⚡</text>
                  <text class="rail-text">{{ autoPlaying ? t('memoryMatch.autoPairing') : t('memoryMatch.autoPair') }}</text>
                </view>
                <view class="rail-btn" :class="{ disabled: acting || round.availableDestiny <= 0 }" @click="onDestinyDraw">
                  <text class="rail-icon">✨</text>
                  <text class="rail-text">{{ t('memoryMatch.destinyDraw') }}</text>
                </view>
                <view class="rail-btn ghost" :class="{ disabled: acting || !round.terminal }" @click="onComplete">
                  <text class="rail-icon">🏁</text>
                  <text class="rail-text">{{ t('memoryMatch.completeRound') }}</text>
                </view>
              </view>
            </view>

            <view class="board-hint">{{ round.terminal ? t('memoryMatch.roundTerminal') : t('memoryMatch.pairHint') }}</view>

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
        <view class="mm-shell">
          <view class="mm-hero">
            <text class="mm-hero-tag">PAIR MATCH</text>
            <text class="mm-hero-title">{{ config.config.name }}</text>
            <view class="mm-hero-price money">{{ formatMoney(config.config.purchasePrice) }}</view>
            <text class="mm-hero-sub">
              {{ t('memoryMatch.roundsPerPurchase', { n: config.config.roundsPerPurchase }) }}
              · {{ t('memoryMatch.maxPairs', { n: config.config.maxPairCount }) }}
            </text>
          </view>
          <view class="quick-links">
            <view class="quick-link" @click="openRules">
              <text class="quick-icon">📖</text>
              <text>{{ t('memoryMatch.ruleLink') }}</text>
            </view>
            <view class="quick-link buff" @click="openBuff">
              <text class="quick-icon">⚡</text>
              <text>{{ t('memoryMatch.buffLink') }}</text>
            </view>
            <view class="quick-link" @click="openRecords">
              <text class="quick-icon">🧾</text>
              <text>{{ t('memoryMatch.recordsLink') }}</text>
            </view>
          </view>
          <view class="mm-champion">👑 {{ championText() }}</view>

          <!-- 奖档 -->
          <view class="section-head">{{ t('memoryMatch.prizeTiers') }}</view>
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
          <view class="btn-primary start-btn" :class="{ disabled: buying }" @click="onBuyClick">
            {{ t('memoryMatch.startGame') }}
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
$mm-ink: #e9f2ec;
$mm-muted: #8aa696;
$mm-green: #5cf2a2;
$mm-gold: #ffd166;

.memorymatch-page {
  box-sizing: border-box;
  min-height: 100vh;
  padding: calc(env(safe-area-inset-top) + 24rpx) 0 calc(40rpx + env(safe-area-inset-bottom));
  background:
    radial-gradient(circle at 18% 0%, rgba(46, 140, 88, 0.3), transparent 52%),
    radial-gradient(circle at 92% 96%, rgba(122, 84, 20, 0.22), transparent 48%),
    #080b09;
  color: $mm-ink;
}

.mm-shell {
  margin: 0 24rpx;
  padding: 30rpx 28rpx 28rpx;
  border-radius: 28rpx;
  border: 1rpx solid rgba(120, 255, 180, 0.14);
  background: linear-gradient(165deg, rgba(23, 35, 27, 0.98), rgba(9, 15, 11, 0.98));
  box-shadow: 0 24rpx 60rpx rgba(0, 0, 0, 0.45);
  text-align: left;
}

.mm-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16rpx;
}

.mm-heading {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.mm-name {
  font-size: 42rpx;
  font-weight: 800;
  letter-spacing: 2rpx;
  color: #ffffff;
}

.mm-round-progress {
  margin-top: 6rpx;
  font-size: 24rpx;
  color: $mm-muted;
}

.mm-skip {
  flex-shrink: 0;
  padding: 10rpx 28rpx;
  border-radius: 999rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.18);
  color: #cfd8d2;
  font-size: 24rpx;
  line-height: 1.4;

  &.disabled {
    opacity: 0.45;
  }
}

.mm-champion {
  margin: 14rpx 0 2rpx;
  font-size: 24rpx;
  color: $mm-gold;
}

.quick-links {
  display: flex;
  gap: 12rpx;
  margin: 18rpx 0 4rpx;
}

.quick-link {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8rpx;
  padding: 14rpx 0;
  border-radius: 14rpx;
  border: 1rpx solid rgba(120, 255, 180, 0.26);
  background: rgba(32, 92, 60, 0.28);
  color: #9ff0c3;
  font-size: 24rpx;
  line-height: 1.3;

  &.buff {
    border-color: rgba(255, 210, 90, 0.32);
    background: rgba(120, 92, 20, 0.28);
    color: #ffe08a;
  }
}

.quick-icon {
  font-size: 26rpx;
}

.stat-row {
  display: flex;
  gap: 16rpx;
  margin: 22rpx 0 16rpx;
}

.stat-box {
  position: relative;
  flex: 1;
  overflow: hidden;
  padding: 18rpx 22rpx;
  border-radius: 18rpx;
  border: 1rpx solid rgba(120, 255, 180, 0.18);
  background: linear-gradient(180deg, rgba(20, 50, 33, 0.92), rgba(8, 20, 13, 0.92));

  &::after {
    content: '';
    position: absolute;
    top: -46rpx;
    right: -36rpx;
    width: 140rpx;
    height: 140rpx;
    border-radius: 50%;
    background: radial-gradient(circle, rgba(92, 242, 162, 0.18), transparent 70%);
  }
}

.stat-label {
  display: block;
  font-size: 24rpx;
  letter-spacing: 2rpx;
  color: #8fd9b1;
}

.stat-value {
  display: block;
  margin-top: 2rpx;
  font-size: 64rpx;
  font-weight: 900;
  line-height: 1.05;
  color: #ffffff;
  text-shadow: 0 0 24rpx rgba(92, 242, 162, 0.35);
}

.board-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10rpx 20rpx;
  margin-bottom: 16rpx;
  font-size: 22rpx;
  color: $mm-muted;

  .destiny-count {
    color: $mm-gold;
  }
}

// 3x3 牌桌与右侧操作栏
.board-stage {
  display: flex;
  align-items: stretch;
  gap: 14rpx;
}

.board-grid {
  flex: 1;
  display: flex;
  flex-wrap: wrap;
  align-content: flex-start;
  padding: 10rpx;
  border-radius: 22rpx;
  border: 1rpx solid rgba(120, 255, 180, 0.16);
  background:
    radial-gradient(circle at 50% 38%, rgba(30, 80, 52, 0.35), rgba(5, 10, 7, 0.96) 72%),
    #060a07;
  box-shadow: inset 0 0 60rpx rgba(0, 0, 0, 0.6);
}

.board-cell {
  box-sizing: border-box;
  width: calc(33.333% - 10rpx);
  height: 158rpx;
  margin: 5rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 14rpx;
  border: 1rpx dashed rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.02);
  transition: transform 0.15s ease, box-shadow 0.15s ease, border-color 0.15s ease;

  &.filled {
    border-style: solid;
    border-color: rgba(126, 255, 179, 0.25);
    background: linear-gradient(180deg, #1d2f24, #0f1c15);
    box-shadow: 0 6rpx 16rpx rgba(0, 0, 0, 0.35);
  }

  &.selected {
    transform: translateY(-4rpx);
    border-color: $mm-gold;
    box-shadow: 0 0 0 3rpx rgba(255, 209, 102, 0.35), 0 0 30rpx rgba(255, 209, 102, 0.25);
  }
}

.board-face {
  width: 100%;
  height: 100%;
  padding: 8rpx;
  box-sizing: border-box;
}

.board-glyph {
  font-size: 64rpx;
  font-weight: 900;
  color: $mm-green;
}

.board-empty {
  display: flex;
  align-items: center;
  justify-content: center;
}

.board-empty-mark {
  font-size: 28rpx;
  color: rgba(255, 255, 255, 0.07);
}

.board-side {
  display: flex;
  flex-direction: column;
  gap: 12rpx;
  width: 148rpx;
  flex-shrink: 0;
}

.rail-btn {
  flex: 1;
  min-height: 96rpx;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6rpx;
  padding: 10rpx 6rpx;
  border-radius: 16rpx;
  border: 1rpx solid rgba(120, 255, 180, 0.25);
  background: linear-gradient(180deg, rgba(26, 70, 45, 0.85), rgba(9, 22, 14, 0.9));
  color: #bff5d6;
  font-size: 22rpx;
  line-height: 1.25;
  text-align: center;

  &.gold {
    border-color: rgba(255, 210, 90, 0.55);
    background: linear-gradient(180deg, rgba(120, 88, 20, 0.85), rgba(40, 28, 6, 0.9));
    color: #ffe08a;
    box-shadow: 0 0 24rpx rgba(255, 190, 60, 0.18);
  }

  &.ghost {
    border-color: rgba(255, 255, 255, 0.16);
    background: rgba(255, 255, 255, 0.04);
    color: #cfd8d2;
  }

  &.disabled {
    opacity: 0.38;
  }
}

.rail-icon {
  font-size: 30rpx;
}

.rail-text {
  word-break: break-all;
}

.board-hint {
  margin-top: 14rpx;
  font-size: 23rpx;
  line-height: 1.6;
  color: $mm-muted;
  text-align: center;
}

.rounds-list {
  display: flex;
  flex-wrap: wrap;
  gap: 10rpx;
  margin-top: 18rpx;
}

.round-chip {
  display: flex;
  align-items: center;
  gap: 10rpx;
  padding: 6rpx 18rpx;
  border-radius: 999rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.03);
  font-size: 22rpx;
  color: #b6c9bd;

  &.active {
    border-color: rgba(92, 242, 162, 0.55);
    color: $mm-green;
  }

  &.done {
    border-color: rgba(255, 209, 102, 0.5);
    color: $mm-gold;
  }

  &.skipped {
    opacity: 0.5;
  }
}

.round-chip-state {
  font-size: 20rpx;
}

// 命运选择盘
.destiny-panel {
  margin-top: 10rpx;
}

.panel-title {
  margin: 18rpx 0 14rpx;
  font-size: 28rpx;
  font-weight: 700;
  color: #d9f5e4;
}

.pattern-grid {
  display: flex;
  flex-wrap: wrap;
  margin: 0 -6rpx;
}

.pattern-tile {
  box-sizing: border-box;
  width: calc(33.333% - 12rpx);
  margin: 6rpx;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8rpx;
  padding: 14rpx 8rpx 12rpx;
  border-radius: 16rpx;
  border: 1rpx solid rgba(120, 255, 180, 0.18);
  background: linear-gradient(180deg, #1a2b21, #0d1711);

  &.disabled {
    opacity: 0.5;
  }
}

.pattern-face {
  width: 120rpx;
  height: 150rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 10rpx;
  background: radial-gradient(circle at 50% 35%, rgba(44, 110, 74, 0.5), rgba(9, 20, 14, 0.95));
  box-shadow: inset 0 0 24rpx rgba(0, 0, 0, 0.5);
}

.pattern-img {
  width: 96rpx;
  height: 120rpx;
}

.pattern-glyph {
  font-size: 56rpx;
  font-weight: 900;
  color: $mm-green;
}

.pattern-name {
  font-size: 22rpx;
  color: #9fbfae;
}

// 购买页
.mm-hero {
  padding: 20rpx 8rpx 6rpx;
}

.mm-hero-tag {
  display: inline-block;
  padding: 4rpx 18rpx;
  border-radius: 999rpx;
  border: 1rpx solid rgba(92, 242, 162, 0.35);
  color: $mm-green;
  font-size: 20rpx;
  letter-spacing: 6rpx;
}

.mm-hero-title {
  display: block;
  margin-top: 18rpx;
  font-size: 56rpx;
  font-weight: 900;
  letter-spacing: 3rpx;
  color: #ffffff;
}

.mm-hero-price {
  margin-top: 8rpx;
  font-size: 46rpx;
  font-weight: 800;
  color: $mm-gold;
}

.mm-hero-sub {
  display: block;
  margin-top: 6rpx;
  font-size: 24rpx;
  color: $mm-muted;
}

.section-head {
  margin: 26rpx 0 12rpx;
  font-size: 26rpx;
  font-weight: 700;
  letter-spacing: 2rpx;
  color: #b9d7c6;
}

.entries-list {
  display: flex;
  flex-direction: column;
  gap: 10rpx;
  margin-bottom: 8rpx;
}

.entry-row {
  display: flex;
  align-items: center;
  gap: 14rpx;
  padding: 16rpx 20rpx;
  border-radius: 14rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.03);
  font-size: 25rpx;
}

.entry-pairs {
  flex-shrink: 0;
  font-weight: 800;
  color: $mm-gold;
}

.entry-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #dcebe2;
}

.entry-remain {
  flex-shrink: 0;
  font-size: 22rpx;
  color: $mm-muted;
}

.preset-grid {
  display: flex;
  flex-wrap: wrap;
  margin: 0 -6rpx;
}

.preset-tile {
  position: relative;
  box-sizing: border-box;
  width: calc(33.333% - 12rpx);
  margin: 6rpx;
  padding: 20rpx 8rpx;
  border-radius: 12rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.03);
  color: #dcebe2;
  font-size: 24rpx;
  text-align: center;

  &.selected {
    border-color: $mm-gold;
    color: #ffe08a;
    box-shadow: 0 0 0 2rpx rgba(255, 209, 102, 0.28);
  }

  &.disabled {
    opacity: 0.4;
  }
}

.preset-name {
  font-size: 24rpx;
}

.preset-lock {
  position: absolute;
  top: 4rpx;
  right: 8rpx;
  font-size: 22rpx;
}

.quote-card {
  margin: 26rpx 0 18rpx;
  padding: 20rpx 24rpx;
  border-radius: 16rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.03);
}

.quote-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 26rpx;
  color: $mm-muted;

  .money {
    font-size: 32rpx;
    font-weight: 800;
    color: $mm-gold;
  }
}

.start-btn {
  margin-top: 20rpx;
  border-radius: 999rpx;
  background: linear-gradient(135deg, #2fd98a, #0f9d63);
  font-size: 34rpx;
  font-weight: 700;
  letter-spacing: 6rpx;
  line-height: 92rpx;
  box-shadow: 0 12rpx 30rpx rgba(24, 180, 110, 0.35);
}

.wallet-tip {
  margin-top: 14rpx;
  text-align: center;
  font-size: 22rpx;
  color: $mm-muted;
}

.money {
  color: $mm-gold;
}

// 结算弹层
.modal-mask .modal-card {
  border: 1rpx solid rgba(120, 255, 180, 0.18);
  background: linear-gradient(165deg, #1a2b21, #0c1510);
  color: $mm-ink;
}

.settle-title {
  margin-bottom: 24rpx;
  font-size: 34rpx;
  font-weight: 700;
  text-align: center;
}

.settle-body {
  text-align: center;
}

.settle-emoji {
  margin-bottom: 20rpx;
  font-size: 112rpx;
}

.settle-row {
  display: flex;
  justify-content: space-between;
  padding: 12rpx 8rpx;
  font-size: 28rpx;
}

.settle-strong {
  font-weight: 700;
  color: $mm-gold;
}

.settle-hash {
  padding: 12rpx 8rpx 20rpx;
  text-align: left;
  font-size: 24rpx;
  color: $mm-muted;
}

.settle-hash-label {
  margin-right: 12rpx;
}

.settle-hash-value {
  font-size: 22rpx;
  word-break: break-all;
}

.settle-body .mm-champion {
  margin-bottom: 24rpx;
}

.settle-empty {
  text-align: center;
  color: $mm-muted;
  font-size: 26rpx;
}

.spinner {
  width: 48rpx;
  height: 48rpx;
  margin: 0 auto 12rpx;
  border: 4rpx solid rgba(255, 255, 255, 0.12);
  border-top-color: $mm-green;
  border-radius: 50%;
  animation: mm-spin 0.8s linear infinite;
}

@keyframes mm-spin {
  to {
    transform: rotate(360deg);
  }
}

.mm-card.center {
  margin: 0 24rpx;
  border-radius: 28rpx;
  border: 1rpx solid rgba(120, 255, 180, 0.14);
  background: linear-gradient(165deg, rgba(23, 35, 27, 0.98), rgba(9, 15, 11, 0.98));
  color: $mm-muted;
  text-align: center;

  .mm-empty {
    color: $mm-muted;
    font-size: 26rpx;
  }
}
</style>

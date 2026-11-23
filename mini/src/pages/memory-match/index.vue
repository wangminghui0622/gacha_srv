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
  MemoryMatchBackPreset,
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

// 默认兼容 1 号命运牌局；从主题列表进入时通过 ?configId= 指定后台配置的奖池。
const DEFAULT_CONFIG_ID = 1
const configId = ref(DEFAULT_CONFIG_ID)

function storageKey(): string {
  return `mm_session_${configId.value}`
}

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

// 主题页弹层
const faceModalOpen = ref(false)
const backModalOpen = ref(false)
const prizeModalOpen = ref(false)
const championModalOpen = ref(false)

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

// 页面展示模式：购买前预览 / 选吉祥卡 / 牌局
const displayMode = computed<'preview' | 'destiny' | 'board' | 'loading'>(() => {
  if (session.value?.state === STATE_AWAITING_DESTINY || session.value?.state === STATE_AWAITING_SECOND) {
    return 'destiny'
  }
  if (inPlay.value) {
    return round.value ? 'board' : 'loading'
  }
  return 'preview'
})

// 当前生效的牌背预设（会话携带 or 默认 back1）
const activeBackPreset = computed<MemoryMatchBackPreset | null>(() => {
  const presets = config.value?.config.backPresets ?? []
  if (presets.length === 0) return null
  const bid =
    session.value?.backPresetId ||
    backPresetId.value ||
    config.value?.config.defaultBackPresetId ||
    presets[0]?.presetId
  return presets.find((p) => p.presetId === bid) ?? presets[0] ?? null
})

const heroImage = computed(() => {
  return (
    activeBackPreset.value?.faceBackgroundImageUrl ||
    activeBackPreset.value?.backImageUrl ||
    faceOfPattern(1)?.imageUrl ||
    ''
  )
})

const luckyPatternId = computed(() => session.value?.destinyPatternId || round.value?.destinyPatternId || 1)
const luckyCardImage = computed(() => faceOfPattern(luckyPatternId.value)?.imageUrl || faceOfPattern(1)?.imageUrl || '')
// 未开局时九宫格展示牌背（默认 back1），不是卡面图案。
const previewCardImage = computed(() => activeBackPreset.value?.backImageUrl || activeBackPreset.value?.faceBackgroundImageUrl || '')

const displayDestiny = computed(() => (displayMode.value === 'board' ? round.value?.availableDestiny ?? 0 : 0))
const displayPairs = computed(() => (displayMode.value === 'board' ? round.value?.pairCount ?? 0 : 0))

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

const championPending = computed(() => config.value?.champion.pendingIncome || '0.00')

// ── 读取 ──

async function loadConfig() {
  try {
    config.value = await memoryMatchApi.getMemoryMatchConfig(configId.value)
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
    uni.setStorageSync(storageKey(), String(sessionId))
  } catch {
    // 存储不可用：忽略，仅失去断点续玩
  }
}

function clearSessionId() {
  try {
    uni.removeStorageSync(storageKey())
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
    await memoryMatchApi.setMemoryMatchAppearance(configId.value, facePresetId.value, backPresetId.value)
    // 每次购买生成新幂等键：同一次购买重试复用、再次购买是新事务
    const key = randomUUID()
    const res = await memoryMatchApi.purchaseMemoryMatch(
      { configId: configId.value, facePresetId: facePresetId.value, backPresetId: backPresetId.value },
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

function goBack() {
  uni.navigateBack({ delta: 1 })
}

function onShare() {
  uni.showToast({ title: '请使用右上角分享', icon: 'none' })
}

function openFacePicker() {
  faceModalOpen.value = true
}

function openBackPicker() {
  backModalOpen.value = true
}

function openPrizes() {
  prizeModalOpen.value = true
}

function openChampion() {
  championModalOpen.value = true
}

function chooseFacePreset(id: number) {
  selectFacePreset(id)
  if (facePresetId.value === id) faceModalOpen.value = false
}

function chooseBackPreset(id: number) {
  selectBackPreset(id)
  if (backPresetId.value === id) backModalOpen.value = false
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

onLoad(async (query?: Record<string, string | undefined>) => {
  const queryId = Number(query?.configId)
  if (Number.isFinite(queryId) && queryId > 0) {
    configId.value = queryId
  }
  await loadConfig()
  await refreshProfile()
  if (!resumeAllowed()) {
    clearSessionId()
    return
  }
  // 断点续玩：上次会话未完成则直接回到牌局
  try {
    const saved = uni.getStorageSync(storageKey())
    if (saved) {
      await loadSession(Number(saved))
    }
  } catch {
    // storage 不可用：忽略
  }
})
</script>

<template>
  <view class="mm-page">
    <template v-if="config">
      <!-- 顶栏 -->
      <view class="mm-topbar">
        <view class="top-btn" @click="goBack">‹</view>
        <text class="top-title">{{ config.config.name }}</text>
        <view class="top-btn share" @click="onShare">↗</view>
      </view>

      <!-- 主题英雄区 + 左右入口 -->
      <view class="hero-wrap">
        <view class="hero-ring"></view>
        <view class="hero-platform"></view>
        <image v-if="heroImage" :src="heroImage" class="hero-img" mode="aspectFit" />
        <view v-else class="hero-fallback">🥷</view>

        <view class="side-rail left">
          <view class="side-btn" @click="openFacePicker">
            <text class="side-icon">🎴</text>
            <text class="side-text">卡面预设</text>
          </view>
          <view class="side-btn" @click="openBackPicker">
            <text class="side-icon">🃏</text>
            <text class="side-text">卡背预设</text>
          </view>
          <view class="side-btn gold" @click="openChampion">
            <text class="side-icon">🏆</text>
            <text class="side-text">吉王领域</text>
          </view>
        </view>

        <view class="side-rail right">
          <view class="side-btn" @click="openRules">
            <text class="side-icon">📜</text>
            <text class="side-text">抽赏规则</text>
          </view>
          <view class="side-btn" @click="openPrizes">
            <text class="side-icon">🎁</text>
            <text class="side-text">赏品列表</text>
          </view>
          <view class="side-btn" @click="openRecords">
            <text class="side-icon">🧾</text>
            <text class="side-text">抽赏记录</text>
          </view>
        </view>
      </view>

      <!-- 吉祥卡 / 吉祥数 / 当前碰数 -->
      <view class="stat-row">
        <view class="stat-card lucky-card">
          <view class="lucky-img-wrap">
            <image v-if="luckyCardImage" :src="luckyCardImage" class="lucky-img" mode="aspectFill" />
            <text v-else class="lucky-glyph">吉</text>
          </view>
          <text class="stat-caption">吉祥卡</text>
        </view>
        <view class="stat-card">
          <text class="stat-label">吉祥数</text>
          <text class="stat-value">{{ displayDestiny }}</text>
          <view
            class="stat-action"
            :class="{ disabled: displayMode !== 'board' || displayDestiny <= 0 || acting }"
            @click="onDestinyDraw"
          >
            使用
          </view>
        </view>
        <view class="stat-card">
          <text class="stat-label">当前碰数</text>
          <text class="stat-value">{{ displayPairs }}</text>
          <view
            class="stat-action gold"
            :class="{ disabled: displayMode !== 'board' || acting || autoPlaying || !round || round.terminal }"
            @click="onAutoPair"
          >
            {{ autoPlaying ? '碰牌中' : '开启自动碰' }}
          </view>
        </view>
      </view>

      <!-- 3x3 牌桌 -->
      <view class="card-grid">
        <template v-if="displayMode === 'preview'">
          <view v-for="i in 9" :key="`preview-${i}`" class="card-cell preview">
            <image v-if="previewCardImage" :src="previewCardImage" class="card-img" mode="aspectFill" />
            <text v-else class="card-glyph">{{ i }}</text>
          </view>
        </template>

        <template v-else-if="displayMode === 'destiny'">
          <view
            v-for="p in patternOptions()"
            :key="`destiny-${p.patternId}`"
            class="card-cell destiny"
            :class="{ disabled: acting }"
            @click="pickDestiny(p.patternId, session?.state === STATE_AWAITING_SECOND)"
          >
            <image v-if="p.imageUrl" :src="p.imageUrl" class="card-img" mode="aspectFill" />
            <text v-else class="card-glyph">{{ p.patternId }}</text>
            <text class="destiny-name">{{ p.name }}</text>
          </view>
        </template>

        <template v-else-if="displayMode === 'board'">
          <view
            v-for="slot in boardSlots"
            :key="slot.key"
            class="card-cell"
            :class="{ filled: slot.card > 0, selected: slot.card > 0 && selectedCards.includes(slot.card) }"
            @click="onCellClick(slot.card)"
          >
            <image
              v-if="slot.card > 0 && faceOfPattern(patternOfCard(slot.card))?.imageUrl"
              :src="faceOfPattern(patternOfCard(slot.card))!.imageUrl"
              class="card-img"
              mode="aspectFill"
            />
            <text v-else-if="slot.card > 0" class="card-glyph">{{ patternOfCard(slot.card) }}</text>
            <view v-else class="card-empty"></view>
          </view>
        </template>

        <template v-else>
          <view v-for="i in 9" :key="`loading-${i}`" class="card-cell loading"></view>
        </template>
      </view>

      <view v-if="displayMode === 'destiny'" class="hint">
        {{ session?.state === STATE_AWAITING_SECOND ? t('memoryMatch.pickSecondDestiny') : t('memoryMatch.pickDestiny') }}
      </view>
      <view v-else-if="displayMode === 'board'" class="hint">
        {{ round?.terminal ? t('memoryMatch.roundTerminal') : t('memoryMatch.pairHint') }}
      </view>

      <!-- 底部价格与操作 -->
      <view class="bottom-area">
        <view class="price-row">
          <text class="price-text">{{ formatMoney(config.config.purchasePrice) }}/次</text>
        </view>
        <view v-if="displayMode === 'preview'" class="start-btn" :class="{ disabled: buying }" @click="onBuyClick">
          {{ t('memoryMatch.startGame') }}
        </view>
        <view v-else-if="displayMode === 'board'" class="game-actions">
          <view class="game-btn" :class="{ disabled: acting || !round || !round.terminal }" @click="onComplete">完成回合</view>
          <view class="game-btn" :class="{ disabled: acting || autoPlaying || !round || round.terminal }" @click="onAutoPair">自动碰</view>
          <view class="game-btn danger" :class="{ disabled: acting }" @click="onSkipClick">跳过</view>
        </view>
        <view v-else class="start-btn disabled">
          {{ displayMode === 'destiny' ? '请选择吉祥卡' : '加载中…' }}
        </view>
        <view class="buff-link" @click="openBuff">超吉BUFF规则</view>
      </view>

      <!-- 回合进度 -->
      <view v-if="displayMode === 'board' && session" class="rounds-strip">
        <view
          v-for="r in session.rounds"
          :key="r.roundNo"
          class="round-dot"
          :class="{ active: r.state === 2, done: r.state === 3, skipped: r.state === 4 }"
        >
          {{ r.roundNo }}
        </view>
      </view>

      <!-- 卡面预设弹层 -->
      <view v-if="faceModalOpen" class="mm-modal-mask" @click="faceModalOpen = false">
        <view class="mm-modal" @click.stop>
          <view class="mm-modal-head">
            <text class="mm-modal-title">卡面预设</text>
            <text class="mm-modal-close" @click="faceModalOpen = false">×</text>
          </view>
          <view class="preset-grid">
            <view
              v-for="p in config.config.facePresets"
              :key="p.presetId"
              class="preset-tile"
              :class="{
                selected: facePresetId === p.presetId,
                disabled: buying || (p.presetId !== config.config.defaultFacePresetId && !config.appearance.ownedFacePresetIds.includes(p.presetId)),
              }"
              @click="chooseFacePreset(p.presetId)"
            >
              <image v-if="p.faces[0]?.imageUrl" :src="p.faces[0]!.imageUrl" class="preset-img" mode="aspectFill" />
              <text class="preset-name">{{ p.name }}</text>
              <text v-if="p.presetId !== config.config.defaultFacePresetId && !config.appearance.ownedFacePresetIds.includes(p.presetId)" class="preset-lock">🔒</text>
            </view>
          </view>
        </view>
      </view>

      <!-- 卡背预设弹层 -->
      <view v-if="backModalOpen" class="mm-modal-mask" @click="backModalOpen = false">
        <view class="mm-modal" @click.stop>
          <view class="mm-modal-head">
            <text class="mm-modal-title">卡背预设</text>
            <text class="mm-modal-close" @click="backModalOpen = false">×</text>
          </view>
          <view class="preset-grid">
            <view
              v-for="p in config.config.backPresets"
              :key="p.presetId"
              class="preset-tile"
              :class="{ selected: backPresetId === p.presetId, disabled: buying }"
              @click="chooseBackPreset(p.presetId)"
            >
              <image v-if="p.backImageUrl" :src="p.backImageUrl" class="preset-img" mode="aspectFill" />
              <text class="preset-name">{{ p.name }}</text>
            </view>
          </view>
        </view>
      </view>

      <!-- 赏品列表弹层 -->
      <view v-if="prizeModalOpen" class="mm-modal-mask" @click="prizeModalOpen = false">
        <view class="mm-modal" @click.stop>
          <view class="mm-modal-head">
            <text class="mm-modal-title">赏品列表</text>
            <text class="mm-modal-close" @click="prizeModalOpen = false">×</text>
          </view>
          <view class="entries-list">
            <view v-for="e in config.config.entries" :key="e.entryId" class="entry-row">
              <text class="entry-pairs">{{ t('memoryMatch.entryPairs', { n: e.totalPairCount }) }}</text>
              <text class="entry-name">{{ e.name }} · {{ entryPrizeText(e.prize) }}</text>
              <text class="entry-remain">{{ t('memoryMatch.entryRemain', { n: e.remaining }) }}</text>
            </view>
          </view>
        </view>
      </view>

      <!-- 吉王领域弹层 -->
      <view v-if="championModalOpen" class="mm-modal-mask" @click="championModalOpen = false">
        <view class="mm-modal" @click.stop>
          <view class="mm-modal-head">
            <text class="mm-modal-title">吉王领域</text>
            <text class="mm-modal-close" @click="championModalOpen = false">×</text>
          </view>
          <view class="champion-panel">
            <text class="champion-line">👑 {{ championText() }}</text>
            <text class="champion-pending">待结算收益 ¥{{ championPending }}</text>
            <text class="champion-tip">成为吉王后，后续每局购买的播主收益会累计到待结算收益，换王时结算。</text>
          </view>
        </view>
      </view>

      <!-- 结算弹层 -->
      <view v-if="settlementOpen" class="mm-modal-mask" @click="onSettlementMask">
        <view class="mm-modal settle-modal" @click.stop>
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
            <view class="start-btn" @click="closeSettlement">{{ t('memoryMatch.playAgain') }}</view>
          </view>
          <view v-else class="settle-empty">{{ t('common.empty') }}</view>
        </view>
      </view>
    </template>

    <view v-else class="empty-state">
      <view class="spinner"></view>
      <text>{{ t('memoryMatch.empty') }}</text>
    </view>
  </view>
</template>


<style scoped lang="scss">
$gold: #f3d27a;
$gold-dark: #b07a2a;
$green: #38e08e;
$ink: #eaf5ed;
$muted: #8fb39d;

.mm-page {
  box-sizing: border-box;
  min-height: 100vh;
  padding: calc(env(safe-area-inset-top) + 12rpx) 0 calc(32rpx + env(safe-area-inset-bottom));
  background:
    radial-gradient(circle at 50% 0%, rgba(28, 104, 58, 0.55), transparent 45%),
    radial-gradient(circle at 10% 35%, rgba(24, 80, 44, 0.35), transparent 45%),
    linear-gradient(180deg, #071a0f 0%, #04110a 55%, #020805 100%);
  color: $ink;
}

.mm-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24rpx;
  height: 84rpx;
}

.top-btn {
  width: 64rpx;
  height: 64rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  border: 1rpx solid rgba(243, 210, 122, 0.45);
  background: rgba(15, 45, 27, 0.75);
  color: $gold;
  font-size: 40rpx;
  line-height: 1;
}

.top-btn.share {
  font-size: 32rpx;
}

.top-title {
  flex: 1;
  min-width: 0;
  text-align: center;
  font-size: 36rpx;
  font-weight: 800;
  letter-spacing: 4rpx;
  color: #ffffff;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.hero-wrap {
  position: relative;
  height: 380rpx;
  margin: 4rpx 0 10rpx;
  display: flex;
  align-items: center;
  justify-content: center;
}

.hero-ring {
  position: absolute;
  width: 360rpx;
  height: 360rpx;
  border-radius: 50%;
  border: 2rpx solid rgba(76, 255, 160, 0.18);
  box-shadow: 0 0 80rpx rgba(56, 224, 142, 0.18), inset 0 0 60rpx rgba(56, 224, 142, 0.12);
}

.hero-platform {
  position: absolute;
  bottom: 18rpx;
  width: 300rpx;
  height: 46rpx;
  border-radius: 50%;
  background: radial-gradient(ellipse, rgba(80, 255, 170, 0.35), transparent 70%);
  filter: blur(4rpx);
}

.hero-img {
  position: relative;
  z-index: 1;
  width: 300rpx;
  height: 330rpx;
}

.hero-fallback {
  position: relative;
  z-index: 1;
  font-size: 160rpx;
}

.side-rail {
  position: absolute;
  top: 10rpx;
  bottom: 10rpx;
  width: 132rpx;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  z-index: 3;

  &.left {
    left: 18rpx;
  }

  &.right {
    right: 18rpx;
  }
}

.side-btn {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4rpx;
  min-height: 92rpx;
  padding: 8rpx 4rpx;
  border-radius: 14rpx;
  border: 1rpx solid rgba(243, 210, 122, 0.35);
  background: linear-gradient(180deg, rgba(18, 58, 34, 0.92), rgba(6, 22, 13, 0.94));
  box-shadow: 0 8rpx 18rpx rgba(0, 0, 0, 0.35);
  color: #cfe9d8;

  &.gold {
    border-color: rgba(243, 210, 122, 0.7);
    background: linear-gradient(180deg, rgba(92, 66, 18, 0.9), rgba(28, 20, 6, 0.94));
    color: $gold;
  }
}

.side-icon {
  font-size: 30rpx;
  line-height: 1.1;
}

.side-text {
  font-size: 19rpx;
  line-height: 1.2;
  text-align: center;
}

.stat-row {
  display: flex;
  gap: 12rpx;
  margin: 4rpx 24rpx 16rpx;
}

.stat-card {
  position: relative;
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 14rpx 8rpx;
  border-radius: 12rpx;
  border: 2rpx solid rgba(243, 210, 122, 0.7);
  background: linear-gradient(180deg, rgba(14, 46, 27, 0.95), rgba(5, 18, 11, 0.96));
  box-shadow: inset 0 0 24rpx rgba(0, 0, 0, 0.5);
}

.lucky-card {
  flex: 0 0 172rpx;
}

.lucky-img-wrap {
  width: 116rpx;
  height: 116rpx;
  border-radius: 8rpx;
  overflow: hidden;
  border: 1rpx solid rgba(243, 210, 122, 0.5);
  background: #0b2416;
  display: flex;
  align-items: center;
  justify-content: center;
}

.lucky-img {
  width: 100%;
  height: 100%;
}

.lucky-glyph {
  font-size: 54rpx;
  font-weight: 900;
  color: $gold;
}

.stat-caption {
  margin-top: 8rpx;
  font-size: 21rpx;
  color: #d7c08a;
}

.stat-label {
  font-size: 22rpx;
  letter-spacing: 2rpx;
  color: #a9c9b6;
}

.stat-value {
  margin: 4rpx 0 10rpx;
  font-size: 56rpx;
  font-weight: 900;
  line-height: 1;
  color: #ffffff;
  text-shadow: 0 0 18rpx rgba(56, 224, 142, 0.35);
}

.stat-action {
  min-width: 110rpx;
  padding: 8rpx 18rpx;
  border-radius: 8rpx;
  border: 1rpx solid rgba(190, 205, 198, 0.4);
  background: rgba(255, 255, 255, 0.06);
  color: #d7e3dc;
  font-size: 22rpx;
  text-align: center;

  &.gold {
    border-color: rgba(243, 210, 122, 0.65);
    background: linear-gradient(180deg, rgba(74, 54, 12, 0.8), rgba(30, 22, 6, 0.85));
    color: $gold;
  }

  &.disabled {
    opacity: 0.4;
  }
}

.card-grid {
  display: flex;
  flex-wrap: wrap;
  margin: 0 20rpx;
}

.card-cell {
  position: relative;
  box-sizing: border-box;
  width: calc(33.333% - 10rpx);
  height: 206rpx;
  margin: 5rpx;
  border-radius: 12rpx;
  overflow: hidden;
  border: 2rpx solid rgba(201, 151, 58, 0.75);
  background: linear-gradient(180deg, #123a24, #07180f);
  box-shadow: inset 0 0 22rpx rgba(0, 0, 0, 0.55), 0 8rpx 18rpx rgba(0, 0, 0, 0.25);

  &.filled {
    border-color: rgba(243, 210, 122, 0.85);
  }

  &.selected {
    border-color: #ffe28a;
    box-shadow: 0 0 0 3rpx rgba(255, 226, 138, 0.35), 0 0 30rpx rgba(255, 210, 90, 0.45);
    transform: translateY(-3rpx);
  }

  &.destiny {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
  }

  &.disabled {
    opacity: 0.55;
  }

  &.loading {
    opacity: 0.45;
  }
}

.card-img {
  position: relative;
  z-index: 0;
  display: block;
  width: 100%;
  height: 100%;
}

.card-glyph {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  font-size: 64rpx;
  font-weight: 900;
  color: $green;
}

.card-empty {
  width: 100%;
  height: 100%;
  background:
    radial-gradient(circle at 50% 40%, rgba(46, 130, 80, 0.18), transparent 60%),
    repeating-linear-gradient(45deg, rgba(255, 255, 255, 0.015) 0 8rpx, transparent 8rpx 16rpx);
}

.destiny-name {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  padding: 6rpx 0;
  background: rgba(2, 10, 6, 0.72);
  color: #d9e9df;
  font-size: 18rpx;
  text-align: center;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.hint {
  margin: 12rpx 24rpx 0;
  text-align: center;
  font-size: 22rpx;
  line-height: 1.5;
  color: $muted;
}

.bottom-area {
  margin: 18rpx 32rpx 0;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.price-row {
  margin-bottom: 10rpx;
}

.price-text {
  font-size: 30rpx;
  font-weight: 700;
  color: $gold;
  letter-spacing: 1rpx;
}

.start-btn {
  width: 100%;
  line-height: 92rpx;
  border-radius: 999rpx;
  border: 2rpx solid rgba(170, 255, 200, 0.5);
  background: linear-gradient(135deg, #7dff9d 0%, #1ecf7a 45%, #0b8f56 100%);
  box-shadow: 0 10rpx 28rpx rgba(31, 207, 122, 0.4), inset 0 2rpx 0 rgba(255, 255, 255, 0.45);
  color: #06301c;
  font-size: 38rpx;
  font-weight: 900;
  letter-spacing: 12rpx;
  text-align: center;
  text-indent: 12rpx;

  &.disabled {
    opacity: 0.5;
  }
}

.game-actions {
  width: 100%;
  display: flex;
  gap: 12rpx;
}

.game-btn {
  flex: 1;
  line-height: 78rpx;
  border-radius: 12rpx;
  border: 1rpx solid rgba(243, 210, 122, 0.45);
  background: linear-gradient(180deg, rgba(20, 66, 39, 0.95), rgba(6, 24, 14, 0.95));
  color: #d9f0e2;
  font-size: 24rpx;
  text-align: center;

  &.danger {
    border-color: rgba(255, 120, 120, 0.45);
    color: #ffb3b3;
  }

  &.disabled {
    opacity: 0.35;
  }
}

.buff-link {
  margin-top: 14rpx;
  padding: 6rpx 20rpx;
  border-radius: 999rpx;
  border: 1rpx solid rgba(243, 210, 122, 0.4);
  background: rgba(70, 56, 16, 0.35);
  color: $gold;
  font-size: 20rpx;
}

.rounds-strip {
  display: flex;
  justify-content: center;
  gap: 16rpx;
  margin-top: 18rpx;
}

.round-dot {
  width: 46rpx;
  height: 46rpx;
  line-height: 46rpx;
  border-radius: 50%;
  border: 1rpx solid rgba(255, 255, 255, 0.15);
  background: rgba(255, 255, 255, 0.04);
  color: #aabfb2;
  font-size: 20rpx;
  text-align: center;

  &.active {
    border-color: rgba(56, 224, 142, 0.7);
    color: $green;
  }

  &.done {
    border-color: rgba(243, 210, 122, 0.7);
    color: $gold;
  }

  &.skipped {
    opacity: 0.45;
  }
}

.mm-modal-mask {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40rpx;
  background: rgba(0, 0, 0, 0.62);
}

.mm-modal {
  box-sizing: border-box;
  width: 100%;
  max-height: 80vh;
  overflow-y: auto;
  padding: 28rpx;
  border-radius: 22rpx;
  border: 2rpx solid rgba(243, 210, 122, 0.65);
  background: linear-gradient(165deg, #102a1a 0%, #06120b 100%);
  box-shadow: 0 20rpx 60rpx rgba(0, 0, 0, 0.55), inset 0 0 40rpx rgba(43, 160, 94, 0.12);
  color: $ink;
}

.mm-modal-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20rpx;
}

.mm-modal-title {
  font-size: 30rpx;
  font-weight: 800;
  color: $gold;
  letter-spacing: 2rpx;
}

.mm-modal-close {
  width: 52rpx;
  height: 52rpx;
  line-height: 48rpx;
  text-align: center;
  border-radius: 50%;
  border: 1rpx solid rgba(255, 255, 255, 0.15);
  color: #cfe0d6;
  font-size: 34rpx;
}

.preset-grid {
  display: flex;
  flex-wrap: wrap;
  margin: 0 -6rpx;
}

.preset-tile {
  position: relative;
  box-sizing: border-box;
  width: calc(50% - 12rpx);
  margin: 6rpx;
  padding: 12rpx;
  border-radius: 12rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.03);

  &.selected {
    border-color: $gold;
    box-shadow: 0 0 0 2rpx rgba(243, 210, 122, 0.28);
  }

  &.disabled {
    opacity: 0.4;
  }
}

.preset-img {
  width: 100%;
  height: 150rpx;
  border-radius: 8rpx;
  background: #07180f;
}

.preset-name {
  display: block;
  margin-top: 8rpx;
  font-size: 22rpx;
  text-align: center;
  color: #d7e7dd;
}

.preset-lock {
  position: absolute;
  top: 8rpx;
  right: 10rpx;
  font-size: 22rpx;
}

.entries-list {
  display: flex;
  flex-direction: column;
  gap: 10rpx;
}

.entry-row {
  display: flex;
  align-items: center;
  gap: 12rpx;
  padding: 14rpx 16rpx;
  border-radius: 10rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.035);
  font-size: 23rpx;
}

.entry-pairs {
  flex-shrink: 0;
  font-weight: 800;
  color: $gold;
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
  font-size: 20rpx;
  color: $muted;
}

.champion-panel {
  display: flex;
  flex-direction: column;
  gap: 14rpx;
}

.champion-line {
  font-size: 28rpx;
  color: $gold;
}

.champion-pending {
  font-size: 24rpx;
  color: #b9d7c6;
}

.champion-tip {
  font-size: 22rpx;
  line-height: 1.6;
  color: $muted;
}

.settle-title {
  margin-bottom: 20rpx;
  font-size: 32rpx;
  font-weight: 800;
  text-align: center;
  color: $gold;
}

.settle-body {
  text-align: center;
}

.settle-emoji {
  margin-bottom: 16rpx;
  font-size: 96rpx;
}

.settle-row {
  display: flex;
  justify-content: space-between;
  padding: 10rpx 6rpx;
  font-size: 26rpx;
}

.settle-strong {
  font-weight: 800;
  color: $gold;
}

.settle-hash {
  padding: 10rpx 6rpx 18rpx;
  text-align: left;
  font-size: 22rpx;
  color: $muted;
}

.settle-hash-label {
  margin-right: 10rpx;
}

.settle-hash-value {
  font-size: 20rpx;
  word-break: break-all;
}

.settle-body .mm-champion {
  margin: 10rpx 0 18rpx;
  font-size: 24rpx;
  color: $gold;
}

.settle-empty {
  text-align: center;
  color: $muted;
  font-size: 24rpx;
}

.empty-state {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16rpx;
  color: $muted;
  font-size: 24rpx;
}

.spinner {
  width: 48rpx;
  height: 48rpx;
  border: 4rpx solid rgba(255, 255, 255, 0.12);
  border-top-color: $green;
  border-radius: 50%;
  animation: mm-spin 0.8s linear infinite;
}

@keyframes mm-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>

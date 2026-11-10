<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onUnload } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as campaignApi from '@/services/campaign'
import * as drawApi from '@/services/draw'
import type {
  CampaignItem,
  CampaignPreviewResponse,
  CampaignPrizeItem,
  DrawRecordItem,
  EntitlementItem,
} from '@/services/types'

// 活动中心：签到/兑换码/转盘 + 我的活动宝箱（开箱三态弹层）（web Campaigns 页同流程）
const { t } = useI18n()

const loading = ref(true)
const error = ref('')
const campaigns = ref<CampaignItem[]>([])
const entitlements = ref<EntitlementItem[]>([])
const busyKey = ref('') // 正在提交的活动 campaignKey

// 兑换码输入与预览（按 campaignKey 缓存）
const codeInputs = ref<Record<string, string>>({})
const codePreviews = ref<Record<string, CampaignPreviewResponse>>({})

// 幂等键：同一次逻辑动作重试复用（成功/换动作后重置）
const idemKeys = ref<Record<string, string>>({})

// 中奖弹层
const resultVisible = ref(false)
const resultTitle = ref('')
const resultPrizes = ref<CampaignPrizeItem[]>([])
const resultStreak = ref(0)
const resultSpinsLeft = ref(-1)

// 活动宝箱开箱（M11c）：opening 提交 → polling 轮询 /draw/records → done 结果/失败
const openVisible = ref(false)
const openPhase = ref<'opening' | 'polling' | 'done'>('opening')
const openRecord = ref<DrawRecordItem | null>(null)
const openError = ref('')
const openEntitlement = ref<EntitlementItem | null>(null)
// 幂等键：同一次开箱动作重试复用（成功后资格 USED，重放快路径直接 success）
const openIdemKeys = ref<Record<string, string>>({})
let openPollTimer: ReturnType<typeof setInterval> | null = null

const kindName = computed<Record<number, string>>(() => ({
  1: t('campaign.kindSignIn'),
  2: t('campaign.kindRedeem'),
  3: t('campaign.kindWheel'),
}))

const entitlementStatusMap: Record<number, { text: string; color: string }> = {
  1: { text: 'campaign.entitlementAvailable', color: 'green' },
  2: { text: 'campaign.entitlementLocked', color: 'orange' },
  3: { text: 'campaign.entitlementUsed', color: 'default' },
}

function prizeText(p: CampaignPrizeItem): string {
  switch (p.kind) {
    case 1:
      return t('campaign.prizeCash', { amount: p.amount })
    case 2:
      return t('campaign.prizePoints', { points: p.points })
    case 3:
      return p.couponName || p.couponTemplateNo
    case 4:
      return p.boxName || t('campaign.prizeBox', { boxId: p.boxId })
    default:
      return ''
  }
}

function dateText(unix: number): string {
  const d = new Date(unix * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [list, boxes] = await Promise.all([campaignApi.listCampaigns(), campaignApi.listCampaignBoxes()])
    campaigns.value = list.list
    entitlements.value = boxes.list
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    loading.value = false
  }
}

function showResult(title: string, prizes: CampaignPrizeItem[], streak = 0, spinsLeft = -1) {
  resultTitle.value = title
  resultPrizes.value = prizes
  resultStreak.value = streak
  resultSpinsLeft.value = spinsLeft
  resultVisible.value = true
}

function closeResult() {
  resultVisible.value = false
}

// 参与幂等键：同一动作重试复用
function takeIdemKey(scope: string): string {
  if (!idemKeys.value[scope]) {
    idemKeys.value[scope] = campaignApi.newIdemKey()
  }
  return idemKeys.value[scope]
}

async function signIn(c: CampaignItem) {
  busyKey.value = c.campaignKey
  try {
    const resp = await campaignApi.campaignParticipate(c.campaignKey, '', takeIdemKey(c.campaignKey))
    delete idemKeys.value[c.campaignKey]
    showResult(t('campaign.signInTitle'), resp.prizes, resp.streak)
    await load()
  } catch (err) {
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  } finally {
    busyKey.value = ''
  }
}

async function queryCode(c: CampaignItem) {
  const code = (codeInputs.value[c.campaignKey] ?? '').trim()
  if (!code) {
    delete codePreviews.value[c.campaignKey]
    return
  }
  try {
    codePreviews.value[c.campaignKey] = await campaignApi.campaignPreview(c.campaignKey, code)
  } catch (err) {
    delete codePreviews.value[c.campaignKey]
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  }
}

async function redeem(c: CampaignItem) {
  const code = (codeInputs.value[c.campaignKey] ?? '').trim()
  if (!code) {
    uni.showToast({ title: t('campaign.codeRequired'), icon: 'none' })
    return
  }
  const scope = `${c.campaignKey}:${code}`
  busyKey.value = c.campaignKey
  try {
    const resp = await campaignApi.campaignParticipate(c.campaignKey, code, takeIdemKey(scope))
    delete idemKeys.value[scope]
    showResult(t('campaign.redeemTitle'), resp.prizes)
    await load()
  } catch (err) {
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  } finally {
    busyKey.value = ''
  }
}

function goWheel(c: CampaignItem) {
  uni.navigateTo({ url: `/pages/wheel/index?campaignKey=${c.campaignKey}` })
}

function stopOpenPoll() {
  if (openPollTimer) {
    clearInterval(openPollTimer)
    openPollTimer = null
  }
}

// 轮询抽赏记录：后端未产生记录时返回 2004，继续轮询直到超时（同 Buy 页模式）
function pollOpen(requestId: string) {
  openPhase.value = 'polling'
  let tries = 0
  openPollTimer = setInterval(async () => {
    tries += 1
    try {
      const res = await drawApi.getRecordByRequestId(requestId)
      if (res.list.length > 0) {
        stopOpenPoll()
        openRecord.value = res.list[0] ?? null
        openPhase.value = 'done'
        void load() // 资格已核销（USED），刷新列表
      }
    } catch {
      // 2004（暂无记录）或网络抖动：继续轮询
    }
    if (tries >= 30) {
      stopOpenPoll()
      openRecord.value = null
      openError.value = t('campaign.openFailText')
      openPhase.value = 'done'
    }
  }, 1000)
}

// 开箱：entitlement 全抵零付；同 Key 重试后端幂等（成功/换资格后重置 Key）
async function openBox(e: EntitlementItem) {
  openPhase.value = 'opening'
  openRecord.value = null
  openError.value = ''
  const scope = `open:${e.entitlementNo}`
  if (!openIdemKeys.value[scope]) {
    openIdemKeys.value[scope] = campaignApi.newIdemKey()
  }
  try {
    const res = await campaignApi.openCampaignBox(e.entitlementNo, openIdemKeys.value[scope])
    // status=pending：SAGA 已提交异步开奖；success：同 Key 重试命中已有单号
    if (res.status === 'success') {
      const existing = await drawApi.getRecordByRequestId(res.requestId)
      if (existing.list.length > 0) {
        openRecord.value = existing.list[0] ?? null
        openPhase.value = 'done'
        void load()
        return
      }
    }
    pollOpen(res.requestId)
  } catch (err) {
    openError.value = (err as Error).message
    openPhase.value = 'done'
  }
}

function onOpenClick(e: EntitlementItem) {
  openEntitlement.value = e
  uni.showModal({
    title: t('campaign.openConfirmTitle'),
    content: t('campaign.openConfirmText', { box: e.boxName || `Box #${e.boxId}` }),
    confirmText: t('campaign.open'),
    cancelText: t('common.cancel'),
    success: (res) => {
      if (res.confirm) {
        openVisible.value = true
        void openBox(e)
      }
    },
  })
}

function retryOpen() {
  if (openEntitlement.value) {
    void openBox(openEntitlement.value)
  }
}

function closeOpen() {
  openVisible.value = false
  openPhase.value = 'opening'
  openRecord.value = null
}

// 开箱结果导航（模板方法：uni 不可直接进模板）
function goWarehouse() {
  openVisible.value = false
  uni.navigateTo({ url: '/pages/warehouse/index' })
}

function goRecords() {
  openVisible.value = false
  uni.navigateTo({ url: '/pages/records/index' })
}

onLoad(() => {
  void load()
})
onUnload(stopOpenPoll)
</script>

<template>
  <view>
    <view class="page-title">{{ t('campaign.title') }}</view>
    <view v-if="loading" class="list-empty">{{ t('common.loading') }}</view>
    <view v-else-if="error" class="error-text">{{ error }}</view>
    <template v-else>
      <view v-if="campaigns.length === 0" class="list-empty">{{ t('campaign.empty') }}</view>
      <view v-for="c in campaigns" :key="c.campaignKey" class="gacha-card campaign-card">
        <view class="campaign-head">
          <view class="campaign-head-main">
            <view class="campaign-name">{{ c.name }}</view>
            <view class="campaign-sub">
              {{ dateText(c.startAt) }} ~ {{ dateText(c.endAt) }}
              <text v-if="!c.inWindow" class="tag-chip">{{ t('campaign.outOfWindow') }}</text>
            </view>
          </view>
          <text class="tag-chip tag-blue">{{ kindName[c.kind] ?? c.kind }}</text>
        </view>

        <!-- 签到 -->
        <view v-if="c.kind === 1" class="campaign-body">
          <view class="campaign-sub">
            {{ t('campaign.streak', { n: c.streak }) }} ·
            {{ t('campaign.totalDays', { n: c.totalDays }) }}
            <text v-if="c.signedToday" class="tag-chip tag-green">{{ t('campaign.signedToday') }}</text>
          </view>
          <view
            class="btn-primary"
            :class="{ disabled: c.signedToday || !c.inWindow || busyKey === c.campaignKey }"
            @click="signIn(c)"
          >
            {{ c.signedToday ? t('campaign.signedToday') : t('campaign.signIn') }}
          </view>
        </view>

        <!-- 兑换码 -->
        <view v-else-if="c.kind === 2" class="campaign-body">
          <view class="redeem-row">
            <input
              v-model="codeInputs[c.campaignKey]"
              class="redeem-input"
              :placeholder="t('campaign.codePlaceholder')"
              :maxlength="64"
            />
            <view class="btn-plain redeem-query" :class="{ disabled: !c.inWindow }" @click="queryCode(c)">
              {{ t('campaign.codeQuery') }}
            </view>
          </view>
          <view v-if="codePreviews[c.campaignKey]" class="preview-box">
            <view class="campaign-sub">
              {{
                t('campaign.redeemTimes', {
                  times: codePreviews[c.campaignKey]!.redemptionTimes,
                  limit: codePreviews[c.campaignKey]!.redemptionUserLimit,
                })
              }}
            </view>
            <view class="prize-row">
              <text v-for="(p, i) in codePreviews[c.campaignKey]!.previewPrizes" :key="i" class="tag-chip tag-gold">
                {{ prizeText(p) }}
              </text>
            </view>
          </view>
          <view
            class="btn-primary"
            :class="{ disabled: !c.inWindow || busyKey === c.campaignKey }"
            @click="redeem(c)"
          >
            {{ t('campaign.redeem') }}
          </view>
        </view>

        <!-- 转盘 -->
        <view v-else-if="c.kind === 3" class="campaign-body">
          <view class="campaign-sub">{{ t('campaign.spinsLeft', { n: c.spinsLeft }) }}</view>
          <view class="btn-primary" :class="{ disabled: !c.inWindow }" @click="goWheel(c)">
            {{ t('campaign.wheelEnter') }}
          </view>
        </view>
      </view>

      <!-- 我的活动宝箱 -->
      <view class="section-title section-title-page">{{ t('campaign.entitlements') }}</view>
      <view v-if="entitlements.length === 0" class="list-empty">{{ t('campaign.entitlementEmpty') }}</view>
      <view v-for="e in entitlements" :key="e.entitlementNo" class="gacha-card coupon-card">
        <view class="coupon-main">
          <view class="coupon-name">
            <text>{{ e.boxName || `Box #${e.boxId}` }}</text>
            <text
              class="tag-chip"
              :class="{
                'tag-green': entitlementStatusMap[e.status]?.color === 'green',
                'tag-orange': entitlementStatusMap[e.status]?.color === 'orange',
              }"
            >
              {{ t(entitlementStatusMap[e.status]?.text ?? 'campaign.entitlementAvailable') }}
            </text>
          </view>
          <view class="coupon-sub">{{ t('campaign.grantedAt') }}：{{ dateText(e.grantedAt) }}</view>
        </view>
        <view v-if="e.status === 1" class="btn-primary open-btn" @click="onOpenClick(e)">
          {{ t('campaign.open') }}
        </view>
      </view>
    </template>

    <!-- 中奖弹层 -->
    <view v-if="resultVisible" class="modal-mask" @click="closeResult">
      <view class="modal-card" @click.stop>
        <view class="result-body">
          <view class="result-title">{{ resultTitle }}</view>
          <template v-if="resultPrizes.length > 0">
            <view v-for="(p, i) in resultPrizes" :key="i" class="result-prize accent">
              {{ prizeText(p) }}
            </view>
          </template>
          <view v-else class="result-none">{{ t('campaign.noPrize') }}</view>
          <view v-if="resultStreak > 0" class="campaign-sub">
            {{ t('campaign.streak', { n: resultStreak }) }}
          </view>
          <view v-if="resultSpinsLeft >= 0" class="campaign-sub">
            {{ t('campaign.spinsLeft', { n: resultSpinsLeft }) }}
          </view>
          <view class="btn-plain" @click="closeResult">{{ t('common.confirm') }}</view>
        </view>
      </view>
    </view>

    <!-- 开箱弹层（M11c：提交/轮询/结果三态） -->
    <view v-if="openVisible" class="modal-mask">
      <view class="modal-card">
        <view class="open-body">
          <template v-if="openPhase === 'opening' || openPhase === 'polling'">
            <view v-if="openPhase === 'opening'" class="box-emoji">📦</view>
            <view v-else class="box-emoji shake">🎁</view>
            <view class="open-status">
              {{ openPhase === 'opening' ? t('campaign.openSubmitting') : t('campaign.opening') }}
            </view>
            <view v-if="openPhase === 'polling'" class="spinner"></view>
          </template>
          <template v-else-if="openRecord">
            <view class="result-emoji">{{ openRecord.isPrize ? '🎉' : '🍀' }}</view>
            <view class="result-title">
              {{ openRecord.isPrize ? t('campaign.openWinTitle') : t('buy.thanksTitle') }}
            </view>
            <view v-if="openRecord.isPrize" class="result-prize">{{ openRecord.entryName }}</view>
            <view v-else class="result-thanks">{{ t('buy.thanksText') }}</view>
            <view class="result-actions">
              <view v-if="openRecord.isPrize" class="btn-primary" @click="goWarehouse">
                {{ t('buy.goWarehouse') }}
              </view>
              <view class="btn-plain" @click="goRecords">{{ t('buy.goRecords') }}</view>
              <view class="btn-plain" @click="closeOpen">{{ t('campaign.openDone') }}</view>
            </view>
          </template>
          <template v-else>
            <view class="result-emoji">😢</view>
            <view class="result-title">{{ t('buy.failTitle') }}</view>
            <view class="result-thanks">{{ openError }}</view>
            <view class="result-actions">
              <view class="btn-primary" @click="retryOpen">{{ t('campaign.openRetry') }}</view>
              <view class="btn-plain" @click="closeOpen">{{ t('common.cancel') }}</view>
            </view>
          </template>
        </view>
      </view>
    </view>
  </view>
</template>

<style scoped lang="scss">
.section-title-page {
  margin-left: 24rpx;
}

.campaign-card {
  margin-bottom: 24rpx;

  .campaign-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16rpx;
    margin-bottom: 16rpx;

    .campaign-head-main {
      flex: 1;
      min-width: 0;
    }

    .campaign-name {
      font-size: 32rpx;
      font-weight: 600;
      margin-bottom: 4rpx;
    }
  }

  .campaign-body {
    display: flex;
    flex-direction: column;
    gap: 16rpx;
  }

  .campaign-sub {
    font-size: 24rpx;
    color: var(--color-text-secondary);
  }

  .redeem-row {
    display: flex;
    gap: 16rpx;
    align-items: center;

    .redeem-input {
      flex: 1;
      border: 1rpx solid var(--color-border);
      border-radius: 12rpx;
      padding: 14rpx 20rpx;
      font-size: 26rpx;
      background: var(--color-bg);
      height: 60rpx;
      box-sizing: border-box;
    }

    .redeem-query {
      margin-top: 0;
      line-height: 60rpx;
      padding: 0 24rpx;
      font-size: 26rpx;
    }
  }

  .preview-box {
    padding: 16rpx;
    background: var(--color-bg);
    border-radius: 12rpx;

    .prize-row {
      margin-top: 8rpx;
      display: flex;
      flex-wrap: wrap;
      gap: 8rpx;

      .tag-chip {
        margin-left: 0;
      }
    }
  }
}

.coupon-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24rpx;
  margin-bottom: 16rpx;

  .coupon-main {
    flex: 1;
    min-width: 0;

    .coupon-name {
      font-size: 30rpx;
      font-weight: 600;
      display: flex;
      align-items: center;
    }

    .coupon-sub {
      font-size: 24rpx;
      color: var(--color-text-secondary);
      margin-top: 4rpx;
    }
  }

  .open-btn {
    line-height: 68rpx;
    padding: 0 32rpx;
    font-size: 26rpx;
  }
}

.result-body {
  text-align: center;

  .result-title {
    font-size: 34rpx;
    font-weight: 700;
    margin-bottom: 16rpx;
  }

  .result-prize {
    font-size: 36rpx;
    font-weight: 700;
    color: var(--color-accent);
    margin-bottom: 8rpx;
  }

  .result-none {
    color: var(--color-text-secondary);
    margin-bottom: 8rpx;
  }

  .campaign-sub {
    margin-top: 16rpx;
    font-size: 24rpx;
    color: var(--color-text-secondary);
  }

  .btn-plain {
    margin-top: 32rpx;
  }
}

// 开箱弹层（M11c，同 Buy 页节奏）
.open-body {
  text-align: center;

  .box-emoji {
    font-size: 128rpx;
    margin-bottom: 24rpx;
  }

  .shake {
    animation: shake 0.5s infinite;
  }

  .open-status {
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
}

@keyframes shake {
  0% { transform: rotate(-8deg); }
  50% { transform: rotate(8deg); }
  100% { transform: rotate(-8deg); }
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>

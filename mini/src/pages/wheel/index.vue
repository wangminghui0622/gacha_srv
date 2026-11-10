<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onUnload } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as campaignApi from '@/services/campaign'
import type { CampaignPreviewResponse, CampaignPrizeItem } from '@/services/types'

// 幸运转盘：预览槽位 → 抽奖 → 转盘动画落点 → 中奖弹窗（web Campaigns/Wheel 页同流程）
// 偏离说明（D4→CSS 扇形）：WXSS 无 conic-gradient，改用「圆心零尺寸三角形 + 旋转」拼接扇形
// （border-bottom 三角形 apex 在圆心，r=200rpx，底边 r*tan(θ/2)），落点数学与 web 完全一致
const { t } = useI18n()

const campaignKey = ref('')

const loading = ref(true)
const error = ref('')
const preview = ref<CampaignPreviewResponse | null>(null)
const slots = ref<{ slotIndex: number; label: string; prize: CampaignPrizeItem }[]>([])
const spinning = ref(false)
const rotation = ref(0) // 累计旋转角（顺时针，deg）

const resultVisible = ref(false)
const resultPrizes = ref<CampaignPrizeItem[]>([])

// 幂等键：同一次抽奖网络重试复用（后端重放返回首次结果）
let idemKey = ''
let spinTimer: ReturnType<typeof setTimeout> | null = null

// 转盘颜色：按槽位交替（金色/奶白，经典转盘）
const slotColors = ['#f7c948', '#fff3d6']
const seg = computed(() => (slots.value.length > 0 ? 360 / slots.value.length : 0))
// CSS 扇形法无法渲染 <3 槽（tan 发散），槽数不足时隐藏转盘只留槽位说明
const wheelUsable = computed(() => slots.value.length >= 3)

// 扇形楔子：零尺寸三角形绕圆心旋转拼接；槽位 i 覆盖 [i·seg,(i+1)·seg)（12 点起顺时针，conic 约定）
// 几何要点：border-bottom 三角形朝下 = 180°（conic 约定），落扇区中心需 rotate((i+0.5)·seg − 180)；
// 0×0 元素 left:50% 定位的是 border box 左缘（内容点=三角形 apex 在其右 w 处），marginLeft:−w 把 apex 归位到圆心
function wedgeStyle(i: number) {
  const half = (seg.value / 2) * (Math.PI / 180)
  const w = Math.round(200 * Math.tan(half) * 100) / 100
  const color = slotColors[i % slotColors.length]
  return {
    borderLeft: `${w}rpx solid transparent`,
    borderRight: `${w}rpx solid transparent`,
    borderBottom: `200rpx solid ${color}`,
    marginLeft: `-${w}rpx`,
    transform: `rotate(${(i + 0.5) * seg.value - 180}deg)`,
  }
}

// 槽位标签：绕圆心排布（指针在 12 点钟方向；170rpx 半径，标签贴内圈）
function labelStyle(i: number) {
  const angle = (i + 0.5) * seg.value
  return {
    transform: `rotate(${angle}deg) translateY(-170rpx) rotate(-${angle}deg)`,
  }
}

function prizeText(p: CampaignPrizeItem): string {
  switch (p.kind) {
    case 1:
      return t('campaign.prizeCash', { amount: p.amount })
    case 2:
      return t('campaign.prizePoints', { points: p.points })
    case 3:
      return p.couponName || p.couponTemplateNo
    default:
      return ''
  }
}

// 奖品签名：响应奖品 ↔ 槽位奖品 匹配（转盘每抽恰一个奖品）
function prizeSig(p: CampaignPrizeItem): string {
  return `${p.kind}|${p.couponTemplateNo}|${p.points}|${p.amount}`
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    preview.value = await campaignApi.campaignPreview(campaignKey.value)
    slots.value = preview.value.slots
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    loading.value = false
  }
}

// 指针停在中奖槽位：槽位中心对齐 12 点方向，总角 = 满圈数 × 360 + 对齐角
function landingRotation(slotIndex: number): number {
  const center = (slotIndex + 0.5) * seg.value
  const align = (360 - center) % 360
  return rotation.value + 5 * 360 + align
}

function finishSpin(p: CampaignPrizeItem[]) {
  spinning.value = false
  resultPrizes.value = p
  resultVisible.value = true
  if (preview.value) {
    preview.value.spinsLeft = Math.max(0, preview.value.spinsLeft - 1)
  }
}

async function spin() {
  if (spinning.value || slots.value.length === 0) return
  if (!idemKey) {
    idemKey = campaignApi.newIdemKey() // 一次抽奖动作一个幂等键，失败重试复用
  }
  spinning.value = true
  let resp: Awaited<ReturnType<typeof campaignApi.campaignParticipate>>
  try {
    resp = await campaignApi.campaignParticipate(campaignKey.value, '', idemKey)
  } catch (err) {
    spinning.value = false
    uni.showToast({ title: (err as Error).message, icon: 'none' })
    return
  }
  idemKey = '' // 成功则重置，下次抽奖用新键

  // 定位中奖槽位：响应奖品与槽位奖品签名匹配（兜底随机落点）
  const target = resp.prizes.length > 0 ? resp.prizes[0] : null
  let slotIndex = 0
  if (target) {
    const idx = slots.value.findIndex((s) => prizeSig(s.prize) === prizeSig(target))
    if (idx >= 0) slotIndex = idx
    else slotIndex = Math.floor(Math.random() * slots.value.length)
  }

  rotation.value = landingRotation(slotIndex)
  // 定时收尾：动画结束后展示结果（transitionend 在部分小程序环境不可靠，同 web 定时兜底）
  spinTimer = setTimeout(() => {
    if (spinning.value) finishSpin(resp.prizes)
  }, 3200)
}

function closeResult() {
  resultVisible.value = false
}

onLoad((options) => {
  campaignKey.value = String(options?.campaignKey ?? '')
  void load()
})
onUnload(() => {
  if (spinTimer) {
    clearTimeout(spinTimer)
    spinTimer = null
  }
})
</script>

<template>
  <view>
    <view class="page-title">{{ t('campaign.wheelTitle') }}</view>
    <view v-if="loading" class="list-empty">{{ t('common.loading') }}</view>
    <view v-else-if="error" class="error-text">{{ error }}</view>
    <template v-else-if="slots.length > 0">
      <view class="spins-left">
        {{ t('campaign.spinsLeft', { n: preview?.spinsLeft ?? 0 }) }}
        <text class="daily-limit">（{{ t('campaign.spinDailyLimit', { n: preview?.spinDailyLimit ?? 0 }) }}）</text>
      </view>

      <view v-if="wheelUsable" class="wheel-wrap">
        <view class="wheel-pointer"></view>
        <view class="wheel">
          <view class="wheel-inner" :style="{ transform: `rotate(${rotation}deg)` }">
            <view
              v-for="(s, i) in slots"
              :key="s.slotIndex"
              class="wheel-wedge"
              :style="wedgeStyle(i)"
            ></view>
            <view
              v-for="(s, i) in slots"
              :key="`label-${s.slotIndex}`"
              class="slot-label"
              :style="labelStyle(i)"
            >
              {{ s.label }}
            </view>
          </view>
        </view>
      </view>

      <view class="btn-primary spin-btn" :class="{ disabled: spinning }" @click="spin">
        {{ spinning ? t('campaign.spinning') : t('campaign.spin') }}
      </view>

      <!-- 槽位说明 -->
      <view class="slot-list">
        <view v-for="s in slots" :key="s.slotIndex" class="slot-item">
          <view class="slot-dot" :style="{ background: slotColors[s.slotIndex % slotColors.length] }"></view>
          <text class="slot-label-text">{{ s.label }}</text>
          <text class="slot-prize">{{ prizeText(s.prize) }}</text>
        </view>
      </view>
    </template>
    <view v-else class="list-empty">{{ t('campaign.wheelEmpty') }}</view>

    <!-- 中奖弹窗 -->
    <view v-if="resultVisible" class="modal-mask" @click="closeResult">
      <view class="modal-card" @click.stop>
        <view class="result-body">
          <view class="result-title">{{ t('campaign.winModalTitle') }}</view>
          <template v-if="resultPrizes.length > 0">
            <view v-for="(p, i) in resultPrizes" :key="i" class="result-prize accent">
              {{ prizeText(p) }}
            </view>
          </template>
          <view v-else class="result-none">{{ t('campaign.noPrize') }}</view>
          <view class="btn-plain" @click="closeResult">{{ t('common.confirm') }}</view>
        </view>
      </view>
    </view>
  </view>
</template>

<style scoped lang="scss">
.error-text {
  color: var(--color-danger);
  text-align: center;
}

.spins-left {
  text-align: center;
  font-size: 28rpx;
  margin-bottom: 24rpx;

  .daily-limit {
    color: var(--color-text-secondary);
    font-size: 24rpx;
  }
}

.wheel-wrap {
  position: relative;
  width: 400rpx;
  height: 400rpx;
  margin: 0 auto 48rpx;
}

.wheel-pointer {
  position: absolute;
  top: -12rpx;
  left: 50%;
  margin-left: -20rpx;
  z-index: 2;
  width: 0;
  height: 0;
  border-left: 20rpx solid transparent;
  border-right: 20rpx solid transparent;
  border-top: 36rpx solid var(--color-primary);
  filter: drop-shadow(0 4rpx 4rpx rgba(0, 0, 0, 0.25));
}

.wheel {
  width: 100%;
  height: 100%;
  border-radius: 50%;
  border: 12rpx solid #fff;
  box-shadow: 0 0 0 4rpx var(--color-border), 0 12rpx 32rpx rgba(0, 0, 0, 0.15);
  position: relative;
  overflow: hidden; // 扇形底边超圆处由圆形裁剪
  box-sizing: border-box;

  .wheel-inner {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    transition: transform 2.8s cubic-bezier(0.15, 0.75, 0.25, 1);
  }

  .wheel-wedge {
    position: absolute;
    top: 50%;
    left: 50%;
    width: 0;
    height: 0;
    transform-origin: 50% 0%; // 三角形 apex = 圆心
  }

  .slot-label {
    position: absolute;
    top: 50%;
    left: 50%;
    width: 120rpx;
    margin-left: -60rpx;
    margin-top: -14rpx;
    font-size: 24rpx;
    line-height: 28rpx;
    font-weight: 600;
    color: #5b3a12;
    white-space: nowrap;
    text-align: center;
    pointer-events: none;
  }
}

.spin-btn {
  max-width: 560rpx;
  margin: 0 auto;
  display: block;
}

.slot-list {
  margin-top: 40rpx;

  .slot-item {
    display: flex;
    align-items: center;
    gap: 16rpx;
    padding: 16rpx 24rpx;
    border-bottom: 1rpx solid var(--color-border);

    .slot-dot {
      width: 20rpx;
      height: 20rpx;
      border-radius: 50%;
      border: 1rpx solid rgba(0, 0, 0, 0.15);
      flex-shrink: 0;
    }

    .slot-label-text {
      font-size: 26rpx;
    }

    .slot-prize {
      margin-left: auto;
      color: var(--color-text-secondary);
      font-size: 26rpx;
    }
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
  }

  .result-none {
    color: var(--color-text-secondary);
  }

  .btn-plain {
    margin-top: 32rpx;
  }
}
</style>

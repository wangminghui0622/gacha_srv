<script setup lang="ts">
import { onUnmounted, ref } from 'vue'
import { onShow, onHide } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as rollingApi from '@/services/rolling'

import type { RollingWinItem } from '@/services/types'

// 首页滚动中奖 marquee：轮询 15s，只滚动高档/限量中奖（NORMAL 已被服务端过滤）；
// 用户 id 掩码显示（***后四位），不泄露用户信息。小程序用 swiper 纵向循环替代 web 的 CSS 动画。

const { t } = useI18n()

const wins = ref<RollingWinItem[]>([])
const POLL_MS = 15000

function maskUserId(userId: number): string {
  const s = String(userId)
  return t('rolling.userMask', { suffix: s.length >= 4 ? s.slice(-4) : s.padStart(4, '0') })
}

async function refresh() {
  try {
    const resp = await rollingApi.listRecentWins({ marqueeOnly: true, limit: 10 })
    wins.value = resp.wins
  } catch {
    // 静默失败：保留上一条列表，下轮重试
    console.warn('rolling marquee refresh failed')
  }
}

let timer: ReturnType<typeof setInterval> | undefined
onShow(() => {
  void refresh()
  timer = setInterval(() => void refresh(), POLL_MS)
})
onHide(() => {
  if (timer) clearInterval(timer)
})
onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <view class="rolling-marquee">
    <text class="marquee-title">🎉 {{ t('rolling.title') }}</text>
    <view v-if="wins.length" class="marquee-window">
      <swiper
        class="marquee-swiper"
        vertical
        circular
        autoplay
        :interval="3000"
        :duration="500"
        :disable-touch="true"
      >
        <swiper-item v-for="win in wins" :key="win.requestId">
          <view class="marquee-item">
            <text class="marquee-text">
              {{ maskUserId(win.userId) }} {{ t('rolling.winText', { entry: win.entryName }) }}
            </text>
            <text class="tier-badge" :class="win.tier === 3 ? 'tier-limited' : 'tier-high'">
              {{ win.tier === 3 ? t('rolling.tierLimited') : t('rolling.tierHigh') }}
            </text>
          </view>
        </swiper-item>
      </swiper>
    </view>
    <text v-else class="marquee-empty">{{ t('rolling.empty') }}</text>
  </view>
</template>

<style scoped lang="scss">
.rolling-marquee {
  display: flex;
  align-items: center;
  gap: 16rpx;
  margin: 0 24rpx 24rpx;
  padding: 12rpx 20rpx;
  background: linear-gradient(90deg, #fff7e6, #fff);
  border: 1rpx solid #ffe7ba;
  border-radius: 12rpx;
  font-size: 26rpx;

  .marquee-title {
    flex-shrink: 0;
    font-weight: 600;
    color: var(--color-warning);
    white-space: nowrap;
  }

  .marquee-window {
    flex: 1;
    min-width: 0;
  }

  .marquee-swiper {
    height: 44rpx;
  }

  .marquee-item {
    display: flex;
    align-items: center;
    height: 44rpx;

    .marquee-text {
      flex: 1;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .tier-badge {
      margin-left: 8rpx;
      padding: 0 8rpx;
      border-radius: 6rpx;
      font-size: 22rpx;
      color: #fff;
      flex-shrink: 0;
    }

    .tier-limited {
      background: #d48806;
    }

    .tier-high {
      background: #1677ff;
    }
  }

  .marquee-empty {
    color: var(--color-text-secondary);
  }
}
</style>

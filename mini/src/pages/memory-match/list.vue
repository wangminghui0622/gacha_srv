<template>
  <view class="mm-list-page">
    <view class="list-hero">
      <text class="list-title">对对碰</text>
      <text class="list-sub">选择一个主题开始游戏</text>
    </view>

    <view v-if="loading" class="list-state">加载中…</view>
    <view v-else-if="configs.length === 0" class="list-state">暂无可玩主题</view>
    <view v-else class="theme-list">
      <view v-for="c in configs" :key="c.configId" class="theme-card" @click="open(c.configId)">
        <view class="theme-head">
          <text class="theme-name">{{ c.name }}</text>
          <text class="theme-price">¥{{ c.purchasePrice }}</text>
        </view>
        <text class="theme-meta">{{ c.roundsPerPurchase }} 回合 · 每回合最多 {{ c.maxPairCount }} 对</text>
        <text class="theme-cta">进入主题 ›</text>
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'

import * as memoryMatchApi from '@/services/memoryMatch'
import type { MemoryMatchConfigSummary } from '@/services/types'

const configs = ref<MemoryMatchConfigSummary[]>([])
const loading = ref(true)

onLoad(async () => {
  try {
    const res = await memoryMatchApi.listMemoryMatchConfigs()
    configs.value = res.configs ?? []
  } catch (err) {
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  } finally {
    loading.value = false
  }
})

function open(configId: number) {
  uni.navigateTo({ url: `/pages/memory-match/index?configId=${configId}` })
}
</script>

<style scoped lang="scss">
.mm-list-page {
  box-sizing: border-box;
  min-height: 100vh;
  padding: calc(env(safe-area-inset-top) + 32rpx) 28rpx calc(40rpx + env(safe-area-inset-bottom));
  background:
    radial-gradient(circle at 18% 0%, rgba(46, 140, 88, 0.3), transparent 52%),
    radial-gradient(circle at 92% 96%, rgba(122, 84, 20, 0.22), transparent 48%),
    #080b09;
  color: #e9f2ec;
}

.list-hero {
  padding: 10rpx 8rpx 30rpx;
}

.list-title {
  display: block;
  font-size: 56rpx;
  font-weight: 900;
  letter-spacing: 4rpx;
  color: #ffffff;
}

.list-sub {
  display: block;
  margin-top: 8rpx;
  font-size: 24rpx;
  color: #8aa696;
}

.list-state {
  padding: 80rpx 0;
  text-align: center;
  font-size: 26rpx;
  color: #8aa696;
}

.theme-list {
  display: flex;
  flex-direction: column;
  gap: 20rpx;
}

.theme-card {
  padding: 28rpx;
  border-radius: 22rpx;
  border: 1rpx solid rgba(120, 255, 180, 0.16);
  background: linear-gradient(165deg, rgba(23, 35, 27, 0.98), rgba(9, 15, 11, 0.98));
  box-shadow: 0 16rpx 40rpx rgba(0, 0, 0, 0.35);
}

.theme-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16rpx;
}

.theme-name {
  flex: 1;
  min-width: 0;
  font-size: 34rpx;
  font-weight: 800;
  color: #ffffff;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.theme-price {
  flex-shrink: 0;
  font-size: 32rpx;
  font-weight: 800;
  color: #ffd166;
}

.theme-meta {
  display: block;
  margin-top: 10rpx;
  font-size: 23rpx;
  color: #8aa696;
}

.theme-cta {
  display: block;
  margin-top: 18rpx;
  font-size: 24rpx;
  color: #5cf2a2;
}
</style>

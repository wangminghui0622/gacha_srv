<template>
  <view class="mm-list-page">
    <view v-if="loading" class="list-state">加载中…</view>
    <view v-else-if="configs.length === 0" class="list-state">暂无可玩主题</view>
    <view v-else class="theme-grid">
      <view v-for="c in configs" :key="c.configId" class="theme-card" @click="open(c.configId)">
        <view class="theme-cover">
          <image v-if="coverOf(c.configId)" :src="coverOf(c.configId)" class="theme-cover-img" mode="aspectFill" @error="onCoverError(c.configId)" />
          <view v-else class="theme-cover-fallback">{{ c.name.slice(0, 1) }}</view>
          <text class="theme-tag">新箱</text>
        </view>
        <view class="theme-body">
          <text class="theme-name">{{ c.name }}</text>
          <text class="theme-meta">{{ c.roundsPerPurchase }} 回合 · 每回合最多 {{ c.maxPairCount }} 对</text>
          <view class="theme-foot">
            <text class="theme-price">¥{{ c.purchasePrice }}</text>
            <text class="theme-cta">进入 ›</text>
          </view>
        </view>
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
const covers = ref<Record<number, string>>({})
const loading = ref(true)

onLoad(async () => {
  try {
    const res = await memoryMatchApi.listMemoryMatchConfigs()
    configs.value = res.configs ?? []
    void loadCovers()
  } catch (err) {
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  } finally {
    loading.value = false
  }
})

async function loadCovers() {
  await Promise.all(
    configs.value.map(async (c) => {
      // 后台「赏池管理 → 编辑 → 封面大图/缩略图」优先
      if (c.coverImage && !c.coverImage.startsWith('/')) {
        covers.value[c.configId] = c.coverImage
        return
      }
      try {
        const detail = await memoryMatchApi.getMemoryMatchConfig(c.configId)
        const presets = detail.config.facePresets ?? []
        const preset = presets.find((p) => p.presetId === detail.config.defaultFacePresetId) ?? presets[0]
        const url = preset?.faces?.[0]?.imageUrl
        // 只使用可直接访问的绝对地址；/img/... 这类种子占位图在真机上会 404
        if (url && !url.startsWith('/')) covers.value[c.configId] = url
      } catch {
        // 封面失败不影响进入主题
      }
    }),
  )
}

function coverOf(configId: number): string {
  return covers.value[configId] ?? ''
}

function onCoverError(configId: number) {
  delete covers.value[configId]
}

function open(configId: number) {
  uni.navigateTo({ url: `/pages/memory-match/index?configId=${configId}` })
}
</script>

<style scoped lang="scss">
.mm-list-page {
  box-sizing: border-box;
  min-height: 100vh;
  padding: calc(env(safe-area-inset-top) + 32rpx) 24rpx calc(40rpx + env(safe-area-inset-bottom));
  background:
    radial-gradient(circle at 18% 0%, rgba(46, 140, 88, 0.3), transparent 52%),
    radial-gradient(circle at 92% 96%, rgba(122, 84, 20, 0.22), transparent 48%),
    #080b09;
  color: #e9f2ec;
}

.list-state {
  padding: 80rpx 0;
  text-align: center;
  font-size: 26rpx;
  color: #8aa696;
}

.theme-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20rpx;
}

.theme-card {
  position: relative;
  overflow: hidden;
  border-radius: 18rpx;
  border: 1rpx solid rgba(120, 255, 180, 0.16);
  background: linear-gradient(165deg, rgba(23, 35, 27, 0.98), rgba(9, 15, 11, 0.98));
  box-shadow: 0 16rpx 40rpx rgba(0, 0, 0, 0.35);
}

.theme-cover {
  position: relative;
  height: 300rpx;
  overflow: hidden;
  background:
    radial-gradient(circle at 50% 40%, rgba(46, 130, 80, 0.28), transparent 65%),
    #0c1b12;
}

.theme-cover-img {
  width: 100%;
  height: 100%;
}

.theme-cover-fallback {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  font-size: 88rpx;
  font-weight: 900;
  color: rgba(125, 255, 157, 0.55);
}

.theme-tag {
  position: absolute;
  top: 12rpx;
  right: 12rpx;
  z-index: 3;
  height: 42rpx;
  padding: 0 14rpx;
  border-radius: 8rpx;
  background: linear-gradient(135deg, #c6ff4a, #8ef02f);
  color: #10230a;
  font-size: 22rpx;
  font-weight: 900;
  line-height: 42rpx;
  text-align: center;
}

.theme-body {
  padding: 16rpx 18rpx 18rpx;
}

.theme-name {
  display: block;
  overflow: hidden;
  font-size: 28rpx;
  font-weight: 800;
  color: #ffffff;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.theme-meta {
  display: block;
  margin-top: 6rpx;
  overflow: hidden;
  font-size: 20rpx;
  color: #8aa696;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.theme-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 12rpx;
}

.theme-price {
  font-size: 32rpx;
  font-weight: 900;
  color: #7dff9d;
}

.theme-cta {
  font-size: 22rpx;
  color: #f3d27a;
}
</style>

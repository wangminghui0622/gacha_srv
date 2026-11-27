<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onPullDownRefresh, onShow } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as drawApi from '@/services/draw'

import type { BoxItem } from '@/services/types'

const { t } = useI18n()

const sys = uni.getSystemInfoSync()
const statusBarHeight = ref(sys.statusBarHeight || 0)
const capsuleRight = ref(0)
try {
  // #ifdef MP-WEIXIN
  const rect = (uni as any).getMenuButtonBoundingClientRect?.()
  if (rect) {
    capsuleRight.value = Math.max(0, (sys.windowWidth || 0) - rect.left + 8)
  }
  // #endif
} catch {
  capsuleRight.value = 0
}

const loading = ref(true)
const boxes = ref<BoxItem[]>([])
const keyword = ref('')
const autoFocus = ref(false)

const filtered = computed(() => {
  const value = keyword.value.trim().toLowerCase()
  if (!value) return boxes.value
  return boxes.value.filter((box) => box.boxName.toLowerCase().includes(value))
})

function boxArt(boxId: number): string {
  const index = ((Number(boxId) - 1) % 3 + 3) % 3
  return `/static/art/box-${index + 1}.png`
}

function boxImage(box: BoxItem): string {
  return box.coverImage || box.coverThumb || boxArt(box.boxId)
}

async function load() {
  loading.value = true
  try {
    boxes.value = (await drawApi.listBoxes()).list ?? []
  } catch {
    boxes.value = []
  } finally {
    loading.value = false
  }
}

onLoad((query) => {
  autoFocus.value = query?.focus === '1'
  const initial = query?.keyword
  if (initial) {
    keyword.value = decodeURIComponent(initial)
  }
})

onShow(() => {
  void load()
})

onPullDownRefresh(async () => {
  await load()
  uni.stopPullDownRefresh()
})

function goBack() {
  uni.navigateBack({
    fail: () => uni.reLaunch({ url: '/pages/index/index' }),
  })
}

function goBox(box: BoxItem) {
  uni.navigateTo({ url: `/pages/box-detail/index?boxId=${box.boxId}` })
}
</script>

<template>
  <view class="boxes-page" :style="{ paddingTop: `${statusBarHeight}px` }">
    <view class="nav-row" :style="capsuleRight ? { paddingRight: `${capsuleRight}px` } : undefined">
      <view class="back-btn" @click="goBack">
        <uni-icons type="left" size="22" color="#f4f8f2" />
      </view>
      <text class="nav-title">{{ t('home.allBoxes') }}</text>
      <view class="nav-placeholder" />
    </view>

    <view class="search-box">
      <uni-icons type="search" size="18" color="#7e887a" />
      <input
        v-model="keyword"
        class="search-input"
        :focus="autoFocus"
        :placeholder="t('home.searchPlaceholder')"
        placeholder-class="search-placeholder"
        confirm-type="search"
      />
      <view v-if="keyword" class="clear-btn" @click="keyword = ''">
        <uni-icons type="closeempty" size="16" color="#7e887a" />
      </view>
    </view>

    <view v-if="loading" class="status-box">
      <text class="status-text">{{ t('common.loading') }}</text>
    </view>
    <view v-else-if="!filtered.length" class="status-box">
      <text class="empty-icon">🫧</text>
      <text class="status-text">{{ keyword ? t('home.searchEmpty') : t('home.empty') }}</text>
    </view>
    <view v-else class="box-grid">
      <view v-for="box in filtered" :key="box.boxId" class="grid-card" @click="goBox(box)">
        <view class="grid-art-wrap">
          <image class="grid-art" :src="boxImage(box)" mode="aspectFill" />
          <view class="new-badge">{{ t('home.newBox') }}</view>
          <view v-if="box.soldOut" class="sold-out">{{ t('home.soldOut') }}</view>
        </view>
        <text class="grid-title">{{ box.boxName }}</text>
        <view class="grid-footer">
          <text class="grid-price">¥{{ box.price }}</text>
          <text class="grid-arrow">→</text>
        </view>
      </view>
    </view>
  </view>
</template>

<style scoped lang="scss">
page {
  background: #050706;
}

.boxes-page {
  min-height: 100vh;
  padding-bottom: 60rpx;
  background:
    radial-gradient(circle at 80% 0%, rgba(99, 255, 67, 0.14), transparent 34%),
    repeating-linear-gradient(118deg, rgba(255, 255, 255, 0.012) 0 2rpx, transparent 2rpx 96rpx),
    #050706;
}

.nav-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18rpx 28rpx 10rpx;
}

.back-btn,
.nav-placeholder {
  width: 66rpx;
  height: 66rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba(30, 38, 31, 0.82);
  border: 2rpx solid rgba(255, 255, 255, 0.1);
}

.nav-placeholder {
  background: transparent;
  border-color: transparent;
}

.nav-title {
  font-size: 34rpx;
  font-weight: 800;
  color: #f4f8f2;
}

.search-box {
  display: flex;
  align-items: center;
  gap: 12rpx;
  margin: 18rpx 28rpx 26rpx;
  padding: 0 22rpx;
  height: 76rpx;
  border-radius: 999rpx;
  background: rgba(24, 31, 25, 0.9);
  border: 2rpx solid rgba(182, 255, 59, 0.14);
}

.search-input {
  flex: 1;
  height: 76rpx;
  font-size: 26rpx;
  color: #f4f8f2;
}

.search-placeholder {
  color: #707a6c;
}

.clear-btn {
  width: 40rpx;
  height: 40rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.08);
}

.box-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 22rpx;
  padding: 0 28rpx;
}

.grid-card {
  width: calc((100% - 22rpx) / 2);
  display: flex;
  flex-direction: column;
  padding: 12rpx;
  border-radius: 24rpx;
  background: linear-gradient(160deg, rgba(29, 37, 30, 0.9), rgba(12, 17, 13, 0.96));
  border: 1rpx solid rgba(182, 255, 59, 0.08);
}

.grid-art-wrap {
  position: relative;
  height: 306rpx;
  border-radius: 18rpx;
  overflow: hidden;
  background: #101711;
}

.grid-art {
  width: 100%;
  height: 100%;
}

.new-badge {
  position: absolute;
  top: 0;
  right: 0;
  padding: 4rpx 14rpx;
  border-radius: 0 18rpx 0 16rpx;
  background: #b6ff3b;
  color: #0a0e0b;
  font-size: 20rpx;
  font-weight: 700;
}

.sold-out {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  font-size: 26rpx;
  font-weight: 700;
}

.grid-title {
  margin-top: 12rpx;
  font-size: 26rpx;
  line-height: 36rpx;
  color: #f0f5ed;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 72rpx;
}

.grid-footer {
  margin-top: 4rpx;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.grid-price {
  font-size: 30rpx;
  font-weight: 800;
  color: #b6ff3b;
}

.grid-arrow {
  font-size: 26rpx;
  color: #7e887a;
}

.status-box {
  padding: 120rpx 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12rpx;
}

.empty-icon {
  font-size: 60rpx;
}

.status-text {
  color: #7d867a;
  font-size: 26rpx;
}
</style>

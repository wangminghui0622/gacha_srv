<script setup lang="ts">
import { computed, ref } from 'vue'
import { onPullDownRefresh, onShow } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import BottomTabBar from '@/components/BottomTabBar.vue'
import * as drawApi from '@/services/draw'

import type { BoxItem, WarehouseItem } from '@/services/types'

type TabKey = 'all' | 'spot' | 'presale' | 'ship'
type SortKey = 'time' | 'grade'

interface GradeMeta {
  key: string
  label: string
  cls: string
}

interface WarehouseGroup {
  key: string
  itemIds: number[]
  itemName: string
  boxId: number
  boxName: string
  image: string
  quantity: number
  createdAt: number
  isPresale: boolean
  grade: GradeMeta
}

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
const items = ref<WarehouseItem[]>([])
const boxes = ref<BoxItem[]>([])
const activeTab = ref<TabKey>('all')
const sortBy = ref<SortKey>('time')
const sortAsc = ref(false)
const selectedKeys = ref<string[]>([])

const tabs = computed(() => [
  { key: 'all' as const, label: t('warehouse.tabAll') },
  { key: 'spot' as const, label: t('warehouse.tabSpot') },
  { key: 'presale' as const, label: t('warehouse.tabPresale') },
  { key: 'ship' as const, label: t('warehouse.tabShip') },
])

const boxMap = computed(() => {
  const map = new Map<number, BoxItem>()
  for (const box of boxes.value) {
    map.set(box.boxId, box)
  }
  return map
})

function boxArt(boxId: number): string {
  const index = ((Number(boxId) - 1) % 3 + 3) % 3
  return `/static/art/box-${index + 1}.png`
}

function boxImage(boxId: number): string {
  const box = boxMap.value.get(boxId)
  return box?.coverImage || box?.coverThumb || boxArt(boxId)
}

function gradeMeta(boxId: number): GradeMeta {
  const index = ((Number(boxId) - 1) % 3 + 3) % 3
  const list: GradeMeta[] = [
    { key: 'grade1', label: t('warehouse.grade1'), cls: 'grade-green' },
    { key: 'grade2', label: t('warehouse.grade2'), cls: 'grade-pink' },
    { key: 'grade3', label: t('warehouse.grade3'), cls: 'grade-gold' },
  ]
  return list[index]
}

function isPresaleBox(boxId: number, boxName: string): boolean {
  if (/presale|pre-sale/i.test(boxName) || boxName.includes('预售')) return true
  const box = boxMap.value.get(boxId)
  return /presale|pre-sale/i.test(box?.boxName ?? '')
}

const groups = computed<WarehouseGroup[]>(() => {
  const map = new Map<string, WarehouseGroup>()
  for (const item of items.value) {
    const key = `${item.boxId}|${item.itemName}`
    const existing = map.get(key)
    if (existing) {
      existing.itemIds.push(item.itemId)
      existing.quantity += 1
      existing.createdAt = Math.max(existing.createdAt, item.createdAt)
      continue
    }
    const box = boxMap.value.get(item.boxId)
    map.set(key, {
      key,
      itemIds: [item.itemId],
      itemName: item.itemName,
      boxId: item.boxId,
      boxName: box?.boxName || item.boxName || '',
      image: boxImage(item.boxId),
      quantity: 1,
      createdAt: item.createdAt,
      isPresale: isPresaleBox(item.boxId, box?.boxName || item.boxName || ''),
      grade: gradeMeta(item.boxId),
    })
  }
  return Array.from(map.values())
})

const filteredGroups = computed<WarehouseGroup[]>(() => {
  const list = groups.value.filter((group) => {
    if (activeTab.value === 'spot') return !group.isPresale
    if (activeTab.value === 'presale') return group.isPresale
    return true
  })
  return [...list].sort((a, b) => {
    if (sortBy.value === 'time') {
      return sortAsc.value ? a.createdAt - b.createdAt : b.createdAt - a.createdAt
    }
    const order = a.grade.key.localeCompare(b.grade.key)
    return sortAsc.value ? order : -order
  })
})

const totalCount = computed(() => filteredGroups.value.reduce((sum, group) => sum + group.quantity, 0))

const selectedCount = computed(() => {
  let count = 0
  for (const group of groups.value) {
    if (selectedKeys.value.includes(group.key)) count += group.quantity
  }
  return count
})

function isSelected(key: string): boolean {
  return selectedKeys.value.includes(key)
}

function toggleSelect(key: string) {
  const index = selectedKeys.value.indexOf(key)
  if (index >= 0) {
    selectedKeys.value.splice(index, 1)
  } else {
    selectedKeys.value.push(key)
  }
}

function onTabTap(key: TabKey) {
  activeTab.value = key
  selectedKeys.value = []
}

function onSortTap(key: SortKey) {
  if (sortBy.value === key) {
    sortAsc.value = !sortAsc.value
    return
  }
  sortBy.value = key
  sortAsc.value = key === 'grade'
}

async function load() {
  loading.value = true
  const [warehouseResult, boxesResult] = await Promise.allSettled([
    drawApi.listWarehouse(),
    drawApi.listBoxes(),
  ])
  items.value = warehouseResult.status === 'fulfilled' ? warehouseResult.value.list ?? [] : []
  boxes.value = boxesResult.status === 'fulfilled' ? boxesResult.value.list ?? [] : []
  loading.value = false
}

function hideNativeTabBar() {
  try {
    uni.hideTabBar({ animation: false, fail: () => {} })
  } catch {
    // H5 下无原生 tabBar 时忽略
  }
}

onShow(() => {
  hideNativeTabBar()
  void load()
})

onPullDownRefresh(async () => {
  await load()
  uni.stopPullDownRefresh()
})

function showRules() {
  uni.showModal({
    title: t('warehouse.rulesTitle'),
    content: t('warehouse.rulesText'),
    showCancel: false,
  })
}

function shipSelected() {
  if (selectedCount.value <= 0) {
    uni.showToast({ title: t('warehouse.selectFirst'), icon: 'none' })
    return
  }
  uni.showModal({
    title: t('warehouse.shipTitle'),
    content: t('warehouse.shipPending', { n: selectedCount.value }),
    showCancel: false,
  })
}

function goDraw() {
  uni.switchTab({
    url: '/pages/index/index',
    fail: () => uni.reLaunch({ url: '/pages/index/index' }),
  })
}
</script>

<template>
  <view class="warehouse-page" :style="{ paddingTop: `${statusBarHeight}px` }">
    <view class="top-glow" />

    <view class="warehouse-header" :style="capsuleRight ? { paddingRight: `${capsuleRight}px` } : undefined">
      <view class="title-wrap">
        <text class="page-title">{{ t('warehouse.myTitle') }}</text>
        <view class="title-underline" />
      </view>
      <view class="rules-pill" @click="showRules">
        <text>{{ t('warehouse.rules') }}</text>
        <view class="rules-dot">!</view>
      </view>
    </view>

    <view class="tab-shell">
      <view
        v-for="tab in tabs"
        :key="tab.key"
        class="tab-item"
        :class="{ active: activeTab === tab.key }"
        @click="onTabTap(tab.key)"
      >
        {{ tab.label }}
      </view>
    </view>

    <view class="sort-row">
      <view class="sort-group">
        <view class="sort-btn" :class="{ active: sortBy === 'time' }" @click="onSortTap('time')">
          <text>{{ t('warehouse.sortTime') }}</text>
          <text class="sort-arrows">{{ sortBy === 'time' ? (sortAsc ? '↑' : '↓') : '↕' }}</text>
        </view>
        <view class="sort-btn" :class="{ active: sortBy === 'grade' }" @click="onSortTap('grade')">
          <text>{{ t('warehouse.sortGrade') }}</text>
          <text class="sort-arrows">{{ sortBy === 'grade' ? (sortAsc ? '↑' : '↓') : '↕' }}</text>
        </view>
      </view>
      <view class="count-pill">{{ t('warehouse.count', { n: totalCount }) }}</view>
    </view>

    <view v-if="loading" class="status-box">
      <text class="status-text">{{ t('common.loading') }}</text>
    </view>

    <view v-else-if="!filteredGroups.length" class="status-box empty-state">
      <text class="empty-icon">📦</text>
      <text class="status-text">{{ items.length ? t('warehouse.emptyFilter') : t('warehouse.empty') }}</text>
      <view v-if="!items.length" class="draw-btn" @click="goDraw">{{ t('nav.home') }} →</view>
    </view>

    <view v-else class="prize-grid">
      <view
        v-for="group in filteredGroups"
        :key="group.key"
        class="prize-card"
        :class="{ selected: isSelected(group.key), presale: group.isPresale }"
        @click="toggleSelect(group.key)"
      >
        <view class="prize-art-wrap">
          <image class="prize-art" :src="group.image" mode="aspectFill" />
          <view v-if="group.quantity > 1" class="qty-badge">×{{ group.quantity }}</view>
          <view class="stock-badge">{{ group.isPresale ? t('warehouse.presale') : t('warehouse.spot') }}</view>
          <view class="grade-ribbon" :class="group.grade.cls">{{ group.grade.label }}</view>
          <view v-if="isSelected(group.key)" class="selected-mask">
            <view class="selected-check">✓</view>
          </view>
        </view>
        <text class="prize-name">{{ group.itemName }}</text>
      </view>
    </view>

  </view>

  <view class="ship-bar">
    <view class="ship-btn" @click="shipSelected">
      {{ selectedCount > 0 ? t('warehouse.shipWithCount', { n: selectedCount }) : t('warehouse.ship') }}
    </view>
  </view>

  <BottomTabBar active="warehouse" />
</template>

<style scoped lang="scss">
page {
  background: #050706;
}

.warehouse-page {
  position: relative;
  min-height: 100vh;
  padding-bottom: 340rpx;
  background:
    radial-gradient(circle at 22% 0%, rgba(128, 255, 46, 0.14), transparent 34%),
    repeating-linear-gradient(118deg, rgba(255, 255, 255, 0.012) 0 2rpx, transparent 2rpx 96rpx),
    #050706;
  overflow: hidden;
}

.top-glow {
  position: absolute;
  top: -240rpx;
  left: -100rpx;
  width: 560rpx;
  height: 560rpx;
  border-radius: 50%;
  background: rgba(126, 240, 42, 0.12);
  filter: blur(100rpx);
  pointer-events: none;
}

.warehouse-header {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 26rpx 28rpx 0;
}

.title-wrap {
  position: relative;
}

.page-title {
  font-size: 50rpx;
  line-height: 1.1;
  font-weight: 900;
  font-style: italic;
  letter-spacing: -2rpx;
  color: #f5f9f2;
  text-shadow: 0 0 24rpx rgba(182, 255, 59, 0.16);
}

.title-underline {
  width: 92rpx;
  height: 10rpx;
  margin: 2rpx 0 0 4rpx;
  border-radius: 999rpx;
  background: #b6ff3b;
  transform: rotate(-2deg);
  box-shadow: 0 0 18rpx rgba(182, 255, 59, 0.45);
}

.rules-pill {
  display: flex;
  align-items: center;
  gap: 8rpx;
  margin-top: 10rpx;
  padding: 10rpx 16rpx;
  border-radius: 999rpx;
  background: rgba(35, 43, 35, 0.86);
  border: 1rpx solid rgba(255, 255, 255, 0.1);
  font-size: 22rpx;
  color: #d6ded1;
}

.rules-dot {
  width: 24rpx;
  height: 24rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: #e9f0e4;
  color: #11160f;
  font-size: 18rpx;
  font-weight: 800;
}

.tab-shell {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: center;
  gap: 6rpx;
  margin: 34rpx 28rpx 0;
  padding: 8rpx;
  border-radius: 999rpx;
  background: rgba(22, 28, 23, 0.94);
  border: 1rpx solid rgba(255, 255, 255, 0.05);
}

.tab-item {
  flex: 1;
  height: 64rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 999rpx;
  font-size: 26rpx;
  font-weight: 600;
  color: #c4ccc0;
  transition: background 0.2s ease;
}

.tab-item.active {
  background: #b6ff3b;
  color: #0b0f0a;
  font-weight: 800;
  box-shadow: 0 0 24rpx rgba(182, 255, 59, 0.22);
}

.sort-row {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 20rpx 28rpx 16rpx;
}

.sort-group {
  display: flex;
  align-items: center;
  gap: 22rpx;
}

.sort-btn {
  display: flex;
  align-items: center;
  gap: 6rpx;
  font-size: 24rpx;
  color: #8d9688;
}

.sort-btn.active {
  color: #e9f0e4;
}

.sort-arrows {
  font-size: 20rpx;
  color: #b6ff3b;
  letter-spacing: -4rpx;
}

.count-pill {
  padding: 8rpx 16rpx;
  border-radius: 10rpx;
  background: rgba(35, 43, 35, 0.9);
  border: 1rpx solid rgba(255, 255, 255, 0.12);
  font-size: 22rpx;
  color: #d8e0d3;
}

.prize-grid {
  position: relative;
  z-index: 2;
  display: flex;
  flex-wrap: wrap;
  gap: 14rpx;
  padding: 0 28rpx;
}

.prize-card {
  width: calc((100% - 28rpx) / 3);
  padding: 8rpx 8rpx 12rpx;
  border-radius: 20rpx;
  background: linear-gradient(160deg, rgba(30, 38, 30, 0.94), rgba(11, 16, 12, 0.98));
  border: 2rpx solid rgba(255, 255, 255, 0.06);
  box-sizing: border-box;
  transition: border-color 0.2s ease, transform 0.2s ease;
}

.prize-card.selected {
  border-color: #b6ff3b;
  box-shadow: 0 0 24rpx rgba(182, 255, 59, 0.2);
}

.prize-art-wrap {
  position: relative;
  width: 100%;
  height: 202rpx;
  border-radius: 14rpx;
  overflow: hidden;
  background: #0e140f;
}

.prize-art {
  width: 100%;
  height: 100%;
}

.qty-badge {
  position: absolute;
  top: 6rpx;
  left: 6rpx;
  padding: 2rpx 8rpx;
  border-radius: 8rpx;
  background: rgba(6, 10, 6, 0.78);
  color: #fff;
  font-size: 19rpx;
  font-weight: 700;
}

.stock-badge {
  position: absolute;
  top: 0;
  right: 0;
  padding: 4rpx 10rpx;
  border-radius: 0 14rpx 0 12rpx;
  background: #b6ff3b;
  color: #0a0e09;
  font-size: 18rpx;
  font-weight: 800;
}

.prize-card.presale .stock-badge {
  background: #ffd05a;
}

.grade-ribbon {
  position: absolute;
  left: 0;
  bottom: 0;
  padding: 3rpx 12rpx;
  border-radius: 0 12rpx 0 12rpx;
  color: #fff;
  font-size: 20rpx;
  font-weight: 700;
}

.grade-green {
  background: linear-gradient(90deg, #35c76a, #7dea3b);
}

.grade-pink {
  background: linear-gradient(90deg, #ff4fa1, #ff8bc0);
}

.grade-gold {
  background: linear-gradient(90deg, #ff9b21, #ffd45a);
}

.selected-mask {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(8, 14, 8, 0.55);
}

.selected-check {
  width: 56rpx;
  height: 56rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: #b6ff3b;
  color: #0a0e09;
  font-size: 34rpx;
  font-weight: 900;
  box-shadow: 0 0 24rpx rgba(182, 255, 59, 0.5);
}

.prize-name {
  margin-top: 8rpx;
  font-size: 23rpx;
  line-height: 30rpx;
  color: #e9f0e4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 60rpx;
}

.ship-bar {
  position: fixed;
  left: 40rpx;
  right: 40rpx;
  bottom: calc(132rpx + env(safe-area-inset-bottom));
  z-index: 70;
}

.ship-btn {
  height: 88rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 999rpx;
  background: linear-gradient(135deg, #c9ff6a, #9df42f);
  color: #121a0b;
  font-size: 32rpx;
  font-weight: 800;
  letter-spacing: 2rpx;
  box-shadow: 0 12rpx 34rpx rgba(140, 240, 40, 0.28);
}

.status-box {
  padding: 120rpx 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14rpx;
}

.empty-icon {
  font-size: 64rpx;
}

.status-text {
  color: #7d867a;
  font-size: 26rpx;
}

.draw-btn {
  margin-top: 10rpx;
  padding: 12rpx 28rpx;
  border-radius: 999rpx;
  border: 1rpx solid rgba(182, 255, 59, 0.4);
  color: #b6ff3b;
  font-size: 24rpx;
}
</style>

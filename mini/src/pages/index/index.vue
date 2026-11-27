<script setup lang="ts">
import { computed, ref } from 'vue'
import { onPullDownRefresh, onShow } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import BottomTabBar from '@/components/BottomTabBar.vue'
import HorizontalScroll from '@/components/HorizontalScroll.vue'
import * as drawApi from '@/services/draw'
import * as gashaponApi from '@/services/gashapon'
import * as userApi from '@/services/user'
import { useAuthStore } from '@/store/authStore'

import type { BoxItem, GashaponDetailResponse } from '@/services/types'

type CategoryKey = 'hot' | 'lucky' | 'infinite'
type CardKind = 'box' | 'gashapon'

interface HomeCard {
  key: string
  name: string
  price: string
  image: string
  soldOut: boolean
  kind: CardKind
  boxId?: number
}

interface BannerSlide {
  key: string
  title: string
  price: string
  art: string
  kind: CardKind | 'brand'
  boxId?: number
}

const { t } = useI18n()
const auth = useAuthStore()

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
const gashapon = ref<GashaponDetailResponse | null>(null)
const activeCategory = ref<CategoryKey>('hot')
const bannerIndex = ref(0)

const notices = computed(() => [t('home.notice1'), t('home.notice2'), t('home.notice3')])

const categories = computed(() => [
  { key: 'hot' as const, label: t('home.categories.hot') },
  { key: 'lucky' as const, label: t('home.categories.lucky') },
  { key: 'infinite' as const, label: t('home.categories.infinite') },
  { key: 'match' as const, label: t('home.categories.match') },
  { key: 'arena' as const, label: t('home.categories.arena') },
  { key: 'combo' as const, label: t('home.categories.combo') },
  { key: 'dig' as const, label: t('home.categories.dig') },
  { key: 'group' as const, label: t('home.categories.group') },
])

const quickEntries = computed(() => [
  { key: 'dig', emoji: '🥚', label: t('home.quick.dig'), bg: 'linear-gradient(145deg, #eaff7a, #9be22f)' },
  { key: 'codex', emoji: '📖', label: t('home.quick.codex'), bg: 'linear-gradient(145deg, #ffd85a, #ff9f1a)' },
  { key: 'community', emoji: '💬', label: t('home.quick.community'), bg: 'linear-gradient(145deg, #8dff9d, #24c46a)' },
  { key: 'exchange', emoji: '🎁', label: t('home.quick.exchange'), bg: 'linear-gradient(145deg, #ffcf6a, #ff7a3c)' },
  { key: 'benefits', emoji: '🎉', label: t('home.quick.benefits'), bg: 'linear-gradient(145deg, #fff07a, #ffbf00)' },
])

const heroArts = ['/static/art/hero-1.png', '/static/art/hero-2.png', '/static/art/hero-3.png']

function boxArt(boxId: number | undefined): string {
  const id = Number(boxId ?? 1)
  const index = ((id - 1) % 3 + 3) % 3
  return `/static/art/box-${index + 1}.png`
}

function boxImage(box: BoxItem): string {
  return box.coverImage || box.coverThumb || boxArt(box.boxId)
}

function toCard(box: BoxItem): HomeCard {
  return {
    key: `box-${box.boxId}`,
    name: box.boxName,
    price: box.price,
    image: boxImage(box),
    soldOut: box.soldOut,
    kind: 'box',
    boxId: box.boxId,
  }
}

const infiniteCards = computed<HomeCard[]>(() => boxes.value.map(toCard))

const luckyCards = computed<HomeCard[]>(() => {
  if (gashapon.value) {
    const gashaponCard: HomeCard = {
      key: 'gashapon',
      name: gashapon.value.name,
      price: gashapon.value.price,
      image: '/static/art/gashapon.png',
      soldOut: gashapon.value.remainingTickets <= 0,
      kind: 'gashapon',
    }
    return [gashaponCard, ...boxes.value.slice(0, 2).map(toCard)]
  }
  return boxes.value.slice(0, 3).map(toCard)
})

const bannerSlides = computed<BannerSlide[]>(() => {
  if (boxes.value.length) {
    return boxes.value.slice(0, 3).map((box, index) => ({
      key: `banner-${box.boxId}`,
      title: box.boxName,
      price: box.price,
      art: heroArts[index % heroArts.length],
      kind: 'box' as const,
      boxId: box.boxId,
    }))
  }
  if (gashapon.value) {
    return [
      {
        key: 'banner-gashapon',
        title: gashapon.value.name,
        price: gashapon.value.price,
        art: heroArts[0],
        kind: 'gashapon' as const,
      },
    ]
  }
  return [
    {
      key: 'banner-brand',
      title: t('home.brand'),
      price: '',
      art: heroArts[0],
      kind: 'brand' as const,
    },
  ]
})

const showLucky = computed(() => activeCategory.value === 'hot' || activeCategory.value === 'lucky')
const showInfinite = computed(() => activeCategory.value === 'hot' || activeCategory.value === 'infinite')

async function refreshProfile() {
  if (auth.user) return
  try {
    auth.setUser(await userApi.getProfile())
  } catch {
    // 401 由请求适配器统一处理
  }
}

async function load() {
  loading.value = true
  const [boxResult, gasResult] = await Promise.allSettled([
    drawApi.listBoxes(),
    gashaponApi.getGashapon(1),
  ])
  if (boxResult.status === 'fulfilled') {
    boxes.value = boxResult.value.list ?? []
  } else {
    boxes.value = []
  }
  gashapon.value = gasResult.status === 'fulfilled' ? gasResult.value : null
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
  void refreshProfile()
  void load()
})

onPullDownRefresh(async () => {
  await Promise.allSettled([refreshProfile(), load()])
  uni.stopPullDownRefresh()
})

function onCategoryTap(key: string) {
  const routes: Record<string, string> = {
    match: '/pages/memory-match/list',
    arena: '/pages/full-car/index',
    combo: '/pages/combo-chain/index',
    dig: '/pages/dig/index',
    group: '/pages/group-buy/index',
  }
  const route = routes[key]
  if (route) {
    uni.navigateTo({ url: route })
    return
  }
  activeCategory.value = key as CategoryKey
}

function goSearch() {
  uni.navigateTo({ url: '/pages/boxes/index?focus=1' })
}

function goMore(kind: 'lucky' | 'infinite') {
  if (kind === 'lucky' && gashapon.value) {
    uni.navigateTo({ url: '/pages/gashapon/index' })
    return
  }
  uni.navigateTo({ url: '/pages/boxes/index' })
}

function goCard(card: HomeCard) {
  if (card.kind === 'gashapon') {
    uni.navigateTo({ url: '/pages/gashapon/index' })
    return
  }
  if (card.boxId) {
    uni.navigateTo({ url: `/pages/box-detail/index?boxId=${card.boxId}` })
  }
}

function goBanner(slide: BannerSlide) {
  if (slide.kind === 'gashapon') {
    uni.navigateTo({ url: '/pages/gashapon/index' })
    return
  }
  if (slide.boxId) {
    uni.navigateTo({ url: `/pages/box-detail/index?boxId=${slide.boxId}` })
  }
}

function onBannerChange(event: { detail: { current: number } }) {
  bannerIndex.value = event.detail.current
}

function onQuickTap(key: string) {
  if (key === 'dig') {
    uni.navigateTo({ url: '/pages/dig/index' })
    return
  }
  if (key === 'exchange') {
    uni.navigateTo({ url: '/pages/campaigns/index' })
    return
  }
  if (key === 'benefits') {
    uni.navigateTo({ url: '/pages/tasks/index' })
    return
  }
  if (key === 'codex') {
    uni.showModal({
      title: t('home.codexTitle'),
      content: t('home.codexText'),
      showCancel: false,
    })
    return
  }
  uni.showModal({
    title: t('home.communityTitle'),
    content: `${t('home.communityText')}\n${t('home.serviceWechat')}`,
    success: (res) => {
      if (!res.confirm) return
      uni.setClipboardData({
        data: 'jixiangshang',
        success: () => uni.showToast({ title: t('home.communityCopied'), icon: 'none' }),
      })
    },
  })
}
</script>

<template>
  <view class="home-page" :style="{ paddingTop: `${statusBarHeight}px` }">
    <view class="bg-glow bg-glow-a" />
    <view class="bg-glow bg-glow-b" />

    <view class="home-header" :style="capsuleRight ? { paddingRight: `${capsuleRight}px` } : undefined">
      <view class="brand-wrap">
        <text class="brand-text">{{ t('home.brand') }}</text>
        <view class="brand-underline" />
      </view>
      <view class="notice-pill">
        <text class="notice-icon">📢</text>
        <swiper class="notice-swiper" vertical circular autoplay :interval="2800" :duration="450" :disable-touch="true">
          <swiper-item v-for="notice in notices" :key="notice">
            <text class="notice-text">{{ notice }}</text>
          </swiper-item>
        </swiper>
      </view>
      <view class="search-btn" @click="goSearch">
        <uni-icons type="search" size="21" color="#f4f8f2" />
      </view>
    </view>

    <HorizontalScroll class="category-scroll">
      <view class="category-inner">
        <view
          v-for="cat in categories"
          :key="cat.key"
          class="category-item"
          :class="{ active: activeCategory === cat.key }"
          @click="onCategoryTap(cat.key)"
        >
          <text>{{ cat.label }}</text>
          <view v-if="activeCategory === cat.key" class="cat-underline" />
        </view>
      </view>
    </HorizontalScroll>

    <view class="hero-wrap">
      <swiper
        class="hero-swiper"
        circular
        autoplay
        :interval="4200"
        :duration="500"
        @change="onBannerChange"
      >
        <swiper-item v-for="slide in bannerSlides" :key="slide.key">
          <view class="hero-card" @click="goBanner(slide)">
            <image class="hero-image" :src="slide.art" mode="aspectFill" />
            <view class="hero-shade" />
            <view class="hero-copy">
              <text class="hero-tag">{{ t('home.newBox') }}</text>
              <text class="hero-title">{{ slide.title }}</text>
              <text v-if="slide.price" class="hero-price">¥{{ slide.price }}</text>
            </view>
            <view class="hero-line" />
          </view>
        </swiper-item>
      </swiper>
      <view class="hero-dots">
        <view
          v-for="(slide, index) in bannerSlides"
          :key="slide.key"
          class="hero-dot"
          :class="{ active: index === bannerIndex }"
        />
      </view>
    </view>

    <view class="quick-row">
      <view v-for="entry in quickEntries" :key="entry.key" class="quick-item" @click="onQuickTap(entry.key)">
        <view class="quick-icon" :style="{ background: entry.bg }">{{ entry.emoji }}</view>
        <text class="quick-label">{{ entry.label }}</text>
      </view>
    </view>

    <view v-if="loading" class="status-box">
      <text class="status-text">{{ t('common.loading') }}</text>
    </view>

    <template v-else>
      <view v-if="showLucky && luckyCards.length" class="section-card lucky-section">
        <view class="section-head">
          <text class="section-title">{{ t('home.luckySection') }}</text>
          <view class="more-link" @click="goMore('lucky')">
            <text>{{ t('home.viewMore') }}</text>
            <text class="more-arrow">→</text>
          </view>
        </view>
        <HorizontalScroll class="box-scroll">
          <view class="box-row">
            <view v-for="card in luckyCards" :key="card.key" class="box-card" @click="goCard(card)">
              <view class="box-art-wrap">
                <image class="box-art" :src="card.image" mode="aspectFill" />
                <view class="new-badge">{{ t('home.newBox') }}</view>
                <view v-if="card.soldOut" class="sold-out">{{ t('home.soldOut') }}</view>
              </view>
              <text class="box-title">{{ card.name }}</text>
              <text class="box-price">¥{{ card.price }}</text>
            </view>
          </view>
        </HorizontalScroll>
      </view>

      <view v-if="showInfinite && infiniteCards.length" class="section-card infinite-section">
        <view class="section-head">
          <text class="section-title">{{ t('home.infiniteSection') }}</text>
          <view class="more-link" @click="goMore('infinite')">
            <text>{{ t('home.viewMore') }}</text>
            <text class="more-arrow">→</text>
          </view>
        </view>
        <HorizontalScroll class="box-scroll">
          <view class="box-row">
            <view v-for="card in infiniteCards" :key="card.key" class="box-card" @click="goCard(card)">
              <view class="box-art-wrap">
                <image class="box-art" :src="card.image" mode="aspectFill" />
                <view class="new-badge">{{ t('home.newBox') }}</view>
                <view v-if="card.soldOut" class="sold-out">{{ t('home.soldOut') }}</view>
              </view>
              <text class="box-title">{{ card.name }}</text>
              <text class="box-price">¥{{ card.price }}</text>
            </view>
          </view>
        </HorizontalScroll>
      </view>

      <view v-if="!luckyCards.length && !infiniteCards.length" class="status-box">
        <text class="status-text">{{ t('home.empty') }}</text>
      </view>
    </template>

    <BottomTabBar active="home" />
  </view>
</template>

<style scoped lang="scss">
page {
  background: #050706;
}

.home-page {
  min-height: 100vh;
  padding-bottom: 190rpx;
  background:
    radial-gradient(circle at 22% 0%, rgba(107, 255, 52, 0.16), transparent 36%),
    radial-gradient(circle at 95% 18%, rgba(25, 120, 72, 0.16), transparent 34%),
    repeating-linear-gradient(118deg, rgba(255, 255, 255, 0.012) 0 2rpx, transparent 2rpx 96rpx),
    #050706;
  overflow: hidden;
}

.bg-glow {
  position: absolute;
  border-radius: 50%;
  filter: blur(80rpx);
  pointer-events: none;
}

.bg-glow-a {
  width: 420rpx;
  height: 420rpx;
  top: -180rpx;
  left: -120rpx;
  background: rgba(131, 255, 40, 0.12);
}

.bg-glow-b {
  width: 360rpx;
  height: 360rpx;
  top: 260rpx;
  right: -160rpx;
  background: rgba(22, 180, 110, 0.1);
}

.home-header {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: center;
  gap: 18rpx;
  padding: 16rpx 26rpx 0;
}

.brand-wrap {
  position: relative;
  flex-shrink: 0;
  padding-right: 8rpx;
}

.brand-text {
  font-size: 48rpx;
  line-height: 1;
  font-weight: 900;
  font-style: italic;
  letter-spacing: -2rpx;
  color: #b6ff3b;
  text-shadow: 0 0 22rpx rgba(182, 255, 59, 0.3);
}

.brand-underline {
  width: 86rpx;
  height: 10rpx;
  margin: 2rpx 0 0 2rpx;
  border-radius: 999rpx;
  background: #b6ff3b;
  transform: rotate(-2deg);
  box-shadow: 0 0 18rpx rgba(182, 255, 59, 0.5);
}

.notice-pill {
  flex: 1;
  min-width: 0;
  height: 62rpx;
  display: flex;
  align-items: center;
  gap: 8rpx;
  padding: 0 18rpx;
  border: 3rpx solid #b6ff3b;
  border-radius: 999rpx;
  background: rgba(7, 12, 9, 0.86);
  box-shadow: inset 0 0 20rpx rgba(182, 255, 59, 0.08);
}

.notice-icon {
  font-size: 24rpx;
  flex-shrink: 0;
}

.notice-swiper {
  flex: 1;
  min-width: 0;
  height: 40rpx;
}

.notice-swiper swiper-item {
  display: flex;
  align-items: center;
}

.notice-text {
  display: block;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  font-size: 22rpx;
  color: #f0f5ed;
}

.search-btn {
  width: 68rpx;
  height: 68rpx;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba(30, 38, 31, 0.86);
  border: 2rpx solid rgba(255, 255, 255, 0.18);
}

.category-scroll {
  position: relative;
  z-index: 2;
  margin-top: 22rpx;
  width: 100%;
}

.category-inner {
  display: flex;
  align-items: flex-end;
  gap: 38rpx;
  padding: 0 28rpx 16rpx;
  white-space: nowrap;
}

.category-item {
  position: relative;
  flex-shrink: 0;
  padding-bottom: 10rpx;
  font-size: 31rpx;
  font-weight: 600;
  color: #e6ebe3;
}

.category-item.active {
  color: #b6ff3b;
  font-size: 34rpx;
  font-weight: 800;
}

.cat-underline {
  position: absolute;
  left: 8rpx;
  right: 8rpx;
  bottom: 0;
  height: 8rpx;
  border-radius: 999rpx;
  background: #b6ff3b;
  transform: rotate(-3deg);
  box-shadow: 0 0 16rpx rgba(182, 255, 59, 0.5);
}

.hero-wrap {
  position: relative;
  z-index: 2;
  margin: 10rpx 26rpx 0;
}

.hero-swiper {
  height: 342rpx;
  border-radius: 30rpx;
  overflow: hidden;
}

.hero-card {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
  border-radius: 30rpx;
  background: #0b120d;
}

.hero-image {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  width: 100%;
  height: 100%;
}

.hero-shade {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  background: linear-gradient(180deg, rgba(0, 0, 0, 0.08) 20%, rgba(0, 0, 0, 0.68) 100%);
}

.hero-copy {
  position: absolute;
  left: 28rpx;
  right: 28rpx;
  bottom: 28rpx;
  display: flex;
  flex-direction: column;
  gap: 8rpx;
}

.hero-tag {
  align-self: flex-start;
  padding: 2rpx 14rpx;
  border-radius: 999rpx;
  background: #b6ff3b;
  color: #0a0e0b;
  font-size: 20rpx;
  font-weight: 700;
}

.hero-title {
  font-size: 36rpx;
  font-weight: 800;
  color: #fff;
  text-shadow: 0 2rpx 14rpx rgba(0, 0, 0, 0.45);
}

.hero-price {
  font-size: 30rpx;
  font-weight: 800;
  color: #b6ff3b;
}

.hero-line {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 8rpx;
  background: linear-gradient(90deg, #b6ff3b, #4dff9b, #b6ff3b);
  box-shadow: 0 0 24rpx rgba(182, 255, 59, 0.6);
}

.hero-dots {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 18rpx;
  display: flex;
  justify-content: center;
  gap: 8rpx;
  z-index: 2;
}

.hero-dot {
  width: 8rpx;
  height: 8rpx;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.22);
  transition: width 0.2s ease;
}

.hero-dot.active {
  width: 22rpx;
  border-radius: 999rpx;
  background: #b6ff3b;
}

.quick-row {
  position: relative;
  z-index: 2;
  display: flex;
  justify-content: space-between;
  padding: 26rpx 30rpx 8rpx;
}

.quick-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10rpx;
  width: 20%;
}

.quick-icon {
  width: 92rpx;
  height: 92rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 28rpx;
  font-size: 48rpx;
  box-shadow: inset 0 -10rpx 18rpx rgba(0, 0, 0, 0.12), 0 8rpx 22rpx rgba(0, 0, 0, 0.35);
}

.quick-label {
  font-size: 22rpx;
  color: #e2e8de;
  white-space: nowrap;
}

.section-card {
  position: relative;
  z-index: 2;
  margin: 22rpx 24rpx 0;
  padding: 24rpx 4rpx 24rpx 22rpx;
  border-radius: 28rpx;
  background: linear-gradient(160deg, rgba(28, 36, 29, 0.94), rgba(12, 17, 13, 0.96));
  border: 1rpx solid rgba(182, 255, 59, 0.08);
  box-shadow: 0 16rpx 40rpx rgba(0, 0, 0, 0.28);
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-right: 20rpx;
}

.section-title {
  font-size: 34rpx;
  font-weight: 800;
  color: #f4f8f2;
  letter-spacing: 1rpx;
}

.more-link {
  display: flex;
  align-items: center;
  gap: 6rpx;
  font-size: 23rpx;
  color: #c8d1c4;
}

.more-arrow {
  color: #b6ff3b;
  font-weight: 700;
}

.box-scroll {
  margin-top: 20rpx;
  width: 100%;
}

.box-row {
  display: flex;
  gap: 18rpx;
}

.box-card {
  width: 222rpx;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
}

.box-art-wrap {
  position: relative;
  width: 222rpx;
  height: 210rpx;
  border-radius: 20rpx;
  overflow: hidden;
  background: #101711;
}

.box-art {
  width: 100%;
  height: 100%;
}

.new-badge {
  position: absolute;
  top: 0;
  right: 0;
  padding: 4rpx 12rpx;
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

.box-title {
  margin-top: 12rpx;
  font-size: 25rpx;
  line-height: 34rpx;
  color: #f1f5ee;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 68rpx;
}

.box-price {
  margin-top: 4rpx;
  font-size: 30rpx;
  font-weight: 800;
  color: #b6ff3b;
}

.status-box {
  padding: 80rpx 0;
  text-align: center;
}

.status-text {
  color: #7d867a;
  font-size: 26rpx;
}
</style>

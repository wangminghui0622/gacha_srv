<script setup lang="ts">
import { computed, ref } from 'vue'
import { onPullDownRefresh, onShow } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import BottomTabBar from '@/components/BottomTabBar.vue'
import * as userApi from '@/services/user'
import { useAuthStore } from '@/store/authStore'
import { formatMoney } from '@/utils/format'

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
const loading = ref(false)

const displayName = computed(() => auth.user?.nickname || auth.user?.username || '-')
const publicId = computed(() => auth.user?.publicUserId || (auth.user?.userId ? String(auth.user.userId) : '-'))
const balanceText = computed(() => formatMoney(auth.balance))
const avatar = '/static/art/avatar.png'

async function load() {
  loading.value = true
  try {
    auth.setUser(await userApi.getProfile())
  } catch {
    // 401 由请求适配器统一处理
  } finally {
    loading.value = false
  }
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

function goRecords() {
  uni.navigateTo({ url: '/pages/records/index' })
}

function goWarehouse() {
  uni.switchTab({
    url: '/pages/warehouse/index',
    fail: () => uni.reLaunch({ url: '/pages/warehouse/index' }),
  })
}

function goRecharge() {
  uni.navigateTo({ url: '/pages/recharge/index' })
}

function goCoupons() {
  uni.navigateTo({ url: '/pages/coupons/index' })
}

function goCampaigns() {
  uni.navigateTo({ url: '/pages/campaigns/index' })
}

function showAssets() {
  uni.showModal({
    title: t('mine.assetsTitle'),
    content: `${t('mine.balance')}：${balanceText.value}`,
    confirmText: t('mine.recharge'),
    cancelText: t('common.cancel'),
    success: (res) => {
      if (res.confirm) goRecharge()
    },
  })
}

function showEditNickname() {
  uni.showModal({
    title: t('mine.editNickname'),
    content: t('mine.nicknameComing'),
    showCancel: false,
  })
}

function showOrders() {
  uni.showModal({
    title: t('mine.orders'),
    content: t('mine.ordersComing'),
    showCancel: false,
  })
}

function showAddress() {
  uni.showModal({
    title: t('mine.address'),
    content: t('mine.addressComing'),
    showCancel: false,
  })
}

function showService() {
  uni.showModal({
    title: t('mine.serviceTitle'),
    content: t('mine.serviceText'),
    showCancel: false,
  })
}

function showAbout() {
  uni.showModal({
    title: t('mine.aboutTitle'),
    content: t('mine.aboutText'),
    showCancel: false,
  })
}

function showInvite() {
  const content = t('mine.inviteText', { id: publicId.value })
  uni.showModal({
    title: t('mine.inviteTitle'),
    content,
    confirmText: t('mine.copy'),
    cancelText: t('common.cancel'),
    success: (res) => {
      if (!res.confirm) return
      uni.setClipboardData({
        data: content,
        success: () => uni.showToast({ title: t('mine.copied'), icon: 'none' }),
      })
    },
  })
}

function onLogout() {
  uni.showModal({
    title: t('mine.logout'),
    content: t('mine.logoutConfirm'),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    success: (res) => {
      if (!res.confirm) return
      auth.clearAuth()
      uni.reLaunch({ url: '/pages/login/index' })
    },
  })
}
</script>

<template>
  <view class="mine-page" :style="{ paddingTop: `${statusBarHeight}px` }">
    <view class="top-glow" />
    <view class="stripe-glow" />

    <view class="profile-row" :style="capsuleRight ? { paddingRight: `${capsuleRight}px` } : undefined">
      <view class="avatar-ring">
        <image class="avatar-img" :src="avatar" mode="aspectFill" />
      </view>
      <view class="profile-copy">
        <view class="name-row">
          <text class="name-text">{{ displayName }}</text>
          <view class="edit-btn" @click="showEditNickname">
            <uni-icons type="compose" size="18" color="#c9d2c5" />
          </view>
        </view>
        <text class="id-text">{{ t('mine.idLabel') }}：{{ publicId }}</text>
      </view>
    </view>

    <view class="asset-card" @click="showAssets">
      <view class="asset-inner">
        <view class="asset-icon">💰</view>
        <view class="asset-copy">
          <text class="asset-title">{{ t('mine.assets') }}</text>
          <text class="asset-balance">{{ balanceText }}</text>
        </view>
        <view class="asset-link">
          <text>{{ t('mine.viewDetail') }}</text>
          <text class="asset-arrow">›</text>
        </view>
      </view>
      <view class="asset-shine" />
    </view>

    <view class="action-row">
      <view class="action-card records-card" @click="goRecords">
        <text class="action-title">{{ t('mine.records') }}</text>
        <view class="action-art">🧾</view>
        <view class="action-arrow-circle">›</view>
      </view>
      <view class="action-card orders-card" @click="showOrders">
        <text class="action-title">{{ t('mine.orders') }}</text>
        <view class="action-art">📄</view>
        <view class="action-arrow-circle">›</view>
      </view>
    </view>

    <view class="menu-card">
      <view class="menu-row" @click="showAddress">
        <uni-icons type="location" size="21" color="#f0f5ed" />
        <text class="menu-label">{{ t('mine.address') }}</text>
        <uni-icons type="right" size="15" color="#626b5f" />
      </view>
      <!-- #ifdef MP-WEIXIN -->
      <button class="menu-row menu-btn" open-type="contact">
        <uni-icons type="phone" size="21" color="#f0f5ed" />
        <text class="menu-label">{{ t('mine.service') }}</text>
        <uni-icons type="right" size="15" color="#626b5f" />
      </button>
      <!-- #endif -->
      <!-- #ifndef MP-WEIXIN -->
      <view class="menu-row" @click="showService">
        <uni-icons type="phone" size="21" color="#f0f5ed" />
        <text class="menu-label">{{ t('mine.service') }}</text>
        <uni-icons type="right" size="15" color="#626b5f" />
      </view>
      <!-- #endif -->
      <view class="menu-row" @click="goCoupons">
        <uni-icons type="wallet" size="21" color="#f0f5ed" />
        <text class="menu-label">{{ t('mine.wallet') }}</text>
        <uni-icons type="right" size="15" color="#626b5f" />
      </view>
      <view class="menu-row" @click="showAbout">
        <uni-icons type="info" size="21" color="#f0f5ed" />
        <text class="menu-label">{{ t('mine.about') }}</text>
        <uni-icons type="right" size="15" color="#626b5f" />
      </view>
      <view class="menu-row" @click="goCampaigns">
        <uni-icons type="gift" size="21" color="#f0f5ed" />
        <text class="menu-label">{{ t('mine.benefits') }}</text>
        <uni-icons type="right" size="15" color="#626b5f" />
      </view>
      <view class="menu-row" @click="showInvite">
        <uni-icons type="sound" size="21" color="#f0f5ed" />
        <text class="menu-label">{{ t('mine.invite') }}</text>
        <uni-icons type="right" size="15" color="#626b5f" />
      </view>
      <view class="menu-row" @click="onLogout">
        <uni-icons type="undo" size="21" color="#f0f5ed" />
        <text class="menu-label">{{ t('mine.logout') }}</text>
        <uni-icons type="right" size="15" color="#626b5f" />
      </view>
    </view>

    <BottomTabBar active="mine" />
  </view>
</template>

<style scoped lang="scss">
page {
  background: #050706;
}

.mine-page {
  position: relative;
  min-height: 100vh;
  padding-bottom: 190rpx;
  background:
    radial-gradient(circle at 18% 0%, rgba(146, 255, 48, 0.12), transparent 36%),
    repeating-linear-gradient(118deg, rgba(255, 255, 255, 0.012) 0 2rpx, transparent 2rpx 96rpx),
    #050706;
  overflow: hidden;
}

.top-glow {
  position: absolute;
  width: 620rpx;
  height: 620rpx;
  top: -280rpx;
  left: 50%;
  transform: translateX(-50%);
  border-radius: 50%;
  background: rgba(126, 220, 42, 0.1);
  filter: blur(110rpx);
  pointer-events: none;
}

.stripe-glow {
  position: absolute;
  top: 0;
  right: -180rpx;
  width: 600rpx;
  height: 420rpx;
  background: repeating-linear-gradient(105deg, rgba(182, 255, 59, 0.04) 0 2rpx, transparent 2rpx 44rpx);
  transform: rotate(8deg);
  pointer-events: none;
}

.profile-row {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: center;
  gap: 26rpx;
  padding: 34rpx 32rpx 18rpx;
}

.avatar-ring {
  width: 154rpx;
  height: 154rpx;
  padding: 5rpx;
  border-radius: 50%;
  background: linear-gradient(145deg, #b6ff3b, rgba(182, 255, 59, 0.08) 70%);
  box-shadow: 0 0 30rpx rgba(182, 255, 59, 0.22);
  flex-shrink: 0;
}

.avatar-img {
  width: 100%;
  height: 100%;
  border-radius: 50%;
  border: 4rpx solid #0b0f0c;
}

.profile-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 12rpx;
}

.name-row {
  display: flex;
  align-items: center;
  gap: 12rpx;
}

.name-text {
  max-width: 420rpx;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  font-size: 40rpx;
  font-weight: 800;
  color: #f6faf3;
}

.edit-btn {
  width: 44rpx;
  height: 44rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.08);
}

.id-text {
  font-size: 24rpx;
  color: #9da797;
}

.asset-card {
  position: relative;
  z-index: 2;
  margin: 12rpx 28rpx 0;
  padding: 3rpx;
  border-radius: 26rpx;
  background: linear-gradient(120deg, rgba(182, 255, 59, 0.9), rgba(255, 214, 64, 0.55), rgba(182, 255, 59, 0.2));
  box-shadow: 0 14rpx 38rpx rgba(0, 0, 0, 0.35);
  overflow: hidden;
}

.asset-inner {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  gap: 18rpx;
  padding: 24rpx 24rpx;
  border-radius: 23rpx;
  background:
    radial-gradient(circle at 18% 30%, rgba(255, 214, 64, 0.2), transparent 38%),
    linear-gradient(120deg, #1c2418, #0d130e 72%);
}

.asset-icon {
  width: 70rpx;
  height: 70rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 22rpx;
  background: linear-gradient(145deg, #ffe27a, #ffb52e);
  font-size: 36rpx;
  box-shadow: 0 0 24rpx rgba(255, 196, 60, 0.3);
}

.asset-copy {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6rpx;
}

.asset-title {
  font-size: 31rpx;
  font-weight: 800;
  color: #f6f3d9;
  letter-spacing: 2rpx;
}

.asset-balance {
  font-size: 26rpx;
  font-weight: 700;
  color: #d7ff6a;
}

.asset-link {
  display: flex;
  align-items: center;
  gap: 6rpx;
  padding: 8rpx 14rpx;
  border-radius: 999rpx;
  border: 1rpx solid rgba(255, 255, 255, 0.22);
  font-size: 21rpx;
  color: #e9efdc;
}

.asset-arrow {
  font-size: 28rpx;
  line-height: 1;
}

.asset-shine {
  position: absolute;
  top: -60rpx;
  right: -20rpx;
  width: 220rpx;
  height: 220rpx;
  border-radius: 50%;
  background: rgba(255, 224, 120, 0.16);
  filter: blur(30rpx);
}

.action-row {
  position: relative;
  z-index: 2;
  display: flex;
  gap: 18rpx;
  margin: 22rpx 28rpx 0;
}

.action-card {
  position: relative;
  flex: 1;
  height: 188rpx;
  padding: 22rpx;
  border-radius: 26rpx;
  overflow: hidden;
  box-sizing: border-box;
}

.records-card {
  background: linear-gradient(135deg, #ffe54d, #ffc51c);
}

.orders-card {
  background: linear-gradient(135deg, #c8ff66, #8eea2c);
}

.action-title {
  font-size: 32rpx;
  font-weight: 800;
  color: #16200f;
}

.action-art {
  position: absolute;
  right: 16rpx;
  bottom: 4rpx;
  font-size: 100rpx;
  transform: rotate(-6deg);
  opacity: 0.9;
}

.action-arrow-circle {
  position: absolute;
  left: 22rpx;
  bottom: 22rpx;
  width: 46rpx;
  height: 46rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba(10, 14, 8, 0.86);
  color: #fff;
  font-size: 32rpx;
  line-height: 1;
}

.menu-card {
  position: relative;
  z-index: 2;
  margin: 28rpx 28rpx 0;
}

.menu-row {
  display: flex;
  align-items: center;
  gap: 22rpx;
  padding: 30rpx 4rpx;
  border-bottom: 1rpx solid rgba(255, 255, 255, 0.06);
}

.menu-btn {
  width: 100%;
  margin: 0;
  background: transparent;
  border: none;
  border-radius: 0;
  text-align: left;
  line-height: normal;
  color: inherit;
  font-size: inherit;
}

.menu-btn::after {
  border: none;
}

.menu-row:last-child {
  border-bottom: none;
}

.menu-label {
  flex: 1;
  font-size: 30rpx;
  font-weight: 600;
  color: #eef3ea;
}
</style>

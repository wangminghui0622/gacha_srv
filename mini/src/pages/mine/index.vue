<script setup lang="ts">
import { ref } from 'vue'
import { onShow, onPullDownRefresh } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as userApi from '@/services/user'
import { useAuthStore } from '@/store/authStore'
import { formatMoney } from '@/utils/format'

// 我的（tab）：资料卡 + 余额 + 充值 + 功能入口（web Profile 页语义）

const { t } = useI18n()
const auth = useAuthStore()

const loading = ref(true)

async function load() {
  loading.value = true
  try {
    auth.setUser(await userApi.getProfile())
  } catch {
    // 401 由响应适配器统一处理
  } finally {
    loading.value = false
  }
}

onShow(load)
onPullDownRefresh(async () => {
  await load()
  uni.stopPullDownRefresh()
})

function onLogout() {
  uni.showModal({
    title: t('user.logoutConfirm'),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    success: (res) => {
      if (res.confirm) {
        auth.clearAuth()
        uni.reLaunch({ url: '/pages/login/index' })
      }
    },
  })
}

function goRecharge() {
  uni.navigateTo({ url: '/pages/recharge/index' })
}

function goRecords() {
  uni.navigateTo({ url: '/pages/records/index' })
}

function goWarehouse() {
  uni.navigateTo({ url: '/pages/warehouse/index' })
}

function onCouponsTap() {
  uni.navigateTo({ url: '/pages/coupons/index' })
}
</script>

<template>
  <view>
    <view class="page-title">{{ t('user.title') }}</view>

    <view v-if="loading" class="list-empty">{{ t('common.loading') }}</view>
    <template v-else>
      <view v-if="auth.user" class="gacha-card profile-card">
        <view class="avatar">{{ (auth.user.nickname || auth.user.username).slice(0, 1).toUpperCase() }}</view>
        <view class="profile-info">
          <view class="nickname">{{ auth.user.nickname || auth.user.username }}</view>
          <view class="username">{{ t('user.username') }}：{{ auth.user.username }}</view>
        </view>
      </view>

      <view class="gacha-card balance-card">
        <view class="balance-label">{{ t('user.balance') }}</view>
        <view class="balance-value money">{{ formatMoney(auth.balance) }}</view>
        <view class="btn-primary" @click="goRecharge">{{ t('user.recharge') }}</view>
      </view>

      <view class="gacha-card entry-card">
        <view class="entry-row" @click="goRecords">
          <text class="entry-icon">🧾</text>
          <text class="entry-label">{{ t('nav.records') }}</text>
          <text class="entry-arrow">›</text>
        </view>
        <view class="entry-row" @click="goWarehouse">
          <text class="entry-icon">📦</text>
          <text class="entry-label">{{ t('nav.warehouse') }}</text>
          <text class="entry-arrow">›</text>
        </view>
        <view class="entry-row" @click="onCouponsTap">
          <text class="entry-icon">🎟️</text>
          <text class="entry-label">{{ t('coupon.title') }}</text>
          <text class="entry-arrow">›</text>
        </view>
      </view>

      <view class="logout-btn" @click="onLogout">{{ t('user.logout') }}</view>
    </template>
  </view>
</template>

<style scoped lang="scss">
.list-empty {
  text-align: center;
  color: var(--color-text-secondary);
  padding: 48rpx 0;
}

.profile-card {
  display: flex;
  align-items: center;
  gap: 24rpx;

  .avatar {
    width: 112rpx;
    height: 112rpx;
    border-radius: 50%;
    background: var(--color-primary);
    color: #fff;
    font-size: 44rpx;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .profile-info {
    .nickname {
      font-size: 36rpx;
      font-weight: 600;
    }

    .username {
      font-size: 24rpx;
      color: var(--color-text-secondary);
    }
  }
}

.balance-card {
  .balance-label {
    font-size: 26rpx;
    color: var(--color-text-secondary);
  }

  .balance-value {
    font-size: 64rpx;
    font-weight: 700;
    margin-bottom: 24rpx;
  }
}

.entry-card {
  padding: 8rpx 28rpx;

  .entry-row {
    display: flex;
    align-items: center;
    gap: 16rpx;
    padding: 24rpx 0;
    border-bottom: 1rpx solid var(--color-border);

    &:last-child {
      border-bottom: none;
    }

    .entry-icon {
      font-size: 36rpx;
    }

    .entry-label {
      flex: 1;
      font-size: 30rpx;
    }

    .entry-arrow {
      color: var(--color-text-secondary);
    }
  }
}

.logout-btn {
  text-align: center;
  color: var(--color-danger);
  font-size: 30rpx;
  padding: 32rpx 0 48rpx;
}
</style>

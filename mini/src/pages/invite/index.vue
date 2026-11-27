<script setup lang="ts">
import { computed, ref } from 'vue'
import { onShareAppMessage, onShow } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as userApi from '@/services/user'
import { useAuthStore } from '@/store/authStore'

const { t } = useI18n()
const auth = useAuthStore()

const sys = uni.getSystemInfoSync()
const statusBarHeight = ref(sys.statusBarHeight || 0)

const invitedCount = ref(0)
const rewardPoints = ref(0)
const records = ref<Array<{ id: number; nickname: string; createdAt: number; reward: number }>>([])

const inviteCode = computed(() => {
  const userId = Number(auth.user?.userId || 0)
  const raw = (auth.user?.publicUserId || '').replace(/^u/, '')
  // 后端邀请体系接入前，用 publicId + userId base36 后缀生成稳定展示码；
  // 接入后改为后端下发的真实邀请码。
  const suffix = userId.toString(36).toUpperCase().padStart(4, '0')
  const base = raw || (userId ? String(userId) : 'JIXIANG')
  const code = `${base}${suffix}`
  return code.length >= 12 ? code.slice(0, 16) : code.padEnd(12, suffix)
})

async function loadProfile() {
  if (auth.user) return
  try {
    auth.setUser(await userApi.getProfile())
  } catch {
    // 401 由请求适配器统一处理
  }
}

onShow(() => {
  void loadProfile()
})

function toast(message: string) {
  uni.showToast({ title: message, icon: 'none' })
}

function copyCode() {
  uni.setClipboardData({
    data: inviteCode.value,
    success: () => toast(t('invite.copied')),
  })
}

function copyInvite() {
  uni.setClipboardData({
    data: t('invite.shareText', { code: inviteCode.value }),
    success: () => toast(t('invite.copied')),
  })
}

function showRules() {
  uni.showModal({
    title: t('invite.rulesTitle'),
    content: t('invite.rulesText'),
    showCancel: false,
  })
}

function goBack() {
  uni.navigateBack({
    fail: () => uni.reLaunch({ url: '/pages/mine/index' }),
  })
}

// #ifdef MP-WEIXIN
onShareAppMessage(() => ({
  title: t('invite.shareTitle'),
  path: `/pages/login/index?inviteCode=${inviteCode.value}`,
}))
// #endif
</script>

<template>
  <view class="invite-root">
    <view class="invite-page" :style="{ paddingTop: `${statusBarHeight}px` }">
      <view class="hero">
        <image class="hero-art" src="/static/art/hero-2.png" mode="aspectFill" />
        <view class="hero-shade" />
        <view class="nav-row">
          <view class="back-btn" @click="goBack">
            <uni-icons type="left" size="22" color="#ffffff" />
          </view>
          <view class="rules-pill" @click="showRules">{{ t('invite.rules') }}</view>
        </view>
        <view class="hero-copy">
          <text class="hero-title">{{ t('invite.title') }}</text>
          <text class="hero-subtitle">{{ t('invite.subtitle') }}</text>
          <view class="hero-underline" />
        </view>
      </view>

      <view class="main-card">
        <view class="card-heading">
          <text class="card-title">{{ t('invite.myQr') }}</text>
          <view class="card-underline" />
        </view>

        <view class="qr-frame">
          <view class="qr-corner tl" />
          <view class="qr-corner tr" />
          <view class="qr-corner bl" />
          <view class="qr-corner br" />
          <view class="qr-inner">
            <image class="qr-img" src="/static/art/invite-qr.png" mode="aspectFit" />
            <view class="qr-logo">吉</view>
            <view class="qr-mini">
              <uni-icons type="weixin" size="18" color="#ffffff" />
            </view>
          </view>
        </view>

        <view class="code-bar">
          <text class="code-label">{{ t('invite.myCode') }}：</text>
          <text class="code-value">{{ inviteCode }}</text>
          <view class="copy-btn" @click="copyCode">
            <view class="copy-icon">
              <view class="copy-sheet copy-sheet-back" />
              <view class="copy-sheet copy-sheet-front" />
            </view>
            <text>{{ t('invite.copy') }}</text>
          </view>
        </view>

        <view class="stats">
          <view class="stat">
            <text class="stat-label">{{ t('invite.invitedCount') }}</text>
            <text class="stat-value">{{ invitedCount }}</text>
          </view>
          <view class="stat-divider" />
          <view class="stat">
            <text class="stat-label">{{ t('invite.rewardPoints') }}</text>
            <text class="stat-value">{{ rewardPoints }}</text>
          </view>
        </view>
        <text class="reward-note">{{ t('invite.rewardNote') }}</text>
      </view>

      <view class="flow-card">
        <view class="card-heading">
          <text class="card-title">{{ t('invite.flow') }}</text>
          <view class="card-underline" />
        </view>
        <view class="flow-row">
          <view class="flow-step">
            <view class="flow-icon flow-icon-pink">
              <uni-icons type="personadd" size="26" color="#ffffff" />
            </view>
            <text class="flow-label">{{ t('invite.flowShare') }}</text>
          </view>
          <text class="flow-arrow">▶</text>
          <view class="flow-step">
            <view class="flow-icon flow-icon-cyan">
              <uni-icons type="redo" size="26" color="#ffffff" />
            </view>
            <text class="flow-label">{{ t('invite.flowLogin') }}</text>
          </view>
          <text class="flow-arrow">▶</text>
          <view class="flow-step">
            <view class="flow-icon flow-icon-gold">
              <uni-icons type="gift" size="26" color="#ffffff" />
            </view>
            <text class="flow-label">{{ t('invite.flowReward') }}</text>
          </view>
        </view>
      </view>

      <view class="records-card">
        <view class="card-heading">
          <text class="card-title">{{ t('invite.records') }}</text>
          <view class="card-underline" />
        </view>
        <view v-if="records.length" class="records-list">
          <view v-for="record in records" :key="record.id" class="record-row">
            <text class="record-name">{{ record.nickname }}</text>
            <text class="record-reward">+{{ record.reward }}</text>
          </view>
        </view>
        <view v-else class="records-empty">
          <text class="records-emoji">🗒️</text>
          <text class="records-empty-text">{{ t('invite.recordsEmpty') }}</text>
        </view>
      </view>

      <view class="page-bottom-space" />
    </view>

    <view class="invite-bottom">
      <!-- #ifdef MP-WEIXIN -->
      <button class="invite-btn" open-type="share">{{ t('invite.inviteBtn') }}</button>
      <!-- #endif -->
      <!-- #ifndef MP-WEIXIN -->
      <view class="invite-btn" @click="copyInvite">{{ t('invite.inviteBtn') }}</view>
      <!-- #endif -->
    </view>
  </view>
</template>

<style scoped lang="scss">
page {
  background: #1a083c;
}

.invite-root {
  min-height: 100vh;
  background: #1a083c;
}

.invite-page {
  position: relative;
  min-height: 100vh;
  padding-bottom: 240rpx;
  background:
    radial-gradient(circle at 50% 0%, rgba(126, 53, 255, 0.34), transparent 46%),
    repeating-linear-gradient(0deg, rgba(170, 113, 255, 0.07) 0 1rpx, transparent 1rpx 64rpx),
    repeating-linear-gradient(90deg, rgba(170, 113, 255, 0.07) 0 1rpx, transparent 1rpx 64rpx),
    #1a083c;
}

.hero {
  position: relative;
  height: 430rpx;
  overflow: hidden;
}

.hero-art {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  opacity: 0.72;
}

.hero-shade {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  background:
    linear-gradient(180deg, rgba(26, 8, 60, 0.12) 0%, rgba(26, 8, 60, 0.35) 55%, #1a083c 100%),
    radial-gradient(circle at 22% 35%, rgba(255, 79, 154, 0.22), transparent 42%);
}

.nav-row {
  position: absolute;
  top: 18rpx;
  left: 28rpx;
  right: 28rpx;
  z-index: 3;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.back-btn {
  width: 64rpx;
  height: 64rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba(20, 8, 50, 0.5);
  border: 2rpx solid rgba(255, 255, 255, 0.24);
}

.rules-pill {
  padding: 10rpx 22rpx;
  border-radius: 999rpx;
  border: 2rpx solid rgba(255, 255, 255, 0.42);
  background: rgba(61, 24, 120, 0.62);
  color: #ffffff;
  font-size: 22rpx;
}

.hero-copy {
  position: absolute;
  left: 34rpx;
  right: 34rpx;
  bottom: 34rpx;
  z-index: 2;
  display: flex;
  flex-direction: column;
}

.hero-title {
  font-size: 66rpx;
  line-height: 1.12;
  font-weight: 900;
  font-style: italic;
  color: #ffffff;
  letter-spacing: 2rpx;
  text-shadow:
    0 0 22rpx rgba(255, 79, 154, 0.75),
    0 6rpx 0 rgba(74, 17, 128, 0.8);
}

.hero-subtitle {
  margin-top: 4rpx;
  font-size: 58rpx;
  line-height: 1.1;
  font-weight: 900;
  font-style: italic;
  color: #6ff7ff;
  letter-spacing: 1rpx;
  text-shadow:
    0 0 24rpx rgba(95, 245, 255, 0.7),
    0 6rpx 0 rgba(71, 20, 130, 0.9);
}

.hero-underline {
  width: 150rpx;
  height: 10rpx;
  margin-top: 12rpx;
  border-radius: 999rpx;
  background: linear-gradient(90deg, #b6ff3b, #6ff7ff);
  transform: rotate(-2deg);
  box-shadow: 0 0 18rpx rgba(182, 255, 59, 0.5);
}

.main-card,
.flow-card,
.records-card {
  position: relative;
  z-index: 2;
  margin: 24rpx 28rpx 0;
  padding: 28rpx 24rpx;
  border-radius: 30rpx;
  background:
    radial-gradient(circle at 50% 0%, rgba(147, 82, 255, 0.28), transparent 46%),
    linear-gradient(160deg, #4f179f, #340b73 65%, #2b0861);
  border: 2rpx solid rgba(255, 96, 178, 0.48);
  box-shadow:
    0 20rpx 50rpx rgba(8, 0, 28, 0.42),
    inset 0 0 42rpx rgba(139, 75, 255, 0.18);
}

.main-card {
  margin-top: -10rpx;
}

.card-heading {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.card-title {
  font-size: 36rpx;
  font-weight: 800;
  color: #6ff7ff;
  letter-spacing: 2rpx;
  text-shadow: 0 0 18rpx rgba(95, 245, 255, 0.5);
}

.card-underline {
  width: 120rpx;
  height: 7rpx;
  margin-top: 8rpx;
  border-radius: 999rpx;
  background: linear-gradient(90deg, #ff4f9a, #ffb04f);
  transform: rotate(-2deg);
}

.qr-frame {
  position: relative;
  width: 372rpx;
  height: 372rpx;
  margin: 30rpx auto 24rpx;
}

.qr-inner {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20rpx;
  border-radius: 22rpx;
  background: #ffffff;
  box-sizing: border-box;
  box-shadow: 0 10rpx 26rpx rgba(10, 0, 30, 0.32);
}

.qr-img {
  width: 100%;
  height: 100%;
}

.qr-corner {
  position: absolute;
  width: 48rpx;
  height: 48rpx;
  border-color: #ff4f9a;
  border-style: solid;
  z-index: 2;
}

.qr-corner.tl {
  top: -12rpx;
  left: -12rpx;
  border-width: 6rpx 0 0 6rpx;
  border-top-left-radius: 18rpx;
}

.qr-corner.tr {
  top: -12rpx;
  right: -12rpx;
  border-width: 6rpx 6rpx 0 0;
  border-top-right-radius: 18rpx;
}

.qr-corner.bl {
  bottom: -12rpx;
  left: -12rpx;
  border-width: 0 0 6rpx 6rpx;
  border-bottom-left-radius: 18rpx;
}

.qr-corner.br {
  bottom: -12rpx;
  right: -12rpx;
  border-width: 0 6rpx 6rpx 0;
  border-bottom-right-radius: 18rpx;
}

.qr-logo {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 88rpx;
  height: 88rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  border: 6rpx solid #ffffff;
  background: linear-gradient(145deg, #b6ff3b, #27c777);
  color: #ffffff;
  font-size: 40rpx;
  font-weight: 900;
  transform: translate(-50%, -50%);
  box-shadow: 0 0 18rpx rgba(43, 208, 122, 0.5);
}

.qr-mini {
  position: absolute;
  right: 10rpx;
  bottom: 10rpx;
  width: 54rpx;
  height: 54rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  border: 4rpx solid #ffffff;
  background: #07c160;
}

.code-bar {
  display: flex;
  align-items: center;
  gap: 4rpx;
  padding: 16rpx 22rpx;
  border-radius: 999rpx;
  background: linear-gradient(90deg, #ff3d7f, #ff7a9f);
  border: 2rpx solid rgba(255, 187, 216, 0.7);
  box-shadow: 0 10rpx 24rpx rgba(255, 45, 110, 0.26);
}

.code-label {
  font-size: 24rpx;
  color: #ffeaf2;
  white-space: nowrap;
}

.code-value {
  flex: 1;
  min-width: 0;
  font-size: 30rpx;
  font-weight: 800;
  color: #ffffff;
  letter-spacing: 1rpx;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.copy-btn {
  display: flex;
  align-items: center;
  gap: 6rpx;
  padding-left: 16rpx;
  border-left: 1rpx solid rgba(255, 255, 255, 0.4);
  font-size: 24rpx;
  color: #ffffff;
  white-space: nowrap;
}

.copy-icon {
  position: relative;
  width: 24rpx;
  height: 24rpx;
  flex-shrink: 0;
}

.copy-sheet {
  position: absolute;
  width: 15rpx;
  height: 18rpx;
  border: 2rpx solid #ffffff;
  border-radius: 3rpx;
  box-sizing: border-box;
}

.copy-sheet-back {
  top: 0;
  left: 0;
  opacity: 0.65;
}

.copy-sheet-front {
  right: 0;
  bottom: 0;
  background: rgba(255, 255, 255, 0.16);
}

.stats {
  display: flex;
  align-items: center;
  margin-top: 26rpx;
  padding: 26rpx 12rpx;
  border-radius: 20rpx;
  background: rgba(20, 7, 52, 0.45);
  border: 1rpx solid rgba(255, 255, 255, 0.08);
}

.stat {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12rpx;
}

.stat-label {
  font-size: 23rpx;
  color: #d9caff;
}

.stat-value {
  font-size: 48rpx;
  line-height: 1;
  font-weight: 800;
  color: #ffffff;
}

.stat-divider {
  width: 1rpx;
  height: 66rpx;
  background: rgba(255, 255, 255, 0.16);
}

.reward-note {
  display: block;
  margin-top: 18rpx;
  text-align: center;
  font-size: 21rpx;
  color: #c8b6ff;
}

.flow-card {
  margin-top: 24rpx;
}

.flow-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-top: 30rpx;
}

.flow-step {
  width: 30%;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14rpx;
}

.flow-icon {
  width: 88rpx;
  height: 88rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  box-shadow: 0 8rpx 20rpx rgba(10, 0, 30, 0.3);
}

.flow-icon-pink {
  background: linear-gradient(145deg, #ff709f, #ff2d68);
}

.flow-icon-cyan {
  background: linear-gradient(145deg, #6ff7ff, #2c9dff);
}

.flow-icon-gold {
  background: linear-gradient(145deg, #ffd05a, #ff7a3c);
}

.flow-label {
  text-align: center;
  font-size: 21rpx;
  line-height: 1.45;
  color: #f4ecff;
  white-space: pre-line;
}

.flow-arrow {
  margin-top: 32rpx;
  font-size: 18rpx;
  color: #ff6fa9;
  text-shadow: 0 0 12rpx rgba(255, 79, 154, 0.5);
}

.records-card {
  margin-top: 24rpx;
}

.records-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12rpx;
  padding: 36rpx 0 20rpx;
}

.records-emoji {
  font-size: 52rpx;
}

.records-empty-text {
  text-align: center;
  font-size: 23rpx;
  line-height: 1.5;
  color: #cbb8ff;
  white-space: pre-line;
}

.records-list {
  margin-top: 20rpx;
}

.record-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18rpx 8rpx;
  border-bottom: 1rpx solid rgba(255, 255, 255, 0.08);
}

.record-name {
  font-size: 26rpx;
  color: #f4ecff;
}

.record-reward {
  font-size: 28rpx;
  font-weight: 700;
  color: #b6ff3b;
}

.page-bottom-space {
  height: 20rpx;
}

.invite-bottom {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 50;
  padding: 18rpx 40rpx;
  padding-bottom: constant(safe-area-inset-bottom);
  padding-bottom: env(safe-area-inset-bottom);
  background: linear-gradient(180deg, rgba(26, 8, 60, 0), rgba(26, 8, 60, 0.92) 32%, #1a083c);
  box-sizing: border-box;
}

.invite-btn {
  width: 100%;
  height: 92rpx;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 999rpx;
  background: linear-gradient(135deg, #ff2d55, #ff6f91);
  color: #ffffff;
  font-size: 32rpx;
  font-weight: 800;
  line-height: normal;
  letter-spacing: 2rpx;
  box-shadow: 0 12rpx 32rpx rgba(255, 45, 85, 0.42);
}

.invite-btn::after {
  border: none;
}
</style>

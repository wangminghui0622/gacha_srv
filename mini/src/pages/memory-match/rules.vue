<template>
  <view class="rule-page">
    <view class="topbar">
      <text class="back" @click="goBack">‹</text>
      <text class="title">{{ t('memoryMatch.rulesTitle') }}</text>
      <text class="placeholder"></text>
    </view>
    <scroll-view scroll-y class="scroll">
      <text class="content">{{ t('memoryMatch.rulesText') }}</text>
    </scroll-view>
  </view>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

function goBack() {
  uni.navigateBack({ delta: 1 })
}
</script>

<style scoped lang="scss">
.rule-page {
  min-height: 100vh;
  background: #111612;
  color: #d8d8d8;
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: calc(env(safe-area-inset-top) + 28rpx) 32rpx 24rpx;
  background: linear-gradient(180deg, #0b0f0c 0%, #111612 100%);
  border-bottom: 1rpx solid rgba(255, 255, 255, 0.06);

  .back {
    width: 72rpx;
    font-size: 52rpx;
    line-height: 1;
    color: #ffffff;
  }

  .title {
    font-size: 36rpx;
    font-weight: 700;
    color: #ffffff;
    letter-spacing: 2rpx;
  }

  .placeholder {
    width: 72rpx;
  }
}

.scroll {
  height: calc(100vh - 150rpx);
  padding: 0 40rpx;
  box-sizing: border-box;
}

.content {
  display: block;
  padding: 36rpx 0 calc(48rpx + env(safe-area-inset-bottom));
  font-size: 28rpx;
  line-height: 1.85;
  color: #d2d2d2;
  white-space: pre-wrap;
}
</style>

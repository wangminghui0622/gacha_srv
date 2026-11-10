<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

import * as userApi from '@/services/user'
import { useAuthStore } from '@/store/authStore'
import { randomUUID } from '@/utils/id'

// 充值：模拟通道（设计 M16 D5，wx.requestPayment 真支付范围外，与 web 一致走 Recharge RPC）

const { t } = useI18n()
const auth = useAuthStore()

const amount = ref('100')
const loading = ref(false)

async function onRecharge() {
  const num = Number(amount.value)
  if (!num || num <= 0 || num > 100000) {
    uni.showToast({ title: t('user.rechargeAmountPlaceholder'), icon: 'none' })
    return
  }
  loading.value = true
  try {
    // 幂等键：网络重试同一键只到账一次（后端 uk 幂等）
    const res = await userApi.recharge(num.toFixed(2), randomUUID())
    uni.showToast({ title: `${t('user.rechargeSuccess')}：¥${res.balance}`, icon: 'success' })
    auth.setUser(await userApi.getProfile())
    setTimeout(() => uni.switchTab({ url: '/pages/mine/index' }), 600)
  } catch (err) {
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <view>
    <view class="page-title">{{ t('user.rechargeTitle') }}</view>
    <view class="gacha-card recharge-card">
      <view class="form-label">{{ t('user.rechargeAmount') }}</view>
      <view class="amount-row">
        <text class="amount-prefix">¥</text>
        <input
          v-model="amount"
          class="amount-input"
          type="digit"
          :placeholder="t('user.rechargeAmountPlaceholder')"
        />
      </view>
      <view class="recharge-note">{{ t('user.rechargeNote') }}</view>
      <view class="btn-primary" :class="{ disabled: loading }" @click="onRecharge">
        {{ t('user.rechargeSubmit') }}
      </view>
    </view>
  </view>
</template>

<style scoped lang="scss">
.recharge-card {
  .form-label {
    font-size: 26rpx;
    color: var(--color-text-secondary);
    margin-bottom: 12rpx;
  }

  .amount-row {
    display: flex;
    align-items: center;
    gap: 8rpx;
    border-bottom: 1rpx solid var(--color-border);
    margin-bottom: 20rpx;

    .amount-prefix {
      font-size: 40rpx;
      color: var(--color-accent);
      font-weight: 600;
    }

    .amount-input {
      flex: 1;
      font-size: 40rpx;
      height: 88rpx;
    }
  }

  .recharge-note {
    font-size: 24rpx;
    color: var(--color-text-secondary);
    margin-bottom: 24rpx;
  }
}
</style>

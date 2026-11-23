<script setup lang="ts">
import { reactive, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as userApi from '@/services/user'
import { useAuthStore } from '@/store/authStore'
import { tokenStorage } from '@/utils/storage'

// 登录页 = 启动页（设计 M16 D4）：token 有效直接进首页。
// 当前阶段只提供手机号 + 验证码登录，验证码固定 888888（测试用）。

const { t } = useI18n()
const auth = useAuthStore()

const loading = ref(false)

// M15a 手机号登录（tenantId 与 web 一致恒 1）
const phoneForm = reactive({ tenantId: 1, phone: '', code: '888888' })

// 登录成功：写令牌 → 拉资料入 store → 进首页（reLaunch 清栈，登录页出栈）
async function afterAuth(token: string) {
  auth.setAuth(token)
  try {
    auth.setUser(await userApi.getProfile())
  } catch {
    // 资料失败不阻断进首页（401 由适配器统一处理）
  }
  uni.showToast({ title: t('auth.loginSuccess'), icon: 'success' })
  setTimeout(() => uni.reLaunch({ url: '/pages/index/index' }), 600)
}

function showErr(err: unknown) {
  uni.showToast({ title: (err as Error).message, icon: 'none' })
}

// 启动检查：已有 token 且资料可拉 → 免登录直接进首页
onShow(async () => {
  if (!tokenStorage.get()) return
  try {
    auth.setUser(await userApi.getProfile())
    uni.reLaunch({ url: '/pages/index/index' })
  } catch {
    // 401 已由适配器清令牌；其余错误留在登录页
  }
})

// ── 手机号验证码登录（当前验证码固定 888888）──
async function onPhoneLogin() {
  if (!/^1[3-9]\d{9}$/.test(phoneForm.phone) || phoneForm.code.length !== 6) {
    uni.showToast({ title: t('auth.codePlaceholder'), icon: 'none' })
    return
  }
  loading.value = true
  try {
    const res = await userApi.phoneLogin(phoneForm)
    await afterAuth(res.token)
  } catch (err) {
    showErr(err)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <view class="auth-page">
    <view class="app-logo">🎁</view>
    <view class="app-name">{{ t('common.appName') }}</view>

    <view class="gacha-card form-card">
      <view class="form-item">
        <view class="form-label">{{ t('auth.phone') }}</view>
        <input
          v-model="phoneForm.phone"
          class="form-input"
          type="number"
          :maxlength="11"
          :placeholder="t('auth.phonePlaceholder')"
        />
      </view>
      <view class="form-item">
        <view class="form-label">{{ t('auth.code') }}</view>
        <input
          v-model="phoneForm.code"
          class="form-input"
          type="number"
          :maxlength="6"
          :placeholder="t('auth.codePlaceholder')"
        />
      </view>
      <view class="form-hint">{{ t('auth.testCodeHint') }}</view>
      <view class="btn-primary" :class="{ disabled: loading }" @click="onPhoneLogin">
        {{ t('auth.loginSubmit') }}
      </view>
    </view>
  </view>
</template>

<style scoped lang="scss">
.auth-page {
  padding-top: 80rpx;

  .app-logo {
    text-align: center;
    font-size: 96rpx;
  }

  .app-name {
    text-align: center;
    font-size: 44rpx;
    font-weight: 700;
    margin: 16rpx 0 48rpx;
  }

  .form-card {
    padding-top: 40rpx;

    .form-item {
      margin-bottom: 32rpx;

      .form-label {
        font-size: 26rpx;
        color: var(--color-text-secondary);
        margin-bottom: 12rpx;
      }

      .form-input {
        border-bottom: 1rpx solid var(--color-border);
        font-size: 32rpx;
        height: 72rpx;
      }
    }

    .form-hint {
      margin-top: 20rpx;
      text-align: center;
      color: var(--color-text-secondary);
      font-size: 24rpx;
    }
  }
}
</style>

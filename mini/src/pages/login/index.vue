<script setup lang="ts">
import { reactive, ref } from 'vue'
import { onShow, onUnload } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as userApi from '@/services/user'
import { useAuthStore } from '@/store/authStore'
import { tokenStorage } from '@/utils/storage'

// 登录页 = 启动页（设计 M16 D4）：token 有效直接进首页；微信一键登录为主形态
// 微信 tab 从 web 的"手输 code"变为真实 uni.login 按钮（设计 D3，语义不变）

const { t } = useI18n()
const auth = useAuthStore()

type TabKey = 'wechat' | 'phone' | 'login' | 'register'
const tabs: { key: TabKey; label: string }[] = [
  { key: 'wechat', label: t('auth.wechatLogin') },
  { key: 'phone', label: t('auth.phoneLogin') },
  { key: 'login', label: t('auth.login') },
  { key: 'register', label: t('auth.register') },
]
const activeTab = ref<TabKey>('wechat')
const loading = ref(false)

const loginForm = reactive({ username: '', password: '' })
const registerForm = reactive({ username: '', password: '', nickname: '' })
// M15a 手机号登录（tenantId 与 web 一致恒 1）
const phoneForm = reactive({ tenantId: 1, phone: '', code: '' })

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

// ── 微信一键登录：uni.login 拿 code → /auth/wechat/login（M15a 端点，后端零改动）──
async function onWechatLogin() {
  loading.value = true
  try {
    // #ifdef MP-WEIXIN
    const loginRes = await new Promise<UniApp.LoginRes>((resolve, reject) => {
      uni.login({ provider: 'weixin', success: resolve, fail: reject })
    })
    const res = await userApi.wechatLogin({ tenantId: 1, code: loginRes.code, scene: 2 })
    await afterAuth(res.token)
    // #endif
    // #ifndef MP-WEIXIN
    uni.showToast({ title: t('auth.wechatUnavailable'), icon: 'none' })
    // #endif
  } catch (err) {
    // 1013 未绑定 openid 等业务错误：toast 展示后端 msg，引导注册/其他方式登录
    showErr(err)
  } finally {
    loading.value = false
  }
}

// ── 手机号验证码登录：60s 倒计时重发 ──
const sending = ref(false)
const countdown = ref(0)
let countdownTimer: ReturnType<typeof setInterval> | undefined

async function onSendCode() {
  if (!/^1[3-9]\d{9}$/.test(phoneForm.phone)) {
    uni.showToast({ title: t('auth.phonePlaceholder'), icon: 'none' })
    return
  }
  sending.value = true
  try {
    await userApi.requestPhoneCode({ tenantId: phoneForm.tenantId, phone: phoneForm.phone })
    uni.showToast({ title: t('auth.codeSent'), icon: 'none' })
    countdown.value = 60
    countdownTimer = setInterval(() => {
      countdown.value -= 1
      if (countdown.value <= 0 && countdownTimer) {
        clearInterval(countdownTimer)
        countdownTimer = undefined
      }
    }, 1000)
  } catch (err) {
    showErr(err)
  } finally {
    sending.value = false
  }
}

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

// ── 账号登录 / 注册（注册后自动登录，与 web 同流程）──
async function onLogin() {
  if (loginForm.username.length < 3 || loginForm.password.length < 6) {
    uni.showToast({ title: t('auth.usernamePlaceholder'), icon: 'none' })
    return
  }
  loading.value = true
  try {
    const res = await userApi.login(loginForm)
    await afterAuth(res.token)
  } catch (err) {
    showErr(err)
  } finally {
    loading.value = false
  }
}

async function onRegister() {
  if (registerForm.username.length < 3 || registerForm.password.length < 6) {
    uni.showToast({ title: t('auth.usernamePlaceholder'), icon: 'none' })
    return
  }
  loading.value = true
  try {
    await userApi.register({ ...registerForm, nickname: registerForm.nickname || undefined })
    const res = await userApi.login({ username: registerForm.username, password: registerForm.password })
    await afterAuth(res.token)
  } catch (err) {
    showErr(err)
  } finally {
    loading.value = false
  }
}

onUnload(() => {
  if (countdownTimer) clearInterval(countdownTimer)
})
</script>

<template>
  <view class="auth-page">
    <view class="app-logo">🎁</view>
    <view class="app-name">{{ t('common.appName') }}</view>

    <view class="tab-bar">
      <view
        v-for="tab in tabs"
        :key="tab.key"
        class="tab-item"
        :class="{ active: activeTab === tab.key }"
        @click="activeTab = tab.key"
      >
        {{ tab.label }}
      </view>
    </view>

    <!-- 微信一键登录（小程序主形态） -->
    <view v-if="activeTab === 'wechat'" class="gacha-card form-card">
      <view class="btn-primary" :class="{ disabled: loading }" @click="onWechatLogin">
        {{ t('auth.wechatOneTap') }}
      </view>
      <view class="form-hint">{{ t('auth.wechatHint') }}</view>
    </view>

    <!-- 手机号验证码登录 -->
    <view v-else-if="activeTab === 'phone'" class="gacha-card form-card">
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
        <view class="code-row">
          <input
            v-model="phoneForm.code"
            class="form-input code-input"
            type="number"
            :maxlength="6"
            :placeholder="t('auth.codePlaceholder')"
          />
          <view class="code-btn" :class="{ disabled: countdown > 0 || sending }" @click="onSendCode">
            {{ countdown > 0 ? t('auth.resendCountdown', { n: countdown }) : t('auth.sendCode') }}
          </view>
        </view>
      </view>
      <view class="btn-primary" :class="{ disabled: loading }" @click="onPhoneLogin">
        {{ t('auth.loginSubmit') }}
      </view>
    </view>

    <!-- 账号登录 -->
    <view v-else-if="activeTab === 'login'" class="gacha-card form-card">
      <view class="form-item">
        <view class="form-label">{{ t('auth.username') }}</view>
        <input v-model="loginForm.username" class="form-input" :maxlength="32" :placeholder="t('auth.usernamePlaceholder')" />
      </view>
      <view class="form-item">
        <view class="form-label">{{ t('auth.password') }}</view>
        <input
          v-model="loginForm.password"
          class="form-input"
          password
          :maxlength="32"
          :placeholder="t('auth.passwordPlaceholder')"
        />
      </view>
      <view class="btn-primary" :class="{ disabled: loading }" @click="onLogin">
        {{ t('auth.loginSubmit') }}
      </view>
      <view class="switch-link" @click="activeTab = 'register'">{{ t('auth.switchToRegister') }}</view>
    </view>

    <!-- 注册 -->
    <view v-else class="gacha-card form-card">
      <view class="form-item">
        <view class="form-label">{{ t('auth.username') }}</view>
        <input
          v-model="registerForm.username"
          class="form-input"
          :maxlength="32"
          :placeholder="t('auth.usernamePlaceholder')"
        />
      </view>
      <view class="form-item">
        <view class="form-label">{{ t('auth.password') }}</view>
        <input
          v-model="registerForm.password"
          class="form-input"
          password
          :maxlength="32"
          :placeholder="t('auth.passwordPlaceholder')"
        />
      </view>
      <view class="form-item">
        <view class="form-label">{{ t('auth.nickname') }}</view>
        <input
          v-model="registerForm.nickname"
          class="form-input"
          :maxlength="32"
          :placeholder="t('auth.nicknamePlaceholder')"
        />
      </view>
      <view class="btn-primary" :class="{ disabled: loading }" @click="onRegister">
        {{ t('auth.registerSubmit') }}
      </view>
      <view class="switch-link" @click="activeTab = 'login'">{{ t('auth.switchToLogin') }}</view>
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

  .tab-bar {
    display: flex;
    margin: 0 24rpx 24rpx;
    background: var(--color-card);
    border-radius: 12rpx;
    padding: 8rpx;

    .tab-item {
      flex: 1;
      text-align: center;
      font-size: 28rpx;
      color: var(--color-text-secondary);
      line-height: 72rpx;
      border-radius: 8rpx;

      &.active {
        background: var(--color-primary);
        color: #fff;
      }
    }
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

      .code-row {
        display: flex;
        align-items: center;
        gap: 16rpx;

        .code-input {
          flex: 1;
        }

        .code-btn {
          flex-shrink: 0;
          font-size: 26rpx;
          color: var(--color-primary);
          border: 1rpx solid var(--color-primary);
          border-radius: 8rpx;
          padding: 8rpx 20rpx;

          &.disabled {
            color: var(--color-text-secondary);
            border-color: var(--color-border);
          }
        }
      }
    }

    .form-hint {
      margin-top: 20rpx;
      text-align: center;
      color: var(--color-text-secondary);
      font-size: 24rpx;
    }

    .switch-link {
      margin-top: 24rpx;
      text-align: center;
      color: var(--color-primary);
      font-size: 26rpx;
    }
  }
}
</style>

<template>
  <div class="login-page">
    <a-card class="login-card" :title="$t('login.title')">
      <a-form layout="vertical" :model="form" @finish="onSubmit">
        <a-form-item v-if="!totpStage" :label="$t('login.username')" name="username" :rules="[{ required: true, message: t('login.usernameRequired') }]">
          <a-input v-model:value="form.username" autocomplete="username" />
        </a-form-item>
        <a-form-item v-if="!totpStage" :label="$t('login.password')" name="password" :rules="[{ required: true, message: t('login.passwordRequired') }]">
          <a-input-password v-model:value="form.password" autocomplete="current-password" />
        </a-form-item>
        <a-form-item v-if="totpStage && totpSetupRequired" :label="$t('login.bindAuthenticator')">
          <a-alert type="info" show-icon :message="$t('login.bindHint')" />
          <div v-if="totpQrCode" class="qr-wrap">
            <img :src="totpQrCode" alt="TOTP QR" class="qr-image" />
          </div>
          <a-typography-paragraph copyable>{{ totpSecret }}</a-typography-paragraph>
        </a-form-item>
        <a-form-item v-if="totpStage" :label="$t('login.otpCode')" name="otpCode" :rules="[{ required: true, len: 6, message: t('login.otpRule') }]">
          <a-input v-model:value="form.otpCode" maxlength="6" inputmode="numeric" />
        </a-form-item>
        <a-form-item>
          <a-button type="primary" html-type="submit" block :loading="loading">
            {{ totpStage ? $t('login.verifyAndLogin') : devSkipTotpHint ? $t('login.login') : $t('login.nextStep') }}
          </a-button>
          <a-button v-if="totpStage" block style="margin-top: 8px" @click="resetTotpStage">{{ $t('common.back') }}</a-button>
        </a-form-item>
      </a-form>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import QRCode from 'qrcode'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const loading = ref(false)
const totpStage = ref(false)
const totpSetupRequired = ref(false)
const totpSecret = ref('')
const totpUri = ref('')
const totpQrCode = ref('')
const loginTicket = ref('')

const defaultTenant = Number(import.meta.env.VITE_DEFAULT_TENANT_ID || '1')
const devSkipTotpHint = import.meta.env.DEV

const form = reactive({
  username: '',
  password: '',
  otpCode: '',
})

async function renderTotpQrCode() {
  if (!totpUri.value) {
    totpQrCode.value = ''
    return
  }
  try {
    totpQrCode.value = await QRCode.toDataURL(totpUri.value, { width: 184, margin: 1 })
  } catch {
    totpQrCode.value = ''
  }
}

function resetTotpStage() {
  totpStage.value = false
  totpSetupRequired.value = false
  totpSecret.value = ''
  totpUri.value = ''
  totpQrCode.value = ''
  loginTicket.value = ''
  form.otpCode = ''
}

async function onSubmit() {
  loading.value = true
  try {
    const reply = await session.login(
      totpStage.value
        ? {
            // 第二步也必须携带用户名/密码/指纹（网关两步都校验必填）
            tenantId: defaultTenant,
            username: form.username.trim(),
            password: form.password,
            loginTicket: loginTicket.value,
            otpCode: form.otpCode.trim(),
          }
        : {
            tenantId: defaultTenant,
            username: form.username.trim(),
            password: form.password,
          },
    )
    if (reply.session?.accessToken) {
      message.success(t('login.loginSuccess'))
      const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/dashboard'
      await router.push(redirect)
      return
    }
    const challenge = reply.totpChallenge
    if (challenge?.loginTicket) {
      totpStage.value = true
      totpSetupRequired.value = challenge.setupRequired
      loginTicket.value = challenge.loginTicket
      totpSecret.value = challenge.totpSecret ?? ''
      totpUri.value = challenge.totpUri ?? ''
      await renderTotpQrCode()
      form.otpCode = ''
      return
    }
    message.error(t('login.loginAbnormal'))
  } catch {
    // axios 拦截器已提示
  } finally {
    loading.value = false
  }
}
</script>

<style scoped lang="scss">
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #1f1f1f 0%, #434343 100%);
  padding: 24px;
}

.login-card {
  width: 100%;
  max-width: 420px;
}

.qr-wrap {
  margin-top: 12px;
  text-align: center;
}

.qr-image {
  width: 184px;
  height: 184px;
}
</style>

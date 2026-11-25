<template>
  <div class="customer-form-body">
    <a-form :label-col="{ span: 4 }" :wrapper-col="{ span: 20 }" label-align="right">
      <a-form-item label="昵称" required>
        <a-input v-model:value="form.nickname" :maxlength="32" placeholder="请输入昵称" allow-clear />
      </a-form-item>
      <a-form-item label="手机号码">
        <a-input v-model:value="form.phone" :maxlength="11" placeholder="请输入手机号" allow-clear />
      </a-form-item>
      <a-form-item label="头像" required>
        <div class="avatar-upload-wrap">
          <a-upload :show-upload-list="false" accept="image/*" :before-upload="handleAvatarUpload">
            <div class="avatar-upload-box">
              <img v-if="form.avatarUrl" :src="form.avatarUrl" alt="" />
              <template v-else>
                <span class="upload-plus">+</span>
                <span>Upload</span>
              </template>
            </div>
          </a-upload>
          <button
            v-if="form.avatarUrl"
            type="button"
            class="avatar-remove"
            title="移除头像"
            aria-label="移除头像"
            @click.stop.prevent="form.avatarUrl = ''"
          >
            <CloseOutlined />
          </button>
        </div>
      </a-form-item>
      <a-form-item label="返佣比例">
        <div class="rebate-row">
          <a-input-number
            v-model:value="form.rebateRate"
            :min="0"
            :max="100"
            :precision="0"
            :controls="false"
            style="width: 120px"
          />
          <span class="percent">%</span>
        </div>
      </a-form-item>
      <a-form-item label="身份">
        <a-select v-model:value="form.identity" :options="identityOptions" style="width: 100%" />
      </a-form-item>
    </a-form>

    <div class="customer-actions">
      <a-button @click="emit('cancel')">取消</a-button>
      <a-button type="primary" :loading="saving" @click="submit">确定</a-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { CloseOutlined } from '@ant-design/icons-vue'
import { customerClient } from '@/api/customer'
import { uploadFileToOss } from '@/api/upload'
import { newRequestId } from '@/utils/requestId'

const emit = defineEmits<{ (e: 'success'): void; (e: 'cancel'): void }>()

const identityOptions = ['用户', '代理', '达人'].map((value) => ({ value, label: value }))
const saving = ref(false)
const form = reactive({
  nickname: '',
  phone: '',
  avatarUrl: '',
  rebateRate: 10,
  identity: '用户',
})

async function handleAvatarUpload(file: File) {
  try {
    const result = await uploadFileToOss(file, 'avatar')
    if (result.url) form.avatarUrl = result.url
    message.success('头像上传成功')
  } catch {
    message.error('头像上传失败')
  }
  return false
}

function validPhone(phone: string) {
  return /^1[3-9]\d{9}$/.test(phone)
}

async function submit() {
  if (saving.value) return
  const nickname = form.nickname.trim()
  if (!nickname) {
    message.warning('请输入昵称')
    return
  }
  if (!form.avatarUrl) {
    message.warning('请上传头像')
    return
  }
  const phone = form.phone.trim()
  if (phone && !validPhone(phone)) {
    message.warning('手机号格式不正确')
    return
  }
  if (form.rebateRate < 0 || form.rebateRate > 100) {
    message.warning('返佣比例须为 0-100')
    return
  }
  saving.value = true
  try {
    await customerClient(newRequestId()).Create({
      nickname,
      phone,
      avatarUrl: form.avatarUrl,
      rebateRate: Math.trunc(form.rebateRate),
      identity: form.identity,
    })
    message.success('保存成功')
    emit('success')
  } catch {
    // 错误提示由请求层统一处理
  } finally {
    saving.value = false
  }
}
</script>

<style scoped lang="scss">
.customer-form-body {
  padding-top: 4px;
}
.customer-form-body :deep(.ant-form-item) {
  margin-bottom: 22px;
}
.customer-form-body :deep(.ant-form-item-label > label) {
  font-size: 15px;
}
.avatar-upload-wrap {
  position: relative;
  display: inline-flex;
  width: 96px;
  height: 96px;
  line-height: 0;
}
.avatar-upload-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 96px;
  height: 96px;
  overflow: hidden;
  color: #8c8c8c;
  background: #fafafa;
  border: 1px dashed #d9d9d9;
  border-radius: 6px;
}
.avatar-upload-box img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.avatar-remove {
  position: absolute;
  top: 4px;
  right: 4px;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  padding: 0;
  font-size: 12px;
  color: #ff4d4f;
  background: transparent;
  border: 1px solid currentColor;
  border-radius: 50%;
  cursor: pointer;
  opacity: 0;
  pointer-events: none;
}
.avatar-upload-wrap:hover .avatar-remove {
  opacity: 1;
  pointer-events: auto;
}
.avatar-remove:hover {
  color: #cf1322;
}
.upload-plus {
  margin-bottom: 4px;
  font-size: 22px;
  line-height: 1;
}
.rebate-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.percent {
  color: #8c8c8c;
}
.customer-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 8px;
  border-top: 1px solid #f0f0f0;
}
.customer-actions button {
  min-width: 80px;
}
</style>

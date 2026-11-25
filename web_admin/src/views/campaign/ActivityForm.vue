<template>
  <div class="activity-form-body">
    <a-form :label-col="{ span: 4 }" :wrapper-col="{ span: 20 }" label-align="right">
      <a-form-item label="活动名称" required>
        <a-input v-model:value="form.name" :maxlength="128" placeholder="活动名称" allow-clear />
      </a-form-item>
      <a-form-item label="活动描述" required>
        <a-textarea
          v-model:value="form.description"
          :rows="3"
          :maxlength="512"
          show-count
          placeholder="活动描述"
        />
      </a-form-item>
      <a-form-item label="协议">
        <a-select
          v-model:value="form.protocol"
          :options="protocolOptions"
          allow-clear
          placeholder="请选择协议"
        />
      </a-form-item>
      <a-form-item label="是否开启">
        <a-switch v-model:checked="form.enabled" />
      </a-form-item>
      <a-form-item label="活动时间" required>
        <a-range-picker
          v-model:value="form.range"
          show-time
          format="YYYY-MM-DD HH:mm:ss"
          style="width: 100%"
        />
      </a-form-item>
      <a-form-item label="日活动时间" required>
        <div class="time-range-row">
          <a-time-picker
            v-model:value="form.dailyStart"
            value-format="HH:mm:ss"
            format="HH:mm:ss"
            placeholder="开始时间"
            style="width: 100%"
          />
          <span class="range-arrow">→</span>
          <a-time-picker
            v-model:value="form.dailyEnd"
            value-format="HH:mm:ss"
            format="HH:mm:ss"
            placeholder="结束时间"
            style="width: 100%"
          />
        </div>
      </a-form-item>
      <a-form-item label="活动图片列表">
        <div class="image-list">
          <div v-for="(url, index) in form.images" :key="`${url}-${index}`" class="image-item">
            <img :src="url" alt="" />
            <button type="button" class="image-remove" @click="removeImage(index)">
              <CloseOutlined />
            </button>
          </div>
          <a-upload :show-upload-list="false" accept="image/*" multiple :before-upload="handleImageUpload">
            <div class="image-upload-box">
              <span class="upload-plus">+</span>
              <span>Upload</span>
            </div>
          </a-upload>
        </div>
      </a-form-item>
      <a-form-item label="活动图片Map">
        <div class="map-image-wrap">
          <a-upload :show-upload-list="false" accept="image/*" :before-upload="handleMapImageUpload">
            <div class="image-upload-box">
              <img v-if="form.imageMapMain" :src="form.imageMapMain" alt="" />
              <template v-else>
                <span class="upload-plus">+</span>
                <span>Upload</span>
              </template>
            </div>
          </a-upload>
          <button v-if="form.imageMapMain" type="button" class="image-remove" @click="form.imageMapMain = ''">
            <CloseOutlined />
          </button>
        </div>
      </a-form-item>
      <a-form-item label="活动参数Map">
        <div class="params-area">
          <div v-if="form.params.length === 0" class="params-empty">暂无参数配置</div>
          <div v-for="(item, index) in form.params" :key="index" class="param-row">
            <a-input v-model:value="item.key" placeholder="参数名" />
            <a-input v-model:value="item.value" placeholder="参数值" />
            <button type="button" class="image-remove param-remove" @click="removeParam(index)">
              <CloseOutlined />
            </button>
          </div>
          <a-button type="primary" @click="addParam">添加参数</a-button>
        </div>
      </a-form-item>
    </a-form>

    <div class="activity-actions">
      <a-button @click="emit('cancel')">取消</a-button>
      <a-button type="primary" :loading="saving" @click="submit">确定</a-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { message } from 'ant-design-vue'
import { CloseOutlined } from '@ant-design/icons-vue'
import dayjs from 'dayjs'
import type { Dayjs } from 'dayjs'
import { activityClient } from '@/api/activity'
import type { Activity } from '@/api/activity'
import { uploadFileToOss } from '@/api/upload'
import { newRequestId } from '@/utils/requestId'

const props = defineProps<{ record?: Activity | null }>()
const emit = defineEmits<{ (e: 'success'): void; (e: 'cancel'): void }>()

const protocolOptions = ['默认协议', '自定义协议'].map((value) => ({ value, label: value }))
const saving = ref(false)

const form = reactive({
  name: '',
  description: '',
  protocol: undefined as string | undefined,
  enabled: false,
  range: null as [Dayjs, Dayjs] | null,
  dailyStart: '',
  dailyEnd: '',
  images: [] as string[],
  imageMapMain: '',
  params: [] as Array<{ key: string; value: string }>,
})

function resetForm() {
  form.name = ''
  form.description = ''
  form.protocol = undefined
  form.enabled = false
  form.range = null
  form.dailyStart = ''
  form.dailyEnd = ''
  form.images = []
  form.imageMapMain = ''
  form.params = []
}

function applyRecord(record: Activity) {
  form.name = record.name ?? ''
  form.description = record.description ?? ''
  form.protocol = record.protocol || undefined
  form.enabled = record.enabled === true
  const startAt = Number(record.startAtUnixMillis ?? 0)
  const endAt = Number(record.endAtUnixMillis ?? 0)
  form.range = startAt > 0 && endAt > 0 ? [dayjs(startAt), dayjs(endAt)] : null
  form.dailyStart = record.dailyStart ?? ''
  form.dailyEnd = record.dailyEnd ?? ''
  form.images = [...(record.images ?? [])]
  const mapValues = Object.values(record.imageMap ?? {})
  form.imageMapMain = mapValues[0] ?? ''
  form.params = Object.entries(record.params ?? {}).map(([key, value]) => ({ key, value }))
}

watch(
  () => props.record,
  (value) => {
    if (value) {
      applyRecord(value)
    } else {
      resetForm()
    }
  },
  { immediate: true },
)

async function handleImageUpload(file: File) {
  try {
    const result = await uploadFileToOss(file, 'activity')
    if (result.url) form.images.push(result.url)
    message.success('图片上传成功')
  } catch {
    message.error('图片上传失败')
  }
  return false
}

function removeImage(index: number) {
  form.images.splice(index, 1)
}

async function handleMapImageUpload(file: File) {
  try {
    const result = await uploadFileToOss(file, 'activity')
    if (result.url) form.imageMapMain = result.url
    message.success('图片上传成功')
  } catch {
    message.error('图片上传失败')
  }
  return false
}

function addParam() {
  form.params.push({ key: '', value: '' })
}

function removeParam(index: number) {
  form.params.splice(index, 1)
}

function buildParams() {
  const params: Record<string, string> = {}
  for (const item of form.params) {
    const key = item.key.trim()
    if (key) params[key] = item.value
  }
  return params
}

async function submit() {
  if (saving.value) return
  if (!form.name.trim()) {
    message.warning('请输入活动名称')
    return
  }
  if (!form.description.trim()) {
    message.warning('请输入活动描述')
    return
  }
  if (!form.range || form.range.length !== 2) {
    message.warning('请选择活动时间')
    return
  }
  if (!form.dailyStart || !form.dailyEnd) {
    message.warning('请选择日活动时间')
    return
  }
  const payload: Activity = {
    activityKey: props.record?.activityKey ?? '',
    name: form.name.trim(),
    description: form.description.trim(),
    protocol: form.protocol ?? '',
    enabled: form.enabled,
    startAtUnixMillis: form.range[0].valueOf(),
    endAtUnixMillis: form.range[1].valueOf(),
    dailyStart: form.dailyStart,
    dailyEnd: form.dailyEnd,
    images: [...form.images],
    imageMap: form.imageMapMain ? { main: form.imageMapMain } : {},
    params: buildParams(),
  }
  saving.value = true
  try {
    if (props.record?.id) {
      await activityClient(newRequestId()).Update(props.record.id, payload)
    } else {
      await activityClient(newRequestId()).Create(payload)
    }
    message.success('保存成功')
    emit('success')
  } catch {
    // 错误提示由请求层统一处理
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  if (!props.record) resetForm()
})
</script>

<style scoped lang="scss">
.activity-form-body {
  padding-top: 4px;
}
.activity-form-body :deep(.ant-form-item) {
  margin-bottom: 20px;
}
.activity-form-body :deep(.ant-form-item-label > label) {
  font-size: 15px;
}
.time-range-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 24px minmax(0, 1fr);
  align-items: center;
  gap: 10px;
}
.range-arrow {
  text-align: center;
  color: #8c8c8c;
}
.image-list {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
.image-item {
  position: relative;
  width: 96px;
  height: 96px;
  border: 1px solid #f0f0f0;
  border-radius: 6px;
  overflow: hidden;
}
.image-item img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.image-remove {
  position: absolute;
  top: 4px;
  right: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  padding: 0;
  font-size: 12px;
  color: #fff;
  background: rgba(0, 0, 0, 0.5);
  border: none;
  border-radius: 50%;
  cursor: pointer;
}
.image-remove:hover {
  background: #ff4d4f;
}
.image-upload-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 96px;
  height: 96px;
  color: #8c8c8c;
  background: #fafafa;
  border: 1px dashed #d9d9d9;
  border-radius: 6px;
}
.map-image-wrap {
  position: relative;
  display: inline-flex;
  width: 96px;
  height: 96px;
}
.map-image-wrap .image-upload-box {
  overflow: hidden;
}
.map-image-wrap img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.upload-plus {
  margin-bottom: 4px;
  font-size: 22px;
  line-height: 1;
}
.params-area {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-width: 720px;
}
.params-empty {
  padding: 18px;
  color: #8c8c8c;
  text-align: center;
  background: #fafafa;
  border: 1px dashed #e8e8e8;
  border-radius: 6px;
}
.param-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) 24px;
  align-items: center;
  gap: 10px;
  position: relative;
}
.param-remove {
  position: static;
}
.params-area > .ant-btn {
  align-self: flex-start;
}
.activity-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 8px;
  border-top: 1px solid #f0f0f0;
}
.activity-actions button {
  min-width: 80px;
}
</style>

<template>
  <div class="banner-form-body">
    <a-form :label-col="{ span: 3 }" :wrapper-col="{ span: 21 }" label-align="right">
      <a-form-item label="类型" required>
        <a-select v-model:value="form.type" :options="typeOptions" style="width: 100%" />
      </a-form-item>
      <a-form-item label="Banner名称" required>
        <a-input v-model:value="form.name" :maxlength="128" placeholder="Banner名称" allow-clear />
      </a-form-item>
      <a-form-item label="目标赏池ID" required>
        <a-input-number
          v-model:value="form.targetPoolId"
          :min="0"
          :precision="0"
          :controls="false"
          style="width: 100%"
        />
      </a-form-item>
      <a-form-item label="位置" required>
        <a-input-number
          v-model:value="form.position"
          :min="0"
          :precision="0"
          :controls="false"
          style="width: 100%"
        />
      </a-form-item>
      <a-form-item label="弹窗类型">
        <a-select v-model:value="form.popupType" :options="popupTypeOptions" style="width: 100%" />
      </a-form-item>
      <a-form-item label="弹窗目标类型">
        <a-select v-model:value="form.popupTargetType" :options="popupTargetTypeOptions" style="width: 100%" />
      </a-form-item>
      <a-form-item label="弹窗目标ID">
        <a-input v-model:value="form.popupTargetId" placeholder="弹窗目标ID" allow-clear />
      </a-form-item>
      <a-form-item label="弹窗内容">
        <div class="editor-wrap">
          <Toolbar :editor="editorRef" :default-config="toolbarConfig" mode="default" class="editor-toolbar" />
          <Editor
            v-model="form.popupContent"
            :default-config="editorConfig"
            mode="default"
            class="editor-content"
            @on-created="handleCreated"
          />
        </div>
      </a-form-item>
      <a-form-item label="是否展示">
        <a-switch v-model:checked="form.visible" />
      </a-form-item>
      <a-form-item label="图片" required>
        <div class="image-upload-wrap">
          <a-upload :show-upload-list="false" accept="image/*" :before-upload="handleImageUpload">
            <div class="image-upload-box">
              <img v-if="form.imageUrl" :src="form.imageUrl" alt="" />
              <template v-else>
                <span class="upload-plus">+</span>
                <span>Upload</span>
              </template>
            </div>
          </a-upload>
          <button
            v-if="form.imageUrl"
            type="button"
            class="image-remove"
            title="移除图片"
            aria-label="移除图片"
            @click.stop.prevent="form.imageUrl = ''"
          >
            <CloseOutlined />
          </button>
        </div>
      </a-form-item>
    </a-form>

    <div class="banner-actions">
      <a-button @click="emit('cancel')">取消</a-button>
      <a-button type="primary" :loading="saving" @click="submit">确定</a-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, reactive, ref, shallowRef, watch } from 'vue'
import { message } from 'ant-design-vue'
import { CloseOutlined } from '@ant-design/icons-vue'
import { Editor, Toolbar } from '@wangeditor/editor-for-vue'
import type { IDomEditor, IEditorConfig, IToolbarConfig } from '@wangeditor/editor'
import '@wangeditor/editor/dist/css/style.css'
import { bannerClient } from '@/api/banner'
import type { Banner } from '@/api/banner'
import { uploadFileToOss } from '@/api/upload'
import { compressImage } from '@/utils/imageUpload'
import { newRequestId } from '@/utils/requestId'

const props = defineProps<{ banner?: Banner | null }>()
const emit = defineEmits<{ (e: 'success'): void; (e: 'cancel'): void }>()

const typeOptions = ['聚宝盆', '一番赏', '活动吉祥值排行', '活动今日超级潮蛋赏'].map((value) => ({
  value,
  label: value,
}))
const popupTypeOptions = ['不弹窗', '普通弹窗', '富文本弹窗'].map((value) => ({ value, label: value }))
const popupTargetTypeOptions = ['无', '赏池', '活动', '商品', '链接', '页面'].map((value) => ({
  value,
  label: value,
}))

const saving = ref(false)
const editorRef = shallowRef<IDomEditor>()
const toolbarConfig: Partial<IToolbarConfig> = {}
const editorConfig: Partial<IEditorConfig> = {
  placeholder: '请输入内容...',
}

const form = reactive({
  type: '一番赏',
  name: '',
  targetPoolId: 0,
  position: 0,
  popupType: '不弹窗',
  popupTargetType: '无',
  popupTargetId: '',
  popupContent: '',
  visible: false,
  imageUrl: '',
})

function resetForm() {
  form.type = '一番赏'
  form.name = ''
  form.targetPoolId = 0
  form.position = 0
  form.popupType = '不弹窗'
  form.popupTargetType = '无'
  form.popupTargetId = ''
  form.popupContent = ''
  form.visible = false
  form.imageUrl = ''
}

function applyRecord(record: Banner) {
  form.type = record.type || '一番赏'
  form.name = record.name ?? ''
  form.targetPoolId = Number(record.targetPoolId ?? 0)
  form.position = Number(record.position ?? 0)
  form.popupType = record.popupType || '不弹窗'
  form.popupTargetType = record.popupTargetType || '无'
  form.popupTargetId = record.popupTargetId ?? ''
  form.popupContent = record.popupContent ?? ''
  form.visible = record.visible === true
  form.imageUrl = record.imageUrl ?? ''
}

watch(
  () => props.banner,
  (value) => {
    if (value) applyRecord(value)
    else resetForm()
  },
  { immediate: true },
)

function handleCreated(editor: IDomEditor) {
  editorRef.value = editor
}

onBeforeUnmount(() => {
  const editor = editorRef.value
  if (editor) editor.destroy()
})

async function handleImageUpload(file: File) {
  try {
    const uploadFile = await compressImage(file, 1600, 0.92)
    const result = await uploadFileToOss(uploadFile, 'banner')
    if (result.url) form.imageUrl = result.url
    message.success('图片上传成功')
  } catch {
    // 错误提示由请求层统一处理
  }
  return false
}

async function submit() {
  if (saving.value) return
  if (!form.type) {
    message.warning('请选择类型')
    return
  }
  if (!form.name.trim()) {
    message.warning('请输入Banner名称')
    return
  }
  if (!form.imageUrl) {
    message.warning('请上传图片')
    return
  }
  if (form.targetPoolId < 0 || form.position < 0) {
    message.warning('目标赏池ID和位置不能为负数')
    return
  }
  saving.value = true
  try {
    const payload: Banner = {
      type: form.type,
      name: form.name.trim(),
      targetPoolId: Math.trunc(form.targetPoolId),
      position: Math.trunc(form.position),
      popupType: form.popupType,
      popupTargetType: form.popupTargetType,
      popupTargetId: form.popupTargetId.trim(),
      popupContent: form.popupContent,
      visible: form.visible,
      imageUrl: form.imageUrl,
    }
    if (props.banner?.id) {
      await bannerClient(newRequestId()).Update(props.banner.id, payload)
    } else {
      await bannerClient(newRequestId()).Create(payload)
    }
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
.banner-form-body {
  padding-top: 4px;
}
.banner-form-body :deep(.ant-form-item) {
  margin-bottom: 20px;
}
.banner-form-body :deep(.ant-form-item-label > label) {
  font-size: 15px;
}
.editor-wrap {
  width: 100%;
  border: 1px solid #d9d9d9;
  border-radius: 4px;
  overflow: hidden;
}
.editor-toolbar {
  border-bottom: 1px solid #f0f0f0;
}
.editor-content {
  height: 420px;
  overflow-y: hidden;
}
.editor-content :deep(.w-e-text-container),
.editor-content :deep(.w-e-scroll) {
  background: #000;
  color: #fff;
}
.editor-content :deep(.w-e-text-container [data-slate-editor]) {
  color: #fff;
  caret-color: #fff;
}
.editor-content :deep(.w-e-text-placeholder) {
  color: #fff;
}
.image-upload-wrap {
  position: relative;
  display: inline-flex;
  width: 96px;
  height: 96px;
  line-height: 0;
}
.image-upload-box {
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
.image-upload-box img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.image-remove {
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
.image-upload-wrap:hover .image-remove {
  opacity: 1;
  pointer-events: auto;
}
.image-remove:hover {
  color: #cf1322;
}
.upload-plus {
  margin-bottom: 4px;
  font-size: 22px;
  line-height: 1;
}
.banner-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 8px;
  border-top: 1px solid #f0f0f0;
}
.banner-actions button {
  min-width: 80px;
}
</style>

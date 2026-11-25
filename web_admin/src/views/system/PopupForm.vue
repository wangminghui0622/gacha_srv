<template>
  <div class="popup-form-body">
    <a-form :label-col="{ span: 3 }" :wrapper-col="{ span: 21 }" label-align="right">
      <a-form-item label="主动展示">
        <a-switch v-model:checked="form.activeDisplay" />
      </a-form-item>
      <a-form-item label="状态">
        <a-select v-model:value="form.status" :options="statusOptions" style="width: 100%" />
      </a-form-item>
      <a-form-item label="序号">
        <a-input-number
          v-model:value="form.sortOrder"
          :min="0"
          :precision="0"
          :controls="false"
          style="width: 120px"
        />
      </a-form-item>
      <a-form-item label="名称" required>
        <a-input v-model:value="form.name" :maxlength="128" placeholder="名称" allow-clear />
      </a-form-item>
      <a-form-item label="类型">
        <a-select v-model:value="form.type" :options="typeOptions" style="width: 100%" />
      </a-form-item>
      <a-form-item label="条件">
        <a-select v-model:value="form.condition" :options="conditionOptions" style="width: 100%" />
      </a-form-item>
      <a-form-item label="跳转类型">
        <a-select v-model:value="form.jumpType" :options="jumpTypeOptions" style="width: 100%" />
      </a-form-item>
      <a-form-item label="跳转目标">
        <a-input v-model:value="form.jumpTarget" placeholder="跳转目标" allow-clear />
      </a-form-item>
      <a-form-item label="弹窗图片">
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
      <a-form-item label="弹窗内容">
        <div class="editor-wrap">
          <Toolbar :editor="editorRef" :default-config="toolbarConfig" mode="default" class="editor-toolbar" />
          <Editor
            v-model="form.content"
            :default-config="editorConfig"
            mode="default"
            class="editor-content"
            @on-created="handleCreated"
          />
        </div>
      </a-form-item>
    </a-form>

    <div class="popup-actions">
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
import { popupClient } from '@/api/popup'
import type { Popup } from '@/api/popup'
import { uploadFileToOss } from '@/api/upload'
import { compressImage } from '@/utils/imageUpload'
import { newRequestId } from '@/utils/requestId'

const props = defineProps<{ popup?: Popup | null }>()
const emit = defineEmits<{ (e: 'success'): void; (e: 'cancel'): void }>()

const statusOptions = ['未开启', '开启'].map((value) => ({ value, label: value }))
const typeOptions = ['不弹窗', '普通弹窗', '普通弹窗(每日单次)', '仅弹一次'].map((value) => ({
  value,
  label: value,
}))
const conditionOptions = ['无条件', '首次登录', '每日一次', '消费达标'].map((value) => ({
  value,
  label: value,
}))
const jumpTypeOptions = ['无', '对对碰', '赏池', '活动', '商品', '链接', '页面'].map((value) => ({
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
  activeDisplay: false,
  status: '未开启',
  sortOrder: 1,
  name: '',
  type: '不弹窗',
  condition: '无条件',
  jumpType: '无',
  jumpTarget: '',
  imageUrl: '',
  content: '',
})

function resetForm() {
  form.activeDisplay = false
  form.status = '未开启'
  form.sortOrder = 1
  form.name = ''
  form.type = '不弹窗'
  form.condition = '无条件'
  form.jumpType = '无'
  form.jumpTarget = ''
  form.imageUrl = ''
  form.content = ''
}

function applyRecord(record: Popup) {
  form.activeDisplay = record.activeDisplay === true
  form.status = record.status || '未开启'
  form.sortOrder = Number(record.sortOrder ?? 0)
  form.name = record.name ?? ''
  form.type = record.type || '不弹窗'
  form.condition = record.condition || '无条件'
  form.jumpType = record.jumpType || '无'
  form.jumpTarget = record.jumpTarget ?? ''
  form.imageUrl = record.imageUrl ?? ''
  form.content = record.content ?? ''
}

watch(
  () => props.popup,
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
    const uploadFile = await compressImage(file, 800, 0.9)
    const result = await uploadFileToOss(uploadFile, 'popup')
    if (result.url) form.imageUrl = result.url
    message.success('图片上传成功')
  } catch {
    // 错误提示由请求层统一处理
  }
  return false
}

async function submit() {
  if (saving.value) return
  if (!form.name.trim()) {
    message.warning('请输入名称')
    return
  }
  if (form.sortOrder < 0) {
    message.warning('序号不能为负数')
    return
  }
  saving.value = true
  try {
    const payload: Popup = {
      activeDisplay: form.activeDisplay,
      status: form.status,
      sortOrder: Math.trunc(form.sortOrder),
      name: form.name.trim(),
      type: form.type,
      condition: form.condition,
      jumpType: form.jumpType,
      jumpTarget: form.jumpTarget.trim(),
      imageUrl: form.imageUrl,
      content: form.content,
    }
    if (props.popup?.id) {
      await popupClient(newRequestId()).Update(props.popup.id, payload)
    } else {
      await popupClient(newRequestId()).Create(payload)
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
.popup-form-body {
  padding-top: 4px;
}
.popup-form-body :deep(.ant-form-item) {
  margin-bottom: 20px;
}
.popup-form-body :deep(.ant-form-item-label > label) {
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
.popup-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 8px;
  border-top: 1px solid #f0f0f0;
}
.popup-actions button {
  min-width: 80px;
}
</style>

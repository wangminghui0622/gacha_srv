<template>
  <div class="announcement-form-body">
    <a-form :label-col="{ span: 3 }" :wrapper-col="{ span: 21 }" label-align="right">
      <a-form-item label="公告类型">
        <a-select v-model:value="form.type" :options="typeOptions" style="width: 100%" />
      </a-form-item>
      <a-form-item label="公告标题" required>
        <a-input v-model:value="form.title" :maxlength="255" placeholder="公告标题" allow-clear />
      </a-form-item>
      <a-form-item label="公告日期" required>
        <a-date-picker v-model:value="form.publishAt" format="YYYY-MM-DD" style="width: 100%" />
      </a-form-item>
      <a-form-item label="跳转类型">
        <a-select v-model:value="form.jumpType" :options="jumpTypeOptions" allow-clear placeholder="请选择跳转类型" style="width: 100%" />
      </a-form-item>
      <a-form-item label="跳转目标">
        <a-input v-model:value="form.jumpTarget" placeholder="跳转目标" allow-clear />
      </a-form-item>
      <a-form-item label="是否展示">
        <a-switch v-model:checked="form.visible" />
      </a-form-item>
      <a-form-item label="公告内容" required>
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

    <div class="announcement-actions">
      <a-button @click="emit('cancel')">取消</a-button>
      <a-button type="primary" :loading="saving" @click="submit">确定</a-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, reactive, ref, shallowRef, watch } from 'vue'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import type { Dayjs } from 'dayjs'
import { Editor, Toolbar } from '@wangeditor/editor-for-vue'
import type { IDomEditor, IEditorConfig, IToolbarConfig } from '@wangeditor/editor'
import '@wangeditor/editor/dist/css/style.css'
import { announcementClient } from '@/api/announcement'
import type { Announcement } from '@/api/announcement'
import { newRequestId } from '@/utils/requestId'

const props = defineProps<{ announcement?: Announcement | null }>()
const emit = defineEmits<{ (e: 'success'): void; (e: 'cancel'): void }>()

const typeOptions = [
  { value: 0, label: '0' },
  { value: 1, label: '1' },
  { value: 2, label: '2' },
]
const jumpTypeOptions = [
  { value: 'none', label: '无' },
  { value: 'item', label: '商品' },
  { value: 'activity', label: '活动' },
  { value: 'url', label: '链接' },
  { value: 'page', label: '页面' },
]

const saving = ref(false)
const editorRef = shallowRef<IDomEditor>()
const toolbarConfig: Partial<IToolbarConfig> = {}
const editorConfig: Partial<IEditorConfig> = {
  placeholder: '请输入内容...',
}

const form = reactive({
  title: '',
  type: 0,
  publishAt: null as Dayjs | null,
  jumpType: undefined as string | undefined,
  jumpTarget: '',
  visible: false,
  content: '',
})

function resetForm() {
  form.title = ''
  form.type = 0
  form.publishAt = null
  form.jumpType = undefined
  form.jumpTarget = ''
  form.visible = false
  form.content = ''
}

function applyRecord(record: Announcement) {
  form.title = record.title ?? ''
  form.type = Number(record.type ?? 0)
  const publishAt = Number(record.publishAtUnixMillis ?? 0)
  form.publishAt = publishAt > 0 ? dayjs(publishAt) : null
  form.jumpType = record.jumpType || undefined
  form.jumpTarget = record.jumpTarget ?? ''
  form.visible = record.visible === true
  form.content = record.content ?? ''
}

watch(
  () => props.announcement,
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

async function submit() {
  if (saving.value) return
  if (!form.title.trim()) {
    message.warning('请输入公告标题')
    return
  }
  if (!form.publishAt) {
    message.warning('请选择公告日期')
    return
  }
  const plain = form.content
    .replace(/<[^>]*>/g, '')
    .replace(/&nbsp;/g, ' ')
    .trim()
  if (!plain) {
    message.warning('请输入公告内容')
    return
  }
  saving.value = true
  try {
    const payload: Announcement = {
      title: form.title.trim(),
      type: form.type,
      publishAtUnixMillis: form.publishAt.valueOf(),
      jumpType: form.jumpType ?? '',
      jumpTarget: form.jumpTarget.trim(),
      visible: form.visible,
      content: form.content,
    }
    if (props.announcement?.id) {
      await announcementClient(newRequestId()).Update(props.announcement.id, payload)
    } else {
      await announcementClient(newRequestId()).Create(payload)
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
.announcement-form-body {
  padding-top: 4px;
}
.announcement-form-body :deep(.ant-form-item) {
  margin-bottom: 20px;
}
.announcement-form-body :deep(.ant-form-item-label > label) {
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
.announcement-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 8px;
  border-top: 1px solid #f0f0f0;
}
.announcement-actions button {
  min-width: 80px;
}
</style>

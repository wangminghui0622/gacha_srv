<template>
  <section>
    <PageHeader :title="$t('nav.taskPlan')" />
    <a-alert type="info" show-icon :message="$t('taskPlan.alert')" />
    <a-row :gutter="16" style="margin-top: 16px">
      <a-col :xs="24" :xl="9">
        <a-card :title="$t('taskPlan.versionRecord')">
          <template #extra><a-button :loading="loading" :disabled="!can('read')" @click="reload">{{ $t('common.refresh') }}</a-button></template>
          <a-table :columns="columns" :data-source="plans" row-key="version" size="small" :pagination="false">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'status'"><a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag></template>
              <template v-else-if="column.key === 'action'"><a-button type="link" @click="selectPlan(record)">{{ $t('taskPlan.viewCopy') }}</a-button></template>
            </template>
          </a-table>
          <a-button v-if="nextAfterVersion" block style="margin-top: 12px" :loading="loading" @click="loadMore">{{ $t('common.loadMore') }}</a-button>
        </a-card>
      </a-col>
      <a-col :xs="24" :xl="15">
        <a-card :title="$t('taskPlan.planDraft')">
          <a-space wrap>
            <span>{{ $t('taskPlan.basedOnLatest') }}</span><a-input v-model:value="expectedLatestVersion" style="width: 140px" />
            <a-button v-if="selected" @click="copySelected">{{ $t('taskPlan.copyVersion', { version: selected.version }) }}</a-button>
          </a-space>
          <a-tabs style="margin-top: 12px">
            <a-tab-pane key="edit" :tab="$t('taskPlan.structuredConfig')"><TaskPlanEditor v-model="editor" :errors="validationErrors" :disabled="!can('write')" /></a-tab-pane>
            <a-tab-pane key="compare" :tab="$t('taskPlan.versionCompare')">
              <a-select v-model:value="compareVersion" style="width: 180px" :options="compareOptions" :placeholder="$t('taskPlan.selectCompare')" @change="loadComparison" />
              <a-list size="small" :data-source="changeSummary"><template #renderItem="{ item }"><a-list-item>{{ item }}</a-list-item></template></a-list>
              <a-row :gutter="12" style="margin-top: 12px">
                <a-col :span="12"><a-typography-title :level="5">{{ $t('taskPlan.currentEdit') }}</a-typography-title><pre class="json-preview">{{ editorSnapshot }}</pre></a-col>
                <a-col :span="12"><a-typography-title :level="5">{{ $t('taskPlan.versionN', { version: compareVersion || $t('common.none') }) }}</a-typography-title><pre class="json-preview">{{ comparisonSnapshot }}</pre></a-col>
              </a-row>
            </a-tab-pane>
          </a-tabs>
          <a-alert v-if="Object.keys(validationErrors).length" type="error" show-icon :message="validationErrors.plan || validationErrors.memberTiers || $t('taskPlan.errorsN', { n: Object.keys(validationErrors).length })" style="margin-bottom: 12px" />
          <a-alert v-if="selected" type="info" :message="$t('taskPlan.publishOnlySaved', { version: selected.version })" style="margin-bottom: 12px" />
          <a-space wrap>
            <a-button v-if="can('write')" type="primary" :loading="saving" @click="saveDraft">{{ $t('taskPlan.saveNewDraft') }}</a-button>
            <a-button v-if="selected?.status === 'TASK_PLAN_STATUS_DRAFT' && can('request_publication')" :loading="requesting" @click="requestPublication">{{ $t('taskPlan.requestPublication') }}</a-button>
            <a-button v-if="selected?.status === 'TASK_PLAN_STATUS_DRAFT' && can('approve_publication')" :loading="approving" @click="approvePublication">{{ $t('taskPlan.approvePass') }}</a-button>
            <a-popconfirm v-if="selected?.status === 'TASK_PLAN_STATUS_DRAFT' && can('publish')" :title="$t('taskPlan.publishConfirm')" @confirm="publishPlan">
              <a-button danger :loading="publishing">{{ $t('taskPlan.executePublish') }}</a-button>
            </a-popconfirm>
          </a-space>
          <a-descriptions v-if="selected" bordered :column="1" style="margin-top: 16px">
            <a-descriptions-item :label="$t('taskPlan.version')">{{ selected.version }}</a-descriptions-item>
            <a-descriptions-item :label="$t('taskPlan.status')">{{ statusText(selected.status) }}</a-descriptions-item>
            <a-descriptions-item :label="$t('taskPlan.configHash')"><code>{{ selected.configHash }}</code></a-descriptions-item>
            <a-descriptions-item :label="$t('taskPlan.publicationRequest')">
              <template v-if="approval">{{ $t('taskPlan.approvalLine', { requester: approval.requesterId, approver: approval.approved ? approval.approverId : $t('taskPlan.pendingApproval'), state: approval.approved ? $t('taskPlan.approved') : $t('taskPlan.pendingApproval') }) }}</template>
              <template v-else>{{ $t('taskPlan.notRequested') }}</template>
            </a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
    </a-row>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { isAxiosError } from 'axios'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import PageHeader from '@/components/PageHeader.vue'
import TaskPlanEditor from '@/components/task/TaskPlanEditor.vue'
import { contentOf, validateContent, summarizeChanges } from './taskPlanForm'
import { taskClient } from '@/api/task'
import type { TaskPlan, TaskPlanPublicationApproval } from '@/api/task'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const { t } = useI18n()
const can = (action: string) => allows(session.permissionNodes, 'task.plan', action)
const plans = ref<TaskPlan[]>([])
const selected = ref<TaskPlan | null>(null)
const approval = ref<TaskPlanPublicationApproval | null>(null)
const loading = ref(false)
const saving = ref(false)
const requesting = ref(false)
const approving = ref(false)
const publishing = ref(false)
const nextAfterVersion = ref('')
const expectedLatestVersion = ref('0')
const compareVersion = ref<string>()
const comparisonSnapshot = ref('')
const editor = ref(contentOf())
const comparison = ref(contentOf())
const validationErrors = ref<Record<string, string>>({})
const changeSummary = computed(() => summarizeChanges(comparison.value, editor.value))
const columns = computed(() => [
  { title: t('taskPlan.version'), dataIndex: 'version' },
  { title: t('taskPlan.status'), key: 'status' },
  { title: t('taskPlan.tasksCount'), customRender: ({ record }: { record: TaskPlan }) => record.tasks?.length ?? 0 },
  { title: t('common.action'), key: 'action' },
])
const compareOptions = computed(() => plans.value.map((plan) => ({ value: plan.version, label: `${t('taskPlan.versionN', { version: plan.version })} · ${statusText(plan.status)}` })).filter((item) => item.value))
const editorSnapshot = computed(() => formatJSON(editor.value))

const statusKeys: Record<string, string> = {
  TASK_PLAN_STATUS_UNSPECIFIED: 'unspecified',
  TASK_PLAN_STATUS_DRAFT: 'draft',
  TASK_PLAN_STATUS_PUBLISHED: 'published',
  TASK_PLAN_STATUS_RETIRED: 'retired',
}
function statusText(status?: string) {
  const key = statusKeys[status ?? '']
  return key ? t(`taskPlan.statuses.${key}`) : t('common.none')
}
function statusColor(status?: string) {
  return status === 'TASK_PLAN_STATUS_PUBLISHED' ? 'green' : status === 'TASK_PLAN_STATUS_DRAFT' ? 'blue' : 'default'
}
function formatJSON(value: unknown) {
  return JSON.stringify(value, null, 2)
}

async function fetchPage(afterVersion = '') {
  return taskClient.ListPlans({ afterVersion: afterVersion || undefined, pageSize: 20 })
}
async function reload() {
  if (!can('read')) return
  loading.value = true
  try {
    const reply = await fetchPage()
    plans.value = reply.plans ?? []
    nextAfterVersion.value = reply.nextAfterVersion ?? ''
    expectedLatestVersion.value = (await taskClient.GetPlan({ version: '0' })).version ?? '0'
  } finally {
    loading.value = false
  }
}
async function loadMore() {
  loading.value = true
  try {
    const reply = await fetchPage(nextAfterVersion.value)
    plans.value.push(...(reply.plans ?? []))
    nextAfterVersion.value = reply.nextAfterVersion ?? ''
  } finally {
    loading.value = false
  }
}
async function selectPlan(plan: TaskPlan) {
  if (!plan.version) return
  selected.value = await taskClient.GetPlan({ version: plan.version })
  approval.value = null
  try {
    approval.value = await taskClient.GetPlanPublicationApproval({ version: plan.version })
  } catch (error) {
    if (!isAxiosError(error) || error.response?.status !== 404) throw error
    approval.value = null
  }
}
function copySelected() {
  if (!selected.value) return
  editor.value = contentOf(selected.value)
  validationErrors.value = {}
}
async function loadComparison() {
  if (!compareVersion.value) return
  const value = await taskClient.GetPlan({ version: compareVersion.value })
  comparison.value = contentOf(value)
  comparisonSnapshot.value = formatJSON(comparison.value)
}

async function saveDraft() {
  saving.value = true
  try {
    validationErrors.value = validateContent(editor.value)
    if (Object.keys(validationErrors.value).length) throw new Error(t('taskPlan.fixErrors'))
    if (!/^(0|[1-9]\d*)$/.test(expectedLatestVersion.value) || BigInt(expectedLatestVersion.value) > 18446744073709551615n) throw new Error(t('taskPlan.baseVersionInvalid'))
    selected.value = await taskClient.CreatePlanDraft({ expectedLatestVersion: expectedLatestVersion.value, ...editor.value })
    approval.value = null
    await reload()
    message.success(t('taskPlan.draftSaved', { version: selected.value.version }))
  } catch (error) {
    if (error instanceof Error && !('response' in error)) message.warning(error.message)
  } finally {
    saving.value = false
  }
}
function selectedIdentity() {
  const value = selected.value
  if (!value?.version || !value.configHash) throw new Error(t('taskPlan.selectDraftFirst'))
  return { version: value.version, expectedConfigHash: value.configHash }
}
async function requestPublication() {
  requesting.value = true
  try {
    approval.value = await taskClient.RequestPlanPublication(selectedIdentity())
    message.success(t('taskPlan.requestSubmitted'))
  } finally {
    requesting.value = false
  }
}
async function approvePublication() {
  approving.value = true
  try {
    approval.value = await taskClient.ApprovePlanPublication(selectedIdentity())
    message.success(t('taskPlan.approvalDone'))
  } finally {
    approving.value = false
  }
}
async function publishPlan() {
  publishing.value = true
  try {
    selected.value = await taskClient.PublishPlan(selectedIdentity())
    await reload()
    message.success(t('taskPlan.published'))
  } finally {
    publishing.value = false
  }
}
void reload()
</script>

<style scoped>.json-preview { max-height: 360px; overflow: auto; padding: 12px; background: #f5f5f5; white-space: pre-wrap; word-break: break-all; }</style>

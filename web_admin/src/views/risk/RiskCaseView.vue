<template>
  <section>
    <PageHeader :title="$t('risk.title')" />
    <a-alert :message="$t('risk.alert')" type="info" show-icon style="margin-bottom: 16px" />
    <a-card v-if="can('list')" :title="$t('risk.caseList')">
      <a-space wrap style="margin-bottom: 16px">
        <a-select v-model:value="filterStatus" :options="filterOptions" style="width: 160px" :disabled="loading || mutating" />
        <a-button :loading="loading" :disabled="mutating" @click="search">{{ $t('risk.search') }}</a-button>
      </a-space>
      <a-alert v-if="listError" type="error" :message="$t('risk.listFailed')" show-icon />
      <a-table :data-source="cases" :loading="loading" :pagination="false" :row-key="(row: RiskCaseSummary) => row.caseId ?? row.decisionId ?? ''" :scroll="{ x: 1200 }" size="small">
        <a-table-column :title="$t('risk.caseId')" data-index="caseId" /><a-table-column :title="$t('risk.scene')" data-index="sceneCode" />
        <a-table-column :title="$t('risk.userId')" data-index="userId" /><a-table-column :title="$t('risk.deviceId')" data-index="deviceId" />
        <a-table-column :title="$t('risk.riskScore')" data-index="riskScore" />
        <a-table-column :title="$t('risk.decision')"><template #default="{ record }">{{ actionLabel(record.action) }}</template></a-table-column>
        <a-table-column :title="$t('common.status')"><template #default="{ record }">{{ statusLabel(record.status) }}</template></a-table-column>
        <a-table-column :title="$t('risk.assignee')" data-index="assignedActorId" />
        <a-table-column :title="$t('risk.createdAt')"><template #default="{ record }">{{ formatTime(record.createdAtUnixMillis) }}</template></a-table-column>
        <a-table-column :title="$t('common.action')"><template #default="{ record }">
          <a-space>
            <a-popconfirm v-if="can('claim')" :title="$t('risk.claimConfirm')" @confirm="claim(record.caseId)"><a-button type="link" :disabled="loading || mutating || !record.caseId || record.status !== 'RISK_CASE_STATUS_OPEN'">{{ $t('risk.claim') }}</a-button></a-popconfirm>
            <a-button v-if="can('resolve')" type="link" :disabled="loading || mutating || !record.caseId || record.status !== 'RISK_CASE_STATUS_IN_REVIEW'" @click="openResolution(record.caseId)">{{ $t('risk.resolve') }}</a-button>
          </a-space>
        </template></a-table-column>
      </a-table>
      <a-space style="margin-top: 16px">
        <a-button :disabled="loading || mutating || cursors.length < 2" @click="previous">{{ $t('common.previousPage') }}</a-button><span>{{ $t('common.pageN', { n: cursors.length }) }}</span>
        <a-button :disabled="loading || mutating || !nextCursor.id" @click="next">{{ $t('common.nextPage') }}</a-button>
        <a-button :loading="loading" :disabled="mutating" @click="load">{{ $t('common.refresh') }}</a-button>
      </a-space>
    </a-card>
    <a-card v-if="can('claim') || can('resolve')" :title="$t('risk.byId')" style="margin-top: 16px">
      <a-space wrap>
        <a-input v-model:value="caseIdInput" :placeholder="$t('risk.caseId')" :aria-label="$t('risk.caseId')" :disabled="mutating" />
        <a-popconfirm v-if="can('claim')" :title="$t('risk.claimByIdConfirm')" @confirm="claim(caseIdInput)"><a-button :disabled="loading || mutating || !caseIdInput">{{ $t('risk.claim') }}</a-button></a-popconfirm>
        <a-button v-if="can('resolve')" :disabled="loading || mutating || !caseIdInput" @click="openResolution(caseIdInput)">{{ $t('risk.resolve') }}</a-button>
      </a-space>
      <a-descriptions v-if="transition" :column="1" size="small" style="margin-top: 16px">
        <a-descriptions-item :label="$t('risk.result')">{{ transition.caseId }} · {{ transition.status ? statusLabel(transition.status) : '—' }}</a-descriptions-item>
        <a-descriptions-item :label="$t('risk.actor')">{{ transition.actorId }}</a-descriptions-item>
      </a-descriptions>
    </a-card>
    <a-modal v-model:open="resolutionOpen" :title="$t('risk.resolveTitle')" :confirm-loading="mutating" :mask-closable="false" :closable="!mutating" :cancel-button-props="{ disabled: mutating }" @ok="resolve">
      <a-form layout="vertical">
        <a-form-item :label="$t('risk.caseId')"><a-input :value="resolution.caseId" disabled /></a-form-item>
        <a-form-item :label="$t('risk.resolution')"><a-select v-model:value="resolution.status" :options="resolutionOptions" :disabled="mutating" /></a-form-item>
        <a-form-item :label="$t('risk.resolutionCode')"><a-input v-model:value="resolution.code" :disabled="mutating" :placeholder="$t('risk.resolutionCodePlaceholder')" /></a-form-item>
      </a-form>
    </a-modal>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import PageHeader from '@/components/PageHeader.vue'
import { riskClient, riskStatusNumbers, type RiskCaseSummary, type RiskCaseTransition, type RiskStatusName } from '@/api/risk'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'
import { newRequestId } from '@/utils/requestId'

// 响应枚举为名称 → 视图用名称键；List 的 status 请求参数为数字（riskStatusNumbers，见 api/risk.ts）。
const statusKeys: Record<string, string> = { RISK_CASE_STATUS_UNSPECIFIED: 'unspecified', RISK_CASE_STATUS_OPEN: 'open', RISK_CASE_STATUS_IN_REVIEW: 'inReview', RISK_CASE_STATUS_RESOLVED: 'resolved', RISK_CASE_STATUS_DISMISSED: 'dismissed' }
const actionKeys: Record<string, string> = { RISK_DECISION_ACTION_UNSPECIFIED: 'unspecified', RISK_DECISION_ACTION_ALLOW: 'allow', RISK_DECISION_ACTION_OBSERVE: 'observe', RISK_DECISION_ACTION_CHALLENGE: 'challenge', RISK_DECISION_ACTION_LIMIT: 'limit', RISK_DECISION_ACTION_REVIEW: 'review', RISK_DECISION_ACTION_DENY: 'deny' }
const { t } = useI18n()
function statusLabel(value?: string | number) {
  const key = statusKeys[String(value)]
  return key ? t(`risk.statuses.${key}`) : '—'
}
function actionLabel(value?: string) {
  const key = actionKeys[value ?? '']
  return key ? t(`risk.actions.${key}`) : '—'
}
const filterOptions = computed(() => Object.entries(statusKeys).filter(([value]) => value !== 'RISK_CASE_STATUS_UNSPECIFIED').map(([value]) => ({ value, label: t(`risk.statuses.${statusKeys[value]}`) })))
const resolutionOptions = computed(() => [{ value: 'RISK_CASE_STATUS_RESOLVED', label: t('risk.resolved') }, { value: 'RISK_CASE_STATUS_DISMISSED', label: t('risk.dismissed') }])
const session = useSessionStore(), can = (action: string) => allows(session.permissionNodes, 'risk.case', action)
const cases = ref<RiskCaseSummary[]>([]), loading = ref(false), mutating = ref(false), listError = ref(false)
const filterStatus = ref<string>('RISK_CASE_STATUS_OPEN'), appliedStatus = ref<string>('RISK_CASE_STATUS_OPEN')
const cursors = ref([{ time: '', id: '' }]), nextCursor = ref({ time: '', id: '' })
const caseIdInput = ref(''), transition = ref<RiskCaseTransition | null>(null), resolutionOpen = ref(false)
const resolution = reactive({ caseId: '', status: 'RISK_CASE_STATUS_RESOLVED' as RiskStatusName, code: '' })
const commands = new Map<string, { body: string; id: string }>()
function operationId(operation: string, payload: unknown) {
  const body = JSON.stringify(payload), previous = commands.get(operation)
  if (previous?.body === body) return previous.id
  const id = newRequestId(); commands.set(operation, { body, id }); return id
}
function validText(value: string) { return !!value.trim() && new TextEncoder().encode(value).length <= 64 }
function formatTime(value: string | undefined) {
  if (!value || !/^\d+$/.test(value) || BigInt(value) > 8640000000000000n) return '—'
  return new Date(Number(value)).toLocaleString()
}
async function load() {
  if (!can('list')) return
  loading.value = true; listError.value = false
  try {
    const cursor = cursors.value.at(-1)!
    const reply = await riskClient().List({ status: riskStatusNumbers[appliedStatus.value as RiskStatusName], pageSize: 20, beforeCreatedAtUnixMillis: cursor.time || undefined, beforeCaseId: cursor.id || undefined })
    cases.value = reply.cases ?? []
    nextCursor.value = reply.nextBeforeCaseId && reply.nextBeforeCreatedAtUnixMillis && reply.nextBeforeCreatedAtUnixMillis !== '0' ? { id: reply.nextBeforeCaseId, time: reply.nextBeforeCreatedAtUnixMillis } : { id: '', time: '' }
  } catch { listError.value = true; cases.value = []; nextCursor.value = { id: '', time: '' } }
  finally { loading.value = false }
}
function search() { appliedStatus.value = filterStatus.value; cursors.value = [{ time: '', id: '' }]; void load() }
function previous() { if (cursors.value.length > 1) { cursors.value.pop(); void load() } }
function next() { if (nextCursor.value.id) { cursors.value.push({ ...nextCursor.value }); void load() } }
async function claim(id: string | undefined) {
  if (mutating.value || !id || !validText(id)) { message.warning(t('risk.validation.invalidCaseId')); return }
  const key = `claim:${id}`
  mutating.value = true
  try {
    const result = await riskClient(operationId(key, id)).Claim({ caseId: id })
    transition.value = result; commands.delete(key); message.success(t('risk.claimed')); await load()
  } catch { /* 相同命令重试复用幂等键，请求层展示错误。 */ }
  finally { mutating.value = false }
}
function openResolution(id: string | undefined) {
  if (!id || !validText(id)) { message.warning(t('risk.validation.invalidCaseId')); return }
  resolution.caseId = id; resolution.status = 'RISK_CASE_STATUS_RESOLVED'; resolution.code = ''; resolutionOpen.value = true
}
async function resolve() {
  if (mutating.value || !validText(resolution.caseId) || !validText(resolution.code)) { message.warning(t('risk.validation.caseAndCode')); return }
  const payload = { caseId: resolution.caseId, status: resolution.status, resolutionCode: resolution.code }
  const key = `resolve:${resolution.caseId}`
  mutating.value = true
  try {
    transition.value = await riskClient(operationId(key, payload)).Resolve(payload); commands.delete(key)
    resolutionOpen.value = false; message.success(t('risk.resolvedToast')); await load()
  } catch { /* 相同内容重试复用幂等键。 */ }
  finally { mutating.value = false }
}
onMounted(() => { void load() })
</script>

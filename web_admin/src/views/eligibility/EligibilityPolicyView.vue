<template>
  <section>
    <PageHeader :title="$t('eligibility.title')" />
    <a-row :gutter="16">
      <a-col :xs="24" :xl="10">
        <a-card :title="$t('eligibility.policyList')">
          <template #extra><a-button v-if="can('read')" :loading="loading" @click="reload">{{ $t('common.refresh') }}</a-button></template>
          <a-table :data-source="records" :columns="columns" row-key="policy.policyId" :pagination="false" size="small">
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'status'">{{ statusLabel(record.policyVersion?.status) }}</template>
              <template v-else-if="column.key === 'action'"><a-button type="link" @click="edit(record)">{{ $t('eligibility.configure') }}</a-button></template>
            </template>
          </a-table>
          <a-button v-if="nextAfter" block style="margin-top: 12px" :loading="loading" @click="loadMore">{{ $t('common.loadMore') }}</a-button>
        </a-card>
      </a-col>
      <a-col :xs="24" :xl="14">
        <a-card :title="$t('eligibility.policyVersion')">
          <a-alert :message="$t('eligibility.alert')" type="info" show-icon />
          <a-form layout="vertical" style="margin-top: 16px" @finish="saveDraft">
            <a-form-item :label="$t('eligibility.editTarget')">
              <a-radio-group v-model:value="existing">
                <a-radio :value="false">{{ $t('eligibility.newPolicy') }}</a-radio>
                <a-radio :value="true">{{ $t('eligibility.newVersion') }}</a-radio>
              </a-radio-group>
            </a-form-item>
            <a-row :gutter="12">
              <a-col :span="12"><a-form-item :label="$t('eligibility.code')"><a-input v-model:value="form.code" :disabled="existing" :placeholder="$t('eligibility.codePlaceholder')" /></a-form-item></a-col>
              <a-col :span="12"><a-form-item :label="$t('common.name')"><a-input v-model:value="form.name" :maxlength="128" /></a-form-item></a-col>
            </a-row>
            <a-row :gutter="12">
              <a-col :span="12"><a-form-item :label="$t('eligibility.scene')"><a-select v-model:value="form.scene" :options="sceneOptions" /></a-form-item></a-col>
              <a-col :span="12"><a-form-item :label="$t('eligibility.combination')"><a-select v-model:value="form.combination" :options="combinationOptions" /></a-form-item></a-col>
            </a-row>
            <a-form-item :label="$t('eligibility.requiredFacts')"><a-input v-model:value="form.requiredFacts" placeholder="user_level, paid_minor_30d" /></a-form-item>
            <a-form-item :label="$t('eligibility.rules')">
              <a-space v-for="(rule, index) in form.rules" :key="index" direction="vertical" style="display: flex; margin-bottom: 12px">
                <a-input-group compact>
                  <a-input v-model:value="rule.ruleId" style="width: 25%" :placeholder="$t('eligibility.ruleId')" />
                  <a-input v-model:value="rule.failureCode" style="width: 35%" :placeholder="$t('eligibility.failureCode')" />
                  <a-button danger @click="removeRule(index)">{{ $t('common.delete') }}</a-button>
                </a-input-group>
                <a-textarea v-model:value="rule.expression" :rows="2" :placeholder="$t('eligibility.expressionPlaceholder')" spellcheck="false" />
              </a-space>
              <a-button @click="addRule">{{ $t('eligibility.addRule') }}</a-button>
            </a-form-item>
            <a-space wrap>
              <a-button v-if="can(existing ? 'write' : 'create')" type="primary" html-type="submit" :loading="saving">{{ $t('eligibility.saveDraft') }}</a-button>
              <a-popconfirm v-if="current?.policyVersion?.status === 'POLICY_STATUS_DRAFT' && can('publish')" :title="$t('eligibility.publishConfirm')" @confirm="publish">
                <a-button :loading="publishing">{{ $t('eligibility.publishVersion', { version: current.policyVersion.version }) }}</a-button>
              </a-popconfirm>
            </a-space>
          </a-form>
          <a-descriptions v-if="current" bordered :column="1" style="margin-top: 16px">
            <a-descriptions-item :label="$t('eligibility.policyId')">{{ current.policy?.policyId }}</a-descriptions-item>
            <a-descriptions-item :label="$t('eligibility.latestVersion')">{{ current.policy?.latestVersion }}</a-descriptions-item>
            <a-descriptions-item :label="$t('eligibility.currentVersion')">{{ current.policy?.currentVersion || $t('common.none') }}</a-descriptions-item>
            <a-descriptions-item :label="$t('eligibility.configHash')">{{ current.policyVersion?.configHash }}</a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
    </a-row>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import PageHeader from '@/components/PageHeader.vue'
import { eligibilityClient } from '@/api/eligibility'
import type { EligibilityPolicyRecord } from '@/api/eligibility'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'

type RuleForm = { ruleId: string; expression: string; failureCode: string }
const session = useSessionStore()
const { t } = useI18n()
const can = (action: string) => allows(session.permissionNodes, 'eligibility.policy', action)
const records = ref<EligibilityPolicyRecord[]>([])
const nextAfter = ref('')
const loading = ref(false)
const saving = ref(false)
const publishing = ref(false)
const existing = ref(false)
const current = ref<EligibilityPolicyRecord | null>(null)
const form = reactive({ code: '', name: '', scene: 2, combination: 1, requiredFacts: '', rules: [{ ruleId: '', expression: '', failureCode: '' }] as RuleForm[] })
const columns = computed(() => [
  { title: t('eligibility.columns.code'), dataIndex: ['policy', 'code'] },
  { title: t('eligibility.columns.name'), dataIndex: ['policy', 'name'] },
  { title: t('eligibility.columns.version'), dataIndex: ['policy', 'latestVersion'] },
  { title: t('eligibility.columns.status'), key: 'status' },
  { title: t('eligibility.columns.action'), key: 'action' },
])
const sceneOptions = computed(() => [
  { value: 1, label: t('eligibility.scenes.discover') },
  { value: 2, label: t('eligibility.scenes.view') },
  { value: 3, label: t('eligibility.scenes.draw') },
  { value: 4, label: t('eligibility.scenes.book') },
])
const combinationOptions = computed(() => [
  { value: 1, label: t('eligibility.combinations.all') },
  { value: 2, label: t('eligibility.combinations.any') },
])
// 响应枚举为名称 → 视图用数字（请求 wire 数字，见 api/eligibility.ts）。
const sceneNumbers: Record<string, number> = { ELIGIBILITY_SCENE_DISCOVER: 1, ELIGIBILITY_SCENE_VIEW: 2, ELIGIBILITY_SCENE_DRAW: 3, ELIGIBILITY_SCENE_BOOK: 4 }
const combinationNumbers: Record<string, number> = { RULE_COMBINATION_ALL: 1, RULE_COMBINATION_ANY: 2 }
const statusKeys: Record<string, string> = { POLICY_STATUS_UNSPECIFIED: 'unspecified', POLICY_STATUS_DRAFT: 'draft', POLICY_STATUS_PUBLISHED: 'published', POLICY_STATUS_RETIRED: 'retired' }
function statusLabel(value?: string | number) {
  const key = statusKeys[String(value)]
  return key ? t(`eligibility.statuses.${key}`) : t('common.none')
}
function validId(value?: string): value is string {
  return Boolean(value && /^[1-9]\d*$/.test(value))
}
function addRule() {
  form.rules.push({ ruleId: '', expression: '', failureCode: '' })
}
function removeRule(index: number) {
  if (form.rules.length > 1) form.rules.splice(index, 1)
}
async function fetchPage(afterPolicyId = '') {
  return eligibilityClient.List({ afterPolicyId: afterPolicyId || undefined, pageSize: 20 })
}
async function reload() {
  loading.value = true
  try {
    const value = await fetchPage()
    records.value = value.policies ?? []
    nextAfter.value = value.nextAfterPolicyId ?? ''
  } finally {
    loading.value = false
  }
}
async function loadMore() {
  loading.value = true
  try {
    const value = await fetchPage(nextAfter.value)
    records.value.push(...(value.policies ?? []))
    nextAfter.value = value.nextAfterPolicyId ?? ''
  } finally {
    loading.value = false
  }
}
async function edit(record: EligibilityPolicyRecord) {
  const policyId = record.policy?.policyId
  const version = record.policy?.latestVersion
  if (!validId(policyId) || !validId(version)) return
  const value = await eligibilityClient.Get({ policyId, version })
  current.value = value
  existing.value = true
  form.code = value.policy?.code ?? ''
  form.name = value.policy?.name ?? ''
  form.scene = sceneNumbers[String(value.policyVersion?.scene)] ?? 2
  form.combination = combinationNumbers[String(value.policyVersion?.combination)] ?? 1
  form.requiredFacts = (value.policyVersion?.requiredFacts ?? []).join(', ')
  form.rules = (value.policyVersion?.rules ?? []).map((rule) => ({ ruleId: rule.ruleId ?? '', expression: rule.expression ?? '', failureCode: rule.failureCode ?? '' }))
  if (!form.rules.length) addRule()
}
function payload() {
  const requiredFacts = [...new Set(form.requiredFacts.split(',').map((value) => value.trim()).filter(Boolean))]
  const rules = form.rules.map((rule) => ({ ruleId: rule.ruleId.trim(), expression: rule.expression.trim(), failureCode: rule.failureCode.trim() }))
  if (!form.name.trim() || rules.some((rule) => !rule.ruleId || !rule.expression || !rule.failureCode)) throw new Error(t('eligibility.validation.nameRulesRequired'))
  return { name: form.name.trim(), scene: form.scene, combination: form.combination, requiredFacts, rules }
}
async function saveDraft() {
  let value: EligibilityPolicyRecord
  try {
    const body = payload()
    saving.value = true
    if (existing.value) {
      if (!current.value?.policy?.policyId || !current.value.policy.latestVersion) throw new Error(t('eligibility.validation.selectFirst'))
      value = await eligibilityClient.CreateRevision({ policyId: current.value.policy.policyId, expectedLatestVersion: current.value.policy.latestVersion, ...body })
    } else {
      if (!/^[a-z][a-z0-9_.-]{1,63}$/.test(form.code)) throw new Error(t('eligibility.validation.codeInvalid'))
      value = await eligibilityClient.Create({ code: form.code, ...body })
    }
    current.value = value
    existing.value = true
    await edit(value)
    await reload()
    message.success(t('eligibility.draftSaved'))
  } catch (error) {
    if (error instanceof Error && !('response' in error)) message.warning(error.message)
  } finally {
    saving.value = false
  }
}
async function publish() {
  const policyId = current.value?.policy?.policyId
  const version = current.value?.policyVersion?.version
  const expectedConfigHash = current.value?.policyVersion?.configHash
  if (!policyId || !version || !expectedConfigHash) return
  publishing.value = true
  try {
    const value = await eligibilityClient.Publish({ policyId, version, expectedConfigHash })
    await edit(value)
    await reload()
    message.success(t('eligibility.published'))
  } finally {
    publishing.value = false
  }
}
void reload()
</script>

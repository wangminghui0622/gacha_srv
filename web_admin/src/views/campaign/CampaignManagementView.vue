<template>
  <section>
    <PageHeader :title="$t('campaign.title')" />
    <a-card :title="$t('campaign.readPublish')">
      <a-space wrap>
        <a-input v-model:value="campaignId" :placeholder="$t('campaign.campaignId')" :aria-label="$t('campaign.campaignId')" inputmode="numeric" />
        <a-button v-if="can('read')" :loading="reading" :disabled="busy" @click="readCampaign">{{ $t('campaign.readCampaign') }}</a-button>
        <a-popconfirm v-if="can('publish')" :title="$t('campaign.publishConfirm')" @confirm="publishCampaign">
          <a-button type="primary" :loading="publishing" :disabled="busy || !validId(campaignId)">{{ $t('campaign.publishLatest') }}</a-button>
        </a-popconfirm>
      </a-space>
      <a-descriptions v-if="current" bordered :column="1" style="margin-top: 16px">
        <a-descriptions-item :label="$t('campaign.campaignId')">{{ current.campaignId }}</a-descriptions-item>
        <a-descriptions-item :label="$t('common.name')">{{ current.name }}</a-descriptions-item>
        <a-descriptions-item :label="$t('campaign.state')">{{ current.state }}</a-descriptions-item>
        <a-descriptions-item :label="$t('campaign.currentVersion')">{{ current.version }}</a-descriptions-item>
        <a-descriptions-item :label="$t('campaign.publishedVersion')">{{ current.publishedVersion }}</a-descriptions-item>
      </a-descriptions>
    </a-card>
    <a-card :title="$t('campaign.draftConfig')" style="margin-top: 16px">
      <a-alert :message="$t('campaign.draftAlert')" type="info" show-icon />
      <a-form layout="vertical" style="margin-top: 16px" @finish="createDraft">
        <a-form-item :label="$t('campaign.draftTarget')">
          <a-radio-group v-model:value="existing">
            <a-radio :value="false">{{ $t('campaign.newCampaign') }}</a-radio>
            <a-radio :value="true">{{ $t('campaign.newVersion') }}</a-radio>
          </a-radio-group>
        </a-form-item>
        <a-form-item :label="$t('campaign.campaignKey')"><a-input v-model:value="form.campaignKey" :disabled="existing" :placeholder="$t('campaign.keyPlaceholder')" /></a-form-item>
        <a-form-item :label="$t('common.name')"><a-input v-model:value="form.name" :maxlength="128" /></a-form-item>
        <a-form-item :label="$t('campaign.kind')"><a-select v-model:value="form.kind" :options="kindOptions" :disabled="existing" show-search option-filter-prop="label" /></a-form-item>
        <a-row :gutter="16">
          <a-col :xs="24" :md="12"><a-form-item :label="$t('campaign.startAt')"><input v-model="form.start" type="datetime-local" step="1" class="date-input" /></a-form-item></a-col>
          <a-col :xs="24" :md="12"><a-form-item :label="$t('campaign.endAt')"><input v-model="form.end" type="datetime-local" step="1" class="date-input" /></a-form-item></a-col>
        </a-row>
        <a-form-item :label="$t('campaign.configField', { field: definition.field })">
          <a-textarea v-model:value="configText" :rows="16" spellcheck="false" />
          <a-button type="link" @click="resetConfig">{{ $t('campaign.loadSkeleton') }}</a-button>
        </a-form-item>
        <a-button v-if="can('write')" type="primary" html-type="submit" :loading="saving" :disabled="busy">{{ $t('campaign.saveDraft') }}</a-button>
        <a-alert v-if="saveUncertain" :message="$t('campaign.saveUncertain')" type="warning" show-icon style="margin-top: 16px" />
      </a-form>
    </a-card>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { isAxiosError } from 'axios'
import { useI18n } from 'vue-i18n'
import PageHeader from '@/components/PageHeader.vue'
import { campaignClient } from '@/api/campaign'
import type { Campaign, CampaignConfig } from '@/api/campaign'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'

// 本仓仅三种活动（SIGN_IN=1/REDEMPTION_CODE=2/PRIZE_WHEEL=13，见 api/campaign.ts）。
const definitions = [
  { number: 1, name: 'CAMPAIGN_KIND_SIGN_IN', field: 'signIn', labelKey: 'signIn' },
  { number: 2, name: 'CAMPAIGN_KIND_REDEMPTION_CODE', field: 'redemption', labelKey: 'redemption' },
  { number: 13, name: 'CAMPAIGN_KIND_PRIZE_WHEEL', field: 'prizeWheel', labelKey: 'prizeWheel' },
] as const
const session = useSessionStore()
const { t } = useI18n()
const can = (action: string) => allows(session.permissionNodes, 'campaign.definition', action)
const campaignId = ref('')
const existing = ref(false)
const current = ref<Campaign | null>(null)
const form = reactive({ campaignKey: '', name: '', kind: 1, start: '', end: '' })
const definition = computed(() => definitions.find((item) => item.number === form.kind) ?? definitions[0])
const kindOptions = computed(() => definitions.map((item) => ({ value: item.number, label: t(`campaign.kinds.${item.labelKey}`) })))
const configText = ref('')
const reading = ref(false)
const saving = ref(false)
const publishing = ref(false)
const saveUncertain = ref(false)
const busy = computed(() => reading.value || saving.value || publishing.value)
function validId(id: string) {
  return /^[1-9]\d*$/.test(id) && BigInt(id) <= 18446744073709551615n
}
function resetConfig() {
  const value = form.kind === 1 ? { rewards: [{ day: 1, loyaltyPoints: '0', prizes: [] }] } : {}
  configText.value = JSON.stringify({ [definition.value.field]: value }, null, 2)
}
resetConfig()
function dateInput(value: string | undefined) {
  if (!value || !/^\d+$/.test(value) || BigInt(value) > 8640000000000000n) return ''
  const date = new Date(Number(value))
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 19)
}
function wireKindToNumber(kind: string | number | undefined): number {
  if (typeof kind === 'number' && definitions.some((item) => item.number === kind)) return kind
  const byName = definitions.find((item) => item.name === String(kind))
  return byName?.number ?? 1
}
function adopt(value: Campaign) {
  current.value = value
  campaignId.value = value.campaignId ?? ''
  existing.value = true
  form.kind = wireKindToNumber(value.kind)
  form.campaignKey = value.campaignKey ?? ''
  form.name = value.name ?? ''
  form.start = dateInput(value.startAtUnixMillis)
  form.end = dateInput(value.endAtUnixMillis)
  const field = definition.value.field as keyof CampaignConfig
  configText.value = JSON.stringify({ [field]: value.config?.[field] ?? {} }, null, 2)
}
async function readCampaign() {
  if (!validId(campaignId.value)) {
    message.warning(t('campaign.validation.invalidId'))
    return
  }
  reading.value = true
  try {
    adopt(await campaignClient.Get({ campaignId: campaignId.value }))
  } catch {
    // 请求层已展示失败信息。
  } finally {
    reading.value = false
  }
}
function parseConfig(): CampaignConfig {
  // 配置内的 64 位字段保持 ProtoJSON 字符串，避免 JSON.parse 前已丢失整数精度。
  const int64Fields = new Set(['amount', 'refId', 'costItemId', 'loyaltyPoints', 'maxRewardMinor', 'minScoreMinor', 'pointsPerStep', 'stepMinor', 'target', 'thresholdMinor'])
  const value: unknown = JSON.parse(configText.value, (key, child) => {
    const field = key.replace(/_([a-z])/g, (_, letter: string) => letter.toUpperCase())
    if (int64Fields.has(field) && (typeof child !== 'string' || !/^-?\d+$/.test(child))) throw new Error(t('campaign.validation.int64String', { field }))
    if (typeof child === 'number' && !Number.isSafeInteger(child)) throw new Error(t('campaign.validation.safeInteger'))
    return child
  })
  const field = definition.value.field
  if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).length !== 1 || !(field in value)) throw new Error(t('campaign.validation.singleField', { field }))
  const child = (value as Record<string, unknown>)[field]
  if (!child || typeof child !== 'object' || Array.isArray(child)) throw new Error(t('campaign.validation.configObject'))
  return value as CampaignConfig
}
async function createDraft() {
  if (busy.value) return
  const start = Date.parse(form.start)
  const end = Date.parse(form.end)
  if (!form.name.trim() || [...form.name.trim()].length > 128 || !Number.isSafeInteger(start) || start <= 0 || !Number.isSafeInteger(end) || end <= start || (!existing.value && !/^[a-z][a-z0-9_]{0,63}$/.test(form.campaignKey)) || (existing.value && !validId(campaignId.value))) {
    message.warning(t('campaign.validation.fieldsInvalid'))
    return
  }
  let config: CampaignConfig
  try {
    config = parseConfig()
  } catch (error) {
    message.warning(error instanceof Error ? error.message : t('campaign.validation.formatError'))
    return
  }
  saving.value = true
  saveUncertain.value = false
  try {
    // 本仓 CreateDraft：kind 为数字、无 tenantId（服务端默认 1），见 api/campaign.ts。
    const value = await campaignClient.CreateDraft({
      campaignId: existing.value ? campaignId.value : '0',
      campaignKey: form.campaignKey,
      kind: form.kind,
      name: form.name.trim(),
      startAtUnixMillis: String(start),
      endAtUnixMillis: String(end),
      config,
    })
    adopt(value)
    message.success(t('campaign.draftSaved'))
  } catch (error) {
    saveUncertain.value = !isAxiosError(error) || !error.response || error.response.status >= 500
  } finally {
    saving.value = false
  }
}
async function publishCampaign() {
  if (busy.value || !validId(campaignId.value)) return
  publishing.value = true
  try {
    adopt(await campaignClient.Publish({ campaignId: campaignId.value }))
    message.success(t('campaign.published'))
  } catch {
    // 请求层已展示失败信息。
  } finally {
    publishing.value = false
  }
}
</script>

<style scoped>.date-input { width: 100%; border: 1px solid #d9d9d9; border-radius: 6px; padding: 6px 10px; }</style>

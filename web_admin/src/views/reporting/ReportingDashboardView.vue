<template>
  <section>
    <PageHeader :title="$t('reporting.title')" />
    <a-alert :message="$t('reporting.alert')" type="info" show-icon />
    <a-card :title="$t('reporting.dashboardConfig')" style="margin-top: 16px">
      <a-space wrap>
        <a-input v-model:value="lookupId" :placeholder="$t('reporting.dashboardId')" :aria-label="$t('reporting.dashboardId')" :disabled="busy" />
        <a-button v-if="can('read')" :disabled="busy" @click="read">{{ $t('common.read') }}</a-button>
        <a-button v-if="can('write')" :disabled="busy" @click="reset">{{ $t('reporting.newDraft') }}</a-button>
      </a-space>
      <a-descriptions :column="2" style="margin-top: 16px">
        <a-descriptions-item :label="$t('reporting.currentId')">{{ current?.dashboardId ?? $t('reporting.newLabel') }}</a-descriptions-item>
        <a-descriptions-item :label="$t('reporting.versionStatus')">{{ current?.version ?? '—' }} / {{ current?.status ? statusLabel(current.status) : '—' }}</a-descriptions-item>
      </a-descriptions>
      <a-form layout="vertical">
        <a-form-item :label="$t('reporting.dashboardCode')"><a-input v-model:value="code" :disabled="!editable || busy" /></a-form-item>
        <a-form-item :label="$t('reporting.title')"><a-input v-model:value="title" :disabled="!editable || busy" /></a-form-item>
        <a-form-item :label="$t('reporting.widgetsJson')"><a-textarea v-model:value="widgetsJson" :rows="10" :disabled="!editable || busy" /></a-form-item>
      </a-form>
      <a-space>
        <a-button v-if="can('write')" type="primary" :loading="busy" :disabled="!editable" @click="save">{{ $t('reporting.saveDraft') }}</a-button>
        <a-popconfirm v-if="can('publish')" :title="$t('reporting.publishConfirm')" @confirm="publish">
          <a-button :disabled="busy || current?.status !== 'DASHBOARD_STATUS_DRAFT' || !current?.version">{{ $t('reporting.publishSaved') }}</a-button>
        </a-popconfirm>
        <a-button v-if="metricPermission" :disabled="busy" @click="metrics">{{ $t('reporting.readCatalog') }}</a-button>
      </a-space>
      <pre v-if="catalog">{{ JSON.stringify(catalog, null, 2) }}</pre>
    </a-card>
    <a-card v-if="can('query')" :title="$t('reporting.queryPublished')" style="margin-top: 16px">
      <a-form layout="vertical">
        <a-form-item :label="$t('reporting.providerCode')"><a-input v-model:value="provider" :disabled="busy" /></a-form-item>
        <a-form-item :label="$t('reporting.externalAccountId')"><a-input v-model:value="account" :disabled="busy" /></a-form-item>
        <a-form-item :label="$t('reporting.from')"><a-input v-model:value="from" :disabled="busy" /></a-form-item>
        <a-form-item :label="$t('reporting.to')"><a-input v-model:value="to" :disabled="busy" /></a-form-item>
        <a-form-item :label="$t('reporting.dimensions')"><a-textarea v-model:value="dimensionsJson" :disabled="busy" /></a-form-item>
        <a-form-item :label="$t('reporting.maxPoints')"><a-input-number v-model:value="maxPoints" :min="1" :max="2000" :precision="0" :disabled="busy" /></a-form-item>
        <a-checkbox v-model:checked="skipCache" :disabled="busy">{{ $t('reporting.skipCache') }}</a-checkbox>
      </a-form>
      <a-button :loading="busy" @click="query">{{ $t('common.query') }}</a-button>
      <a-alert v-if="result" :message="result.cacheHit ? $t('reporting.cacheHit') : $t('reporting.liveResult')" type="info" />
      <a-alert v-if="result && !result.results?.length" :message="$t('reporting.noResults')" type="info" />
      <a-card v-for="widget in result?.results ?? []" :key="widget.widgetCode" :title="widget.widgetCode" size="small">
        <a-alert v-if="widget.truncated" :message="$t('reporting.truncated')" type="warning" />
        <a-table :data-source="widget.points ?? []" :pagination="{ pageSize: 20 }" size="small">
          <a-table-column :title="$t('reporting.bucketStart')" data-index="bucketStart" /><a-table-column :title="$t('reporting.currency')" data-index="currency" /><a-table-column :title="$t('reporting.value')" data-index="value" />
        </a-table>
      </a-card>
    </a-card>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import PageHeader from '@/components/PageHeader.vue'
import { reportingClient as api } from '@/api/reporting'
import type { Dashboard, DashboardWidget, MetricDefinition, QueryDashboardReply } from '@/api/reporting'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'

const { t } = useI18n()
const session = useSessionStore(), can = (action: string) => allows(session.permissionNodes, 'reporting.dashboard', action)
const metricPermission = computed(() => allows(session.permissionNodes, 'reporting.metric', 'read'))
const statusKeys: Record<string, string> = { DASHBOARD_STATUS_UNSPECIFIED: 'unspecified', DASHBOARD_STATUS_DRAFT: 'draft', DASHBOARD_STATUS_PUBLISHED: 'published', DASHBOARD_STATUS_ARCHIVED: 'archived' }
function statusLabel(value: string) {
  const key = statusKeys[value]
  return key ? t(`reporting.statuses.${key}`) : t('common.unknown')
}
const current = ref<Dashboard>(), result = ref<QueryDashboardReply>(), catalog = ref<{ metrics?: MetricDefinition[] }>(), busy = ref(false)
const lookupId = ref(''), code = ref(''), title = ref(''), widgetsJson = ref('[]')
const provider = ref(''), account = ref(''), from = ref(''), to = ref(''), dimensionsJson = ref('{}'), maxPoints = ref<number | null>(500), skipCache = ref(false)
const editable = computed(() => can('write') && (!current.value || current.value.status === 'DASHBOARD_STATUS_DRAFT'))
function validId(value: string) { return /^[1-9]\d*$/.test(value) && BigInt(value) <= 18446744073709551615n }
function accept(value: Dashboard) {
  current.value = value; lookupId.value = value.dashboardId ?? ''; code.value = value.dashboardCode ?? ''; title.value = value.title ?? ''
  widgetsJson.value = JSON.stringify(value.widgets ?? [], null, 2); result.value = undefined
}
function reset() { current.value = undefined; lookupId.value = ''; code.value = ''; title.value = ''; widgetsJson.value = '[]'; result.value = undefined }
async function run(task: () => Promise<void>, writing = false) {
  if (busy.value) return
  busy.value = true
  try { await task() } catch { if (writing) message.warning(t('common.operationUncertain')) }
  finally { busy.value = false }
}
function read() {
  if (!validId(lookupId.value)) { message.warning(t('reporting.validation.invalidId')); return }
  void run(async () => accept(await api.Get({ dashboardId: lookupId.value })))
}
function parseWidgets(): DashboardWidget[] {
  const value: unknown = JSON.parse(widgetsJson.value)
  if (!Array.isArray(value) || !value.length || value.length > 100) throw new Error(t('reporting.validation.widgetCount'))
  for (const item of value) {
    if (!item || typeof item !== 'object' || Array.isArray(item)) throw new Error(t('reporting.validation.widgetObject'))
    if ('widgetId' in item && (typeof item.widgetId !== 'string' || !/^\d+$/.test(item.widgetId) || BigInt(item.widgetId) > 18446744073709551615n)) throw new Error(t('reporting.validation.widgetIdInvalid'))
    for (const key of ['gridX', 'gridY', 'gridWidth', 'gridHeight']) if (key in item && (!Number.isInteger(item[key]) || item[key] < 0 || item[key] > 4294967295)) throw new Error(t('reporting.validation.layoutFieldInvalid'))
  }
  return value as DashboardWidget[]
}
function save() {
  if (!editable.value || !code.value.trim() || !title.value.trim()) { message.warning(t('reporting.validation.codeTitleRequired')); return }
  let widgets: DashboardWidget[]
  try { widgets = parseWidgets() } catch (error) { message.warning(error instanceof Error ? error.message : t('reporting.validation.widgetsJsonInvalid')); return }
  if (current.value && (!current.value.dashboardId || !validId(current.value.dashboardId) || !current.value.version || !validId(current.value.version))) { message.warning(t('reporting.validation.missingVersion')); return }
  void run(async () => { accept(await api.Save({ dashboardId: current.value?.dashboardId ?? '0', expectedVersion: current.value?.version ?? '0', dashboardCode: code.value, title: title.value, widgets })); message.success(t('reporting.draftSaved')) }, true)
}
function publish() {
  const value = current.value
  if (value?.status !== 'DASHBOARD_STATUS_DRAFT' || !value.dashboardId || !validId(value.dashboardId) || !value.version || !validId(value.version)) return
  const dashboardId = value.dashboardId, expectedVersion = value.version
  void run(async () => { accept(await api.Publish({ dashboardId, expectedVersion })); message.success(t('reporting.published')) }, true)
}
function metrics() { void run(async () => { catalog.value = await api.Metrics() }) }
function query() {
  if (!validId(lookupId.value) || !provider.value.trim() || !account.value.trim()) { message.warning(t('reporting.validation.queryFieldsRequired')); return }
  const iso = /^\d{4}-\d{2}-\d{2}T.*(?:Z|[+-]\d{2}:\d{2})$/
  const start = Date.parse(from.value), end = Date.parse(to.value)
  if (!iso.test(from.value) || !iso.test(to.value) || !Number.isFinite(start) || !Number.isFinite(end) || end <= start || !maxPoints.value || !Number.isInteger(maxPoints.value) || maxPoints.value > 2000 || maxPoints.value < 1) { message.warning(t('reporting.validation.timeRangeInvalid')); return }
  let dimensions: Record<string, string>
  try { const value = JSON.parse(dimensionsJson.value); if (!value || typeof value !== 'object' || Array.isArray(value) || Object.values(value).some(item => typeof item !== 'string')) throw new Error(); dimensions = value } catch { message.warning(t('reporting.validation.dimensionsInvalid')); return }
  result.value = undefined
  void run(async () => { result.value = await api.Query({ dashboardId: lookupId.value, providerCode: provider.value, externalAccountId: account.value, from: new Date(start).toISOString(), to: new Date(end).toISOString(), dimensionValues: dimensions, maxDataPoints: maxPoints.value ?? 500, skipCache: skipCache.value }) })
}
</script>

<style scoped>
pre { overflow: auto; max-height: 360px; white-space: pre-wrap; overflow-wrap: anywhere; }
</style>

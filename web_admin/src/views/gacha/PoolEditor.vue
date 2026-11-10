<template>
  <a-card :title="$t('pool.editorTitle')" style="margin-bottom: 16px">
    <a-alert :message="$t('pool.editorAlert')" type="info" show-icon />
    <a-space wrap style="margin: 16px 0">
      <a-input v-model:value="poolId" :placeholder="$t('pool.poolId')" :aria-label="$t('pool.poolId')" :disabled="busy" />
      <a-input v-model:value="version" :placeholder="$t('pool.versionPlaceholder')" :aria-label="$t('pool.version')" :disabled="busy" />
      <a-button v-if="can('read')" :disabled="busy" @click="read">{{ $t('pool.read') }}</a-button>
      <a-button v-if="can('create')" :disabled="busy" @click="reset">{{ $t('pool.newExample') }}</a-button>
    </a-space>
    <a-descriptions :column="2">
      <a-descriptions-item :label="$t('pool.loadedVersion')">{{ current?.poolId ?? $t('common.none') }} / {{ current?.version ?? $t('common.none') }}</a-descriptions-item>
      <a-descriptions-item :label="$t('common.status')">{{ current?.status ? statusText(current.status) : $t('common.none') }}</a-descriptions-item>
      <a-descriptions-item :label="$t('pool.configHash')">{{ current?.configHash ?? $t('common.none') }}</a-descriptions-item>
    </a-descriptions>
    <a-form layout="vertical">
      <a-form-item :label="$t('pool.configJson')">
        <a-textarea v-model:value="configJson" :rows="15" :disabled="busy || !can('create')" />
      </a-form-item>
    </a-form>
    <a-space wrap>
      <a-button v-if="can('create')" :loading="busy" @click="create">{{ $t('pool.saveDraft') }}</a-button>
      <a-button v-if="can('request_publication')" :disabled="busy || !publishable" @click="transition('request')">{{ $t('pool.requestPublication') }}</a-button>
      <a-popconfirm v-if="can('approve_publication')" :title="$t('pool.approveConfirm')" @confirm="transition('approve')"><a-button :disabled="busy || !publishable">{{ $t('pool.approve') }}</a-button></a-popconfirm>
      <a-popconfirm v-if="can('publish')" :title="$t('pool.publishConfirm')" @confirm="transition('publish')"><a-button :disabled="busy || !publishable">{{ $t('pool.publish') }}</a-button></a-popconfirm>
    </a-space>
    <a-descriptions v-if="approval" :column="2" style="margin-top: 16px">
      <a-descriptions-item :label="$t('pool.requester')">{{ approval.requesterId }}</a-descriptions-item>
      <a-descriptions-item :label="$t('pool.approver')">{{ approval.approverId }}</a-descriptions-item>
      <a-descriptions-item :label="$t('pool.approvalResult')">{{ approval.approved ? $t('pool.approved') : $t('pool.pending') }}</a-descriptions-item>
    </a-descriptions>
  </a-card>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { poolClient } from '@/api/pool'
import type { BoxDraft, PoolPublicationApproval } from '@/api/pool'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'
import { newRequestId } from '@/utils/requestId'

const emit = defineEmits<{ changed: [] }>()
const { t } = useI18n()
const session = useSessionStore()
const can = (action: string) => allows(session.permissionNodes, 'gacha.pool', action)
const statusKeys: Record<string, string> = { POOL_STATUS_UNSPECIFIED: 'unspecified', POOL_STATUS_DRAFT: 'draft', POOL_STATUS_PUBLISHED: 'published', POOL_STATUS_RETIRED: 'retired' }
function statusText(status: string | number) {
  const key = statusKeys[String(status)]
  return key ? t(`pool.statuses.${key}`) : t('common.none')
}
const current = ref<BoxDraft>()
const approval = ref<PoolPublicationApproval>()
const busy = ref(false)
const poolId = ref('')
const version = ref('1')
const configJson = ref('{}')
let pending: { payload: string; key: string } | undefined
function validId(value: string) {
  return /^[1-9]\d*$/.test(value) && BigInt(value) <= 18446744073709551615n
}
const publishable = computed(() => current.value?.status === 'POOL_STATUS_DRAFT' && !!current.value.poolId && validId(String(current.value.poolId)) && !!current.value.version && validId(String(current.value.version)) && validHash(current.value.configHash))
function validHash(value: string | undefined) {
  try {
    return !!value && atob(value).length === 32
  } catch {
    return false
  }
}
function accept(value: BoxDraft) {
  current.value = value
  poolId.value = String(value.poolId ?? '')
  version.value = String(value.version ?? '')
  approval.value = undefined
  const { version: _version, status: _status, configHash: _hash, publishedAtUnixMillis: _time, ...configuration } = value
  configJson.value = JSON.stringify(configuration, null, 2)
}
function reset() {
  current.value = undefined
  approval.value = undefined
  pending = undefined
  // 本仓 BoxDraft 骨架：boxName/price/entries + 滚动规则（无参考 mode/drawPrice）。
  configJson.value = JSON.stringify(
    {
      poolId: validId(poolId.value) ? poolId.value : '1',
      boxName: '新宝箱',
      price: '100',
      description: '',
      rollingRulesJson: '{}',
      entries: [
        { entryId: '1', name: '谢谢参与', weight: 30, quantity: 1000, isPrize: false },
        { entryId: '2', name: '限定皮肤', weight: 1, quantity: 1, isPrize: true, displayValueFen: '5000', rollingLimited: true },
      ],
    },
    null,
    2,
  )
}
async function run(task: () => Promise<void>) {
  if (busy.value) return
  busy.value = true
  try {
    await task()
  } catch {
    // 请求层显示服务端错误，不自动重试。
  } finally {
    busy.value = false
  }
}
function read() {
  if (!validId(poolId.value) || !validId(version.value)) {
    message.warning(t('pool.validation.invalidIdVersion'))
    return
  }
  void run(async () => accept(await poolClient().Get({ poolId: poolId.value, version: version.value })))
}
function validateIntegers(value: unknown, key = '') {
  const wide = /(?:Id|Ids|Minor|UnixMillis)$/.test(key) || ['version', 'weight', 'policyVersion', 'loyaltyPointsPerMajor'].includes(key)
  if (Array.isArray(value)) {
    for (const item of value) validateIntegers(item, key)
    return
  }
  if (value !== null && typeof value === 'object') {
    for (const [name, item] of Object.entries(value)) validateIntegers(item, name)
    return
  }
  if (wide && key !== 'patternId') {
    if (typeof value !== 'string' || !/^\d+$/.test(value) || BigInt(value) > (/Minor$|UnixMillis$/.test(key) ? 9223372036854775807n : 18446744073709551615n)) throw new Error(t('pool.validation.int64String', { key }))
  } else if (typeof value === 'number' && !Number.isSafeInteger(value)) throw new Error(t('pool.validation.safeInteger'))
}
function create() {
  if (!can('create')) return
  let configuration: BoxDraft
  try {
    const value = JSON.parse(configJson.value)
    if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(t('pool.validation.objectRequired'))
    validateIntegers(value)
    if (typeof value.poolId !== 'string' || !validId(value.poolId) || !Array.isArray(value.entries) || !value.entries.length) throw new Error(t('pool.validation.poolIdAndEntries'))
    for (const field of ['version', 'status', 'configHash', 'publishedAtUnixMillis']) if (field in value) throw new Error(t('pool.validation.noGeneratedFields'))
    configuration = value as BoxDraft
  } catch (error) {
    message.warning(error instanceof Error ? error.message : t('pool.validation.formatError'))
    return
  }
  const payload = JSON.stringify(configuration)
  if (pending?.payload !== payload) pending = { payload, key: newRequestId() }
  const key = pending!.key
  void run(async () => {
    // Idempotency-Key 走 poolClient(key) 头；body 为 {box}（本仓 CreatePoolDraftReq）。
    accept(await poolClient(key).Create({ box: configuration }))
    pending = undefined
    emit('changed')
    message.success(t('pool.draftSaved'))
  })
}
function transition(action: 'request' | 'approve' | 'publish') {
  const value = current.value
  if (!publishable.value || !value) return
  const request = { poolId: String(value.poolId), version: String(value.version), expectedConfigHash: value.configHash! }
  void run(async () => {
    if (action === 'request') approval.value = await poolClient().RequestPublication(request)
    else if (action === 'approve') approval.value = await poolClient().ApprovePublication(request)
    else {
      accept(await poolClient().Publish(request))
      emit('changed')
    }
    message.success(action === 'request' ? t('pool.requestSubmitted') : action === 'approve' ? t('pool.approvalDone') : t('pool.published'))
  })
}
</script>

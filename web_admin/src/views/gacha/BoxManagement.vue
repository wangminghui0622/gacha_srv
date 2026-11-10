<template>
  <a-card :title="$t('box.title')" style="margin-bottom: 16px">
    <a-alert :message="$t('box.alert')" type="info" show-icon />
    <a-space wrap style="margin: 16px 0">
      <a-input v-model:value="boxId" :placeholder="$t('box.boxId')" :aria-label="$t('box.boxId')" :disabled="busy" />
      <a-button v-if="can('read')" :loading="busy" @click="read">{{ $t('box.read') }}</a-button>
    </a-space>
    <a-alert v-if="error" :message="$t('box.failed')" type="error" />
    <a-descriptions v-if="view" :column="2" bordered size="small">
      <a-descriptions-item :label="$t('box.boxId')">{{ view.boxId }}</a-descriptions-item>
      <a-descriptions-item :label="$t('box.boxName')">{{ view.boxName }}</a-descriptions-item>
      <a-descriptions-item :label="$t('box.price')">{{ view.price }}</a-descriptions-item>
      <a-descriptions-item :label="$t('box.soldOut')">{{ view.soldOut }}</a-descriptions-item>
      <a-descriptions-item :label="$t('box.description')" :span="2">{{ view.description || $t('common.none') }}</a-descriptions-item>
    </a-descriptions>
    <a-table v-if="view" :data-source="view.entries ?? []" :pagination="{ pageSize: 20 }" :row-key="(row: BoxDetailEntry) => row.entryId ?? ''" size="small" style="margin-top: 16px">
      <a-table-column :title="$t('pool.entryId')" data-index="entryId" />
      <a-table-column :title="$t('pool.name')" data-index="name" />
      <a-table-column :title="$t('pool.weight')" data-index="weight" />
      <a-table-column :title="$t('pool.remaining')" data-index="remaining" />
      <a-table-column :title="$t('pool.isPrize')" data-index="isPrize" />
      <a-table-column :title="$t('pool.probability')" data-index="probability" />
    </a-table>
    <a-collapse v-if="view" style="margin-top: 16px"><a-collapse-panel key="state" :header="$t('box.fullView')"><pre>{{ JSON.stringify(view, null, 2) }}</pre></a-collapse-panel></a-collapse>
    <a-form v-if="can('assign_lucky')" layout="vertical" style="margin-top: 16px">
      <a-form-item :label="$t('box.gashaponConfigId')"><a-input v-model:value="configId" :disabled="busy" /></a-form-item>
      <a-form-item :label="$t('box.userId')"><a-input v-model:value="userId" :disabled="busy" /></a-form-item>
      <a-form-item :label="$t('box.prizeIndex')"><a-input-number v-model:value="prizeIndex" :min="0" :max="4294967295" :precision="0" :disabled="busy" /></a-form-item>
      <a-popconfirm :title="$t('box.assignConfirm')" @confirm="assign"><a-button :disabled="busy || !configId">{{ $t('box.assign') }}</a-button></a-popconfirm>
    </a-form>
  </a-card>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { poolClient } from '@/api/pool'
import type { BoxDetail, BoxDetailEntry } from '@/api/pool'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'

const props = defineProps<{ initialBoxId?: string }>()
const { t } = useI18n()
const session = useSessionStore()
const can = (action: string) => allows(session.permissionNodes, 'gacha.pool', action)
const boxId = ref(props.initialBoxId ?? '')
const busy = ref(false)
const error = ref(false)
const view = ref<BoxDetail>()
// 本仓无 CreateBox 路由（M14 裁决）：箱子只读；幸运指派走扭蛋配置。
const configId = ref('')
const userId = ref('')
const prizeIndex = ref<number | null>(0)
function validId(value: string) {
  return /^[1-9]\d*$/.test(value) && BigInt(value) <= 18446744073709551615n
}
function validIndex(value: number | null) {
  return value !== null && Number.isInteger(value) && value >= 0 && value <= 4294967295
}
async function run(task: () => Promise<void>) {
  if (busy.value) return
  busy.value = true
  error.value = false
  try {
    await task()
  } catch {
    error.value = true
  } finally {
    busy.value = false
  }
}
function read() {
  if (!validId(boxId.value)) {
    message.warning(t('box.validation.invalidBoxId'))
    return
  }
  const id = boxId.value
  void run(async () => {
    view.value = undefined
    const result = await poolClient().GetBoxView({ boxId: id })
    if (boxId.value === id) view.value = result
  })
}
function assign() {
  if (!validId(configId.value) || !validId(userId.value) || !validIndex(prizeIndex.value)) {
    message.warning(t('box.validation.invalidAssign'))
    return
  }
  void run(async () => {
    await poolClient().AssignLuckyUser({ configId: configId.value, userId: userId.value, prizeIndex: prizeIndex.value ?? 0 })
    message.success(t('box.assigned'))
  })
}
watch(
  () => props.initialBoxId,
  (id) => {
    boxId.value = id ?? ''
    if (id && can('read')) read()
  },
  { immediate: true },
)
watch(boxId, (id) => {
  if (view.value?.boxId !== id) view.value = undefined
})
</script>

<style scoped>pre { overflow: auto; max-height: 480px; white-space: pre-wrap; overflow-wrap: anywhere; }</style>

<template>
  <section>
    <PageHeader :title="$t('promotion.title')" />
    <a-card v-if="can('promotion.coupon_template', 'read')" :title="$t('promotion.publishedVersions')" style="margin-bottom: 16px">
      <a-alert v-if="listError" type="error" :message="$t('promotion.listFailed')" show-icon />
      <a-table :data-source="templates" :loading="listLoading" :pagination="false" :row-key="(row: CouponTemplate) => `${row.templateId}:${row.version}`" :scroll="{ x: 700 }" size="small">
        <a-table-column :title="$t('promotion.templateId')" data-index="templateId" />
        <a-table-column :title="$t('common.version')" data-index="version" />
        <a-table-column :title="$t('promotion.kind')"><template #default="{ record }">{{ kindLabel(record.kind) ?? record.kind }}</template></a-table-column>
        <a-table-column :title="$t('promotion.amountMinor')" data-index="amountMinor" />
        <a-table-column :title="$t('promotion.currency')" data-index="currency" />
        <a-table-column :title="$t('common.action')"><template #default="{ record }"><a-button type="link" @click="selectTemplate(record)">{{ $t('promotion.loadConfig') }}</a-button></template></a-table-column>
      </a-table>
      <a-space style="margin-top: 16px">
        <a-button :disabled="listLoading || cursors.length < 2" @click="previousPage">{{ $t('common.previousPage') }}</a-button>
        <span>{{ $t('common.pageN', { n: cursors.length }) }}</span>
        <a-button :disabled="listLoading || !nextCursor.id" @click="nextPage">{{ $t('common.nextPage') }}</a-button>
        <a-button :loading="listLoading" @click="loadTemplates">{{ $t('common.refresh') }}</a-button>
      </a-space>
    </a-card>
    <a-alert :message="$t('promotion.immutableAlert')" type="info" show-icon style="margin-bottom: 16px" />
    <a-card :title="$t('promotion.templateConfig')" :loading="reading">
      <a-form layout="vertical" @finish="publish">
        <a-row :gutter="16">
          <a-col :xs="24" :md="8"><a-form-item :label="$t('promotion.templateId')"><a-input v-model:value="draft.templateId" inputmode="numeric" /></a-form-item></a-col>
          <a-col :xs="24" :md="8"><a-form-item :label="$t('promotion.couponKind')"><a-select v-model:value="draft.kind" :options="kindOptions" /></a-form-item></a-col>
          <a-col :xs="24" :md="8"><a-form-item :label="$t('promotion.funder')"><a-select v-model:value="draft.funder" :options="funderOptions" /></a-form-item></a-col>
          <a-col :xs="24" :md="8"><a-form-item :label="$t('promotion.scene')"><a-select v-model:value="draft.scene" :options="sceneOptions" /></a-form-item></a-col>
          <a-col v-for="field in textFields" :key="field.key" :xs="24" :md="8">
            <a-form-item :label="field.label"><a-input v-model:value="draft[field.key]" /></a-form-item>
          </a-col>
          <a-col v-for="field in numberFields" :key="field.key" :xs="24" :md="8">
            <a-form-item :label="field.label"><a-input-number v-model:value="draft[field.key]" :min="0" :max="field.max" :precision="0" style="width: 100%" /></a-form-item>
          </a-col>
        </a-row>
        <a-space wrap>
          <a-button v-if="can('promotion.coupon_template', 'read')" :loading="reading" @click="readTemplate">{{ $t('promotion.readLatest') }}</a-button>
          <a-button v-if="can('promotion.coupon_template', 'publish')" type="primary" html-type="submit" :loading="publishing">{{ $t('promotion.publishNew') }}</a-button>
          <span v-if="publishedVersion">{{ $t('promotion.readVersion', { version: publishedVersion }) }}</span>
        </a-space>
      </a-form>
    </a-card>
    <a-card v-if="can('promotion.tenant_loyalty', 'read') || can('promotion.tenant_loyalty', 'write')" :title="$t('promotion.loyaltyTitle')" style="margin-top: 16px" :loading="loyaltyLoading">
      <a-alert :message="$t('promotion.loyaltyAlert')" type="info" show-icon />
      <a-form layout="vertical" style="margin-top: 16px" @finish="saveLoyalty">
        <a-form-item :label="$t('promotion.loyaltyPointsPerMajor')"><a-input v-model:value="loyalty.loyaltyPointsPerMajor" inputmode="numeric" /></a-form-item>
        <a-form-item :label="$t('promotion.maxGrossBps')"><a-input-number v-model:value="loyalty.maxGrossBps" :min="0" :max="10000" :precision="0" /></a-form-item>
        <a-space>
          <a-button v-if="can('promotion.tenant_loyalty', 'read')" @click="loadLoyalty">{{ $t('promotion.readConfig') }}</a-button>
          <a-button v-if="can('promotion.tenant_loyalty', 'write')" type="primary" html-type="submit" :loading="loyaltySaving">{{ $t('promotion.saveConfig') }}</a-button>
        </a-space>
      </a-form>
    </a-card>
    <a-card v-if="can('promotion.user_coupon', 'grant')" :title="$t('promotion.grantTitle')" style="margin-top: 16px">
      <a-form layout="vertical" @finish="grant">
        <a-form-item :label="$t('promotion.userId')"><a-input v-model:value="grantForm.userId" inputmode="numeric" /></a-form-item>
        <a-form-item :label="$t('promotion.templateId')"><a-input v-model:value="grantForm.templateId" inputmode="numeric" /></a-form-item>
        <a-form-item :label="$t('promotion.count')"><a-input-number v-model:value="grantForm.count" :min="1" :max="1000" :precision="0" /></a-form-item>
        <a-form-item :label="$t('promotion.customAmountMinor')"><a-input v-model:value="grantForm.customAmountMinor" inputmode="numeric" /></a-form-item>
        <a-button type="primary" html-type="submit" :loading="granting">{{ $t('promotion.confirmGrant') }}</a-button>
      </a-form>
      <a-table v-if="issued.length" :data-source="issued" row-key="id" size="small" :scroll="{ x: 600 }" style="margin-top: 16px">
        <a-table-column :title="$t('promotion.couponId')" data-index="id" />
        <a-table-column :title="$t('promotion.userId')" data-index="userId" />
        <a-table-column :title="$t('promotion.templateVersion')" data-index="templateVersion" />
        <a-table-column :title="$t('common.status')" data-index="status" />
      </a-table>
    </a-card>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import PageHeader from '@/components/PageHeader.vue'
import { promotionClient } from '@/api/promotion'
import type { CouponTemplate, UserCoupon } from '@/api/promotion'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'
import { newRequestId } from '@/utils/requestId'

const session = useSessionStore()
const { t } = useI18n()
const can = (resource: string, action: string) => allows(session.permissionNodes, resource, action)
const kindKeys: Record<string, string> = {
  COUPON_TEMPLATE_KIND_FIXED_MINOR: 'fixedMinor',
  COUPON_TEMPLATE_KIND_PERCENT_BPS: 'percentBps',
  COUPON_TEMPLATE_KIND_FREE: 'free',
  COUPON_TEMPLATE_KIND_CUSTOM_MINOR: 'customMinor',
  COUPON_TEMPLATE_KIND_DRAW_REWARD: 'drawReward',
}
function kindLabel(kind: string | number | undefined) {
  const key = kindKeys[String(kind)]
  return key ? t(`promotion.kinds.${key}`) : undefined
}
const kindOptions = computed(() =>
  Object.entries(kindKeys).map(([value, key]) => ({ value, label: t(`promotion.kinds.${key}`) })),
)
const funderOptions = computed(() => [
  { value: 'DISCOUNT_FUNDER_PLATFORM', label: t('promotion.funders.platform') },
  { value: 'DISCOUNT_FUNDER_MERCHANT', label: t('promotion.funders.merchant') },
])
const sceneOptions = computed(() => [
  { value: 'CHECKOUT_SCENE_UNSPECIFIED', label: t('promotion.scenes.all') },
  { value: 'CHECKOUT_SCENE_GACHA_DRAW', label: t('promotion.scenes.gachaDraw') },
  { value: 'CHECKOUT_SCENE_MEMORY_MATCH_PURCHASE', label: t('promotion.scenes.memoryMatch') },
])
const draft = reactive<CouponTemplate>({
  templateId: '',
  version: undefined,
  tenantId: undefined,
  status: undefined,
  kind: 'COUPON_TEMPLATE_KIND_FIXED_MINOR',
  amountMinor: '0',
  minGrossMinor: '0',
  currency: 'CNY',
  funder: 'DISCOUNT_FUNDER_PLATFORM',
  scene: 'CHECKOUT_SCENE_UNSPECIFIED',
  percentBps: 0,
  minUnitPriceMinor: '0',
  maxUnitPriceMinor: '0',
  minItemCount: 0,
  maxItemCount: 0,
  targetBusinessKey: '',
  validityDays: 0,
  rewardTemplateId: '0',
  rewardCount: 0,
})
const textFields = computed(() => [
  { key: 'currency', label: t('promotion.fields.currency') },
  { key: 'amountMinor', label: t('promotion.fields.amountMinor') },
  { key: 'minGrossMinor', label: t('promotion.fields.minGrossMinor') },
  { key: 'minUnitPriceMinor', label: t('promotion.fields.minUnitPriceMinor') },
  { key: 'maxUnitPriceMinor', label: t('promotion.fields.maxUnitPriceMinor') },
  { key: 'targetBusinessKey', label: t('promotion.fields.targetBusinessKey') },
  { key: 'rewardTemplateId', label: t('promotion.fields.rewardTemplateId') },
] as const)
const numberFields = computed(() => [
  { key: 'percentBps', label: t('promotion.fields.percentBps'), max: 10000 },
  { key: 'minItemCount', label: t('promotion.fields.minItemCount'), max: 4294967295 },
  { key: 'maxItemCount', label: t('promotion.fields.maxItemCount'), max: 4294967295 },
  { key: 'validityDays', label: t('promotion.fields.validityDays'), max: 4294967295 },
  { key: 'rewardCount', label: t('promotion.fields.rewardCount'), max: 1000 },
] as const)
const grantForm = reactive({ userId: '', templateId: '', count: 1, customAmountMinor: '0' })
const reading = ref(false)
const publishing = ref(false)
const granting = ref(false)
const publishedVersion = ref('')
const issued = ref<UserCoupon[]>([])
const templates = ref<CouponTemplate[]>([])
const listLoading = ref(false)
const listError = ref(false)
const cursors = ref([{ id: '', version: '' }])
const nextCursor = ref({ id: '', version: '' })
const loyalty = reactive({ loyaltyPointsPerMajor: '0', maxGrossBps: 0 })
const loyaltyLoading = ref(false)
const loyaltySaving = ref(false)
function selectTemplate(row: CouponTemplate) {
  Object.assign(draft, row)
  publishedVersion.value = String(row.version ?? '')
  grantForm.templateId = String(row.templateId ?? '')
}
async function loadTemplates() {
  listLoading.value = true
  listError.value = false
  try {
    const cursor = cursors.value.at(-1)!
    const result = await promotionClient().ListCouponTemplates({ tenantId: tenant(), afterTemplateId: cursor.id || '0', afterVersion: cursor.version || '0', pageSize: 20 })
    templates.value = result.templates ?? []
    nextCursor.value = { id: result.nextAfterTemplateId && result.nextAfterTemplateId !== '0' ? result.nextAfterTemplateId : '', version: result.nextAfterVersion ?? '' }
  } catch {
    listError.value = true
    templates.value = []
    nextCursor.value = { id: '', version: '' }
  } finally {
    listLoading.value = false
  }
}
function nextPage() {
  if (nextCursor.value.id) {
    cursors.value.push({ ...nextCursor.value })
    void loadTemplates()
  }
}
function previousPage() {
  if (cursors.value.length > 1) {
    cursors.value.pop()
    void loadTemplates()
  }
}
async function loadLoyalty() {
  loyaltyLoading.value = true
  try {
    const result = await promotionClient().GetLoyalty({ tenantId: tenant() })
    loyalty.loyaltyPointsPerMajor = result.loyaltyPointsPerMajor ?? '0'
    loyalty.maxGrossBps = result.maxGrossBps ?? 0
  } catch {
    // 请求层已展示失败信息；不覆盖当前输入。
  } finally {
    loyaltyLoading.value = false
  }
}
async function saveLoyalty() {
  if (!/^\d+$/.test(loyalty.loyaltyPointsPerMajor) || BigInt(loyalty.loyaltyPointsPerMajor) > 18446744073709551615n || !Number.isInteger(loyalty.maxGrossBps) || loyalty.maxGrossBps < 0 || loyalty.maxGrossBps > 10000) {
    message.warning(t('promotion.validation.loyaltyInvalid'))
    return
  }
  loyaltySaving.value = true
  try {
    await promotionClient().PutLoyalty({ defaults: { tenantId: tenant(), ...loyalty } })
    message.success(t('promotion.loyaltySaved'))
  } catch {
    // 请求层已展示失败信息。
  } finally {
    loyaltySaving.value = false
  }
}
onMounted(() => {
  if (can('promotion.coupon_template', 'read')) void loadTemplates()
  if (can('promotion.tenant_loyalty', 'read')) void loadLoyalty()
})
// 相同内容的失败重试复用命令 ID；成功后下一次提交创建新命令。
const commands = new Map<string, { body: string; id: string }>()
function commandId(operation: string, payload: unknown) {
  const body = JSON.stringify(payload)
  const previous = commands.get(operation)
  if (previous?.body === body) return previous.id
  const id = newRequestId()
  commands.set(operation, { body, id })
  return id
}
function positiveId(value: string | undefined) {
  return !!value && /^[1-9]\d*$/.test(value) && BigInt(value) <= 18446744073709551615n
}
function tenant() {
  if (!session.tenantId) throw new Error(t('promotion.validation.sessionMissingTenant'))
  return String(session.tenantId)
}
async function readTemplate() {
  if (!positiveId(draft.templateId as string | undefined)) {
    message.warning(t('promotion.validation.invalidTemplateId'))
    return
  }
  reading.value = true
  try {
    const result = await promotionClient().GetCouponTemplate({ tenantId: tenant(), templateId: String(draft.templateId) })
    Object.assign(draft, result)
    publishedVersion.value = String(result.version ?? '')
  } finally {
    reading.value = false
  }
}
async function publish() {
  if (!positiveId(draft.templateId as string | undefined)) {
    message.warning(t('promotion.validation.invalidTemplateId'))
    return
  }
  for (const key of ['amountMinor', 'minGrossMinor', 'minUnitPriceMinor', 'maxUnitPriceMinor'] as const) {
    const value = draft[key] ?? '0'
    if (!/^\d+$/.test(String(value)) || BigInt(value) > 9223372036854775807n) {
      message.warning(t('promotion.validation.amountInvalid'))
      return
    }
  }
  const template: CouponTemplate = { ...draft, tenantId: tenant(), version: undefined, status: undefined }
  if (template.kind !== 'COUPON_TEMPLATE_KIND_PERCENT_BPS') template.percentBps = 0
  if (['COUPON_TEMPLATE_KIND_PERCENT_BPS', 'COUPON_TEMPLATE_KIND_FREE', 'COUPON_TEMPLATE_KIND_DRAW_REWARD'].includes(String(template.kind ?? ''))) template.amountMinor = '0'
  if (template.kind !== 'COUPON_TEMPLATE_KIND_DRAW_REWARD') {
    template.rewardTemplateId = '0'
    template.rewardCount = 0
  }
  if (template.kind === 'COUPON_TEMPLATE_KIND_DRAW_REWARD' && !positiveId(template.rewardTemplateId as string | undefined)) {
    message.warning(t('promotion.validation.rewardTemplateRequired'))
    return
  }
  publishing.value = true
  try {
    const result = await promotionClient(commandId('publish', template)).PublishCouponTemplate({ template })
    Object.assign(draft, result)
    publishedVersion.value = String(result.version ?? '')
    commands.delete('publish')
    message.success(t('promotion.published'))
  } finally {
    publishing.value = false
  }
}
async function grant() {
  if (!positiveId(grantForm.userId) || !positiveId(grantForm.templateId) || !Number.isInteger(grantForm.count) || grantForm.count < 1 || grantForm.count > 1000 || !/^\d+$/.test(grantForm.customAmountMinor) || BigInt(grantForm.customAmountMinor) > 9223372036854775807n) {
    message.warning(t('promotion.validation.grantInvalid'))
    return
  }
  const payload = { ...grantForm, tenantId: tenant() }
  granting.value = true
  try {
    const result = await promotionClient(commandId('grant', payload)).GrantCoupons(payload)
    issued.value = result.coupons ?? []
    commands.delete('grant')
    message.success(t('promotion.grantSuccess'))
  } finally {
    granting.value = false
  }
}
</script>

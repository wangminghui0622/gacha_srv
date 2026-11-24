<template>
  <section class="coupon-page">
    <div class="coupon-toolbar">
      <a-button class="coupon-query-btn" type="primary" :loading="loading" @click="reload">{{ $t('couponAdmin.query') }}</a-button>
      <a-button v-if="canPublish && activeTab === 'coupons'" class="coupon-create-btn" @click="openCreate">{{ $t('couponAdmin.create') }}</a-button>
      <a-button v-if="canPublish" class="coupon-grant-btn" @click="openDrawCreate">{{ $t('couponAdmin.grantRule') }}</a-button>
    </div>

    <a-tabs v-model:activeKey="activeTab" class="coupon-tabs">
      <a-tab-pane key="coupons" :tab="$t('couponAdmin.tabCoupons')">
        <a-table
          :data-source="pagedTemplates"
          :loading="loading"
          :pagination="false"
          :row-key="(row: CouponTemplate) => String(row.templateId)"
          :scroll="{ x: 1180 }"
          size="middle"
        >
          <a-table-column :title="$t('couponAdmin.id')" width="80" align="center">
            <template #default="{ record }">{{ record.templateId }}</template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.name')" min-width="300" align="center">
            <template #default="{ record }">{{ record.name || '-' }}</template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.type')" width="160" align="center">
            <template #default="{ record }">{{ kindText(record.kind) }}</template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.usageLimit')" width="130" align="center">
            <template #default="{ record }">{{ usageLimitText(record) }}</template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.limitTarget')" width="150" align="center">
            <template #default="{ record }">{{ limitTargetText(record) }}</template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.status')" width="110" align="center">
            <template #default="{ record }">
              <a class="coupon-status-link" @click="toggleStatus(record)">{{ isEnabled(record) ? $t('couponAdmin.enabled') : $t('couponAdmin.disabled') }}</a>
            </template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.validityDays')" width="120" align="center">
            <template #default="{ record }">{{ record.validityDays ?? 0 }}</template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.action')" width="130" align="center" fixed="right">
            <template #default="{ record }">
              <a-button class="coupon-edit-btn" size="small" @click="openEdit(record)">{{ $t('couponAdmin.edit') }}</a-button>
            </template>
          </a-table-column>
        </a-table>

        <div class="coupon-footer">
          <a-pagination
            v-model:current="page"
            v-model:page-size="pageSize"
            :total="latestCouponTemplates.length"
            :page-size-options="['15', '30', '50']"
            show-size-changer
            show-less-items
            :locale="{ items_per_page: $t('couponAdmin.itemsPerPage') }"
          />
        </div>
      </a-tab-pane>
      <a-tab-pane key="rules" :tab="$t('couponAdmin.tabGrantRules')">
        <a-table
          :data-source="pagedDrawTemplates"
          :loading="loading"
          :pagination="false"
          :row-key="(row: CouponTemplate) => String(row.templateId)"
          :scroll="{ x: 980 }"
          size="middle"
        >
          <a-table-column :title="$t('couponAdmin.id')" width="80" align="center">
            <template #default="{ record }">{{ record.templateId }}</template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.couponTemplateName')" min-width="220" align="center">
            <template #default="{ record }">{{ rewardCouponText(record) }}</template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.couponCount')" width="110" align="center">
            <template #default="{ record }">{{ record.rewardCount ?? 0 }}</template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.startTime')" min-width="180" align="center">
            <template #default="{ record }">{{ startTimeText(record) }}</template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.drawCount')" width="130" align="center">
            <template #default="{ record }">{{ record.minItemCount ?? 0 }}</template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.status')" width="110" align="center">
            <template #default="{ record }">
              <a class="coupon-status-link" @click="toggleStatus(record)">{{ isEnabled(record) ? $t('couponAdmin.enabled') : $t('couponAdmin.disabled') }}</a>
            </template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.action')" width="110" align="center" fixed="right">
            <template #default="{ record }">
              <a-button class="coupon-edit-btn" size="small" @click="openDrawEdit(record)">{{ $t('couponAdmin.edit') }}</a-button>
            </template>
          </a-table-column>
        </a-table>

        <div class="coupon-footer">
          <a-pagination
            v-model:current="drawPage"
            v-model:page-size="drawPageSize"
            :total="latestDrawTemplates.length"
            :page-size-options="['15', '30', '50']"
            show-size-changer
            show-less-items
            :locale="{ items_per_page: $t('couponAdmin.itemsPerPage') }"
          />
        </div>
      </a-tab-pane>
    </a-tabs>

    <a-modal
      v-model:open="modalOpen"
      centered
      :title="$t('couponAdmin.modalTitle')"
      width="760px"
      :style="{ maxWidth: '96vw' }"
    >
      <div class="coupon-choice-area">
        <div class="coupon-choice-grid">
          <span class="coupon-choice-title coupon-choice-label">
            <span class="coupon-choice-mark">*</span>
            <span class="coupon-choice-text">{{ $t('couponAdmin.typeLabel') }}：</span>
          </span>
          <a-radio-group v-model:value="form.kind" class="coupon-radio-group coupon-radio-group-type">
            <a-radio v-for="option in kindOptions" :key="option.value" :value="option.value">{{ option.label }}</a-radio>
          </a-radio-group>
          <span class="coupon-choice-title coupon-choice-label">
            <span class="coupon-choice-mark"></span>
            <span class="coupon-choice-text">{{ $t('couponAdmin.scopeLabel') }}：</span>
          </span>
          <a-radio-group v-model:value="form.scope" class="coupon-radio-group coupon-radio-group-scope">
            <a-radio v-for="option in scopeOptions" :key="option.value" :value="option.value">{{ option.label }}</a-radio>
          </a-radio-group>
        </div>
      </div>

      <a-form class="coupon-form" :label-col="{ span: 7 }" :wrapper-col="{ span: 17 }">
        <div class="coupon-form-grid">
          <a-form-item class="coupon-form-left" :label="$t('couponAdmin.nameLabel')" required>
            <a-input v-model:value="form.name" :placeholder="$t('couponAdmin.nameLabel')" :maxlength="128" style="width: 100%" />
          </a-form-item>
          <a-form-item class="coupon-form-left" :label="$t('couponAdmin.descriptionLabel')" required>
            <a-input v-model:value="form.description" :placeholder="$t('couponAdmin.descriptionLabel')" :maxlength="512" style="width: 100%" />
          </a-form-item>
          <a-form-item class="coupon-form-left" :label="$t('couponAdmin.validityDaysLabel')" required>
            <div class="coupon-validity-row">
              <a-input-number v-model:value="form.validityDays" :min="0" :precision="0" :controls="false" style="width: 4em" />
              <span class="coupon-inline-label">{{ $t('couponAdmin.naturalDay') }}</span>
              <a-switch v-model:checked="form.naturalDay" />
            </div>
          </a-form-item>
          <div class="coupon-form-left coupon-price-row" style="padding-left: 14px; box-sizing: border-box">
            <div class="coupon-price-pair" style="gap: 0; flex: none">
              <span class="coupon-price-label" style="display: inline-flex; align-items: center; white-space: nowrap; flex: none; width: auto">
                <span style="display: inline-block; flex: 0 0 90.7px; width: 90.7px; text-align: justify; text-align-last: justify">{{ $t('couponAdmin.priceMinShort') }}</span>
                <span style="margin-left: 2px; margin-right: 8px">:</span>
              </span>
              <a-input-number v-model:value="form.minUnitPriceYuan" :min="0" :precision="0" :controls="false" style="width: 60px" />
            </div>
            <div class="coupon-price-pair" style="gap: 0; flex: none">
              <span class="coupon-price-label" style="display: inline-flex; align-items: center; white-space: nowrap; flex: none; width: auto">
                <span style="display: inline-block; flex: 0 0 90.7px; width: 90.7px; text-align: justify; text-align-last: justify">{{ $t('couponAdmin.priceMaxShort') }}</span>
                <span style="margin-left: 2px; margin-right: 8px">:</span>
              </span>
              <a-input-number v-model:value="form.maxUnitPriceYuan" :min="0" :precision="0" :controls="false" style="width: 60px" />
            </div>
          </div>
          <a-form-item
            v-if="isPercent"
            class="coupon-form-right coupon-form-discount"
            :class="{ 'coupon-form-discount-bottom': form.scope === 'mode' || form.scope === 'pool' }"
            :label="$t('couponAdmin.discountPercent')"
            required
          >
            <a-input-number
              v-model:value="form.discountPercent"
              :min="0.01"
              :max="100"
              :precision="2"
              :placeholder="$t('couponAdmin.discountPlaceholder')"
              style="width: 210px"
            />
          </a-form-item>
          <a-form-item class="coupon-form-right coupon-form-row1" :label="$t('couponAdmin.totalThreshold')" :required="isFixedOrCustom">
            <a-input-number v-model:value="form.minGrossYuan" :min="0" :precision="2" style="width: 210px" />
          </a-form-item>
          <a-form-item class="coupon-form-right coupon-form-row2" :label="$t('couponAdmin.rewardAmount')" :required="amountRequired">
            <a-input-number v-model:value="form.rewardAmountYuan" :min="0" :precision="2" style="width: 210px" />
          </a-form-item>
          <a-form-item
            v-if="form.scope === 'mode'"
            class="coupon-form-right coupon-form-follow"
            :label="$t('couponAdmin.useMode')"
            required
          >
            <a-select v-model:value="form.useMode" :options="useModeOptions" :placeholder="$t('couponAdmin.useMode')" style="width: 210px" />
          </a-form-item>
          <a-form-item
            v-if="form.scope === 'pool'"
            class="coupon-form-right coupon-form-follow"
            :label="$t('couponAdmin.poolId')"
            required
          >
            <a-input v-model:value="form.poolId" :placeholder="$t('couponAdmin.poolId')" inputmode="numeric" style="width: 210px" />
          </a-form-item>
        </div>
      </a-form>
      <template #footer>
        <div class="coupon-modal-footer" style="display: flex; justify-content: center; gap: 12px">
          <a-button @click="modalOpen = false">{{ $t('couponAdmin.cancel') }}</a-button>
          <a-button type="primary" :loading="saving" @click="save">{{ $t('couponAdmin.confirm') }}</a-button>
        </div>
      </template>
    </a-modal>

    <a-modal v-model:open="drawOpen" centered :title="$t('couponAdmin.grantRuleTitle')" width="720px" :footer="null">
      <a-form class="coupon-grant-form" :label-col="{ span: 6 }" :wrapper-col="{ span: 18 }">
        <a-form-item :label="$t('couponAdmin.couponTemplateName')" required>
          <a-select
            v-model:value="drawForm.rewardTemplateId"
            :options="drawCouponOptions"
            show-search
            option-filter-prop="label"
            :placeholder="$t('couponAdmin.couponTemplatePlaceholder')"
            style="width: 16em"
          />
        </a-form-item>
        <a-form-item :label="$t('couponAdmin.couponCount')" required>
          <a-input-number v-model:value="drawForm.rewardCount" :min="1" :max="100" :precision="0" style="width: 16em" />
        </a-form-item>
        <a-form-item :label="$t('couponAdmin.startTime')" required>
          <a-date-picker v-model:value="drawForm.startAt" show-time format="YYYY-MM-DD HH:mm:ss" style="width: 16em" />
        </a-form-item>
        <a-form-item :label="$t('couponAdmin.drawCount')" required>
          <a-input-number v-model:value="drawForm.minItemCount" :min="1" :precision="0" style="width: 16em" />
        </a-form-item>
      </a-form>
      <div class="coupon-modal-footer" style="display: flex; justify-content: center; gap: 12px">
        <a-button @click="drawOpen = false">{{ $t('couponAdmin.cancel') }}</a-button>
        <a-button type="primary" :loading="drawSaving" @click="submitDraw">{{ $t('couponAdmin.confirm') }}</a-button>
      </div>
    </a-modal>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import type { Dayjs } from 'dayjs'
import { useI18n } from 'vue-i18n'
import { promotionClient } from '@/api/promotion'
import type { CouponTemplate } from '@/api/promotion'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'
import { newRequestId } from '@/utils/requestId'

const session = useSessionStore()
const { t } = useI18n()
const canRead = computed(() => allows(session.permissionNodes, 'promotion.coupon_template', 'read'))
const canPublish = computed(() => allows(session.permissionNodes, 'promotion.coupon_template', 'publish'))
const activeTab = ref('coupons')
const drawPage = ref(1)
const drawPageSize = ref(15)

const rawTemplates = ref<CouponTemplate[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(15)

// 优惠券列表完善：同一 templateId 仅展示最新版本，前端按 15 条/页分页；
// 状态切换通过发布新版本实现（禁用态 status=2，兼容旧数据的 enabled 字段）。
const latestTemplates = computed(() => {
  const latest = new Map<string, CouponTemplate>()
  for (const item of rawTemplates.value) {
    const key = String(item.templateId ?? '')
    const current = latest.get(key)
    if (!current || Number(item.version ?? 0) > Number(current.version ?? 0)) latest.set(key, item)
  }
  return [...latest.values()].sort((a, b) => Number(b.templateId ?? 0) - Number(a.templateId ?? 0))
})
const latestCouponTemplates = computed(() =>
  latestTemplates.value.filter((item) => normalizeKind(item.kind) !== 'COUPON_TEMPLATE_KIND_DRAW_REWARD'),
)
const latestDrawTemplates = computed(() =>
  latestTemplates.value.filter((item) => normalizeKind(item.kind) === 'COUPON_TEMPLATE_KIND_DRAW_REWARD'),
)
const pagedTemplates = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return latestCouponTemplates.value.slice(start, start + pageSize.value)
})
const pagedDrawTemplates = computed(() => {
  const start = (drawPage.value - 1) * drawPageSize.value
  return latestDrawTemplates.value.slice(start, start + drawPageSize.value)
})

function tenant() {
  if (!session.tenantId) throw new Error('tenant missing')
  return String(session.tenantId)
}

async function load() {
  if (!canRead.value) return
  loading.value = true
  try {
    const all: CouponTemplate[] = []
    let afterTemplateId = '0'
    let afterVersion = '0'
    for (let i = 0; i < 20; i += 1) {
      const result = await promotionClient().ListCouponTemplates({
        tenantId: tenant(),
        afterTemplateId,
        afterVersion,
        pageSize: 100,
      })
      all.push(...(result.templates ?? []))
      const nextId = result.nextAfterTemplateId && result.nextAfterTemplateId !== '0' ? result.nextAfterTemplateId : ''
      if (!nextId) break
      afterTemplateId = nextId
      afterVersion = result.nextAfterVersion ?? '0'
    }
    rawTemplates.value = all
    const maxPage = Math.max(1, Math.ceil(latestCouponTemplates.value.length / pageSize.value))
    if (page.value > maxPage) page.value = maxPage
    const maxDrawPage = Math.max(1, Math.ceil(latestDrawTemplates.value.length / drawPageSize.value))
    if (drawPage.value > maxDrawPage) drawPage.value = maxDrawPage
  } catch {
    message.error(t('couponAdmin.loadFailed'))
  } finally {
    loading.value = false
  }
}

function reload() {
  page.value = 1
  drawPage.value = 1
  void load()
}

function kindText(kind: string | number | undefined) {
  switch (String(kind ?? '')) {
    case '1':
    case 'COUPON_TEMPLATE_KIND_FIXED_MINOR':
      return t('couponAdmin.kindFixed')
    case '2':
    case 'COUPON_TEMPLATE_KIND_PERCENT_BPS':
      return t('couponAdmin.kindPercent')
    case '3':
    case 'COUPON_TEMPLATE_KIND_FREE':
      return t('couponAdmin.kindFree')
    case '4':
    case 'COUPON_TEMPLATE_KIND_CUSTOM_MINOR':
      return t('couponAdmin.kindCustom')
    case '5':
    case 'COUPON_TEMPLATE_KIND_DRAW_REWARD':
      return t('couponAdmin.kindDrawReward')
    default:
      return '-'
  }
}

function isEnabled(record: CouponTemplate) {
  const status = String(record.status ?? '')
  if (status === '2' || status.endsWith('DISABLED')) return false
  return record.enabled !== false
}

function usageLimitText(record: CouponTemplate) {
  const scope = couponScope(record)
  if (scope.kind === 'mode') return t('couponAdmin.scopeMode')
  if (scope.kind === 'pool') return t('couponAdmin.scopePool')
  return t('couponAdmin.unlimited')
}

function limitTargetText(record: CouponTemplate) {
  const scope = couponScope(record)
  if (scope.kind === 'mode') return modeLabel(scope.target)
  if (scope.kind === 'pool') return t('couponAdmin.poolTarget', { id: scope.target })
  return '-'
}

function targetBusinessKey(record: CouponTemplate) {
  return String(record.targetBusinessKey ?? '').trim()
}

const kindOptions = computed(() => [
  { value: 'COUPON_TEMPLATE_KIND_FIXED_MINOR', label: t('couponAdmin.kindFixed') },
  { value: 'COUPON_TEMPLATE_KIND_CUSTOM_MINOR', label: t('couponAdmin.kindCustom') },
  { value: 'COUPON_TEMPLATE_KIND_FREE', label: t('couponAdmin.kindFree') },
  { value: 'COUPON_TEMPLATE_KIND_PERCENT_BPS', label: t('couponAdmin.kindPercent') },
])
const scopeOptions = computed(() => [
  { value: 'unlimited', label: t('couponAdmin.scopeUnlimited') },
  { value: 'mode', label: t('couponAdmin.scopeMode') },
  { value: 'pool', label: t('couponAdmin.scopePool') },
])
const useModeOptions = computed(() => [
  { value: 'gashapon', label: t('poolAdmin.modeGashapon') },
  { value: 'fullcar', label: t('poolAdmin.modeFullcar') },
  { value: 'combochain', label: t('poolAdmin.modeCombochain') },
  { value: 'dig', label: t('poolAdmin.modeDig') },
  { value: 'groupbuy', label: t('poolAdmin.modeGroupbuy') },
  { value: 'memorymatch', label: t('poolAdmin.modeMemorymatch') },
])
const isFixedOrCustom = computed(
  () => form.kind === 'COUPON_TEMPLATE_KIND_FIXED_MINOR' || form.kind === 'COUPON_TEMPLATE_KIND_CUSTOM_MINOR',
)
const isPercent = computed(() => form.kind === 'COUPON_TEMPLATE_KIND_PERCENT_BPS')
const amountRequired = computed(() => isFixedOrCustom.value)

const modalOpen = ref(false)
const saving = ref(false)
const togglingId = ref('')
const form = reactive({
  templateId: '',
  name: '',
  description: '',
  kind: undefined as string | undefined,
  scope: undefined as string | undefined,
  useMode: undefined as string | undefined,
  poolId: '',
  validityDays: 0,
  naturalDay: false,
  minGrossYuan: 0,
  minUnitPriceYuan: 0,
  maxUnitPriceYuan: 0,
  minItemCount: 0,
  maxItemCount: 0,
  rewardAmountYuan: 0,
  discountPercent: undefined as number | undefined,
  rewardCount: 0,
  rewardTemplateId: '0',
  enabled: true,
})

function fenToYuan(value: string | number | undefined) {
  return Number(value ?? 0) / 100
}
function yuanToFen(value: number | undefined) {
  return Math.max(0, Math.round(Number(value ?? 0) * 100))
}
type CouponScope = { kind: 'unlimited' | 'mode' | 'pool'; target: string }

function couponScope(record: CouponTemplate): CouponScope {
  const key = targetBusinessKey(record)
  if (key.startsWith('mode:')) return { kind: 'mode', target: key.slice(5) }
  if (key.startsWith('pool:')) return { kind: 'pool', target: key.slice(5) }
  switch (Number(record.scene ?? 0)) {
    case 1:
      return { kind: 'mode', target: 'gashapon' }
    case 2:
      return { kind: 'mode', target: 'memorymatch' }
    default:
      return { kind: 'unlimited', target: '' }
  }
}

function sceneForSave() {
  if (form.scope !== 'mode') return 0
  if (form.useMode === 'gashapon') return 1
  if (form.useMode === 'memorymatch') return 2
  return 0
}

function targetBusinessKeyForSave() {
  if (form.scope === 'mode' && form.useMode) return `mode:${form.useMode}`
  if (form.scope === 'pool' && form.poolId.trim()) return `pool:${form.poolId.trim()}`
  return ''
}

function modeLabel(mode: string) {
  return useModeOptions.value.find((item) => item.value === mode)?.label ?? mode
}

function scopeOf(record: CouponTemplate) {
  return couponScope(record).kind
}

function openCreate() {
  Object.assign(form, {
    templateId: String(Math.max(0, ...latestTemplates.value.map((item) => Number(item.templateId ?? 0))) + 1),
    name: '',
    description: '',
    kind: 'COUPON_TEMPLATE_KIND_FIXED_MINOR',
    scope: 'unlimited',
    useMode: undefined,
    poolId: '',
    validityDays: 0,
    naturalDay: false,
    minGrossYuan: 0,
    minUnitPriceYuan: 5,
    maxUnitPriceYuan: 100,
    minItemCount: 0,
    maxItemCount: 0,
    rewardAmountYuan: 0,
    discountPercent: undefined,
    rewardCount: 0,
    rewardTemplateId: '0',
    enabled: true,
  })
  modalOpen.value = true
}

function openEdit(record: CouponTemplate) {
  const scope = couponScope(record)
  Object.assign(form, {
    templateId: String(record.templateId ?? ''),
    name: String(record.name ?? ''),
    description: String(record.description ?? ''),
    kind: normalizeKind(record.kind),
    scope: scope.kind,
    useMode: scope.kind === 'mode' ? scope.target : undefined,
    poolId: scope.kind === 'pool' ? scope.target : '',
    validityDays: Number(record.validityDays ?? 0),
    naturalDay: record.naturalDay === true,
    minGrossYuan: fenToYuan(record.minGrossMinor),
    minUnitPriceYuan: fenToYuan(record.minUnitPriceMinor),
    maxUnitPriceYuan: fenToYuan(record.maxUnitPriceMinor),
    minItemCount: Number(record.minItemCount ?? 0),
    maxItemCount: Number(record.maxItemCount ?? 0),
    rewardAmountYuan: fenToYuan(record.amountMinor),
    discountPercent: Number(record.percentBps ?? 0) / 100,
    rewardCount: Number(record.rewardCount ?? 0),
    rewardTemplateId: String(record.rewardTemplateId ?? '0'),
    enabled: isEnabled(record),
  })
  modalOpen.value = true
}

const drawOpen = ref(false)
const drawSaving = ref(false)
const drawForm = reactive({
  templateId: '',
  rewardTemplateId: undefined as string | undefined,
  rewardCount: 20,
  startAt: dayjs() as Dayjs | null,
  minItemCount: 1,
  enabled: true,
})
const drawCouponOptions = computed(() =>
  latestTemplates.value
    .filter((item) => normalizeKind(item.kind) !== 'COUPON_TEMPLATE_KIND_DRAW_REWARD')
    .map((item) => ({
      value: String(item.templateId ?? ''),
      label: `${item.name || '-'} (ID ${item.templateId})`,
    })),
)
const drawSelectedCoupon = computed(() =>
  latestTemplates.value.find(
    (item) => String(item.templateId ?? '') === String(drawForm.rewardTemplateId ?? ''),
  ),
)

function rewardCouponText(record: CouponTemplate) {
  const id = String(record.rewardTemplateId ?? '')
  if (!id || id === '0') return '-'
  const coupon = latestTemplates.value.find((item) => String(item.templateId ?? '') === id)
  return coupon?.name ? `${coupon.name} (ID ${id})` : `ID ${id}`
}

function startTimeText(record: CouponTemplate) {
  const ms = Number(record.startAtUnixMillis ?? 0)
  return ms > 0 ? dayjs(ms).format('YYYY-MM-DD HH:mm:ss') : t('couponAdmin.immediate')
}

function openDrawCreate() {
  Object.assign(drawForm, {
    templateId: String(Math.max(0, ...latestTemplates.value.map((item) => Number(item.templateId ?? 0))) + 1),
    rewardTemplateId: undefined,
    rewardCount: 20,
    startAt: dayjs(),
    minItemCount: 1,
    enabled: true,
  })
  drawOpen.value = true
}

function openDrawEdit(record: CouponTemplate) {
  const startMs = Number(record.startAtUnixMillis ?? 0)
  const rewardTemplateId = Number(record.rewardTemplateId ?? 0) > 0 ? String(record.rewardTemplateId) : undefined
  Object.assign(drawForm, {
    templateId: String(record.templateId ?? ''),
    rewardTemplateId,
    rewardCount: Number(record.rewardCount ?? 1),
    startAt: startMs > 0 ? dayjs(startMs) : dayjs(),
    minItemCount: Number(record.minItemCount ?? 0),
    enabled: isEnabled(record),
  })
  drawOpen.value = true
}

async function submitDraw() {
  if (!canPublish.value) return
  const rewardTemplateId = drawForm.rewardTemplateId ?? ''
  if (!positiveId(rewardTemplateId)) {
    message.warning(t('couponAdmin.couponRequired'))
    return
  }
  const rewardCount = Math.trunc(drawForm.rewardCount)
  if (rewardCount <= 0) {
    message.warning(t('couponAdmin.couponCountRequired'))
    return
  }
  if (!drawForm.startAt) {
    message.warning(t('couponAdmin.startTimeRequired'))
    return
  }
  const minItemCount = Math.trunc(drawForm.minItemCount)
  if (minItemCount <= 0) {
    message.warning(t('couponAdmin.drawCountRequired'))
    return
  }
  const templateId = drawForm.templateId || String(Math.max(0, ...latestTemplates.value.map((item) => Number(item.templateId ?? 0))) + 1)
  if (!positiveId(templateId)) {
    message.warning(t('couponAdmin.typeRequired'))
    return
  }
  drawSaving.value = true
  try {
    await promotionClient(newRequestId()).PublishCouponTemplate({
      template: {
        templateId,
        tenantId: tenant(),
        name: drawSelectedCoupon.value?.name || `券池-${templateId}`,
        description: drawSelectedCoupon.value?.description || '',
        kind: 'COUPON_TEMPLATE_KIND_DRAW_REWARD',
        amountMinor: 0,
        percentBps: 0,
        minGrossMinor: 0,
        minUnitPriceMinor: 0,
        maxUnitPriceMinor: 0,
        minItemCount,
        maxItemCount: 0,
        currency: 'CNY',
        funder: 'DISCOUNT_FUNDER_PLATFORM',
        validityDays: 0,
        naturalDay: false,
        enabled: drawForm.enabled,
        rewardTemplateId,
        rewardCount,
        startAtUnixMillis: drawForm.startAt.valueOf(),
      },
    })
    message.success(t('couponAdmin.saved'))
    drawOpen.value = false
    activeTab.value = 'rules'
    await load()
  } catch {
    // 错误提示由请求层统一处理
  } finally {
    drawSaving.value = false
  }
}

function normalizeKind(kind: string | number | undefined) {
  switch (String(kind ?? '')) {
    case '1':
      return 'COUPON_TEMPLATE_KIND_FIXED_MINOR'
    case '2':
      return 'COUPON_TEMPLATE_KIND_PERCENT_BPS'
    case '3':
      return 'COUPON_TEMPLATE_KIND_FREE'
    case '4':
      return 'COUPON_TEMPLATE_KIND_CUSTOM_MINOR'
    case '5':
      return 'COUPON_TEMPLATE_KIND_DRAW_REWARD'
    default:
      return String(kind ?? 'COUPON_TEMPLATE_KIND_FIXED_MINOR')
  }
}

function positiveId(value: string) {
  return /^[1-9]\d*$/.test(value) && BigInt(value) <= 18446744073709551615n
}

async function save() {
  if (!form.name.trim()) {
    message.warning(t('couponAdmin.nameRequired'))
    return
  }
  if (!form.description.trim()) {
    message.warning(t('couponAdmin.descriptionRequired'))
    return
  }
  if (!form.kind) {
    message.warning(t('couponAdmin.typeRequired'))
    return
  }
  if (!Number.isInteger(form.validityDays) || form.validityDays < 0) {
    message.warning(t('couponAdmin.validityRequired'))
    return
  }
  if (form.scope === 'mode' && !form.useMode) {
    message.warning(t('couponAdmin.useModeRequired'))
    return
  }
  if (form.scope === 'pool' && !positiveId(form.poolId.trim())) {
    message.warning(t('couponAdmin.poolIdRequired'))
    return
  }
  const templateId = form.templateId || String(Math.max(0, ...latestTemplates.value.map((item) => Number(item.templateId ?? 0))) + 1)
  if (!positiveId(templateId)) {
    message.warning(t('couponAdmin.typeRequired'))
    return
  }
  const kind = form.kind
  if (isFixedOrCustom.value && (yuanToFen(form.rewardAmountYuan) <= 0 || yuanToFen(form.minGrossYuan) <= 0)) {
    message.warning(t('couponAdmin.amountMustBePositive'))
    return
  }
  if (isPercent.value && (!form.discountPercent || form.discountPercent <= 0 || form.discountPercent > 100)) {
    message.warning(t('couponAdmin.discountMustBePositive'))
    return
  }
  const minUnitPriceMinor = yuanToFen(form.minUnitPriceYuan)
  const maxUnitPriceMinor = yuanToFen(form.maxUnitPriceYuan)
  if (maxUnitPriceMinor > 0 && minUnitPriceMinor > maxUnitPriceMinor) {
    message.warning(t('couponAdmin.unitPriceOrderInvalid'))
    return
  }
  const template: CouponTemplate = {
    templateId,
    tenantId: tenant(),
    name: form.name.trim(),
    description: form.description.trim(),
    kind,
    amountMinor: isFixedOrCustom.value ? yuanToFen(form.rewardAmountYuan) : 0,
    percentBps: isPercent.value ? Math.round((form.discountPercent ?? 0) * 100) : 0,
    minGrossMinor: yuanToFen(form.minGrossYuan),
    minUnitPriceMinor,
    maxUnitPriceMinor,
    minItemCount: 0,
    maxItemCount: 0,
    currency: 'CNY',
    funder: 'DISCOUNT_FUNDER_PLATFORM',
    scene: sceneForSave(),
    targetBusinessKey: targetBusinessKeyForSave(),
    validityDays: Math.trunc(form.validityDays),
    naturalDay: form.naturalDay,
    enabled: form.enabled,
    rewardTemplateId: form.rewardTemplateId.trim() || '0',
    rewardCount: Math.max(0, Math.trunc(form.rewardCount)),
  }
  saving.value = true
  try {
    await promotionClient(newRequestId()).PublishCouponTemplate({ template })
    message.success(t('couponAdmin.saved'))
    modalOpen.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function toggleStatus(record: CouponTemplate) {
  if (!canPublish.value) return
  const id = String(record.templateId ?? '')
  if (!id) return
  togglingId.value = id
  try {
    const latest = await promotionClient().GetCouponTemplate({ tenantId: tenant(), templateId: id })
    await promotionClient(newRequestId()).PublishCouponTemplate({
      template: {
        ...latest,
        version: undefined,
        status: undefined,
        enabled: !isEnabled(record),
      },
    })
    message.success(t('couponAdmin.statusSaved'))
    await load()
  } finally {
    togglingId.value = ''
  }
}

onMounted(() => {
  if (canRead.value) void load()
})
</script>

<style scoped lang="scss">
.coupon-page {
  padding: 16px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}
.coupon-toolbar {
  display: flex;
  gap: 10px;
  margin-bottom: 14px;
}
.coupon-query-btn {
  background: #1eaa59;
  border-color: #1eaa59;
}
.coupon-create-btn {
  color: #1eaa59;
  border-color: #b7e4c7;
}
.coupon-status-link {
  color: #303133;
  cursor: pointer;
}
.coupon-status-link:hover {
  color: #409eff;
}
.coupon-edit-btn {
  color: #fff;
  background: #f59e0b;
  border-color: #f59e0b;
}
.coupon-edit-btn:hover {
  color: #fff;
  background: #d97706;
  border-color: #d97706;
}
.coupon-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}
.coupon-modal-footer {
  display: flex;
  justify-content: center;
  gap: 12px;
}
.coupon-modal-footer :deep(.ant-btn) {
  font-size: 15px;
}
.coupon-tabs :deep(.ant-tabs-nav) {
  margin-bottom: 12px;
}
.coupon-grant-form :deep(.ant-form-item-label) {
  display: flex;
  align-items: center;
  justify-content: flex-end;
}
.coupon-grant-form :deep(.ant-form-item-label > label) {
  font-size: 15px;
  line-height: 32px;
}
.coupon-grant-form :deep(.ant-input),
.coupon-grant-form :deep(.ant-input-number),
.coupon-grant-form :deep(.ant-select),
.coupon-grant-form :deep(.ant-picker) {
  font-size: 15px;
  height: 32px;
}
.coupon-grant-form :deep(.ant-radio-wrapper) {
  font-size: 15px;
}
.coupon-red-label {
  color: #ff4d4f;
}
.coupon-choice-area {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  margin: 4px 0 24px;
  padding-top: 12px;
}
.coupon-form-grid {
  display: grid;
  grid-template-columns: minmax(0, 349px) minmax(0, 1fr);
  column-gap: 16px;
  align-items: start;
}
.coupon-form-left {
  grid-column: 1;
}
.coupon-form-right {
  grid-column: 2;
}
.coupon-form-row1 {
  grid-row: 1;
}
.coupon-form-row2 {
  grid-row: 2;
}
.coupon-form-row3 {
  grid-row: 3;
}
.coupon-form-row4 {
  grid-row: 4;
}
.coupon-form-row5 {
  grid-row: 5;
}
.coupon-form-discount {
  grid-row: 3;
}
.coupon-form-discount.coupon-form-discount-bottom {
  grid-row: 4;
}
.coupon-form-follow {
  grid-row: 3;
}
.coupon-form :deep(.ant-form-item-label) {
  flex: 0 0 118px;
  max-width: 118px;
  display: flex;
  align-items: center;
  justify-content: flex-end;
}
.coupon-form :deep(.ant-form-item-label > label) {
  position: relative;
  display: inline-block;
  width: 104px;
  font-size: 15px;
  line-height: 32px;
  white-space: nowrap;
  text-align: justify;
  text-align-last: justify;
  vertical-align: middle;
}
.coupon-form :deep(.ant-form-item-label > label::before) {
  position: absolute !important;
  left: -11px !important;
  margin: 0 !important;
}
.coupon-form :deep(.ant-form-item-control) {
  flex: 1 1 auto;
  min-width: 0;
  max-width: 100%;
}
.coupon-form :deep(.ant-input),
.coupon-form :deep(.ant-input-number),
.coupon-form :deep(.ant-input-number-input),
.coupon-form :deep(.ant-select) {
  font-size: 15px;
}
.coupon-form :deep(.ant-input),
.coupon-form :deep(.ant-input-number),
.coupon-form :deep(.ant-select) {
  height: 32px;
}
.coupon-form-left :deep(.ant-input),
.coupon-form-left :deep(.ant-input-number) {
  max-width: 300px;
}
.coupon-validity-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.coupon-price-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
  width: 100%;
  margin-bottom: 24px;
  padding-left: 14px;
}
.coupon-price-pair {
  display: flex;
  align-items: center;
  gap: 13px;
  white-space: nowrap;
}
.coupon-price-label {
  display: inline-block;
  width: 90.7px;
  color: rgba(0, 0, 0, 0.88);
  font-size: 15px;
  text-align: justify;
  text-align-last: justify;
}
.coupon-inline-label {
  color: rgba(0, 0, 0, 0.88);
  font-size: 15px;
  white-space: nowrap;
}
.coupon-choice-grid {
  display: grid;
  grid-template-columns: auto auto;
  align-items: center;
  justify-content: start;
  gap: 24px 10px;
  width: 100%;
}
.coupon-choice-label {
  display: flex;
  align-items: center;
  width: 104px;
  font-size: 15px;
}
.coupon-choice-mark {
  flex: none;
  width: 10px;
  margin-right: 4px;
  color: #ff4d4f;
  text-align: center;
}
.coupon-choice-text {
  flex: 1;
  white-space: nowrap;
  text-align: justify;
  text-align-last: justify;
}
.coupon-choice-title {
  color: #303133;
  font-size: 15px;
  font-weight: 400;
  white-space: nowrap;
}
.coupon-radio-group {
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: 8px;
}
.coupon-radio-group-type {
  width: 571px;
  max-width: 100%;
  justify-content: space-between;
}
.coupon-radio-group-scope {
  display: grid;
  grid-template-columns: repeat(3, 174.7px);
  width: 524.1px;
  max-width: 100%;
  gap: 0;
}
.coupon-radio-group-scope :deep(.ant-radio-wrapper) {
  justify-self: start;
}
.coupon-radio-group :deep(.ant-radio-wrapper) {
  margin-inline-end: 0;
  font-size: 15px;
}
</style>

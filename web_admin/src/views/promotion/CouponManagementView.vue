<template>
  <section class="coupon-page">
    <div class="coupon-toolbar">
      <a-button class="coupon-query-btn" type="primary" :loading="loading" @click="reload">{{ $t('couponAdmin.query') }}</a-button>
      <a-button v-if="canPublish && activeTab === 'coupons'" class="coupon-create-btn" @click="openCreate">{{ $t('couponAdmin.create') }}</a-button>
      <a-button v-if="canPublish" class="coupon-grant-btn" @click="openDrawCreate">{{ $t('couponAdmin.grantRule') }}</a-button>
      <a-button v-if="canGrant && activeTab === 'rules'" class="coupon-grant-user-btn" @click="openGrant()">{{ $t('couponAdmin.grantCoupon') }}</a-button>
      <a-button v-if="canPublish && activeTab === 'rules'" class="coupon-ledger-btn" @click="openLedger()">{{ $t('couponAdmin.allLedger') }}</a-button>
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
          :data-source="pagedPoolEntries"
          :loading="loading"
          :pagination="false"
          :row-key="(row: CouponPoolEntry) => String(row.id)"
          :scroll="{ x: 1180 }"
          size="middle"
        >
          <a-table-column :title="$t('couponAdmin.id')" width="80" align="center">
            <template #default="{ record }">{{ record.id }}</template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.couponTemplateName')" min-width="220" align="center">
            <template #default="{ record }">{{ poolTemplateText(record) }}</template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.initialCount')" width="110" align="center">
            <template #default="{ record }">{{ record.count ?? 0 }}</template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.remainingCount')" width="110" align="center">
            <template #default="{ record }">{{ record.remainingCount ?? 0 }}</template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.startTime')" min-width="180" align="center">
            <template #default="{ record }">{{ poolStartTimeText(record) }}</template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.drawCount')" width="130" align="center">
            <template #default="{ record }">{{ record.drawCount ?? 0 }}</template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.status')" width="110" align="center">
            <template #default="{ record }">
              <a class="coupon-status-link" @click="togglePoolStatus(record)">{{ poolEnabled(record) ? $t('couponAdmin.enabled') : $t('couponAdmin.disabled') }}</a>
            </template>
          </a-table-column>
          <a-table-column :title="$t('couponAdmin.action')" width="280" align="center" fixed="right">
            <template #default="{ record }">
              <a-space>
                <a-button class="coupon-edit-btn" size="small" @click="openDrawEdit(record)">{{ $t('couponAdmin.edit') }}</a-button>
                <a-button size="small" @click="openAdjust(record)">{{ $t('couponAdmin.adjustStock') }}</a-button>
                <a-button size="small" @click="openLedger(record)">{{ $t('couponAdmin.ledger') }}</a-button>
              </a-space>
            </template>
          </a-table-column>
        </a-table>

        <div class="coupon-footer">
          <a-pagination
            v-model:current="drawPage"
            v-model:page-size="drawPageSize"
            :total="poolEntries.length"
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

    <a-modal v-model:open="drawOpen" centered :title="drawModalTitle" width="520px" :style="{ maxWidth: '96vw' }" :footer="null">
      <a-form class="coupon-grant-form" :label-col="{ span: 7 }" :wrapper-col="{ span: 17 }">
        <a-form-item :label="$t('couponAdmin.couponTemplateName')" required>
          <a-select
            v-model:value="drawForm.rewardTemplateId"
            :options="drawCouponOptions"
            show-search
            option-filter-prop="label"
            :placeholder="$t('couponAdmin.couponTemplatePlaceholder')"
            style="width: 16em"
            @change="onRewardTemplateChange"
          />
        </a-form-item>
        <a-form-item :label="$t('couponAdmin.couponCount')" required>
          <a-input-number v-model:value="drawForm.count" :min="1" :max="1000" :precision="0" :disabled="!!drawForm.id" style="width: 16em" />
        </a-form-item>
        <a-form-item v-if="drawForm.id" :label="$t('couponAdmin.remainingCount')">
          <span class="coupon-remaining-text">{{ drawForm.remainingCount }}</span>
        </a-form-item>
        <a-form-item :label="$t('couponAdmin.startTime')" required>
          <a-date-picker v-model:value="drawForm.startAt" show-time format="YYYY-MM-DD HH:mm:ss" style="width: 16em" />
        </a-form-item>
        <a-form-item :label="$t('couponAdmin.drawCount')" required>
          <a-input-number v-model:value="drawForm.drawCount" :min="1" :precision="0" style="width: 16em" />
        </a-form-item>
        <a-form-item :label="$t('couponAdmin.enableLabel')">
          <div class="coupon-switch-row">
            <a-switch v-model:checked="drawForm.enabled" :disabled="!drawForm.rewardTemplateId" />
          </div>
        </a-form-item>
      </a-form>
      <div class="coupon-modal-footer" style="display: flex; justify-content: center; gap: 12px">
        <a-button @click="drawOpen = false">{{ $t('couponAdmin.cancel') }}</a-button>
        <a-button type="primary" :loading="drawSaving" @click="submitDraw">{{ $t('couponAdmin.confirm') }}</a-button>
      </div>
    </a-modal>

    <a-modal v-model:open="adjustOpen" centered :title="$t('couponAdmin.adjustTitle')" width="460px" :style="{ maxWidth: '96vw' }" :footer="null">
      <a-form class="coupon-grant-form" :label-col="{ span: 7 }" :wrapper-col="{ span: 17 }">
        <a-form-item :label="$t('couponAdmin.couponTemplateName')">
          <span>{{ adjustTarget ? poolTemplateText(adjustTarget) : '-' }}</span>
        </a-form-item>
        <a-form-item :label="$t('couponAdmin.remainingCount')">
          <span class="coupon-remaining-text">{{ adjustTarget?.remainingCount ?? 0 }}</span>
        </a-form-item>
        <a-form-item :label="$t('couponAdmin.adjustMode')" required>
          <a-radio-group v-model:value="adjustForm.mode">
            <a-radio value="add">{{ $t('couponAdmin.adjustAdd') }}</a-radio>
            <a-radio value="sub">{{ $t('couponAdmin.adjustSub') }}</a-radio>
          </a-radio-group>
        </a-form-item>
        <a-form-item :label="$t('couponAdmin.adjustAmount')" required>
          <a-input-number v-model:value="adjustForm.amount" :min="1" :precision="0" style="width: 16em" />
        </a-form-item>
        <a-form-item :label="$t('couponAdmin.remark')">
          <a-input v-model:value="adjustForm.remark" :maxlength="255" :placeholder="$t('couponAdmin.remarkPlaceholder')" />
        </a-form-item>
      </a-form>
      <div class="coupon-modal-footer" style="display: flex; justify-content: center; gap: 12px">
        <a-button @click="adjustOpen = false">{{ $t('couponAdmin.cancel') }}</a-button>
        <a-button type="primary" :loading="adjustSaving" @click="submitAdjust">{{ $t('couponAdmin.confirm') }}</a-button>
      </div>
    </a-modal>

    <a-modal v-model:open="ledgerOpen" centered :title="ledgerTitle" width="980px" :style="{ maxWidth: '96vw' }" :footer="null">
      <a-table
        :data-source="ledgerEntries"
        :loading="ledgerLoading"
        :pagination="false"
        :row-key="(row: CouponLedgerEntry) => String(row.id)"
        :scroll="{ x: 900 }"
        size="small"
      >
        <a-table-column :title="$t('couponAdmin.ledgerTime')" width="170" align="center">
          <template #default="{ record }">{{ ledgerTimeText(record) }}</template>
        </a-table-column>
        <a-table-column :title="$t('couponAdmin.ledgerEvent')" width="120" align="center">
          <template #default="{ record }">{{ ledgerEventText(record.eventType) }}</template>
        </a-table-column>
        <a-table-column :title="$t('couponAdmin.ledgerDelta')" width="90" align="center">
          <template #default="{ record }">{{ ledgerDeltaText(record) }}</template>
        </a-table-column>
        <a-table-column :title="$t('couponAdmin.ledgerBeforeAfter')" width="150" align="center">
          <template #default="{ record }">{{ ledgerBeforeAfterText(record) }}</template>
        </a-table-column>
        <a-table-column :title="$t('couponAdmin.ledgerOperator')" width="150" align="center">
          <template #default="{ record }">{{ ledgerOperatorText(record) }}</template>
        </a-table-column>
        <a-table-column :title="$t('couponAdmin.ledgerRef')" min-width="150" align="center">
          <template #default="{ record }">{{ ledgerRefText(record) }}</template>
        </a-table-column>
        <a-table-column :title="$t('couponAdmin.remark')" min-width="160" align="center">
          <template #default="{ record }">{{ record.remark || '-' }}</template>
        </a-table-column>
      </a-table>
      <div class="coupon-footer">
        <a-pagination
          v-model:current="ledgerPage"
          v-model:page-size="ledgerPageSize"
          :total="ledgerTotal"
          :page-size-options="['20', '50', '100']"
          show-size-changer
          show-less-items
          :locale="{ items_per_page: $t('couponAdmin.itemsPerPage') }"
          @change="loadLedger"
        />
      </div>
    </a-modal>

    <a-modal v-model:open="grantOpen" centered :title="$t('couponAdmin.grantCouponTitle')" width="520px" :style="{ maxWidth: '96vw' }" :footer="null">
      <a-form class="coupon-grant-form" :label-col="{ span: 7 }" :wrapper-col="{ span: 17 }">
        <a-form-item :label="$t('couponAdmin.poolBatch')" required>
          <a-select
            v-model:value="grantForm.poolEntryId"
            :options="grantPoolOptions"
            show-search
            option-filter-prop="label"
            :placeholder="$t('couponAdmin.poolBatchPlaceholder')"
            style="width: 16em"
          />
        </a-form-item>
        <a-form-item :label="$t('couponAdmin.grantUserId')" required>
          <a-input v-model:value="grantForm.userId" inputmode="numeric" :placeholder="$t('couponAdmin.grantUserId')" style="width: 16em" />
        </a-form-item>
        <a-form-item :label="$t('couponAdmin.grantCount')" required>
          <a-input-number v-model:value="grantForm.count" :min="1" :max="1000" :precision="0" style="width: 16em" />
        </a-form-item>
        <a-form-item :label="$t('couponAdmin.customAmountLabel')">
          <a-input v-model:value="grantForm.customAmountMinor" inputmode="numeric" :placeholder="$t('couponAdmin.customAmountPlaceholder')" style="width: 16em" />
        </a-form-item>
      </a-form>
      <div class="coupon-modal-footer" style="display: flex; justify-content: center; gap: 12px">
        <a-button @click="grantOpen = false">{{ $t('couponAdmin.cancel') }}</a-button>
        <a-button type="primary" :loading="grantSaving" @click="submitGrant">{{ $t('couponAdmin.confirm') }}</a-button>
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
import type { CouponLedgerEntry, CouponPoolEntry, CouponTemplate } from '@/api/promotion'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'
import { newRequestId } from '@/utils/requestId'

const session = useSessionStore()
const { t } = useI18n()
const canRead = computed(() => allows(session.permissionNodes, 'promotion.coupon_template', 'read'))
const canPublish = computed(() => allows(session.permissionNodes, 'promotion.coupon_template', 'publish'))
const canGrant = computed(() => allows(session.permissionNodes, 'promotion.user_coupon', 'grant'))
const activeTab = ref('coupons')
const drawPage = ref(1)
const drawPageSize = ref(15)

const rawTemplates = ref<CouponTemplate[]>([])
const poolEntries = ref<CouponPoolEntry[]>([])
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
function isRealCouponKind(kind: string | number | undefined) {
  switch (String(kind ?? '')) {
    case '1':
    case 'COUPON_TEMPLATE_KIND_FIXED_MINOR':
    case '2':
    case 'COUPON_TEMPLATE_KIND_PERCENT_BPS':
    case '3':
    case 'COUPON_TEMPLATE_KIND_FREE':
    case '4':
    case 'COUPON_TEMPLATE_KIND_CUSTOM_MINOR':
      return true
    default:
      return false
  }
}

const latestCouponTemplates = computed(() =>
  latestTemplates.value.filter((item) => isRealCouponKind(item.kind)),
)
const pagedTemplates = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return latestCouponTemplates.value.slice(start, start + pageSize.value)
})
const pagedPoolEntries = computed(() => {
  const start = (drawPage.value - 1) * drawPageSize.value
  return poolEntries.value.slice(start, start + drawPageSize.value)
})

function tenant() {
  if (!session.tenantId) throw new Error('tenant missing')
  return String(session.tenantId)
}

async function loadAllTemplates() {
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
  return all
}

async function loadAllPoolEntries() {
  const all: CouponPoolEntry[] = []
  let pageNo = 1
  for (let i = 0; i < 20; i += 1) {
    const result = await promotionClient().ListCouponPoolEntries({
      tenantId: tenant(),
      page: pageNo,
      pageSize: 100,
    })
    const entries = result.entries ?? []
    all.push(...entries)
    if (entries.length === 0 || all.length >= Number(result.total ?? 0)) break
    pageNo += 1
  }
  return all
}

async function load() {
  if (!canRead.value) return
  loading.value = true
  try {
    const [templates, pools] = await Promise.all([loadAllTemplates(), loadAllPoolEntries()])
    rawTemplates.value = templates
    poolEntries.value = pools
    const maxPage = Math.max(1, Math.ceil(latestCouponTemplates.value.length / pageSize.value))
    if (page.value > maxPage) page.value = maxPage
    const maxDrawPage = Math.max(1, Math.ceil(poolEntries.value.length / drawPageSize.value))
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
  rewardAmountYuan: 0,
  discountPercent: undefined as number | undefined,
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
    rewardAmountYuan: 0,
    discountPercent: undefined,
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
    rewardAmountYuan: fenToYuan(record.amountMinor),
    discountPercent: Number(record.percentBps ?? 0) / 100,
    enabled: isEnabled(record),
  })
  modalOpen.value = true
}

const drawOpen = ref(false)
const drawSaving = ref(false)
function defaultDrawStartAt() {
  // 当前时间向上取整到下一小时，再延后 6 小时
  return dayjs().startOf('hour').add(7, 'hour')
}

const drawForm = reactive({
  id: '',
  rewardTemplateId: undefined as string | undefined,
  count: 20,
  remainingCount: 0,
  startAt: defaultDrawStartAt() as Dayjs | null,
  drawCount: 1,
  enabled: false,
})
const drawModalTitle = computed(() =>
  drawForm.id ? t('couponAdmin.editCouponTitle') : t('couponAdmin.grantRuleTitle'),
)
const drawCouponOptions = computed(() =>
  latestCouponTemplates.value.map((item) => ({
    value: String(item.templateId ?? ''),
    label: `${item.name || '-'} (ID ${item.templateId})`,
  })),
)

function poolTemplateText(record: CouponPoolEntry) {
  const id = String(record.templateId ?? '')
  if (!id || id === '0') return '-'
  const coupon = latestCouponTemplates.value.find((item) => String(item.templateId ?? '') === id)
  const name = String(record.templateName || coupon?.name || '')
  return name ? `${name} (ID ${id})` : `ID ${id}`
}

function poolStartTimeText(record: CouponPoolEntry) {
  const ms = Number(record.startAtUnixMillis ?? 0)
  return ms > 0 ? dayjs(ms).format('YYYY-MM-DD HH:mm:ss') : t('couponAdmin.immediate')
}

function poolEnabled(record: CouponPoolEntry) {
  // protojson 省略 false 标量字段：缺省即视为禁用
  return record.enabled === true
}

function onRewardTemplateChange(value: string | undefined) {
  if (!value) {
    drawForm.enabled = false
    return
  }
  // 新建时选中模板即默认启用；编辑时保留原有启用状态
  if (!drawForm.id) drawForm.enabled = true
}

function openDrawCreate() {
  Object.assign(drawForm, {
    id: '',
    rewardTemplateId: undefined,
    count: 20,
    remainingCount: 0,
    startAt: defaultDrawStartAt(),
    drawCount: 1,
    enabled: false,
  })
  drawOpen.value = true
}

function openDrawEdit(record: CouponPoolEntry) {
  const startMs = Number(record.startAtUnixMillis ?? 0)
  const rewardTemplateId = Number(record.templateId ?? 0) > 0 ? String(record.templateId) : undefined
  Object.assign(drawForm, {
    id: String(record.id ?? ''),
    rewardTemplateId,
    count: Number(record.count ?? 1),
    remainingCount: Number(record.remainingCount ?? 0),
    startAt: startMs > 0 ? dayjs(startMs) : defaultDrawStartAt(),
    drawCount: Number(record.drawCount ?? 1),
    enabled: poolEnabled(record),
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
  const count = Math.trunc(drawForm.count)
  if (count <= 0) {
    message.warning(t('couponAdmin.couponCountRequired'))
    return
  }
  if (!drawForm.startAt) {
    message.warning(t('couponAdmin.startTimeRequired'))
    return
  }
  const drawCount = Math.trunc(drawForm.drawCount)
  if (drawCount <= 0) {
    message.warning(t('couponAdmin.drawCountRequired'))
    return
  }
  const entry: CouponPoolEntry = {
    templateId: rewardTemplateId,
    count,
    drawCount,
    startAtUnixMillis: drawForm.startAt.valueOf(),
    enabled: drawForm.enabled,
  }
  drawSaving.value = true
  try {
    if (drawForm.id && positiveId(drawForm.id)) {
      await promotionClient(newRequestId()).UpdateCouponPoolEntry({
        tenantId: tenant(),
        entryId: drawForm.id,
        entry,
      })
    } else {
      await promotionClient(newRequestId()).CreateCouponPoolEntry({ tenantId: tenant(), entry })
    }
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

const adjustOpen = ref(false)
const adjustSaving = ref(false)
const adjustTarget = ref<CouponPoolEntry | null>(null)
const adjustForm = reactive({
  mode: 'add' as 'add' | 'sub',
  amount: 1,
  remark: '',
})

function openAdjust(record: CouponPoolEntry) {
  adjustTarget.value = record
  adjustForm.mode = 'add'
  adjustForm.amount = 1
  adjustForm.remark = ''
  adjustOpen.value = true
}

async function submitAdjust() {
  if (!canPublish.value || !adjustTarget.value) return
  const entryId = String(adjustTarget.value.id ?? '')
  if (!positiveId(entryId)) return
  const amount = Math.trunc(adjustForm.amount)
  if (amount <= 0) {
    message.warning(t('couponAdmin.adjustAmountRequired'))
    return
  }
  adjustSaving.value = true
  try {
    await promotionClient(newRequestId()).AdjustCouponPoolEntry({
      tenantId: tenant(),
      entryId,
      delta: adjustForm.mode === 'add' ? amount : -amount,
      remark: adjustForm.remark.trim(),
    })
    message.success(t('couponAdmin.adjustSaved'))
    adjustOpen.value = false
    await load()
  } catch {
    // 错误提示由请求层统一处理
  } finally {
    adjustSaving.value = false
  }
}

const ledgerOpen = ref(false)
const ledgerLoading = ref(false)
const ledgerEntries = ref<CouponLedgerEntry[]>([])
const ledgerTotal = ref(0)
const ledgerPage = ref(1)
const ledgerPageSize = ref(20)
const ledgerEntryId = ref('')
const ledgerTitle = computed(() =>
  ledgerEntryId.value
    ? `${t('couponAdmin.ledgerTitle')} #${ledgerEntryId.value}`
    : t('couponAdmin.allLedger'),
)

function openLedger(record?: CouponPoolEntry) {
  ledgerEntryId.value = record ? String(record.id ?? '') : ''
  ledgerPage.value = 1
  ledgerOpen.value = true
  void loadLedger()
}

async function loadLedger() {
  if (!canPublish.value) return
  ledgerLoading.value = true
  try {
    const result = await promotionClient().ListCouponLedger({
      tenantId: tenant(),
      entryId: ledgerEntryId.value || undefined,
      page: ledgerPage.value,
      pageSize: ledgerPageSize.value,
    })
    ledgerEntries.value = result.entries ?? []
    ledgerTotal.value = Number(result.total ?? 0)
  } catch {
    // 错误提示由请求层统一处理
  } finally {
    ledgerLoading.value = false
  }
}

function ledgerTimeText(record: CouponLedgerEntry) {
  const ms = Number(record.createdAtUnixMillis ?? 0)
  return ms > 0 ? dayjs(ms).format('YYYY-MM-DD HH:mm:ss') : '-'
}

function ledgerEventText(eventType: string | undefined) {
  switch (String(eventType ?? '')) {
    case 'INIT':
      return t('couponAdmin.eventInit')
    case 'ADMIN_ADD':
      return t('couponAdmin.eventAdminAdd')
    case 'ADMIN_SUB':
      return t('couponAdmin.eventAdminSub')
    case 'GRANT':
      return t('couponAdmin.eventGrant')
    case 'LOCK':
      return t('couponAdmin.eventLock')
    case 'USE':
      return t('couponAdmin.eventUse')
    case 'RELEASE':
      return t('couponAdmin.eventRelease')
    case 'EXPIRE':
      return t('couponAdmin.eventExpire')
    default:
      return String(eventType ?? '-')
  }
}

function ledgerDeltaText(record: CouponLedgerEntry) {
  const delta = Number(record.delta ?? 0)
  if (delta > 0) return `+${delta}`
  if (delta < 0) return String(delta)
  return '-'
}

function couponStatusText(status: number | undefined) {
  switch (Number(status ?? 0)) {
    case 1:
      return t('couponAdmin.statusAvailable')
    case 2:
      return t('couponAdmin.statusLocked')
    case 3:
      return t('couponAdmin.statusUsed')
    case 4:
      return t('couponAdmin.statusExpired')
    default:
      return '-'
  }
}

function ledgerBeforeAfterText(record: CouponLedgerEntry) {
  switch (String(record.eventType ?? '')) {
    case 'LOCK':
    case 'USE':
    case 'RELEASE':
    case 'EXPIRE':
      return `${couponStatusText(record.beforeStatus)} → ${couponStatusText(record.afterStatus)}`
    default:
      return `${record.beforeCount ?? 0} → ${record.afterCount ?? 0}`
  }
}

function ledgerOperatorText(record: CouponLedgerEntry) {
  const id = Number(record.operatorId ?? 0) > 0 ? ` #${record.operatorId}` : ''
  switch (String(record.operatorType ?? '')) {
    case 'ADMIN':
      return `${t('couponAdmin.operatorAdmin')}${id}`
    case 'USER':
      return `${t('couponAdmin.operatorUser')}${id}`
    default:
      return t('couponAdmin.operatorSystem')
  }
}

function ledgerRefText(record: CouponLedgerEntry) {
  if (Number(record.userCouponId ?? 0) > 0) {
    return `${t('couponAdmin.userCouponShort')} #${record.userCouponId}`
  }
  if (record.orderId) {
    return `${t('couponAdmin.orderShort')} ${record.orderId}`
  }
  if (Number(record.entryId ?? 0) > 0) {
    return `${t('couponAdmin.poolShort')} #${record.entryId}`
  }
  return '-'
}

const grantOpen = ref(false)
const grantSaving = ref(false)
const grantForm = reactive({
  poolEntryId: undefined as string | undefined,
  userId: '',
  count: 1,
  customAmountMinor: '',
})
const grantPoolOptions = computed(() =>
  poolEntries.value
    .filter((entry) => poolEnabled(entry) && Number(entry.remainingCount ?? 0) > 0)
    .map((entry) => ({
      value: String(entry.id ?? ''),
      label: `${poolTemplateText(entry)} · ${t('couponAdmin.remainingCount')} ${entry.remainingCount ?? 0}`,
    })),
)

function openGrant(record?: CouponPoolEntry) {
  grantForm.userId = ''
  grantForm.count = 1
  grantForm.customAmountMinor = ''
  grantForm.poolEntryId = record
    ? String(record.id ?? '')
    : grantPoolOptions.value[0]?.value
  grantOpen.value = true
}

async function submitGrant() {
  if (!canGrant.value) return
  const pool = poolEntries.value.find((entry) => String(entry.id ?? '') === String(grantForm.poolEntryId ?? ''))
  if (!pool) {
    message.warning(t('couponAdmin.poolBatchRequired'))
    return
  }
  const userId = grantForm.userId.trim()
  if (!positiveId(userId)) {
    message.warning(t('couponAdmin.userIdRequired'))
    return
  }
  const count = Math.trunc(grantForm.count)
  if (count <= 0 || count > 1000) {
    message.warning(t('couponAdmin.grantCountInvalid'))
    return
  }
  if (count > Number(pool.remainingCount ?? 0)) {
    message.warning(t('couponAdmin.grantStockInsufficient'))
    return
  }
  const custom = grantForm.customAmountMinor.trim()
  if (custom && !/^\d+$/.test(custom)) {
    message.warning(t('couponAdmin.customAmountInvalid'))
    return
  }
  grantSaving.value = true
  try {
    await promotionClient(newRequestId()).GrantCoupons({
      tenantId: tenant(),
      userId,
      templateId: String(pool.templateId),
      count,
      poolEntryId: String(pool.id),
      customAmountMinor: custom || undefined,
    })
    message.success(t('couponAdmin.grantSuccess', { count }))
    grantOpen.value = false
    await load()
  } catch {
    // 错误提示由请求层统一处理
  } finally {
    grantSaving.value = false
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

async function togglePoolStatus(record: CouponPoolEntry) {
  if (!canPublish.value) return
  const id = String(record.id ?? '')
  if (!id || !positiveId(id)) return
  drawSaving.value = true
  try {
    await promotionClient(newRequestId()).UpdateCouponPoolEntry({
      tenantId: tenant(),
      entryId: id,
      entry: {
        templateId: record.templateId,
        count: Number(record.count ?? 0),
        drawCount: Number(record.drawCount ?? 0),
        startAtUnixMillis: Number(record.startAtUnixMillis ?? 0),
        enabled: !poolEnabled(record),
      },
    })
    message.success(t('couponAdmin.statusSaved'))
    await load()
  } finally {
    drawSaving.value = false
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
.coupon-create-btn,
.coupon-grant-user-btn {
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
.coupon-switch-row {
  display: flex;
  align-items: center;
  height: 32px;
}
.coupon-remaining-text {
  font-size: 15px;
  font-weight: 600;
  line-height: 32px;
  color: #1eaa59;
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

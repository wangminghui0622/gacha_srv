<template>
  <div class="create-form">
    <div class="form-layout">
      <div class="form-left">
      <a-form :label-col="{ span: 9 }" :wrapper-col="{ span: 15 }" label-align="right">
        <a-form-item label="主题名称" required>
          <a-input v-model:value="form.name" placeholder="主题名称" />
        </a-form-item>
        <a-form-item label="IP">
          <a-select v-model:value="form.ipId" :options="ipOptions" placeholder="IP" show-search option-filter-prop="label" />
        </a-form-item>
        <a-form-item label="玩法类型" required>
          <a-select v-model:value="form.mode" :options="modeOptions" />
        </a-form-item>

        <template v-if="form.mode === 'memorymatch'">
          <a-form-item label="回合数" required>
            <a-input-number v-model:value="mmConfig.roundsPerPurchase" :min="1" :max="100" :precision="0" style="width: 100%" />
          </a-form-item>
          <a-form-item label="每回合最大碰数" required>
            <a-input-number v-model:value="mmConfig.maxPairCount" :min="1" :max="27" :precision="0" style="width: 100%" />
            <div class="field-hint">总碰数上限：{{ mmConfig.roundsPerPurchase * mmConfig.maxPairCount }}</div>
          </a-form-item>
        </template>

        <a-form-item label="活动开始时间">
          <a-date-picker v-model:value="form.startAt" show-time style="width: 100%" placeholder="活动开始时间" />
        </a-form-item>
        <a-form-item label="活动结束时间">
          <a-date-picker v-model:value="form.endAt" show-time style="width: 100%" placeholder="活动结束时间" />
        </a-form-item>
        <a-form-item label="价格" required>
          <a-input-number v-model:value="form.priceYuan" :min="0" :precision="2" style="width: 100%" />
        </a-form-item>
        <a-form-item label="折扣(%)" required>
          <a-input-number v-model:value="form.discount" :min="0" :max="100" :precision="2" style="width: 100%" />
        </a-form-item>
        <a-form-item label="箱子数量" required>
          <a-input-number v-model:value="form.boxCount" :min="1" :precision="0" style="width: 100%" />
        </a-form-item>
        <a-form-item label="排序权重">
          <a-input-number v-model:value="form.sortWeight" :min="0" :precision="0" style="width: 100%" />
        </a-form-item>

        <a-form-item label="优惠券">
          <div class="coupon-config">
            <a-space>
              <a-input-number v-model:value="couponId" :min="0" :precision="0" placeholder="优惠券id" style="width: 150px" />
              <a-button @click="addCoupon">添加</a-button>
            </a-space>
            <a-table :data-source="coupons" :pagination="false" row-key="id" size="small" :locale="{ emptyText: '暂无数据' }" class="coupon-table">
              <a-table-column title="ID" data-index="id" width="100" />
              <a-table-column title="名称" data-index="name" />
              <a-table-column title="描述" data-index="description" />
              <a-table-column title="状态" data-index="status" width="100" />
              <a-table-column title="操作" width="90">
                <template #default="{ record }">
                  <a-button type="link" danger size="small" @click="removeCoupon(record.id)">删除</a-button>
                </template>
              </a-table-column>
            </a-table>
          </div>
        </a-form-item>

        <a-form-item :wrapper-col="{ span: 15, offset: 9 }">
          <div class="cover-row">
            <div class="cover-item">
              <div class="cover-label">封面大图</div>
              <a-upload :show-upload-list="false" accept="image/*" :before-upload="(file: File) => uploadImage(file, 'coverImage')">
                <div class="image-upload-box">
                  <img v-if="form.coverImage" :src="form.coverImage" alt="" />
                  <template v-else><PlusOutlined /><span>Upload</span></template>
                </div>
              </a-upload>
            </div>
            <div class="cover-item">
              <div class="cover-label">封面缩略图</div>
              <a-upload :show-upload-list="false" accept="image/*" :before-upload="(file: File) => uploadImage(file, 'coverThumb')">
                <div class="image-upload-box">
                  <img v-if="form.coverThumb" :src="form.coverThumb" alt="" />
                  <template v-else><PlusOutlined /><span>Upload</span></template>
                </div>
              </a-upload>
            </div>
          </div>
        </a-form-item>
      </a-form>

      <div class="switch-panel">
        <div class="switch-panel-title">其他设置</div>
        <div class="switch-grid">
          <label v-for="item in switchItems" :key="item.key" class="switch-item">
            <a-switch v-model:checked="form[item.key]" size="small" />
            <span>{{ item.label }}</span>
          </label>
        </div>
      </div>
      </div>

      <div class="form-right">
        <div class="prize-section">
          <div class="prize-label">{{ form.mode === 'memorymatch' ? '奖档配置：' : '赏品配置：' }}</div>
          <div v-if="form.mode === 'memorymatch'" class="prize-hint">
            <span>每个目标碰数对应一个奖档（数量固定为 1），必须唯一；总碰数上限 {{ memoryTargetMax }}；未配置的档位不会中奖。</span>
            <a-button type="link" size="small" @click="autoAssignTargets">自动填目标碰数</a-button>
          </div>
          <div v-if="form.mode === 'memorymatch'" class="target-summary">
            <div class="target-summary-line">
              <span class="target-summary-label">已配置：</span>
              <a-tag v-for="value in memoryTargetUnique" :key="value" :color="memoryTargetBadgeColor(value)">
                {{ value }}
              </a-tag>
              <span v-if="!memoryTargetUnique.length" class="target-summary-muted">暂无</span>
            </div>
            <div v-if="memoryTargetDuplicates.length" class="target-summary-error">
              重复目标碰数：{{ memoryTargetDuplicates.join('、') }}，请改成不同的值
            </div>
            <div v-else-if="memoryTargetOutOfRange.length" class="target-summary-error">
              超出 0-{{ memoryTargetMax }}：{{ memoryTargetOutOfRange.join('、') }}
            </div>
            <div v-else-if="memoryMissingTargets.length" class="target-summary-muted">
              未配置：{{ memoryMissingTargets.join('、') }}（这些总碰数不会中奖）
            </div>
            <div v-else-if="memoryTargetUnique.length" class="target-summary-ok">
              已覆盖 0-{{ memoryTargetMax }} 全部档位
            </div>
          </div>
          <div class="prize-config">
            <div ref="prizeTableWrap" class="prize-table-wrap">
              <GroupBuyPrizeTable
                v-if="isGroupBuy"
                :data="displayPrizes"
                :probability="probability"
                :custom-row="prizeCustomRow"
                @toggle="togglePrizeExpand"
                @remove="confirmRemove"
                @child="openChildPicker"
              />
              <DefaultPrizeTable
                v-else
                :data="displayPrizes"
                :probability="probability"
                :custom-row="prizeCustomRow"
                :show-target="form.mode === 'memorymatch'"
                @toggle="togglePrizeExpand"
                @remove="confirmRemove"
                @target-change="setPrizeTarget"
              />
            </div>

            <div class="prize-inline">
              <a-select
                v-if="form.mode !== 'memorymatch'"
                v-model:value="prizeDraft.prizeGrade"
                :options="gradeOptions"
                style="width: 90px"
              />
              <a-select
                v-if="form.mode !== 'memorymatch'"
                v-model:value="prizeDraft.prizeType"
                :options="prizeTypeOptions"
                style="width: 85px"
              />
              <div class="prize-picker">
                <template v-if="prizeDraft.prizeType === 2">
                  <a-button @click="openSkuPicker()">选择赏品</a-button>
                </template>
                <template v-else-if="prizeDraft.prizeType === 3">
                  <a-input-number v-model:value="prizeDraft.childPoolId" :min="1" :precision="0" placeholder="赏箱ID" style="width: 120px" />
                  <a-button :loading="boxLoading" @click="useBoxId">使用赏箱ID</a-button>
                </template>
                <a-input v-else v-model:value="prizeDraft.name" placeholder="赏品名称" />
              </div>
              <span class="inline-label">中奖记录展示：</span>
              <a-switch v-model:checked="prizeDraft.isPrize" />
            </div>
          </div>
        </div>
      </div>
    </div>

      <div class="create-actions">
        <a-button class="btn-create" type="primary" :loading="saving" @click="submit">{{ initialPoolId ? '更新' : '创建' }}</a-button>
        <a-button @click="emit('cancel')">取消</a-button>
      </div>

    <a-modal
      v-if="skuModalOpen"
      v-model:open="skuModalOpen"
      :footer="null"
      width="96vw"
      :style="{ maxWidth: '1800px' }"
      centered
      :body-style="{ maxHeight: 'calc(100vh - 120px)', overflowY: 'auto' }"
      :destroy-on-close="true"
      @cancel="onSkuPickerCancel"
    >
      <template #title>
        <div class="sku-picker-title">选择赏品</div>
      </template>
      <div class="sku-picker">
        <a-alert
          v-if="childParent"
          type="info"
          show-icon
          :message="`父赏：${childParent.name}`"
          style="margin-bottom: 12px"
        />
        <a-alert
          v-else-if="form.mode === 'memorymatch'"
          type="info"
          show-icon
          message="对对碰：同一商品只能添加一次，数量固定为 1"
          style="margin-bottom: 12px"
        />
        <div class="sku-toolbar">
          <a-input
            v-model:value="skuFilters.keyword"
            placeholder="商品名称 / 商品ID"
            allow-clear
            style="width: 220px"
            @input="onSkuKeywordInput"
            @press-enter="searchSku"
          />
          <a-select
            v-model:value="skuFilters.brandId"
            placeholder="厂商"
            :options="skuBrandOptions"
            allow-clear
            show-search
            option-filter-prop="label"
            style="width: 160px"
            @change="onSkuBrandChange"
          />
          <a-select
            v-model:value="skuFilters.seriesId"
            placeholder="系列"
            :options="skuSeriesOptions"
            allow-clear
            show-search
            option-filter-prop="label"
            style="width: 160px"
            @change="onSkuFilterChange"
          />
          <a-select
            v-model:value="skuFilters.status"
            placeholder="状态"
            :options="skuStatusOptions"
            allow-clear
            style="width: 120px"
            @change="onSkuFilterChange"
          />
        </div>

        <div ref="skuTableWrap" class="sku-table-wrap">
        <a-table
          :data-source="skuItems"
          :loading="skuLoading"
          :pagination="false"
          :row-key="(record: AdminProductItem) => record.productId ?? ''"
          :row-selection="skuRowSelection"
          :custom-row="skuCustomRow"
          size="small"
        >
          <a-table-column title="ID" :width="190" align="center">
            <template #default="{ record }">
              <span :class="{ 'sku-located-id': String(record.productId ?? '') === locatedSkuId }">{{ record.productId }}</span>
            </template>
          </a-table-column>
          <a-table-column title="名称" align="center">
            <template #default="{ record }">
              <span :class="{ 'sku-located-id': String(record.productId ?? '') === locatedSkuId }">{{ record.title }}</span>
            </template>
          </a-table-column>
          <a-table-column title="成本价" :width="110" align="center">
            <template #default="{ record }">{{ skuMoney(record.costAmountMinor) }}</template>
          </a-table-column>
          <a-table-column title="IP" data-index="ipName" :width="90" align="center" />
          <a-table-column title="厂商系列" :width="150" align="center">
            <template #default="{ record }">
              <div class="brand-series">
                <span>{{ record.brandName || '-' }}</span>
                <span v-if="record.seriesName && record.seriesName !== record.brandName">{{ record.seriesName }}</span>
                <span v-else-if="!record.seriesName">-</span>
              </div>
            </template>
          </a-table-column>
          <a-table-column title="状态" :width="100" align="center">
            <template #default="{ record }">
              <a-tag :color="record.saleType === 'PREORDER' ? 'orange' : 'blue'">{{ skuSaleTypeText(record.saleType) }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="缩略图" :width="90" align="center">
            <template #default="{ record }">
              <a-image v-if="record.coverUrl" :src="record.coverUrl" :width="40" :height="40" :preview="false" />
              <span v-else>—</span>
            </template>
          </a-table-column>
          <a-table-column v-if="form.mode !== 'memorymatch'" title="数量" :width="80" align="center">
            <template #default="{ record }">
              <a-input-number
                v-model:value="skuQuantities[record.productId ?? '']"
                class="sku-qty-input"
                :min="1"
                :precision="0"
                :controls="false"
                size="small"
                style="width: 64px"
                @change="onSkuQuantityChange(record)"
              />
            </template>
          </a-table-column>
        </a-table>
        </div>

        <div class="sku-footer">
          <a-pagination
            :current="skuPage"
            :page-size="skuPageSize"
            :total="skuTotal"
            :page-size-options="skuPageSizeOptions"
            :locale="{ items_per_page: '条/页' }"
            show-size-changer
            show-less-items
            @change="onSkuPageChange"
            @show-size-change="onSkuPageSizeChange"
          />
        </div>

        <div class="sku-confirm">
          <a-button type="primary" class="sku-confirm-btn" @click="confirmSkuSelection">确认</a-button>
          <a-button class="sku-cancel-btn" @click="cancelSkuPicker">取消</a-button>
        </div>
      </div>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { message, Modal } from 'ant-design-vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import type { Dayjs } from 'dayjs'
import {
  catalogAdminProductClient,
  catalogBrandClient,
  catalogIpClient,
  catalogSeriesClient,
  type AdminProductItem,
  type BrandItem,
  type IpItem,
  type SeriesItem,
} from '@/api/catalog'
import { listPrizeGrades, type PrizeGrade } from '@/api/prizeGrade'
import { playPoolClient, type PlayPoolCreateInitial, type PlayPoolPrize } from '@/api/playPool'
import { poolClient } from '@/api/pool'
import { uploadFileToOss } from '@/api/upload'
import DefaultPrizeTable from '@/views/gacha/prize/DefaultPrizeTable.vue'
import GroupBuyPrizeTable from '@/views/gacha/prize/GroupBuyPrizeTable.vue'
import type { DisplayPrize } from '@/views/gacha/prize/prizeTypes'

const props = defineProps<{ initial?: PlayPoolCreateInitial | null }>()
const emit = defineEmits<{ success: []; cancel: [] }>()
const saving = ref(false)
const initialPoolId = ref<string | undefined>(undefined)
const initialMode = ref('')
const initialConfigJson = ref<string | null>(null)
const ips = ref<IpItem[]>([])
const grades = ref<PrizeGrade[]>([])

const modeOptions = [
  { label: '一番赏', value: 'gashapon' },
  { label: '无限赏', value: 'fullcar' },
  { label: '串串赏', value: 'combochain' },
  { label: '挖宝赏', value: 'dig' },
  { label: '拼团赏', value: 'groupbuy' },
  { label: '对对碰', value: 'memorymatch' },
]


const mmConfig = reactive({
  roundsPerPurchase: 3,
  maxPairCount: 3,
})

const form = reactive({
  name: '',
  ipId: undefined as string | undefined,
  mode: 'gashapon',
  startAt: null as Dayjs | null,
  endAt: null as Dayjs | null,
  priceYuan: 1,
  discount: 0,
  isOnSale: false,
  isNew: false,
  isRecommended: false,
  isBargain: false,
  isPresale: false,
  isMiniapp: false,
  isWelfare: false,
  excludeTask: false,
  allowBoxInBox: false,
  boxCount: 1,
  sortWeight: 0,
  coverImage: '',
  coverThumb: '',
})

// 对对碰没有子赏/箱子奖，类型只保留商品。
const prizeTypeOptions = computed(() => [
  { label: '商品', value: 2 },
  ...(form.mode === 'memorymatch' ? [] : [{ label: '箱子', value: 3 }]),
])

const switchItems = [
  { key: 'isOnSale', label: '是否上架' },
  { key: 'isNew', label: '是否新品' },
  { key: 'isRecommended', label: '是否推荐' },
  { key: 'isBargain', label: '是否捡漏' },
  { key: 'isPresale', label: '是否预售' },
  { key: 'isMiniapp', label: '是否小程序展示' },
  { key: 'isWelfare', label: '是否福利箱' },
  { key: 'excludeTask', label: '不参与任务' },
  { key: 'allowBoxInBox', label: '允许作为箱中箱' },
] as const

const prizes = ref<PlayPoolPrize[]>([])
const expandedPrizeIndexes = ref<Set<number>>(new Set())

const memoryTargetMax = computed(
  () => Math.max(0, Math.round(mmConfig.roundsPerPurchase) * Math.round(mmConfig.maxPairCount)),
)
const memoryTargetValues = computed(() => prizes.value.map((prize) => Number(prize.targetValue ?? 0)))
const memoryTargetUnique = computed(() =>
  Array.from(new Set(memoryTargetValues.value)).sort((a, b) => a - b),
)
const memoryTargetDuplicates = computed(() => {
  const seen = new Set<number>()
  const duplicates = new Set<number>()
  for (const value of memoryTargetValues.value) {
    if (seen.has(value)) duplicates.add(value)
    else seen.add(value)
  }
  return Array.from(duplicates).sort((a, b) => a - b)
})
const memoryTargetOutOfRange = computed(() =>
  Array.from(
    new Set(
      memoryTargetValues.value.filter(
        (value) => !Number.isInteger(value) || value < 0 || value > memoryTargetMax.value,
      ),
    ),
  ).sort((a, b) => a - b),
)
const memoryMissingTargets = computed(() => {
  if (memoryTargetMax.value > 30) return []
  const used = new Set(memoryTargetUnique.value)
  return Array.from({ length: memoryTargetMax.value + 1 }, (_, index) => index).filter(
    (value) => !used.has(value),
  )
})

function memoryTargetBadgeColor(value: number): string {
  if (!Number.isInteger(value) || value < 0 || value > memoryTargetMax.value) return 'red'
  if (memoryTargetDuplicates.value.includes(value)) return 'red'
  return 'green'
}

function applyMemoryMatchConfig(raw?: string | null) {
  let rounds = 3
  let maxPairs = 3
  if (raw) {
    try {
      const cfg = JSON.parse(raw) as { roundsPerPurchase?: unknown; maxPairCount?: unknown }
      const r = Number(cfg.roundsPerPurchase)
      const m = Number(cfg.maxPairCount)
      if (Number.isFinite(r) && r >= 1) rounds = Math.min(100, Math.round(r))
      if (Number.isFinite(m) && m >= 1) maxPairs = Math.min(27, Math.round(m))
    } catch {
      // 配置 JSON 非法时保留默认值，提交时由后端校验
    }
  }
  mmConfig.roundsPerPurchase = rounds
  mmConfig.maxPairCount = maxPairs
}

function applyInitial(initial?: PlayPoolCreateInitial | null) {
  if (!initial) return
  initialPoolId.value = initial.poolId
  initialMode.value = initial.mode ?? 'gashapon'
  initialConfigJson.value = initial.configJson ?? null
  form.mode = initialMode.value
  applyMemoryMatchConfig(initial.configJson)
  form.name = initial.name ?? ''
  form.priceYuan = initial.priceYuan ?? 1
  form.ipId = initial.ipId || undefined
  form.boxCount = initial.periods ?? 1
  form.sortWeight = initial.weight ?? 0
  form.coverImage = initial.coverImage ?? ''
  form.coverThumb = initial.coverThumb ?? ''
  const tags = new Set(initial.tags ?? [])
  form.isOnSale = tags.has('展示')
  form.isNew = tags.has('新品')
  form.isRecommended = tags.has('推荐')
  form.isBargain = tags.has('捡漏')
  form.isPresale = tags.has('预售')
  if (initial.prizes?.length) {
    prizes.value = initial.prizes.map((prize, index) => ({
      ...prize,
      prizeGrade: initialMode.value === 'memorymatch' ? 'PairCard' : prize.prizeGrade,
      prizeIndex: index,
    }))
  }
  if (initialMode.value === 'memorymatch') {
    ensureMemoryMatchNoPrizeTier()
    normalizeMemoryMatchQuantities()
  }
}

watch(() => props.initial, applyInitial, { immediate: true })

const displayPrizes = computed<DisplayPrize[]>(() => {
  const rows: DisplayPrize[] = []
  let displayIndex = 0
  let badgeIndex = 0
  for (const prize of prizes.value) {
    const total = form.mode === 'memorymatch' ? 1 : Number(prize.quantity ?? 1)
    const expanded = form.mode !== 'memorymatch' && total > 1 && expandedPrizeIndexes.value.has(prize.prizeIndex)
    const copies = expanded ? total : 1
    const prizeBadgeIndex = total > 1 ? badgeIndex++ : -1
    for (let copy = 0; copy < copies; copy += 1) {
      displayIndex += 1
      rows.push({
        ...prize,
        __rowKey: `${prize.prizeIndex}-${copy}`,
        __copyIndex: copy,
        __displayIndex: displayIndex,
        __expanded: expanded,
        __badgeIndex: prizeBadgeIndex,
      })
    }
  }
  return rows
})
const prizeTableWrap = ref<HTMLElement | null>(null)
const prizeDraft = reactive({
  prizeGrade: 'SP',
  prizeType: 2,
  name: '',
  childPoolId: 0,
  isPrize: true,
})

if (form.mode === 'memorymatch') {
  prizeDraft.prizeGrade = 'PairCard'
}

// 对对碰奖档的目标碰数必须唯一；新加的赏品自动取当前未占用的最小值，避免全为 0。
function nextMemoryTargetValue(): number {
  const used = new Set(prizes.value.map((prize) => Number(prize.targetValue ?? 0)))
  let value = 0
  while (used.has(value)) value += 1
  return value
}

function newPrizeTargetValue(): number {
  return form.mode === 'memorymatch' ? nextMemoryTargetValue() : 0
}

function autoAssignTargets() {
  const used = new Set<number>()
  for (const prize of prizes.value) {
    let target = Number(prize.targetValue ?? 0)
    if (!Number.isInteger(target) || target < 0 || used.has(target)) {
      target = 0
      while (used.has(target)) target += 1
      prize.targetValue = target
    }
    used.add(target)
  }
  message.success('已自动填充目标碰数')
}

// 对齐老系统：对对碰默认带一条 0 碰「谢谢参与」无奖档；
// 它只做结算兜底，不参与目标碰数分配，概率显示 0.00。
function ensureMemoryMatchNoPrizeTier() {
  if (form.mode !== 'memorymatch') return
  if (prizes.value.some((prize) => Number(prize.targetValue ?? 0) === 0)) return
  prizes.value.unshift({
    prizeIndex: 0,
    prizeGrade: 'PairCard',
    specialKind: '',
    prizeType: 1,
    name: '谢谢参与',
    skuId: 0,
    childPoolId: 0,
    imageUrl: '',
    targetValue: 0,
    weight: 1,
    quantity: -1,
    remaining: -1,
    isPrize: false,
    costFen: 0,
  })
  reindexPrizes()
}
const skuModalOpen = ref(false)
const childParent = ref<PlayPoolPrize | null>(null)
const locatedSkuId = ref('')
const skuTableWrap = ref<HTMLElement | null>(null)
const boxLoading = ref(false)
const skuLoading = ref(false)
const skuItems = ref<AdminProductItem[]>([])
const skuBrands = ref<BrandItem[]>([])
const skuSeries = ref<SeriesItem[]>([])
const skuOptionsLoaded = ref(false)
const skuPage = ref(1)
const skuPageSize = ref(15)
const skuPageSizeOptions = ['15', '30', '50']
const skuTotal = ref(0)
const skuFilters = reactive({
  keyword: '',
  brandId: undefined as string | undefined,
  seriesId: undefined as string | undefined,
  status: 'PRODUCT_STATUS_ACTIVE' as string | undefined,
})
const skuStatusOptions = [
  { label: '草稿', value: 'PRODUCT_STATUS_DRAFT' },
  { label: '生效', value: 'PRODUCT_STATUS_ACTIVE' },
  { label: '归档', value: 'PRODUCT_STATUS_ARCHIVED' },
]
const selectedSkuKeys = ref<string[]>([])
const skuQuantities = ref<Record<string, number>>({})
const skuProductCache = new Map<string, AdminProductItem>()
const skuTouchedKeys = new Set<string>()
const skuRemovedIds = new Set<string>()

watch(
  () => prizeDraft.childPoolId,
  () => {
    if (prizeDraft.prizeType === 3) prizeDraft.name = ''
  },
)
watch(
  () => prizeDraft.prizeType,
  (type) => {
    if (type !== 3) prizeDraft.name = ''
  },
)

const isGroupBuy = computed(() => form.mode === 'groupbuy')

watch(
  () => form.mode,
  (mode, oldMode) => {
    if (mode === 'memorymatch') {
      prizeDraft.prizeGrade = 'PairCard'
      if (prizeDraft.prizeType === 3) {
        prizeDraft.prizeType = 2
        prizeDraft.childPoolId = 0
      }
      ensureMemoryMatchNoPrizeTier()
      normalizeMemoryMatchQuantities()
    } else if (prizeDraft.prizeGrade === 'PairCard') {
      prizeDraft.prizeGrade = 'SP'
    }
    if (oldMode === 'memorymatch' && mode !== 'memorymatch') {
      const before = prizes.value.length
      prizes.value = prizes.value.filter(
        (prize) =>
          !(
            prize.prizeType === 1 &&
            Number(prize.targetValue ?? 0) === 0 &&
            prize.name === '谢谢参与'
          ),
      )
      if (prizes.value.length !== before) reindexPrizes()
    }
    if (oldMode !== 'groupbuy' || mode === 'groupbuy') return
    if (!prizes.value.some((prize) => prize.hasParent)) return
    Modal.confirm({
      title: '切换玩法将移除子赏',
      content: '当前赏品中存在子赏，切换到非拼团玩法后会移除这些子赏，是否继续？',
      okText: '继续',
      cancelText: '取消',
      okType: 'danger',
      onOk: () => {
        removeChildPrizes()
        message.info('已移除子赏配置')
      },
      onCancel: () => {
        form.mode = oldMode
      },
    })
  },
)

const coupons = ref<Array<{ id: number; name: string; description: string; status: string }>>([])
const couponId = ref<number | null>(null)

const ipOptions = computed(() => ips.value.filter((item) => item.ipId && item.name).map((item) => ({ label: item.name as string, value: item.ipId as string })))
const gradeOptions = computed(() => {
  if (form.mode === 'memorymatch') {
    return [{ label: 'PairCard', value: 'PairCard' }]
  }
  const source = grades.value.length > 0 ? grades.value.map((item) => item.code) : ['SP', 'SSR', 'SS', 'S', 'A', 'B', 'Last']
  return Array.from(new Set(source)).map((code) => ({ label: code, value: code }))
})
const skuBrandOptions = computed(() =>
  skuBrands.value.filter((item) => item.brandId && item.name).map((item) => ({ label: item.name as string, value: item.brandId as string })),
)
const allSkuSeriesOptions = computed(() =>
  skuSeries.value
    .filter((item) => item.seriesId && item.name)
    .map((item) => ({ label: item.name as string, value: item.seriesId as string, brandId: item.brandId ?? '' })),
)
const skuSeriesOptions = computed(() =>
  skuFilters.brandId ? allSkuSeriesOptions.value.filter((item) => item.brandId === skuFilters.brandId) : allSkuSeriesOptions.value,
)
const skuRowSelection = computed(() => ({
  selectedRowKeys: selectedSkuKeys.value,
  onChange: (keys: (string | number)[]) => {
    const pageKeys = new Set(skuItems.value.map((item) => item.productId ?? ''))
    const otherKeys = selectedSkuKeys.value.filter((key) => !pageKeys.has(key))
    const nextKeys = [...new Set([...otherKeys, ...keys.map((key) => String(key))])]
    const prevSet = new Set(selectedSkuKeys.value)
    const nextSet = new Set(nextKeys)
    for (const item of skuItems.value) {
      const id = item.productId
      if (!id || prevSet.has(id) === nextSet.has(id)) continue
      skuTouchedKeys.add(id)
      if (nextSet.has(id)) {
        skuRemovedIds.delete(id)
      } else if (isExistingSku(id)) {
        skuRemovedIds.add(id)
      }
    }
    selectedSkuKeys.value = nextKeys
  },
}))

async function loadOptions() {
  const [ipReply, gradeReply] = await Promise.all([
    catalogIpClient.List({ keyword: '', page: 1, pageSize: 100 }).catch(() => ({ ips: [] })),
    listPrizeGrades().catch(() => ({ items: [] })),
  ])
  ips.value = ipReply.ips ?? []
  grades.value = gradeReply.items ?? []
}

async function useBoxId() {
  const id = Number(prizeDraft.childPoolId ?? 0)
  if (id <= 0) {
    message.warning('请输入赏箱ID')
    return
  }
  boxLoading.value = true
  try {
    const box = await poolClient().GetBoxView({ boxId: String(id) }, { silentError: true })
    if (prizeDraft.prizeType !== 3 || Number(prizeDraft.childPoolId ?? 0) !== id) return
    const name = box.boxName?.trim()
    if (!name) {
      prizeDraft.name = ''
      message.warning(`未找到赏箱ID：${id}`)
      return
    }
    prizeDraft.name = name
    message.success(`已使用赏箱：${name}`)
  } catch {
    if (prizeDraft.prizeType === 3 && Number(prizeDraft.childPoolId ?? 0) === id) {
      prizeDraft.name = ''
    }
    message.warning(`未找到赏箱ID：${id}`)
  } finally {
    boxLoading.value = false
  }
}

function removeChildPrizes() {
  if (!prizes.value.some((prize) => prize.hasParent)) return
  prizes.value = prizes.value.filter((prize) => !prize.hasParent)
  reindexPrizes()
}

function reindexPrizes() {
  const indexMap = new Map<number, number>()
  prizes.value.forEach((prize, index) => indexMap.set(prize.prizeIndex, index))
  prizes.value = prizes.value.map((prize, index) => ({
    ...prize,
    prizeIndex: index,
    parentIndex: prize.hasParent && prize.parentIndex !== undefined ? indexMap.get(prize.parentIndex) ?? -1 : prize.parentIndex,
  }))
  expandedPrizeIndexes.value = new Set()
}

// 对对碰每个目标碰数只对应一件赏品，数量固定为 1，禁止展开多份；
// 历史草稿里可能残留同一商品多行，进入编辑时顺手去重。
function normalizeMemoryMatchQuantities() {
  if (form.mode !== 'memorymatch') return
  expandedPrizeIndexes.value = new Set()
  const seenSkus = new Set<number>()
  const kept: PlayPoolPrize[] = []
  for (const prize of prizes.value) {
    if (prize.prizeType === 2) {
      const skuId = Number(prize.skuId ?? 0)
      if (skuId > 0 && seenSkus.has(skuId)) continue
      if (skuId > 0) seenSkus.add(skuId)
      prize.quantity = 1
      prize.remaining = 1
    }
    kept.push(prize)
  }
  if (kept.length !== prizes.value.length) {
    prizes.value = kept
    reindexPrizes()
  }
}

function togglePrizeExpand(prize: PlayPoolPrize) {
  if (form.mode === 'memorymatch') return
  if (Number(prize.quantity ?? 0) <= 1) return
  const next = new Set(expandedPrizeIndexes.value)
  if (next.has(prize.prizeIndex)) next.delete(prize.prizeIndex)
  else next.add(prize.prizeIndex)
  expandedPrizeIndexes.value = next
}

function deleteConfirmTitle(record: PlayPoolPrize): string {
  if (record.hasParent) return '确定删除该子赏吗？'
  const hasChildren = prizes.value.some((item) => item.hasParent && item.parentIndex === record.prizeIndex)
  return hasChildren ? '确定删除该赏品吗？其子赏会一并删除。' : '确定删除该赏品吗？'
}

function confirmRemove(record: PlayPoolPrize) {
  Modal.confirm({
    title: deleteConfirmTitle(record),
    okText: '确认',
    cancelText: '取消',
    okType: 'danger',
    closable: true,
    centered: true,
    width: 320,
    wrapClassName: 'pool-delete-confirm',
    onOk: () => removePrize(record.prizeIndex),
  })
}

function removePrize(index: number) {
  const target = prizes.value.find((item) => item.prizeIndex === index)
  if (!target) return
  const removeIndexes = new Set<number>([index])
  if (!target.hasParent) {
    for (const item of prizes.value) {
      if (item.hasParent && item.parentIndex === index) removeIndexes.add(item.prizeIndex)
    }
  }
  prizes.value = prizes.value.filter((item) => !removeIndexes.has(item.prizeIndex))
  reindexPrizes()
}

// 与后端目标分配保持一致：只按“有奖档”的剩余库存权重计算概率；
// 无奖档（谢谢参与）概率固定 0.00，不参与分配。
function prizeAllocationWeight(prize: PlayPoolPrize): number {
  if (prize.isPrize === false) return 0
  const remaining = Number(prize.remaining ?? NaN)
  if (Number.isFinite(remaining)) {
    if (remaining === 0) return 0
    if (remaining < 0) return 1
    return remaining
  }
  return Math.max(0, Number(prize.quantity ?? 0))
}

const totalPrizeUnits = computed(() =>
  prizes.value.reduce((sum, prize) => sum + prizeAllocationWeight(prize), 0),
)

function probability(prize: DisplayPrize): string {
  if (totalPrizeUnits.value <= 0) return '0.00'
  const weight = prize.__expanded ? 1 : prizeAllocationWeight(prize)
  return ((100 / totalPrizeUnits.value) * weight).toFixed(2)
}

async function loadSkuOptions() {
  const [brandReply, seriesReply] = await Promise.all([
    catalogBrandClient.List({ keyword: '', page: 1, pageSize: 100 }).catch(() => ({ brands: [] })),
    catalogSeriesClient.List({ keyword: '', brandId: '', page: 1, pageSize: 100 }).catch(() => ({ series: [] })),
  ])
  skuBrands.value = brandReply.brands ?? []
  skuSeries.value = seriesReply.series ?? []
  skuOptionsLoaded.value = true
}

function existingSkuPrizeMap(): Map<number, PlayPoolPrize> {
  const map = new Map<number, PlayPoolPrize>()
  for (const prize of prizes.value) {
    const skuId = Number(prize.skuId ?? 0)
    if (prize.prizeType === 2 && skuId > 0 && !prize.hasParent && !map.has(skuId)) {
      map.set(skuId, prize)
    }
  }
  return map
}

function isExistingSku(id: string): boolean {
  return prizes.value.some(
    (prize) => prize.prizeType === 2 && !prize.hasParent && Number(prize.skuId ?? 0) === Number(id),
  )
}

function resetSkuPickerState() {
  selectedSkuKeys.value = []
  skuQuantities.value = {}
  skuProductCache.clear()
  skuTouchedKeys.clear()
  skuRemovedIds.clear()
}

function syncSkuSelection(products: AdminProductItem[]) {
  const childMode = !!childParent.value
  const existing = childMode ? new Map<number, PlayPoolPrize>() : existingSkuPrizeMap()
  const quantities = { ...skuQuantities.value }
  const selected = new Set(selectedSkuKeys.value)
  for (const item of products) {
    const key = item.productId
    if (!key) continue
    skuProductCache.set(key, item)
    if (quantities[key] === undefined) {
      const prize = existing.get(Number(key))
      quantities[key] = form.mode === 'memorymatch'
        ? 1
        : prize ? Math.max(1, Math.trunc(Number(prize.quantity ?? 1))) : 1
    }
    if (!childMode && !skuTouchedKeys.has(key) && existing.has(Number(key))) {
      selected.add(key)
    }
  }
  skuQuantities.value = quantities
  selectedSkuKeys.value = [...selected]
}

async function loadSkuProducts() {
  skuLoading.value = true
  try {
    const reply = await catalogAdminProductClient.List({
      keyword: skuFilters.keyword.trim(),
      brandId: skuFilters.brandId ?? '',
      seriesId: skuFilters.seriesId ?? '',
      ipId: '',
      saleType: '',
      minPriceMinor: 0,
      maxPriceMinor: 0,
      sortBy: '',
      sortOrder: '',
      page: skuPage.value,
      pageSize: skuPageSize.value,
      status: skuFilters.status ?? '',
    })
    const products = reply.products ?? []
    skuItems.value = products
    syncSkuSelection(products)
    skuTotal.value = Number(reply.total ?? 0)
    skuPage.value = Number(reply.page ?? skuPage.value)
    skuPageSize.value = Number(reply.pageSize ?? skuPageSize.value)
  } catch {
    skuItems.value = []
    skuTotal.value = 0
  } finally {
    skuLoading.value = false
  }
}

interface SkuPickerOptions {
  keyword?: string
  status?: string | null
  brandId?: string
  seriesId?: string
  page?: number
  childParent?: PlayPoolPrize | null
  locatedSkuId?: string
}

function scrollToLocatedSku() {
  if (!locatedSkuId.value) return
  void nextTick(() => {
    requestAnimationFrame(() => {
      const wrap = skuTableWrap.value
      if (!wrap) return
      const located = wrap.querySelector('.sku-located-id')
      const row = (wrap.querySelector('.sku-located-row') ?? located?.closest('tr')) as HTMLElement | null
      if (!row) return
      row.scrollIntoView({ block: 'center', behavior: 'smooth' })
      row.focus({ preventScroll: true })
    })
  })
}

function openSkuPicker(options: SkuPickerOptions = {}) {
  childParent.value = options.childParent ?? null
  locatedSkuId.value = options.locatedSkuId ?? ''
  skuModalOpen.value = true
  skuFilters.keyword = options.keyword ?? ''
  skuFilters.brandId = options.brandId
  skuFilters.seriesId = options.seriesId
  skuFilters.status = options.status === null ? undefined : options.status ?? 'PRODUCT_STATUS_ACTIVE'
  skuPage.value = options.page ?? 1
  resetSkuPickerState()
  if (!skuOptionsLoaded.value) void loadSkuOptions()
  void loadSkuProducts().then(() => scrollToLocatedSku())
}

function openChildPicker(record: PlayPoolPrize) {
  if (form.mode !== 'groupbuy') {
    message.warning('仅拼团赏支持子赏')
    return
  }
  if (record.hasParent) {
    message.warning('子赏不能再添加子赏')
    return
  }
  openSkuPicker({ childParent: record, status: 'PRODUCT_STATUS_ACTIVE' })
}

function onSkuPickerCancel() {
  skuModalOpen.value = false
  childParent.value = null
  resetSkuPickerState()
}

async function findSkuPage(
  filters: { brandId?: string; seriesId?: string; status?: string | null },
  productId: number,
): Promise<number | null> {
  const pageSize = skuPageSize.value
  const fetchPage = (page: number) =>
    catalogAdminProductClient.List({
      keyword: '',
      brandId: filters.brandId ?? '',
      seriesId: filters.seriesId ?? '',
      ipId: '',
      saleType: '',
      minPriceMinor: 0,
      maxPriceMinor: 0,
      sortBy: '',
      sortOrder: '',
      page,
      pageSize,
      status: filters.status ?? '',
    })
  const contains = (reply: Awaited<ReturnType<typeof fetchPage>>, id: number) =>
    (reply.products ?? []).some((item) => Number(item.productId ?? 0) === id)

  let first
  try {
    first = await fetchPage(1)
  } catch {
    return null
  }
  const total = Number(first.total ?? 0)
  if (total <= 0) return null
  if (contains(first, productId)) return 1

  const totalPages = Math.ceil(total / pageSize)
  let low = 2
  let high = totalPages
  while (low <= high) {
    const mid = Math.floor((low + high) / 2)
    let reply
    try {
      reply = await fetchPage(mid)
    } catch {
      return null
    }
    const products = reply.products ?? []
    if (products.length === 0) return null
    if (contains(reply, productId)) return mid
    const firstId = Number(products[0]?.productId ?? 0)
    const lastId = Number(products[products.length - 1]?.productId ?? 0)
    if (productId > firstId) {
      high = mid - 1
    } else if (productId < lastId) {
      low = mid + 1
    } else {
      return null
    }
  }
  return null
}

async function openSkuPickerForPrize(record: PlayPoolPrize) {
  const productId = Number(record.skuId ?? 0)
  if (!productId) {
    openSkuPicker({ status: null })
    return
  }
  try {
    const detail = await catalogAdminProductClient.Get(String(productId))
    const product = detail.product
    if (product?.brandId || product?.seriesId) {
      const page = await findSkuPage({ brandId: product.brandId, seriesId: product.seriesId, status: null }, productId)
      if (page) {
        openSkuPicker({ brandId: product.brandId, seriesId: product.seriesId, status: null, page, locatedSkuId: String(productId) })
        return
      }
    }
  } catch {
    // 查询商品详情或定位页码失败时回退到按商品 ID 定位。
  }
  openSkuPicker({ keyword: String(productId), status: null, locatedSkuId: String(productId) })
}

let skuKeywordTimer: number | undefined

function searchSku() {
  if (skuKeywordTimer) {
    window.clearTimeout(skuKeywordTimer)
    skuKeywordTimer = undefined
  }
  skuPage.value = 1
  void loadSkuProducts()
}

function onSkuKeywordInput() {
  if (skuKeywordTimer) window.clearTimeout(skuKeywordTimer)
  skuKeywordTimer = window.setTimeout(() => {
    skuKeywordTimer = undefined
    skuPage.value = 1
    void loadSkuProducts()
  }, 300)
}

function onSkuFilterChange() {
  skuPage.value = 1
  void loadSkuProducts()
}

function onSkuBrandChange() {
  if (skuFilters.seriesId && !skuSeriesOptions.value.some((item) => item.value === skuFilters.seriesId)) {
    skuFilters.seriesId = undefined
  }
  onSkuFilterChange()
}

function onSkuPageChange(nextPage: number) {
  skuPage.value = nextPage
  void loadSkuProducts()
}

function onSkuPageSizeChange(_current: number, nextPageSize: number) {
  skuPageSize.value = nextPageSize
  skuPage.value = 1
  void loadSkuProducts()
}

function skuSaleTypeText(value?: string): string {
  return value === 'PREORDER' ? '预售' : '现货'
}

function skuMoney(value?: string): string {
  return String(Number(value ?? 0) / 100)
}

function onSkuQuantityChange(record: AdminProductItem) {
  const id = record.productId
  if (!id) return
  skuTouchedKeys.add(id)
  skuRemovedIds.delete(id)
  if (!selectedSkuKeys.value.includes(id)) {
    selectedSkuKeys.value = [...selectedSkuKeys.value, id]
  }
}

function toggleSkuSelection(record: AdminProductItem) {
  const id = record.productId
  if (!id) return
  skuTouchedKeys.add(id)
  if (selectedSkuKeys.value.includes(id)) {
    selectedSkuKeys.value = selectedSkuKeys.value.filter((key) => key !== id)
    if (isExistingSku(id)) skuRemovedIds.add(id)
  } else {
    selectedSkuKeys.value = [...selectedSkuKeys.value, id]
    skuRemovedIds.delete(id)
  }
}

function skuCustomRow(record: AdminProductItem) {
  const located = String(record.productId ?? '') === locatedSkuId.value
  return {
    class: located ? 'sku-located-row' : undefined,
    tabindex: located ? -1 : undefined,
    style: { cursor: 'pointer' },
    onClick: (event: MouseEvent) => {
      const target = event.target as HTMLElement | null
      if (target?.closest('input, textarea, button, a, .ant-checkbox-wrapper, .ant-input-number, .ant-image')) return
      toggleSkuSelection(record)
    },
  }
}

function setPrizeTarget(record: DisplayPrize, value: number | null) {
  const prize = prizes.value.find((item) => item.prizeIndex === record.prizeIndex)
  if (prize) prize.targetValue = value ?? 0
}

function prizeCustomRow(record: PlayPoolPrize) {
  const clickable = record.prizeType === 2 && !record.hasParent
  return {
    style: clickable ? { cursor: 'pointer' } : undefined,
    onClick: (event: MouseEvent) => {
      if (!clickable) return
      const target = event.target as HTMLElement | null
      if (target?.closest('button, a, input, textarea, .ant-select, .ant-switch')) return
      void openSkuPickerForPrize(record)
    },
  }
}

function cancelSkuPicker() {
  skuModalOpen.value = false
  childParent.value = null
  resetSkuPickerState()
}

function confirmSkuSelection() {
  if (childParent.value) {
    const parent = childParent.value
    const selected = selectedSkuKeys.value
      .map((key) => skuProductCache.get(key))
      .filter((item): item is AdminProductItem => !!item)
    if (selected.length === 0) {
      message.warning('请先勾选赏品')
      return
    }
    for (const item of selected) {
      const id = Number(item.productId ?? 0)
      if (!id) continue
      const quantity = Math.max(1, Math.trunc(Number(skuQuantities.value[item.productId ?? ''] ?? 1)))
      prizes.value.push({
        prizeIndex: prizes.value.length,
        prizeGrade: parent.prizeGrade || prizeDraft.prizeGrade,
        specialKind: '',
        prizeType: 2,
        name: item.title?.trim() || `SKU ${id}`,
        skuId: id,
        childPoolId: 0,
        imageUrl: item.coverUrl || '',
        weight: 1,
        quantity,
        remaining: quantity,
        isPrize: true,
        costFen: Number(item.costAmountMinor ?? 0),
        hasParent: true,
        parentIndex: parent.prizeIndex,
      })
    }
    reindexPrizes()
    skuModalOpen.value = false
    childParent.value = null
    resetSkuPickerState()
    message.success(`已添加 ${selected.length} 个子赏`)
    void nextTick(() => {
      window.setTimeout(() => {
        const wrap = prizeTableWrap.value
        if (wrap) wrap.scrollTop = wrap.scrollHeight
      }, 0)
    })
    return
  }

  const existing = existingSkuPrizeMap()
  const selected = selectedSkuKeys.value
    .map((key) => skuProductCache.get(key))
    .filter((item): item is AdminProductItem => !!item)
  if (selected.length === 0 && skuRemovedIds.size === 0) {
    message.warning('请先勾选赏品')
    return
  }

  // 已存在商品：更新勾选后的数量，不重复添加。
  for (const item of selected) {
    const id = Number(item.productId ?? 0)
    const prize = existing.get(id)
    if (!prize) continue
    const quantity = form.mode === 'memorymatch'
      ? 1
      : Math.max(1, Math.trunc(Number(skuQuantities.value[item.productId ?? ''] ?? 1)))
    prize.quantity = quantity
    prize.remaining = quantity
    prize.imageUrl = item.coverUrl || prize.imageUrl || ''
  }

  // 已存在但被取消勾选的商品：无论在哪一页，都从赏品表格移除。
  if (skuRemovedIds.size > 0) {
    prizes.value = prizes.value.filter((prize) => {
      const skuId = Number(prize.skuId ?? 0)
      if (prize.hasParent || prize.prizeType !== 2 || skuId <= 0) return true
      return !skuRemovedIds.has(String(skuId))
    })
  }

  // 新勾选的商品：追加到赏品表格。
  for (const item of selected) {
    const id = Number(item.productId ?? 0)
    if (!id || existing.has(id)) continue
    const quantity = form.mode === 'memorymatch'
      ? 1
      : Math.max(1, Math.trunc(Number(skuQuantities.value[item.productId ?? ''] ?? 1)))
    prizes.value.push({
      prizeIndex: prizes.value.length,
      prizeGrade: form.mode === 'memorymatch' ? 'PairCard' : prizeDraft.prizeGrade,
      specialKind: '',
      prizeType: 2,
      name: item.title?.trim() || `SKU ${id}`,
      skuId: id,
      childPoolId: 0,
      imageUrl: item.coverUrl || '',
      weight: 1,
      quantity,
      remaining: quantity,
      isPrize: prizeDraft.isPrize,
      costFen: Number(item.costAmountMinor ?? 0),
      targetValue: newPrizeTargetValue(),
    })
  }

  reindexPrizes()
  if (form.mode === 'memorymatch') normalizeMemoryMatchQuantities()
  skuModalOpen.value = false
  resetSkuPickerState()
  message.success('赏品选择已更新')
  void nextTick(() => {
    window.setTimeout(() => {
      const wrap = prizeTableWrap.value
      if (wrap) wrap.scrollTop = wrap.scrollHeight
    }, 0)
  })
}

function addCoupon() {
  if (!couponId.value || couponId.value <= 0) return
  if (coupons.value.some((item) => item.id === couponId.value)) return
  coupons.value.push({ id: couponId.value, name: '-', description: '-', status: '待查询' })
  couponId.value = null
}

function removeCoupon(id: number) {
  coupons.value = coupons.value.filter((item) => item.id !== id)
}

async function uploadImage(file: File, field: 'coverImage' | 'coverThumb'): Promise<boolean> {
  if (!file.type.startsWith('image/')) {
    message.warning('请选择图片文件')
    return false
  }
  if (file.size > 10 * 1024 * 1024) {
    message.warning('图片不能超过 10MB')
    return false
  }
  try {
    const reply = await uploadFileToOss(file, 'gacha/pool')
    if (reply.url) form[field] = reply.url
  } catch {
    // 错误提示由请求拦截器统一处理
  }
  return false
}

async function submit() {
  if (!form.name.trim()) {
    message.warning('请输入主题名称')
    return
  }
  if (prizes.value.length === 0) {
    message.warning('请至少添加一个赏品')
    return
  }
  if (form.mode === 'memorymatch' && !prizes.value.some((prize) => prize.isPrize !== false)) {
    message.warning('请至少添加一个有奖档（商品），不能只有谢谢参与')
    return
  }
  const tags: string[] = []
  if (form.isOnSale) tags.push('展示')
  if (form.isNew) tags.push('新品')
  if (form.isRecommended) tags.push('推荐')
  if (form.isBargain) tags.push('捡漏')
  if (form.isPresale) tags.push('预售')
  const adminJson = JSON.stringify({
    startAt: form.startAt ? form.startAt.format('YYYY-MM-DD HH:mm:ss') : '',
    endAt: form.endAt ? form.endAt.format('YYYY-MM-DD HH:mm:ss') : '',
    discount: form.discount,
    isOnSale: form.isOnSale,
    isNew: form.isNew,
    isRecommended: form.isRecommended,
    isBargain: form.isBargain,
    isPresale: form.isPresale,
    isMiniapp: form.isMiniapp,
    isWelfare: form.isWelfare,
    excludeTask: form.excludeTask,
    allowBoxInBox: form.allowBoxInBox,
    coverImage: form.coverImage,
    coverThumb: form.coverThumb,
    coupons: coupons.value,
  })
  const configs: Record<string, string> = {
    gashapon: '{}',
    fullcar: '{"seatCount":4}',
    combochain: '{"ticketsPerCar":2,"totalCars":10,"tierTriggerProbabilityBps":300}',
    dig: '{"cellCount":9}',
    groupbuy: '{"targetBetCount":3,"perBetAwardLimit":0}',
    memorymatch: '{"followGlobal":true,"roundsPerPurchase":3,"maxPairCount":3,"patternWeights":[1,1,1,1,1,1,1,1,1],"facePresets":[{"presetId":1,"name":"默认","faces":[]}],"backPresets":[{"presetId":1,"name":"默认","backImageUrl":"","faceBackgroundImageUrl":""}]}',
  }
  if (form.mode === 'memorymatch') {
    if (!Number.isFinite(mmConfig.roundsPerPurchase) || mmConfig.roundsPerPurchase < 1 || mmConfig.roundsPerPurchase > 100) {
      message.warning('回合数需为 1-100')
      return
    }
    if (!Number.isFinite(mmConfig.maxPairCount) || mmConfig.maxPairCount < 1 || mmConfig.maxPairCount > 27) {
      message.warning('每回合最大碰数需为 1-27')
      return
    }
    const maxTarget = Math.round(mmConfig.roundsPerPurchase) * Math.round(mmConfig.maxPairCount)
    normalizeMemoryMatchQuantities()
    const usedTargets = new Map<number, number>()
    const usedSkus = new Set<number>()
    for (let i = 0; i < prizes.value.length; i += 1) {
      const prize = prizes.value[i]
      if (!prize) continue
      if (Number(prize.prizeType ?? 0) === 3 || Number(prize.childPoolId ?? 0) > 0 || prize.hasParent) {
        message.error(`第 ${i + 1} 个奖档是箱子/子赏，对对碰不支持，请改成商品或删除`)
        return
      }
      const skuId = Number(prize.skuId ?? 0)
      if (Number(prize.prizeType ?? 0) === 2 && skuId > 0) {
        if (usedSkus.has(skuId)) {
          message.error(`同一商品不能重复配置：${prize.name}`)
          return
        }
        usedSkus.add(skuId)
      }
      const target = Number(prize.targetValue ?? 0)
      if (!Number.isInteger(target) || target < 0) {
        message.warning(`第 ${i + 1} 个奖档的目标碰数必须是 0-${maxTarget} 的整数`)
        return
      }
      if (target > maxTarget) {
        message.warning(`第 ${i + 1} 个奖档的目标碰数 ${target} 超过总碰数上限 ${maxTarget}`)
        return
      }
      if (usedTargets.has(target)) {
        message.error(`目标碰数重复：${target}（第 ${usedTargets.get(target)! + 1} 个和第 ${i + 1} 个奖档），请改成不同的值`)
        return
      }
      usedTargets.set(target, i)
      if (Number(prize.weight ?? 0) <= 0) {
        message.warning(`第 ${i + 1} 个奖档的权重必须大于 0`)
        return
      }
    }
  }
  const rawConfig =
    initialConfigJson.value && form.mode === initialMode.value
      ? initialConfigJson.value
      : configs[form.mode] ?? '{}'
  let configObject: Record<string, unknown> = {}
  try {
    const parsed = JSON.parse(rawConfig) as unknown
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      configObject = parsed as Record<string, unknown>
    }
  } catch {
    configObject = {}
  }
  if (form.mode === 'memorymatch') {
    configObject.roundsPerPurchase = Math.round(mmConfig.roundsPerPurchase)
    configObject.maxPairCount = Math.round(mmConfig.maxPairCount)
  }
  const configJson = JSON.stringify(configObject)
  saving.value = true
  try {
    await playPoolClient.CreateDraft({
      idempotencyKey: crypto.randomUUID(),
      poolId: initialPoolId.value,
      mode: form.mode,
      name: form.name.trim(),
      priceFen: Math.round((form.priceYuan || 0) * 100),
      configJson,
      prizes: prizes.value,
      ipId: form.ipId,
      tags,
      periods: form.boxCount,
      weight: form.sortWeight,
      adminJson,
    })
    message.success(initialPoolId.value ? '赏池已更新' : '赏池已创建')
    emit('success')
  } catch {
    message.error('创建失败，请检查赏品配置')
  } finally {
    saving.value = false
  }
}

onMounted(loadOptions)
</script>

<style scoped lang="scss">
.create-form {
  width: 100%;
}

.field-hint {
  margin-top: 4px;
  font-size: 12px;
  line-height: 1.4;
  color: #8a94a6;
}

.form-layout {
  display: grid;
  grid-template-columns: clamp(360px, 30%, 480px) minmax(0, 1fr);
  gap: 28px;
  align-items: stretch;
}

.form-left,
.form-right {
  min-width: 0;
}

.form-right {
  position: relative;
  display: flex;
  flex-direction: column;
}

.prize-section {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
  margin-bottom: 0;
}

.prize-label {
  padding: 6px 0 10px;
  color: #606266;
  font-size: 14px;
  font-weight: 500;
  text-align: left;
}

.prize-hint {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin: -4px 0 8px;
  color: #8a94a6;
  font-size: 12px;
  line-height: 1.5;
  text-align: left;
}

.target-summary {
  margin: 0 0 10px;
  padding: 8px 10px;
  border: 1px solid #f0f0f0;
  border-radius: 6px;
  background: #fafafa;
  font-size: 12px;
  line-height: 1.6;
  text-align: left;
}

.target-summary-line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
}

.target-summary-label {
  color: #606266;
}

.target-summary :deep(.ant-tag) {
  margin-inline-end: 0;
}

.target-summary-muted {
  color: #8a94a6;
}

.target-summary-error {
  margin-top: 4px;
  color: #cf1322;
}

.target-summary-ok {
  margin-top: 4px;
  color: #389e0d;
}

.prize-config {
  display: flex;
  flex: 1;
  flex-direction: column;
  width: 100%;
  min-width: 0;
  min-height: 0;
  border: 1px solid #f0f0f0;
}

.prize-table-wrap {
  flex: 1;
  min-height: 0;
  overflow-x: hidden;
  overflow-y: auto;
}

.prize-table-wrap :deep(.ant-table-tbody > tr > td) {
  padding-top: 8px;
  padding-bottom: 8px;
}

.prize-inline {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
  padding: 8px;
  border-top: 1px solid #f0f0f0;
}

.prize-picker {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 8px;
}

.inline-label {
  color: #606266;
  font-size: 13px;
  white-space: nowrap;
}

.sku-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 12px;
}

.sku-located-id {
  color: #ff4d4f;
  font-weight: 600;
}

:deep(.sku-located-row > td:first-child) {
  box-shadow: inset 4px 0 0 #1677ff;
}

:deep(.sku-located-row:focus) {
  outline: 2px solid #1677ff;
  outline-offset: -2px;
}

.sku-table-wrap {
  height: calc(100vh - 350px);
  min-height: 240px;
  overflow-x: auto;
  overflow-y: scroll;
}

.sku-table-wrap :deep(.ant-table-thead th) {
  position: sticky;
  top: 0;
  z-index: 2;
  background: #fafafa;
}

.sku-qty-input :deep(.ant-input-number-input) {
  text-align: center;
}

.sku-picker-title {
  text-align: center;
}

:global(.pool-delete-confirm .ant-modal-confirm-btns) {
  display: flex;
  flex-direction: row-reverse;
  justify-content: center;
  gap: 8px;
}

:global(.pool-delete-confirm .ant-modal-confirm-btns .ant-btn + .ant-btn) {
  margin-left: 0;
}

.sku-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}

.sku-confirm {
  display: flex;
  justify-content: center;
  gap: 10px;
  margin-top: 12px;
}

.sku-cancel-btn {
  min-width: 96px;
}

.sku-confirm-btn {
  min-width: 96px;
  background: #42b983 !important;
  border-color: #42b983 !important;
}

.sku-confirm-btn:hover {
  background: #36a374 !important;
  border-color: #36a374 !important;
}

.brand-series {
  display: flex;
  flex-direction: column;
  line-height: 1.4;
}

.coupon-config {
  width: 100%;
}

.coupon-table {
  width: calc(127.7778% - 3px);
  max-width: none;
  margin-top: 10px;
  margin-left: calc(-27.7778% + 3px);
  border: 1px solid #f0f0f0;
}

.cover-row {
  display: flex;
  align-items: flex-start;
  gap: 30px;
}

.cover-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.cover-label {
  color: #606266;
  font-size: 13px;
  text-align: center;
}

.image-upload-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 96px;
  height: 96px;
  overflow: hidden;
  color: #8c8c8c;
  border: 1px dashed #d9d9d9;
  border-radius: 6px;
  background: #fafafa;
}

.image-upload-box img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.image-upload-box span {
  margin-top: 4px;
  font-size: 12px;
}

.switch-panel {
  margin: 4px 0 0;
  padding: 14px 16px;
  background: #fafafa;
  border: 1px solid #f0f0f0;
  border-radius: 4px;
}

.switch-panel-title {
  margin-bottom: 10px;
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}

.switch-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px 8px;
}

.switch-item {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  color: #606266;
  font-size: 13px;
  cursor: pointer;
}

.switch-item span {
  min-width: 0;
  line-height: 1.4;
}

.create-actions {
  display: flex;
  justify-content: center;
  gap: 10px;
  margin: 16px 0 0;
}

.create-actions button {
  min-width: 72px;
}

.btn-create {
  min-width: 72px;
  background: #42b983 !important;
  border-color: #42b983 !important;
}

.btn-create:hover {
  background: #36a374 !important;
  border-color: #36a374 !important;
}
</style>

<template>
  <div class="create-form">
    <div class="form-layout">
      <div class="form-left">
      <a-form :label-col="{ span: 5 }" :wrapper-col="{ span: 18 }" label-align="right">
        <a-form-item label="主题名称" required>
          <a-input v-model:value="form.name" placeholder="主题名称" />
        </a-form-item>
        <a-form-item label="IP" required>
          <a-select v-model:value="form.ipId" :options="ipOptions" placeholder="IP" show-search option-filter-prop="label" />
        </a-form-item>
        <a-form-item label="玩法类型" required>
          <a-select v-model:value="form.mode" :options="modeOptions" />
        </a-form-item>

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
              <a-input-number v-model:value="couponId" :min="0" :precision="0" placeholder="优惠券id" style="width: 160px" />
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

        <a-form-item :wrapper-col="{ span: 18, offset: 5 }">
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
          <div class="prize-label">赏品配置：</div>
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
                @toggle="togglePrizeExpand"
                @remove="confirmRemove"
              />
            </div>

            <div class="prize-inline">
              <a-select v-model:value="prizeDraft.prizeGrade" :options="gradeOptions" style="width: 90px" />
              <a-select v-model:value="prizeDraft.prizeType" :options="prizeTypeOptions" style="width: 85px" />
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
        <a-button class="btn-create" type="primary" :loading="saving" @click="submit">创建</a-button>
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

        <div class="sku-table-wrap">
        <a-table
          :data-source="skuItems"
          :loading="skuLoading"
          :pagination="false"
          :row-key="(record: AdminProductItem) => record.productId ?? ''"
          :row-selection="skuRowSelection"
          :custom-row="skuCustomRow"
          size="small"
        >
          <a-table-column title="ID" data-index="productId" :width="190" align="center" />
          <a-table-column title="名称" data-index="title" align="center" />
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
          <a-table-column title="数量" :width="80" align="center">
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
import { playPoolClient, type PlayPoolPrize } from '@/api/playPool'
import { poolClient } from '@/api/pool'
import DefaultPrizeTable from '@/views/gacha/prize/DefaultPrizeTable.vue'
import GroupBuyPrizeTable from '@/views/gacha/prize/GroupBuyPrizeTable.vue'
import type { DisplayPrize } from '@/views/gacha/prize/prizeTypes'

const emit = defineEmits<{ success: []; cancel: [] }>()
const saving = ref(false)
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
const prizeTypeOptions = [
  { label: '商品', value: 2 },
  { label: '箱子', value: 3 },
]

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
const displayPrizes = computed<DisplayPrize[]>(() => {
  const rows: DisplayPrize[] = []
  let displayIndex = 0
  let badgeIndex = 0
  for (const prize of prizes.value) {
    const total = Number(prize.quantity ?? 1)
    const expanded = total > 1 && expandedPrizeIndexes.value.has(prize.prizeIndex)
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
const skuModalOpen = ref(false)
const childParent = ref<PlayPoolPrize | null>(null)
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

function togglePrizeExpand(prize: PlayPoolPrize) {
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

const totalPrizeUnits = computed(() =>
  prizes.value.reduce((sum, prize) => sum + Math.max(0, Number(prize.quantity ?? 0)), 0),
)

function probability(prize: DisplayPrize): string {
  if (totalPrizeUnits.value <= 0) return '0'
  const factor = prize.__expanded ? 1 : Math.max(0, Number(prize.quantity ?? 1))
  return ((100 / totalPrizeUnits.value) * factor).toFixed(2)
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
      quantities[key] = prize ? Math.max(1, Math.trunc(Number(prize.quantity ?? 1))) : 1
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
}

function openSkuPicker(options: SkuPickerOptions = {}) {
  childParent.value = options.childParent ?? null
  skuModalOpen.value = true
  skuFilters.keyword = options.keyword ?? ''
  skuFilters.brandId = options.brandId
  skuFilters.seriesId = options.seriesId
  skuFilters.status = options.status === null ? undefined : options.status ?? 'PRODUCT_STATUS_ACTIVE'
  skuPage.value = options.page ?? 1
  resetSkuPickerState()
  if (!skuOptionsLoaded.value) void loadSkuOptions()
  void loadSkuProducts()
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
        openSkuPicker({ brandId: product.brandId, seriesId: product.seriesId, status: null, page })
        return
      }
    }
  } catch {
    // 查询商品详情或定位页码失败时回退到按商品 ID 定位。
  }
  openSkuPicker({ keyword: String(productId), status: null })
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
  return {
    style: { cursor: 'pointer' },
    onClick: (event: MouseEvent) => {
      const target = event.target as HTMLElement | null
      if (target?.closest('input, textarea, button, a, .ant-checkbox-wrapper, .ant-input-number, .ant-image')) return
      toggleSkuSelection(record)
    },
  }
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
    const quantity = Math.max(1, Math.trunc(Number(skuQuantities.value[item.productId ?? ''] ?? 1)))
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
    const quantity = Math.max(1, Math.trunc(Number(skuQuantities.value[item.productId ?? ''] ?? 1)))
    prizes.value.push({
      prizeIndex: prizes.value.length,
      prizeGrade: prizeDraft.prizeGrade,
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
    })
  }

  reindexPrizes()
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

function readImage(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result ?? ''))
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(file)
  })
}

async function uploadImage(file: File, field: 'coverImage' | 'coverThumb'): Promise<boolean> {
  if (!file.type.startsWith('image/')) {
    message.warning('请选择图片文件')
    return false
  }
  if (file.size > 2 * 1024 * 1024) {
    message.warning('图片不能超过 2MB')
    return false
  }
  try {
    form[field] = await readImage(file)
  } catch {
    // 忽略读取失败
  }
  return false
}

async function submit() {
  if (!form.name.trim()) {
    message.warning('请输入主题名称')
    return
  }
  if (!form.ipId) {
    message.warning('请选择 IP')
    return
  }
  if (prizes.value.length === 0) {
    message.warning('请至少添加一个赏品')
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
    memorymatch: '{"roundsPerPurchase":3,"maxPairCount":3,"patternWeights":[1,1,1,1,1,1,1,1,1],"facePresets":[{"presetId":1,"name":"默认","faces":[]}],"backPresets":[{"presetId":1,"name":"默认","backImageUrl":"","faceBackgroundImageUrl":""}]}',
  }
  saving.value = true
  try {
    await playPoolClient.CreateDraft({
      idempotencyKey: crypto.randomUUID(),
      mode: form.mode,
      name: form.name.trim(),
      priceFen: Math.round((form.priceYuan || 0) * 100),
      configJson: configs[form.mode] ?? '{}',
      prizes: prizes.value,
      ipId: form.ipId,
      tags,
      periods: form.boxCount,
      weight: form.sortWeight,
      adminJson,
    })
    message.success('赏池已创建')
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
  gap: 40px;
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

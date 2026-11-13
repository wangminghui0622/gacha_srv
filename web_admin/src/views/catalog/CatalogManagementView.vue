<template>
  <div class="product-page">
    <div class="product-toolbar">
      <a-input
        v-model:value="filters.keyword"
        class="keyword-input"
        :placeholder="$t('catalog.keywordPlaceholder')"
        allow-clear
        @press-enter="search"
      />
      <a-select
        v-model:value="filters.brandId"
        class="filter-select"
        :placeholder="$t('catalog.brandPlaceholder')"
        :options="brandOptions"
        allow-clear
        show-search
        option-filter-prop="label"
        @change="onFilterBrandChange"
      />
      <a-select
        v-model:value="filters.seriesId"
        class="filter-select"
        :placeholder="$t('catalog.seriesPlaceholder')"
        :options="filteredSeriesOptions"
        allow-clear
        show-search
        option-filter-prop="label"
      />
      <a-select
        v-model:value="filters.ipId"
        class="filter-select"
        :placeholder="$t('catalog.ipPlaceholder')"
        :options="ipOptions"
        allow-clear
        show-search
        option-filter-prop="label"
      />
      <a-select
        v-model:value="filters.saleType"
        class="filter-select"
        :placeholder="$t('catalog.saleTypePlaceholder')"
        :options="saleTypeOptions"
        allow-clear
      />
      <a-input-number
        v-model:value="filters.minPrice"
        class="price-input"
        :controls="false"
        :min="0"
        :placeholder="$t('catalog.minPricePlaceholder')"
      />
      <span class="price-sep">-</span>
      <a-input-number
        v-model:value="filters.maxPrice"
        class="price-input"
        :controls="false"
        :min="0"
        :placeholder="$t('catalog.maxPricePlaceholder')"
      />
      <a-button class="btn-query" type="primary" :loading="loading" @click="search">{{ $t('catalog.query') }}</a-button>
      <a-button class="btn-reset" @click="reset">{{ $t('catalog.reset') }}</a-button>
      <a-button v-if="canCreate" class="btn-create" @click="openCreate">{{ $t('catalog.create') }}</a-button>
    </div>

    <a-table
      :data-source="items"
      :loading="loading"
      :pagination="false"
      :row-key="(record: AdminProductItem) => record.productId ?? ''"
      :row-selection="rowSelection"
      :scroll="{ x: 1560 }"
      size="middle"
      @change="onTableChange"
    >
      <a-table-column :title="$t('catalog.id')" data-index="productId" width="170" align="center" />
      <a-table-column :title="$t('catalog.name')" data-index="title" min-width="420" align="center" />
      <a-table-column :title="$t('catalog.ip')" data-index="ipName" width="90" align="center" />
      <a-table-column :title="$t('catalog.brandSeries')" width="150" align="center">
        <template #default="{ record }">
          <div class="brand-series">
            <span>{{ record.brandName || '-' }}</span>
            <span>{{ record.seriesName || '-' }}</span>
          </div>
        </template>
      </a-table-column>
      <a-table-column :title="$t('catalog.status')" width="100" align="center">
        <template #default="{ record }">
          <a-tag :color="record.saleType === 'PREORDER' ? 'orange' : 'blue'">
            {{ saleTypeText(record.saleType) }}
          </a-tag>
        </template>
      </a-table-column>
      <a-table-column
        key="shippedQuantity"
        :title="$t('catalog.shippedQuantity')"
        data-index="shippedQuantity"
        width="130"
        align="center"
        :sorter="true"
      >
        <template #default="{ record }">{{ record.shippedQuantity ?? '0' }}</template>
      </a-table-column>
      <a-table-column
        key="costAmountMinor"
        :title="$t('catalog.costPrice')"
        data-index="costAmountMinor"
        width="110"
        align="center"
        :sorter="true"
      >
        <template #default="{ record }">{{ formatMoney(record.costAmountMinor) }}</template>
      </a-table-column>
      <a-table-column :title="$t('catalog.cover')" width="100" align="center">
        <template #default="{ record }">
          <a-image v-if="record.coverUrl" :src="record.coverUrl" :width="40" :height="40" :preview="false" />
        </template>
      </a-table-column>
      <a-table-column :title="$t('catalog.action')" width="220" align="center" fixed="right">
        <template #default="{ record }">
          <a-space :size="4">
            <a-button v-if="canCreate" class="btn-copy" size="small" :loading="copyingId === record.productId" @click="copy(record)">{{ $t('catalog.copy') }}</a-button>
            <a-button v-if="canWrite" class="btn-edit" size="small" @click="openEdit(record)">{{ $t('catalog.edit') }}</a-button>
            <a-button class="btn-detail" size="small" @click="detail(record)">{{ $t('catalog.detail') }}</a-button>
          </a-space>
        </template>
      </a-table-column>
    </a-table>

    <div class="product-footer">
      <a-pagination
        :current="page"
        :page-size="pageSize"
        :total="total"
        :page-size-options="pageSizeOptions"
        :locale="paginationLocale"
        show-size-changer
        show-less-items
        @change="onPageChange"
        @show-size-change="onPageSizeChange"
      />
    </div>

    <a-modal
      v-model:open="modalOpen"
      :title="editing ? $t('catalog.editTitle') : $t('catalog.createTitle')"
      :confirm-loading="saving"
      :mask-closable="false"
      width="680px"
      @ok="save"
    >
      <a-form :label-col="{ span: 5 }" :wrapper-col="{ span: 18 }">
        <a-form-item :label="$t('catalog.nameLabel')" required>
          <a-input v-model:value="form.title" :maxlength="255" :placeholder="$t('catalog.nameLabel')" />
        </a-form-item>
        <a-form-item :label="$t('catalog.codeLabel')">
          <a-input v-model:value="form.productCode" :disabled="!!editing" :maxlength="64" :placeholder="$t('catalog.codePlaceholder')" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="$t('catalog.ipLabel')" :label-col="{ span: 10 }" :wrapper-col="{ span: 13 }">
              <a-select v-model:value="form.ipId" :options="ipOptions" allow-clear show-search option-filter-prop="label" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="$t('catalog.saleTypeLabel')" :label-col="{ span: 10 }" :wrapper-col="{ span: 13 }">
              <a-select v-model:value="form.saleType" :options="saleTypeOptions" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="$t('catalog.brandLabel')" :label-col="{ span: 10 }" :wrapper-col="{ span: 13 }">
              <a-select v-model:value="form.brandId" :options="brandOptions" allow-clear show-search option-filter-prop="label" @change="onFormBrandChange" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="$t('catalog.seriesLabel')" :label-col="{ span: 10 }" :wrapper-col="{ span: 13 }">
              <a-select v-model:value="form.seriesId" :options="modalSeriesOptions" allow-clear show-search option-filter-prop="label" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="$t('catalog.costLabel')" :label-col="{ span: 10 }" :wrapper-col="{ span: 13 }">
              <a-input-number v-model:value="form.costYuan" :min="0" :precision="2" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="$t('catalog.priceLabel')" :label-col="{ span: 10 }" :wrapper-col="{ span: 13 }">
              <a-input-number v-model:value="form.saleYuan" :min="0" :precision="2" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item :label="$t('catalog.coverLabel')">
          <a-input v-model:value="form.coverUrl" :maxlength="512" placeholder="/covers/popmart-1.svg" />
        </a-form-item>
        <a-form-item v-if="form.coverUrl.trim()" label=" " :colon="false">
          <a-image :src="form.coverUrl.trim()" :width="72" :height="72" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'
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

interface TableSorter {
  columnKey?: string
  order?: 'ascend' | 'descend' | null
}

const session = useSessionStore()
const router = useRouter()
const { t } = useI18n()
const canCreate = computed(() => allows(session.permissionNodes, 'catalog.product', 'create'))
const canWrite = computed(() => allows(session.permissionNodes, 'catalog.product', 'write'))

const filters = reactive({
  keyword: '',
  brandId: undefined as string | undefined,
  seriesId: undefined as string | undefined,
  ipId: undefined as string | undefined,
  saleType: undefined as string | undefined,
  minPrice: undefined as number | undefined,
  maxPrice: undefined as number | undefined,
})
const items = ref<AdminProductItem[]>([])
const ips = ref<IpItem[]>([])
const brands = ref<BrandItem[]>([])
const series = ref<SeriesItem[]>([])
const page = ref(1)
const pageSize = ref(15)
const total = ref(0)
const loading = ref(false)
const copyingId = ref<string | undefined>(undefined)
const sortBy = ref('')
const sortOrder = ref('')
const pageSizeOptions = ['15', '30', '50']
const paginationLocale = computed(() => ({ items_per_page: t('catalog.itemsPerPage') }))

const ipOptions = computed(() =>
  ips.value.filter((item) => item.ipId && item.name).map((item) => ({ label: item.name as string, value: item.ipId as string })),
)
const brandOptions = computed(() =>
  brands.value.filter((item) => item.brandId && item.name).map((item) => ({ label: item.name as string, value: item.brandId as string })),
)
const allSeriesOptions = computed(() =>
  series.value
    .filter((item) => item.seriesId && item.name)
    .map((item) => ({ label: item.name as string, value: item.seriesId as string, brandId: item.brandId ?? '' })),
)
const filteredSeriesOptions = computed(() =>
  filters.brandId ? allSeriesOptions.value.filter((item) => item.brandId === filters.brandId) : allSeriesOptions.value,
)
const saleTypeOptions = computed(() => [
  { label: t('catalog.saleTypeInStock'), value: 'IN_STOCK' },
  { label: t('catalog.saleTypePreorder'), value: 'PREORDER' },
])

const selectedRowKeys = ref<string[]>([])
const rowSelection = computed(() => ({
  selectedRowKeys: selectedRowKeys.value,
  onChange: (keys: (string | number)[]) => {
    selectedRowKeys.value = keys.map((key) => String(key))
  },
}))

const modalOpen = ref(false)
const saving = ref(false)
const editing = ref<AdminProductItem | null>(null)
const form = reactive({
  productCode: '',
  title: '',
  ipId: undefined as string | undefined,
  brandId: undefined as string | undefined,
  seriesId: undefined as string | undefined,
  saleType: 'IN_STOCK',
  costYuan: 0,
  saleYuan: 0,
  coverUrl: '',
})
const modalSeriesOptions = computed(() =>
  form.brandId ? allSeriesOptions.value.filter((item) => item.brandId === form.brandId) : allSeriesOptions.value,
)

async function loadOptions() {
  const [ipReply, brandReply, seriesReply] = await Promise.all([
    catalogIpClient.List({ keyword: '', page: 1, pageSize: 100 }).catch(() => ({ ips: [] })),
    catalogBrandClient.List({ keyword: '', page: 1, pageSize: 100 }).catch(() => ({ brands: [] })),
    catalogSeriesClient.List({ keyword: '', brandId: '', page: 1, pageSize: 100 }).catch(() => ({ series: [] })),
  ])
  ips.value = ipReply.ips ?? []
  brands.value = brandReply.brands ?? []
  series.value = seriesReply.series ?? []
}

async function load() {
  loading.value = true
  try {
    const reply = await catalogAdminProductClient.List({
      keyword: filters.keyword.trim(),
      brandId: filters.brandId ?? '',
      seriesId: filters.seriesId ?? '',
      ipId: filters.ipId ?? '',
      saleType: filters.saleType ?? '',
      minPriceMinor: filters.minPrice ? Math.round(filters.minPrice * 100) : 0,
      maxPriceMinor: filters.maxPrice ? Math.round(filters.maxPrice * 100) : 0,
      sortBy: sortBy.value,
      sortOrder: sortOrder.value,
      page: page.value,
      pageSize: pageSize.value,
    })
    items.value = reply.products ?? []
    total.value = Number(reply.total ?? 0)
    page.value = Number(reply.page ?? page.value)
    pageSize.value = Number(reply.pageSize ?? pageSize.value)
    selectedRowKeys.value = []
  } catch {
    items.value = []
    total.value = 0
    selectedRowKeys.value = []
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  void load()
}

function reset() {
  filters.keyword = ''
  filters.brandId = undefined
  filters.seriesId = undefined
  filters.ipId = undefined
  filters.saleType = undefined
  filters.minPrice = undefined
  filters.maxPrice = undefined
  sortBy.value = ''
  sortOrder.value = ''
  page.value = 1
  void load()
}

function onFilterBrandChange() {
  if (filters.seriesId && !filteredSeriesOptions.value.some((item) => item.value === filters.seriesId)) {
    filters.seriesId = undefined
  }
}

function onPageChange(nextPage: number) {
  page.value = nextPage
  void load()
}

function onPageSizeChange(_current: number, nextPageSize: number) {
  pageSize.value = nextPageSize
  page.value = 1
  void load()
}

function onTableChange(_pagination: unknown, _filters: unknown, sorter: TableSorter | TableSorter[]) {
  const active = Array.isArray(sorter) ? sorter[0] : sorter
  if (active?.order && active.columnKey) {
    sortBy.value = active.columnKey
    sortOrder.value = active.order === 'ascend' ? 'asc' : 'desc'
  } else {
    sortBy.value = ''
    sortOrder.value = ''
  }
  page.value = 1
  void load()
}

function saleTypeText(value?: string): string {
  return value === 'PREORDER' ? t('catalog.saleTypePreorder') : t('catalog.saleTypeInStock')
}

function formatMoney(value?: string): string {
  return String(Number(value ?? 0) / 100)
}

function openCreate() {
  editing.value = null
  form.productCode = ''
  form.title = ''
  form.ipId = undefined
  form.brandId = undefined
  form.seriesId = undefined
  form.saleType = 'IN_STOCK'
  form.costYuan = 0
  form.saleYuan = 0
  form.coverUrl = ''
  modalOpen.value = true
}

function openEdit(record: AdminProductItem) {
  editing.value = record
  form.productCode = record.productCode ?? ''
  form.title = record.title ?? ''
  form.ipId = record.ipId || undefined
  form.brandId = record.brandId || undefined
  form.seriesId = record.seriesId || undefined
  form.saleType = record.saleType || 'IN_STOCK'
  form.costYuan = Number(record.costAmountMinor ?? 0) / 100
  form.saleYuan = Number(record.saleAmountMinor ?? 0) / 100
  form.coverUrl = record.coverUrl ?? ''
  modalOpen.value = true
}

function onFormBrandChange() {
  if (form.seriesId && !modalSeriesOptions.value.some((item) => item.value === form.seriesId)) {
    form.seriesId = undefined
  }
}

async function save() {
  const title = form.title.trim()
  if (!title) {
    message.warning(t('catalog.nameRequired'))
    return
  }
  if (typeof form.costYuan !== 'number' || !Number.isFinite(form.costYuan) || form.costYuan < 0) {
    message.warning(t('catalog.costPriceInvalid'))
    return
  }
  if (typeof form.saleYuan !== 'number' || !Number.isFinite(form.saleYuan) || form.saleYuan < 0) {
    message.warning(t('catalog.salePriceInvalid'))
    return
  }
  const costAmountMinor = Math.round(form.costYuan * 100)
  const saleAmountMinor = Math.round(form.saleYuan * 100)
  saving.value = true
  try {
    if (editing.value?.productId) {
      await catalogAdminProductClient.Update(editing.value.productId, {
        title,
        ipId: form.ipId ?? '',
        brandId: form.brandId ?? '',
        seriesId: form.seriesId ?? '',
        saleType: form.saleType,
        costAmountMinor,
        saleAmountMinor,
        coverUrl: form.coverUrl.trim(),
      })
      message.success(t('catalog.productUpdated'))
    } else {
      await catalogAdminProductClient.Create({
        productCode: form.productCode.trim(),
        title,
        ipId: form.ipId ?? '',
        brandId: form.brandId ?? '',
        seriesId: form.seriesId ?? '',
        saleType: form.saleType,
        costAmountMinor,
        saleAmountMinor,
        coverUrl: form.coverUrl.trim(),
      })
      message.success(t('catalog.productCreated'))
      page.value = 1
    }
    modalOpen.value = false
    await load()
  } catch {
    // 请求拦截器已展示服务端错误；保留弹窗便于修改后重试。
  } finally {
    saving.value = false
  }
}

async function copy(record: AdminProductItem) {
  if (!record.productId) return
  copyingId.value = record.productId
  try {
    await catalogAdminProductClient.Copy(record.productId)
    message.success(t('catalog.productCopied'))
    page.value = 1
    await load()
  } catch {
    // 请求拦截器已展示失败信息。
  } finally {
    copyingId.value = undefined
  }
}

function detail(record: AdminProductItem) {
  if (record.productId) router.push(`/catalog/products/${record.productId}`)
}

onMounted(async () => {
  await loadOptions()
  await load()
})
</script>

<style scoped lang="scss">
.product-page {
  min-height: calc(100vh - 132px);
  padding: 14px 16px 18px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}

.product-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
}

.keyword-input {
  width: 200px;
}

.filter-select {
  width: 150px;
}

.price-input {
  width: 100px;
}

.price-sep {
  color: #909399;
}

.btn-query {
  background: #42b983 !important;
  border-color: #42b983 !important;
}

.btn-query:hover {
  background: #36a374 !important;
  border-color: #36a374 !important;
}

.btn-reset,
.btn-create {
  color: #606266;
  border-color: #dcdfe6;
  background: #fff;
}

.btn-reset:hover,
.btn-create:hover {
  color: #409eff;
  border-color: #409eff;
}

.brand-series {
  display: flex;
  flex-direction: column;
  line-height: 1.5;
  color: #606266;
}

.btn-copy {
  color: #606266;
  border-color: #dcdfe6;
  background: #fff;
}

.btn-edit {
  min-width: 54px;
  background: #ff9f1a !important;
  border-color: #ff9f1a !important;
  color: #fff !important;
}

.btn-edit:hover {
  background: #f2940f !important;
  border-color: #f2940f !important;
}

.btn-detail {
  min-width: 54px;
  background: #909399 !important;
  border-color: #909399 !important;
  color: #fff !important;
}

.btn-detail:hover {
  background: #7f8286 !important;
  border-color: #7f8286 !important;
}

.product-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>

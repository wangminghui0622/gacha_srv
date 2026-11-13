<template>
  <div class="series-page">
    <div class="series-toolbar">
      <a-input
        v-model:value="keyword"
        class="series-search"
        :placeholder="$t('catalogSeries.searchPlaceholder')"
        allow-clear
        @press-enter="search"
      />
      <a-select
        v-model:value="brandId"
        class="brand-filter"
        :placeholder="$t('catalogSeries.brandFilterPlaceholder')"
        :options="brandOptions"
        allow-clear
        show-search
        option-filter-prop="label"
      />
      <a-button class="btn-query" type="primary" :loading="loading" @click="search">{{ $t('catalogSeries.query') }}</a-button>
      <a-button v-if="canCreate" class="btn-create" @click="openCreate">{{ $t('catalogSeries.create') }}</a-button>
    </div>

    <a-table
      :data-source="items"
      :loading="loading"
      :pagination="false"
      :row-key="(record: SeriesItem) => record.seriesId ?? ''"
      :row-selection="rowSelection"
      :scroll="{ x: 900 }"
      size="middle"
    >
      <a-table-column :title="$t('common.id')" data-index="seriesId" width="90" align="center" />
      <a-table-column :title="$t('catalogSeries.name')" data-index="name" align="center" />
      <a-table-column :title="$t('catalogSeries.brand')" data-index="brandName" align="center" />
      <a-table-column :title="$t('catalogSeries.sortWeight')" data-index="sortWeight" width="140" align="center" />
      <a-table-column :title="$t('common.action')" width="120" align="center">
        <template #default="{ record }">
          <a-button v-if="canWrite" class="btn-edit" size="small" @click="openEdit(record)">{{ $t('catalogSeries.edit') }}</a-button>
        </template>
      </a-table-column>
    </a-table>

    <div class="series-footer">
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
      :title="editing ? $t('catalogSeries.editTitle') : $t('catalogSeries.createTitle')"
      :confirm-loading="saving"
      :mask-closable="false"
      width="520px"
      @ok="save"
    >
      <a-form :label-col="{ span: 6 }" :wrapper-col="{ span: 17 }">
        <a-form-item :label="$t('catalogSeries.nameLabel')" required>
          <a-input v-model:value="form.name" :maxlength="128" :placeholder="$t('catalogSeries.namePlaceholder')" />
        </a-form-item>
        <a-form-item :label="$t('catalogSeries.brandLabel')" required>
          <a-select
            v-model:value="form.brandId"
            :options="brandOptions"
            :placeholder="$t('catalogSeries.brandPlaceholder')"
            show-search
            option-filter-prop="label"
          />
        </a-form-item>
        <a-form-item :label="$t('catalogSeries.sortWeight')" required>
          <a-input-number v-model:value="form.sortWeight" :min="0" :max="1000000" :precision="0" style="width: 100%" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'
import { catalogBrandClient, catalogSeriesClient, type BrandItem, type SeriesItem } from '@/api/catalog'

const session = useSessionStore()
const { t } = useI18n()
const canCreate = computed(() => allows(session.permissionNodes, 'catalog.product', 'create'))
const canWrite = computed(() => allows(session.permissionNodes, 'catalog.product', 'write'))

const keyword = ref('')
const brandId = ref<string | undefined>(undefined)
const brands = ref<BrandItem[]>([])
const items = ref<SeriesItem[]>([])
const page = ref(1)
const pageSize = ref(15)
const total = ref(0)
const loading = ref(false)
const pageSizeOptions = ['15', '30', '50']
const paginationLocale = computed(() => ({ items_per_page: t('catalogSeries.itemsPerPage') }))
const brandOptions = computed(() =>
  brands.value
    .filter((brand) => brand.brandId && brand.name)
    .map((brand) => ({ label: brand.name as string, value: brand.brandId as string })),
)

const selectedRowKeys = ref<string[]>([])
const rowSelection = computed(() => ({
  selectedRowKeys: selectedRowKeys.value,
  onChange: (keys: (string | number)[]) => {
    selectedRowKeys.value = keys.map((key) => String(key))
  },
}))

const modalOpen = ref(false)
const saving = ref(false)
const editing = ref<SeriesItem | null>(null)
const form = reactive({
  name: '',
  brandId: undefined as string | undefined,
  sortWeight: 0,
})

async function loadBrands() {
  try {
    const reply = await catalogBrandClient.List({ keyword: '', page: 1, pageSize: 100 })
    brands.value = reply.brands ?? []
  } catch {
    brands.value = []
  }
}

async function load() {
  loading.value = true
  try {
    const reply = await catalogSeriesClient.List({
      keyword: keyword.value.trim(),
      brandId: brandId.value ?? '',
      page: page.value,
      pageSize: pageSize.value,
    })
    items.value = reply.series ?? []
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

function onPageChange(nextPage: number) {
  page.value = nextPage
  void load()
}

function onPageSizeChange(_current: number, nextPageSize: number) {
  pageSize.value = nextPageSize
  page.value = 1
  void load()
}

function openCreate() {
  editing.value = null
  form.name = ''
  form.brandId = undefined
  form.sortWeight = 0
  modalOpen.value = true
}

function openEdit(record: SeriesItem) {
  editing.value = record
  form.name = record.name ?? ''
  form.brandId = record.brandId
  form.sortWeight = record.sortWeight ?? 0
  modalOpen.value = true
}

async function save() {
  const name = form.name.trim()
  if (!name) {
    message.warning(t('catalogSeries.nameRequired'))
    return
  }
  if (!form.brandId) {
    message.warning(t('catalogSeries.brandRequired'))
    return
  }
  if (typeof form.sortWeight !== 'number' || !Number.isFinite(form.sortWeight) || form.sortWeight < 0) {
    message.warning(t('catalogSeries.sortWeightInvalid'))
    return
  }
  saving.value = true
  try {
    if (editing.value?.seriesId) {
      await catalogSeriesClient.Update(editing.value.seriesId, {
        name,
        brandId: form.brandId,
        sortWeight: form.sortWeight,
      })
      message.success(t('catalogSeries.updated'))
    } else {
      await catalogSeriesClient.Create({
        name,
        brandId: form.brandId,
        sortWeight: form.sortWeight,
      })
      message.success(t('catalogSeries.created'))
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

onMounted(async () => {
  await loadBrands()
  await load()
})
</script>

<style scoped lang="scss">
.series-page {
  min-height: calc(100vh - 132px);
  padding: 14px 16px 18px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}

.series-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
}

.series-search {
  width: 180px;
}

.brand-filter {
  width: 180px;
}

.btn-query {
  background: #42b983 !important;
  border-color: #42b983 !important;
}

.btn-query:hover {
  background: #36a374 !important;
  border-color: #36a374 !important;
}

.btn-create {
  color: #606266;
  border-color: #dcdfe6;
  background: #fff;
}

.btn-create:hover {
  color: #409eff;
  border-color: #409eff;
}

.btn-edit {
  min-width: 58px;
  background: #ff9f1a !important;
  border-color: #ff9f1a !important;
  color: #fff !important;
}

.btn-edit:hover {
  background: #f2940f !important;
  border-color: #f2940f !important;
}

.series-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>

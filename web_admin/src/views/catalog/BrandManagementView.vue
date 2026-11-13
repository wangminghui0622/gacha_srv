<template>
  <div class="brand-page">
    <div class="brand-toolbar">
      <a-input
        v-model:value="keyword"
        class="brand-search"
        :placeholder="$t('catalogBrand.searchPlaceholder')"
        allow-clear
        @press-enter="search"
      />
      <a-button class="btn-query" type="primary" :loading="loading" @click="search">{{ $t('catalogBrand.query') }}</a-button>
      <a-button v-if="canCreate" class="btn-create" @click="openCreate">{{ $t('catalogBrand.create') }}</a-button>
    </div>

    <a-table
      :data-source="items"
      :loading="loading"
      :pagination="false"
      :row-key="(record: BrandItem) => record.brandId ?? ''"
      :row-selection="rowSelection"
      :scroll="{ x: 860 }"
      size="middle"
    >
      <a-table-column :title="$t('common.id')" data-index="brandId" width="90" align="center" />
      <a-table-column :title="$t('catalogBrand.name')" data-index="name" align="center" />
      <a-table-column :title="$t('catalogBrand.logo')" width="180" align="center">
        <template #default="{ record }">
          <a-image v-if="record.logo" :src="record.logo" :width="40" :height="40" :preview="false" />
        </template>
      </a-table-column>
      <a-table-column :title="$t('common.action')" width="120" align="center">
        <template #default="{ record }">
          <a-button v-if="canWrite" class="btn-edit" size="small" @click="openEdit(record)">{{ $t('catalogBrand.edit') }}</a-button>
        </template>
      </a-table-column>
    </a-table>

    <div class="brand-footer">
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
      :title="editing ? $t('catalogBrand.editTitle') : $t('catalogBrand.createTitle')"
      :confirm-loading="saving"
      :mask-closable="false"
      width="520px"
      @ok="save"
    >
      <a-form :label-col="{ span: 6 }" :wrapper-col="{ span: 17 }">
        <a-form-item :label="$t('catalogBrand.nameLabel')" required>
          <a-input v-model:value="form.name" :maxlength="128" :placeholder="$t('catalogBrand.namePlaceholder')" />
        </a-form-item>
        <a-form-item :label="$t('catalogBrand.logoLabel')">
          <a-input v-model:value="form.logo" :maxlength="512" :placeholder="$t('catalogBrand.logoPlaceholder')" />
        </a-form-item>
        <a-form-item v-if="form.logo.trim()" label=" " :colon="false">
          <a-image :src="form.logo.trim()" :width="72" :height="72" />
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
import { catalogBrandClient, type BrandItem } from '@/api/catalog'

const session = useSessionStore()
const { t } = useI18n()
const canCreate = computed(() => allows(session.permissionNodes, 'catalog.product', 'create'))
const canWrite = computed(() => allows(session.permissionNodes, 'catalog.product', 'write'))

const keyword = ref('')
const items = ref<BrandItem[]>([])
const page = ref(1)
const pageSize = ref(15)
const total = ref(0)
const loading = ref(false)
const pageSizeOptions = ['15', '30', '50']
const paginationLocale = computed(() => ({ items_per_page: t('catalogBrand.itemsPerPage') }))

const selectedRowKeys = ref<string[]>([])
const rowSelection = computed(() => ({
  selectedRowKeys: selectedRowKeys.value,
  onChange: (keys: (string | number)[]) => {
    selectedRowKeys.value = keys.map((key) => String(key))
  },
}))

const modalOpen = ref(false)
const saving = ref(false)
const editing = ref<BrandItem | null>(null)
const form = reactive({
  name: '',
  logo: '',
})

async function load() {
  loading.value = true
  try {
    const reply = await catalogBrandClient.List({
      keyword: keyword.value.trim(),
      page: page.value,
      pageSize: pageSize.value,
    })
    items.value = reply.brands ?? []
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
  form.logo = ''
  modalOpen.value = true
}

function openEdit(record: BrandItem) {
  editing.value = record
  form.name = record.name ?? ''
  form.logo = record.logo ?? ''
  modalOpen.value = true
}

async function save() {
  const name = form.name.trim()
  const logo = form.logo.trim()
  if (!name) {
    message.warning(t('catalogBrand.nameRequired'))
    return
  }
  if (logo.length > 512) {
    message.warning(t('catalogBrand.logoTooLong'))
    return
  }
  saving.value = true
  try {
    if (editing.value?.brandId) {
      await catalogBrandClient.Update(editing.value.brandId, { name, logo })
      message.success(t('catalogBrand.updated'))
    } else {
      await catalogBrandClient.Create({ name, logo })
      message.success(t('catalogBrand.created'))
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

onMounted(load)
</script>

<style scoped lang="scss">
.brand-page {
  min-height: calc(100vh - 132px);
  padding: 14px 16px 18px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}

.brand-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
}

.brand-search {
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

.brand-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>

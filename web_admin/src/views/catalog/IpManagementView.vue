<template>
  <div class="ip-page">
    <div class="ip-toolbar">
      <a-input
        v-model:value="keyword"
        class="ip-search"
        :placeholder="$t('catalogIp.searchPlaceholder')"
        allow-clear
        @press-enter="search"
      />
      <a-button class="btn-query" type="primary" :loading="loading" @click="search">{{ $t('catalogIp.query') }}</a-button>
      <a-button v-if="canCreate" class="btn-create" @click="openCreate">{{ $t('catalogIp.create') }}</a-button>
    </div>

    <a-table
      :data-source="items"
      :loading="loading"
      :pagination="false"
      :row-key="(record: IpItem) => record.ipId ?? ''"
      :row-selection="rowSelection"
      :scroll="{ x: 860 }"
      size="middle"
    >
      <a-table-column :title="$t('common.id')" data-index="ipId" width="90" align="center" />
      <a-table-column :title="$t('catalogIp.name')" data-index="name" align="center" />
      <a-table-column :title="$t('catalogIp.sortWeight')" data-index="sortWeight" width="140" align="center" />
      <a-table-column :title="$t('common.action')" width="120" align="center">
        <template #default="{ record }">
          <a-button v-if="canWrite" class="btn-edit" size="small" @click="openEdit(record)">{{ $t('catalogIp.edit') }}</a-button>
        </template>
      </a-table-column>
    </a-table>

    <div class="ip-footer">
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
      :title="editing ? $t('catalogIp.editTitle') : $t('catalogIp.createTitle')"
      :confirm-loading="saving"
      :mask-closable="false"
      width="460px"
      @ok="save"
    >
      <a-form :label-col="{ span: 5 }" :wrapper-col="{ span: 18 }">
        <a-form-item :label="$t('catalogIp.nameLabel')" required>
          <a-input v-model:value="form.name" :maxlength="128" :placeholder="$t('catalogIp.namePlaceholder')" />
        </a-form-item>
        <a-form-item :label="$t('catalogIp.sortWeight')" required>
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
import { catalogIpClient, type IpItem } from '@/api/catalog'

const session = useSessionStore()
const { t } = useI18n()
const canCreate = computed(() => allows(session.permissionNodes, 'catalog.product', 'create'))
const canWrite = computed(() => allows(session.permissionNodes, 'catalog.product', 'write'))

const keyword = ref('')
const items = ref<IpItem[]>([])
const page = ref(1)
const pageSize = ref(15)
const total = ref(0)
const loading = ref(false)
const pageSizeOptions = ['15', '30', '50']
const paginationLocale = computed(() => ({ items_per_page: t('catalogIp.itemsPerPage') }))

const selectedRowKeys = ref<string[]>([])
const rowSelection = computed(() => ({
  selectedRowKeys: selectedRowKeys.value,
  onChange: (keys: (string | number)[]) => {
    selectedRowKeys.value = keys.map((key) => String(key))
  },
}))

const modalOpen = ref(false)
const saving = ref(false)
const editing = ref<IpItem | null>(null)
const form = reactive({
  name: '',
  sortWeight: 0,
})

async function load() {
  loading.value = true
  try {
    const reply = await catalogIpClient.List({
      keyword: keyword.value.trim(),
      page: page.value,
      pageSize: pageSize.value,
    })
    items.value = reply.ips ?? []
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
  form.sortWeight = 0
  modalOpen.value = true
}

function openEdit(record: IpItem) {
  editing.value = record
  form.name = record.name ?? ''
  form.sortWeight = record.sortWeight ?? 0
  modalOpen.value = true
}

async function save() {
  const name = form.name.trim()
  if (!name) {
    message.warning(t('catalogIp.nameRequired'))
    return
  }
  if (typeof form.sortWeight !== 'number' || !Number.isFinite(form.sortWeight) || form.sortWeight < 0) {
    message.warning(t('catalogIp.sortWeightInvalid'))
    return
  }
  saving.value = true
  try {
    if (editing.value?.ipId) {
      await catalogIpClient.Update(editing.value.ipId, { name, sortWeight: form.sortWeight })
      message.success(t('catalogIp.updated'))
    } else {
      await catalogIpClient.Create({ name, sortWeight: form.sortWeight })
      message.success(t('catalogIp.created'))
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
.ip-page {
  min-height: calc(100vh - 132px);
  padding: 14px 16px 18px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}

.ip-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
}

.ip-search {
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

.ip-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>

<template>
  <div class="pool-page">
    <div class="pool-toolbar">
      <a-input v-model:value="filters.keyword" class="filter-keyword" :placeholder="$t('poolAdmin.keywordPlaceholder')" allow-clear @press-enter="search" />
      <a-select v-model:value="filters.mode" class="filter-select" :placeholder="$t('poolAdmin.modePlaceholder')" :options="modeOptions" allow-clear />
      <a-select v-model:value="filters.ipId" class="filter-select" :placeholder="$t('poolAdmin.ipPlaceholder')" :options="ipOptions" allow-clear show-search option-filter-prop="label" />
      <a-select v-model:value="filters.status" class="filter-select" :placeholder="$t('poolAdmin.statusPlaceholder')" :options="statusOptions" allow-clear />
      <a-input v-model:value="filters.tag" class="filter-tag" :placeholder="$t('poolAdmin.tagPlaceholder')" allow-clear @press-enter="search" />
      <a-select v-model:value="filters.sortOrder" class="filter-sort" :options="sortOptions" />
      <a-button class="btn-query" type="primary" :loading="loading" @click="search">{{ $t('poolAdmin.query') }}</a-button>
      <a-button class="btn-plain" @click="openReference">{{ $t('poolAdmin.referenceQuery') }}</a-button>
      <a-button v-if="canCreate" class="btn-plain" @click="openCreate">{{ $t('poolAdmin.create') }}</a-button>
      <a-button v-if="canWrite" class="btn-plain" @click="saveWeights">{{ $t('poolAdmin.saveSort') }}</a-button>
      <a-button v-if="canWrite" class="btn-plain" @click="deleteSelected">{{ $t('poolAdmin.deletePools') }}</a-button>
    </div>

    <a-table
      :data-source="items"
      :loading="loading"
      :pagination="false"
      :row-key="(record: AdminPlayPool) => record.poolId ?? ''"
      :row-selection="rowSelection"
      size="middle"
      :scroll="{ x: 1700 }"
    >
      <a-table-column :title="$t('poolAdmin.id')" data-index="poolId" width="190" align="center" />
      <a-table-column :title="$t('poolAdmin.name')" data-index="name" min-width="300" align="center" />
      <a-table-column :title="$t('poolAdmin.price')" width="110" align="center">
        <template #default="{ record }">{{ formatPrice(record.priceFen) }}</template>
      </a-table-column>
      <a-table-column :title="$t('poolAdmin.startAt')" width="180" align="center">
        <template #default="{ record }">{{ formatTime(record.startAt) }}</template>
      </a-table-column>
      <a-table-column :title="$t('poolAdmin.status')" width="100" align="center">
        <template #default="{ record }">{{ statusText(record.status) }}</template>
      </a-table-column>
      <a-table-column :title="$t('poolAdmin.gameType')" width="130" align="center">
        <template #default="{ record }">{{ modeText(record.mode) }}</template>
      </a-table-column>
      <a-table-column :title="$t('poolAdmin.tags')" width="320" align="center">
        <template #default="{ record }">
          <a-space :size="4" wrap>
            <a-tag v-for="tag in record.tags ?? []" :key="tag" class="pool-tag">{{ tag }}</a-tag>
          </a-space>
        </template>
      </a-table-column>
      <a-table-column :title="$t('poolAdmin.periods')" data-index="periods" width="90" align="center" />
      <a-table-column :title="$t('poolAdmin.weight')" width="120" align="center">
        <template #default="{ record }">
          <a-input-number
            :value="weightOf(record)"
            :controls="false"
            :min="0"
            :precision="0"
            class="weight-input"
            @change="(value: number | null) => setWeight(record, value)"
          />
        </template>
      </a-table-column>
      <a-table-column :title="$t('poolAdmin.action')" width="240" align="center" fixed="right">
        <template #default="{ record }">
          <a-space :size="4">
            <a-button v-if="canCreate" class="btn-copy" size="small" :loading="copyingId === record.poolId" @click="copyPool(record)">{{ $t('poolAdmin.copy') }}</a-button>
            <a-button v-if="canWrite" class="btn-edit" size="small" @click="openEdit(record)">{{ $t('poolAdmin.edit') }}</a-button>
            <a-button v-if="record.mode === 'memorymatch'" class="btn-match" size="small" @click="openMatch(record)">{{ $t('poolAdmin.pairMatch') }}</a-button>
          </a-space>
        </template>
      </a-table-column>
    </a-table>

    <div class="pool-footer">
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
      v-model:open="createModalOpen"
      :title="$t('poolAdmin.createTitle')"
      centered
      width="1180px"
      :footer="null"
      :destroy-on-close="true"
      :body-style="{ maxHeight: 'calc(100vh - 150px)', overflowY: 'auto' }"
    >
      <PlayPoolCreateForm @success="onCreateSuccess" />
    </a-modal>

    <a-modal v-model:open="createOpen" :title="$t('poolAdmin.createTitle')" :confirm-loading="creating" width="760px" @ok="submitCreate">
      <a-form :label-col="{ span: 5 }" :wrapper-col="{ span: 18 }">
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="$t('poolAdmin.modeLabel')" required>
              <a-select v-model:value="createForm.mode" :options="modeOptions" @change="resetSkeleton" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="$t('poolAdmin.nameLabel')" required>
              <a-input v-model:value="createForm.name" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="$t('poolAdmin.priceLabel')">
              <a-input-number v-model:value="createForm.priceYuan" :min="0" :precision="2" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="$t('poolAdmin.ipLabel')">
              <a-select v-model:value="createForm.ipId" :options="ipOptions" allow-clear show-search option-filter-prop="label" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="$t('poolAdmin.tagsLabel')">
              <a-select v-model:value="createForm.tags" mode="tags" :token-separators="[',']" />
            </a-form-item>
          </a-col>
          <a-col :span="6">
            <a-form-item :label="$t('poolAdmin.periodsLabel')" :label-col="{ span: 10 }" :wrapper-col="{ span: 13 }">
              <a-input-number v-model:value="createForm.periods" :min="0" :precision="0" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="6">
            <a-form-item :label="$t('poolAdmin.weightLabel')" :label-col="{ span: 10 }" :wrapper-col="{ span: 13 }">
              <a-input-number v-model:value="createForm.weight" :min="0" :precision="0" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item :label="$t('poolAdmin.configJson')">
          <a-textarea v-model:value="createForm.configJson" :rows="4" spellcheck="false" />
        </a-form-item>
        <a-form-item :label="$t('poolAdmin.prizesJson')">
          <a-textarea v-model:value="createForm.prizesJson" :rows="9" spellcheck="false" />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:open="editOpen" :title="$t('poolAdmin.editTitle')" :confirm-loading="editingSaving" width="620px" @ok="submitEdit">
      <a-form :label-col="{ span: 6 }" :wrapper-col="{ span: 17 }">
        <a-form-item :label="$t('poolAdmin.nameLabel')" required>
          <a-input v-model:value="editForm.name" />
        </a-form-item>
        <a-form-item :label="$t('poolAdmin.priceLabel')">
          <a-input-number v-model:value="editForm.priceYuan" :min="0" :precision="2" style="width: 100%" />
        </a-form-item>
        <a-form-item :label="$t('poolAdmin.ipLabel')">
          <a-select v-model:value="editForm.ipId" :options="ipOptions" allow-clear show-search option-filter-prop="label" />
        </a-form-item>
        <a-form-item :label="$t('poolAdmin.tagsLabel')">
          <a-select v-model:value="editForm.tags" mode="tags" :token-separators="[',']" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="$t('poolAdmin.periodsLabel')" :label-col="{ span: 10 }" :wrapper-col="{ span: 13 }">
              <a-input-number v-model:value="editForm.periods" :min="0" :precision="0" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="$t('poolAdmin.weightLabel')" :label-col="{ span: 10 }" :wrapper-col="{ span: 13 }">
              <a-input-number v-model:value="editForm.weight" :min="0" :precision="0" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>
      </a-form>
    </a-modal>

    <a-modal v-model:open="referenceOpen" :title="$t('poolAdmin.referenceTitle')" :footer="null" width="560px">
      <a-space class="reference-search">
        <a-input v-model:value="referencePoolId" :placeholder="$t('poolAdmin.referencePlaceholder')" allow-clear @press-enter="queryReference" />
        <a-button type="primary" :loading="referenceLoading" @click="queryReference">{{ $t('poolAdmin.query') }}</a-button>
      </a-space>
      <a-empty v-if="!referenceLoading && referenceResults.length === 0" :description="$t('poolAdmin.referenceEmpty')" />
      <a-list v-else :data-source="referenceResults" size="small">
        <template #renderItem="{ item }">
          <a-list-item>{{ item.poolId }} · {{ item.name }}</a-list-item>
        </template>
      </a-list>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message, Modal } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'
import { catalogIpClient, type IpItem } from '@/api/catalog'
import { playPoolClient, type AdminPlayPool, type PlayPoolPrize, type PlayPoolVersion } from '@/api/playPool'
import PlayPoolCreateForm from '@/views/gacha/PlayPoolCreateForm.vue'

const session = useSessionStore()
const router = useRouter()
const { t } = useI18n()
const canCreate = computed(() => allows(session.permissionNodes, 'gacha.pool', 'create'))
const canWrite = computed(() => allows(session.permissionNodes, 'gacha.pool', 'write'))

const filters = reactive({
  keyword: '',
  mode: undefined as string | undefined,
  ipId: undefined as string | undefined,
  status: undefined as string | undefined,
  tag: '',
  sortOrder: 'desc',
})
const items = ref<AdminPlayPool[]>([])
const ips = ref<IpItem[]>([])
const page = ref(1)
const pageSize = ref(15)
const total = ref(0)
const loading = ref(false)
const copyingId = ref<string | undefined>(undefined)
const pageSizeOptions = ['15', '30', '50']
const paginationLocale = computed(() => ({ items_per_page: t('poolAdmin.itemsPerPage') }))
const weights = reactive<Record<string, number>>({})

const modeOptions = computed(() => [
  { label: t('poolAdmin.modeGashapon'), value: 'gashapon' },
  { label: t('poolAdmin.modeFullcar'), value: 'fullcar' },
  { label: t('poolAdmin.modeCombochain'), value: 'combochain' },
  { label: t('poolAdmin.modeDig'), value: 'dig' },
  { label: t('poolAdmin.modeGroupbuy'), value: 'groupbuy' },
  { label: t('poolAdmin.modeMemorymatch'), value: 'memorymatch' },
])
const statusOptions = computed(() => [
  { label: t('poolAdmin.statusDraft'), value: 'draft' },
  { label: t('poolAdmin.statusPublished'), value: 'published' },
  { label: t('poolAdmin.statusRetired'), value: 'retired' },
])
const sortOptions = computed(() => [
  { label: t('poolAdmin.sortDesc'), value: 'desc' },
  { label: t('poolAdmin.sortAsc'), value: 'asc' },
])
const ipOptions = computed(() => ips.value.filter((item) => item.ipId && item.name).map((item) => ({ label: item.name as string, value: item.ipId as string })))

const selectedRowKeys = ref<string[]>([])
const rowSelection = computed(() => ({
  selectedRowKeys: selectedRowKeys.value,
  onChange: (keys: (string | number)[]) => {
    selectedRowKeys.value = keys.map((key) => String(key))
  },
}))

const createModalOpen = ref(false)
const createOpen = ref(false)
const creating = ref(false)
const createForm = reactive({
  mode: 'gashapon',
  name: '',
  priceYuan: 100,
  ipId: undefined as string | undefined,
  tags: [] as string[],
  periods: 0,
  weight: 0,
  configJson: '{}',
  prizesJson: '[]',
})

const editOpen = ref(false)
const editingSaving = ref(false)
const editing = ref<AdminPlayPool | null>(null)
const editForm = reactive({
  name: '',
  priceYuan: 0,
  ipId: undefined as string | undefined,
  tags: [] as string[],
  periods: 0,
  weight: 0,
})

const referenceOpen = ref(false)
const referenceLoading = ref(false)
const referencePoolId = ref('')
const referenceResults = ref<AdminPlayPool[]>([])

const configSkeleton: Record<string, string> = {
  gashapon: '{}',
  fullcar: '{"seatCount":4}',
  combochain: '{"ticketsPerCar":2,"totalCars":10,"tierTriggerProbabilityBps":300}',
  dig: '{"cellCount":9}',
  groupbuy: '{"targetBetCount":3,"perBetAwardLimit":0}',
  memorymatch: '{"roundsPerPurchase":3,"maxPairCount":3,"patternWeights":[1,1,1,1,1,1,1,1,1],"facePresets":[{"presetId":1,"name":"默认","faces":[]}],"backPresets":[{"presetId":1,"name":"默认","backImageUrl":"","faceBackgroundImageUrl":""}]}',
}
const prizeSkeleton: Record<string, unknown[]> = {
  gashapon: [
    { prizeIndex: 0, prizeType: 1, name: '普通赏', weight: 10, quantity: 10, isPrize: true },
    { prizeIndex: 1, prizeType: 1, name: 'LAST赏', specialKind: 'LAST', cashFen: 500, quantity: 1, isPrize: true },
  ],
  fullcar: [
    { prizeIndex: 0, prizeType: 1, name: '谢谢参与', weight: 30, quantity: -1, isPrize: false },
    { prizeIndex: 1, prizeType: 2, name: '实物手办', skuId: 1, weight: 10, quantity: 5, isPrize: true },
    { prizeIndex: 2, prizeType: 1, name: '整车结算', specialKind: 'SETTLEMENT', targetValue: 1, weight: 1, cashFen: 500, quantity: 0 },
  ],
  combochain: [
    { prizeIndex: 0, prizeType: 1, name: '全局奖', specialKind: 'GLOBAL', targetValue: 0, cashFen: 100, quantity: 1, isPrize: true },
    { prizeIndex: 1, prizeType: 1, name: '连击奖', specialKind: 'COMBO_TIER', weight: 1, quantity: 1, cashFen: 100, isPrize: true, extraJson: '{"tierWeight":1}' },
  ],
  dig: [
    { prizeIndex: 0, prizeType: 1, name: '现金奖', targetValue: 0, cashFen: 50, isPrize: true },
    { prizeIndex: 1, prizeType: 1, name: '空奖', targetValue: 1, isPrize: false },
  ],
  groupbuy: [
    { prizeIndex: 0, prizeType: 1, name: '普通奖', weight: 10, quantity: 2, isPrize: true },
    { prizeIndex: 1, prizeType: 1, name: '幸运奖', specialKind: 'LUCKY', cashFen: 500, quantity: 1, isPrize: true },
  ],
  memorymatch: [
    { prizeIndex: 0, prizeType: 1, name: '现金奖', targetValue: 3, weight: 1, cashFen: 500, quantity: 10, isPrize: true },
    { prizeIndex: 1, prizeType: 1, name: '谢谢参与', targetValue: 0, weight: 1, quantity: -1, isPrize: false },
  ],
}

function resetSkeleton() {
  createForm.configJson = configSkeleton[createForm.mode] ?? '{}'
  createForm.prizesJson = JSON.stringify(prizeSkeleton[createForm.mode] ?? [], null, 2)
}

async function loadOptions() {
  try {
    const reply = await catalogIpClient.List({ keyword: '', page: 1, pageSize: 100 })
    ips.value = reply.ips ?? []
  } catch {
    ips.value = []
  }
}

async function load() {
  loading.value = true
  try {
    const reply = await playPoolClient.AdminList({
      keyword: filters.keyword.trim(),
      mode: filters.mode ?? '',
      ipId: filters.ipId ?? '',
      status: filters.status ?? '',
      tag: filters.tag.trim(),
      sortOrder: filters.sortOrder,
      page: page.value,
      pageSize: pageSize.value,
    })
    items.value = reply.items ?? []
    total.value = Number(reply.total ?? 0)
    page.value = Number(reply.page ?? page.value)
    pageSize.value = Number(reply.pageSize ?? pageSize.value)
    for (const item of items.value) {
      if (item.poolId) weights[item.poolId] = Number(item.weight ?? 0)
    }
    selectedRowKeys.value = []
  } catch {
    items.value = []
    total.value = 0
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

function onPageSizeChange(_current: number, size: number) {
  pageSize.value = size
  page.value = 1
  void load()
}

function weightOf(record: AdminPlayPool): number {
  return record.poolId ? weights[record.poolId] ?? 0 : 0
}

function setWeight(record: AdminPlayPool, value: number | null) {
  if (!record.poolId) return
  weights[record.poolId] = Math.max(0, Math.trunc(value ?? 0))
}

function openCreate() {
  createModalOpen.value = true
}

function onCreateSuccess() {
  createModalOpen.value = false
  page.value = 1
  void load()
}

async function copyPool(record: AdminPlayPool) {
  if (!record.poolId) return
  copyingId.value = record.poolId
  try {
    const detail = record.latestVersion
      ? await playPoolClient.Get({ poolId: record.poolId, version: record.latestVersion })
      : null
    createForm.mode = record.mode ?? detail?.mode ?? 'gashapon'
    createForm.name = `${record.name ?? ''}${t('poolAdmin.copySuffix')}`
    createForm.priceYuan = Number(detail?.priceFen ?? record.priceFen ?? 0) / 100
    createForm.ipId = record.ipId || undefined
    createForm.tags = [...(record.tags ?? [])]
    createForm.periods = Number(record.periods ?? 0)
    createForm.weight = Number(record.weight ?? 0)
    createForm.configJson = detail?.configJson ?? configSkeleton[createForm.mode] ?? '{}'
    createForm.prizesJson = JSON.stringify(detail?.prizes ?? prizeSkeleton[createForm.mode] ?? [], null, 2)
    createOpen.value = true
  } catch {
    message.error(t('poolAdmin.detailLoadFailed'))
  } finally {
    copyingId.value = undefined
  }
}

async function submitCreate() {
  if (!createForm.name.trim()) {
    message.warning(t('poolAdmin.nameRequired'))
    return
  }
  let prizes: unknown[]
  try {
    prizes = JSON.parse(createForm.prizesJson)
    if (!Array.isArray(prizes) || prizes.length === 0) throw new Error('empty')
    JSON.parse(createForm.configJson || '{}')
  } catch {
    message.error(t('poolAdmin.jsonInvalid'))
    return
  }
  creating.value = true
  try {
    await playPoolClient.CreateDraft({
      idempotencyKey: crypto.randomUUID(),
      mode: createForm.mode,
      name: createForm.name.trim(),
      priceFen: Math.round((createForm.priceYuan || 0) * 100),
      configJson: createForm.configJson || '{}',
      prizes: prizes as PlayPoolPrize[],
      ipId: createForm.ipId ?? '',
      tags: createForm.tags,
      periods: createForm.periods,
      weight: createForm.weight,
    })
    message.success(t('poolAdmin.created'))
    createOpen.value = false
    page.value = 1
    await load()
  } catch {
    message.error(t('poolAdmin.createFailed'))
  } finally {
    creating.value = false
  }
}

function openEdit(record: AdminPlayPool) {
  editing.value = record
  editForm.name = record.name ?? ''
  editForm.priceYuan = Number(record.priceFen ?? 0) / 100
  editForm.ipId = record.ipId || undefined
  editForm.tags = [...(record.tags ?? [])]
  editForm.periods = Number(record.periods ?? 0)
  editForm.weight = Number(record.weight ?? 0)
  editOpen.value = true
}

async function submitEdit() {
  if (!editing.value?.poolId) return
  if (!editForm.name.trim()) {
    message.warning(t('poolAdmin.nameRequired'))
    return
  }
  editingSaving.value = true
  try {
    await playPoolClient.AdminUpdate(editing.value.poolId, {
      name: editForm.name.trim(),
      ipId: editForm.ipId ?? '',
      tags: editForm.tags,
      periods: editForm.periods,
      weight: editForm.weight,
      priceFen: Math.round((editForm.priceYuan || 0) * 100),
    })
    message.success(t('poolAdmin.updated'))
    editOpen.value = false
    await load()
  } catch {
    message.error(t('poolAdmin.updateFailed'))
  } finally {
    editingSaving.value = false
  }
}

async function saveWeights() {
  const payload = items.value
    .filter((item) => item.poolId)
    .map((item) => ({ poolId: item.poolId as string, weight: weights[item.poolId as string] ?? 0 }))
  if (payload.length === 0) return
  try {
    await playPoolClient.SaveAdminWeights(payload)
    message.success(t('poolAdmin.weightsSaved'))
  } catch {
    message.error(t('poolAdmin.weightsSaveFailed'))
  }
}

function deleteSelected() {
  if (selectedRowKeys.value.length === 0) {
    message.warning(t('poolAdmin.selectToDelete'))
    return
  }
  Modal.confirm({
    title: t('poolAdmin.deletePools'),
    content: t('poolAdmin.deleteConfirm'),
    okText: t('common.delete'),
    cancelText: t('poolAdmin.cancel'),
    okType: 'danger',
    onOk: async () => {
      await playPoolClient.DeleteAdminPools(selectedRowKeys.value)
      message.success(t('poolAdmin.deleted'))
      selectedRowKeys.value = []
      await load()
    },
  })
}

function openReference() {
  referencePoolId.value = ''
  referenceResults.value = []
  referenceOpen.value = true
}

async function queryReference() {
  if (!referencePoolId.value.trim()) return
  referenceLoading.value = true
  try {
    const reply = await playPoolClient.AdminList({
      keyword: referencePoolId.value.trim(),
      mode: '',
      ipId: '',
      status: '',
      tag: '',
      sortOrder: 'desc',
      page: 1,
      pageSize: 50,
    })
    referenceResults.value = reply.items ?? []
  } catch {
    referenceResults.value = []
  } finally {
    referenceLoading.value = false
  }
}

function openMatch(record: AdminPlayPool) {
  if (record.poolId) router.push({ path: '/gacha/match', query: { poolId: record.poolId } })
}

function modeText(mode?: string): string {
  switch (mode) {
    case 'gashapon': return t('poolAdmin.modeGashapon')
    case 'fullcar': return t('poolAdmin.modeFullcar')
    case 'combochain': return t('poolAdmin.modeCombochain')
    case 'dig': return t('poolAdmin.modeDig')
    case 'groupbuy': return t('poolAdmin.modeGroupbuy')
    case 'memorymatch': return t('poolAdmin.modeMemorymatch')
    default: return mode || '-'
  }
}

function statusText(status?: string | number): string {
  switch (String(status ?? '')) {
    case '2': return t('poolAdmin.statusPublished')
    case '3': return t('poolAdmin.statusRetired')
    default: return t('poolAdmin.statusDraft')
  }
}

function formatPrice(value?: string): string {
  return String(Number(value ?? 0) / 100)
}

function formatTime(value?: string): string {
  const seconds = Number(value ?? 0)
  if (!seconds) return ''
  return new Date(seconds * 1000).toLocaleString()
}

onMounted(async () => {
  await loadOptions()
  await load()
})
</script>

<style scoped lang="scss">
.pool-page {
  min-height: calc(100vh - 132px);
  padding: 14px 16px 18px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}

.pool-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
}

.filter-keyword { width: 180px; }
.filter-select { width: 150px; }
.filter-tag { width: 150px; }
.filter-sort { width: 160px; }

.btn-query {
  background: #42b983 !important;
  border-color: #42b983 !important;
}

.btn-query:hover {
  background: #36a374 !important;
  border-color: #36a374 !important;
}

.btn-plain {
  color: #606266;
  border-color: #dcdfe6;
  background: #fff;
}

.btn-plain:hover {
  color: #409eff;
  border-color: #409eff;
}

.pool-tag {
  color: #e6a23c;
  border-color: #f3d19e;
  background: #fdf6ec;
}

.weight-input {
  width: 84px;
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

.btn-match {
  min-width: 64px;
  background: #2d8cf0 !important;
  border-color: #2d8cf0 !important;
  color: #fff !important;
}

.pool-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.reference-search {
  display: flex;
  width: 100%;
  margin-bottom: 12px;
}

.reference-search .ant-input {
  flex: 1;
}
</style>

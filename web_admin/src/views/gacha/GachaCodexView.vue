<template>
  <div class="codex-page console-list-page">
    <div class="page-head">
      <h2>图鉴集合</h2>
    </div>

    <div class="toolbar">
      <a-input
        v-model:value="query.keyword"
        class="toolbar-input"
        placeholder="集合名称/编码"
        allow-clear
        @press-enter="search"
      />
      <a-select v-model:value="query.status" :options="statusFilterOptions" class="toolbar-status" />
      <a-button type="primary" :loading="loading" @click="search">查询</a-button>
      <a-button v-if="canCreate" @click="openCreate">新建集合</a-button>
    </div>

    <a-alert
      type="info"
      show-icon
      class="notice"
      message="核心规则发布后锁定；调整商品数量、限时时间或奖励门槛时应创建新版本。"
    />

    <a-table
      :data-source="items"
      :columns="columns"
      row-key="id"
      :loading="loading"
      :pagination="false"
      size="middle"
      :scroll="{ x: 1180 }"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'status'">
          <a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag>
        </template>
        <template v-else-if="column.key === 'accumulate'">
          {{ accumulateText(record.accumulateType) }}
        </template>
        <template v-else-if="column.key === 'cycle'">
          <a-tag :color="cycleColor(record.cycleType)">{{ cycleText(record.cycleType) }}</a-tag>
        </template>
        <template v-else-if="column.key === 'sort'">
          {{ Number(record.sort ?? 0) }}
        </template>
        <template v-else-if="column.key === 'action'">
          <a-space :size="4">
            <a-button v-if="canWrite" size="small" @click="openEdit(record)">配置</a-button>
            <a-button size="small" @click="openData(record)">数据</a-button>
            <a-button v-if="canWrite" size="small" @click="openStatus(record)">状态</a-button>
          </a-space>
        </template>
      </template>
    </a-table>

    <div class="pager console-list-footer">
      <a-pagination
        v-model:current="query.page"
        v-model:page-size="query.pageSize"
        :total="total"
        :page-size-options="['20', '50', '100']"
        show-size-changer
        show-less-items
        @change="load"
        @show-size-change="onPageSizeChange"
      />
    </div>

    <a-modal
      v-model:open="editOpen"
      :title="editingId ? '配置图鉴集合' : '新建图鉴集合'"
      :confirm-loading="saving"
      width="680px"
      @ok="save"
    >
      <a-form :label-col="{ span: 6 }" :wrapper-col="{ span: 16 }">
        <a-form-item label="集合编码" required>
          <a-input v-model:value="form.code" :disabled="!!editingId" placeholder="如 COL_20261009073326_25A8FDD8EE9F" />
        </a-form-item>
        <a-form-item label="集合名称" required>
          <a-input v-model:value="form.name" placeholder="请输入集合名称" />
        </a-form-item>
        <a-form-item label="累计方式">
          <a-select v-model:value="form.accumulateType" :options="accumulateOptions" />
        </a-form-item>
        <a-form-item label="循环领取">
          <a-select v-model:value="form.cycleType" :options="cycleOptions" />
        </a-form-item>
        <a-form-item label="排序">
          <a-input-number v-model:value="form.sort" :min="0" :precision="0" style="width: 100%" />
        </a-form-item>
        <a-form-item label="状态">
          <a-select v-model:value="form.status" :options="statusOptions" />
        </a-form-item>
        <a-form-item label="限时开始">
          <a-input v-model:value="form.startAt" placeholder="可选，如 2026-09-22 00:00:00" />
        </a-form-item>
        <a-form-item label="限时结束">
          <a-input v-model:value="form.endAt" placeholder="可选" />
        </a-form-item>
        <a-form-item label="配置 JSON">
          <a-textarea v-model:value="form.configJson" :rows="6" spellcheck="false" placeholder='{"items":[],"rewards":[]}' />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:open="dataOpen" title="图鉴数据" :footer="null" width="620px">
      <div class="data-grid">
        <div class="data-item"><span>参与人数</span><strong>{{ data.participants }}</strong></div>
        <div class="data-item"><span>已完成</span><strong>{{ data.completed }}</strong></div>
        <div class="data-item"><span>已领取</span><strong>{{ data.claims }}</strong></div>
        <div class="data-item"><span>待领取</span><strong>{{ data.pendingClaims }}</strong></div>
      </div>
      <a-table
        v-if="data.rewards.length > 0"
        :data-source="data.rewards"
        :pagination="false"
        size="small"
        row-key="name"
      >
        <a-table-column title="奖励" data-index="name" />
        <a-table-column title="已领取" data-index="claimed" width="100" align="center" />
        <a-table-column title="库存" data-index="stock" width="100" align="center" />
      </a-table>
      <a-empty v-else description="暂无数据" />
    </a-modal>

    <a-modal v-model:open="statusOpen" title="更新状态" :confirm-loading="statusSaving" width="420px" @ok="saveStatus">
      <a-select v-model:value="statusValue" :options="statusOptions" style="width: 100%" />
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'
import { codexClient, type CollectionSetData, type CollectionSetItem } from '@/api/codex'

const session = useSessionStore()
const canCreate = computed(() => allows(session.permissionNodes, 'gacha.pool', 'create'))
const canWrite = computed(() => allows(session.permissionNodes, 'gacha.pool', 'write'))

const items = ref<CollectionSetItem[]>([])
const total = ref(0)
const loading = ref(false)
const query = reactive({ keyword: '', status: 0, page: 1, pageSize: 20 })

const statusFilterOptions = [
  { value: 0, label: '全部状态' },
  { value: 1, label: '进行中' },
  { value: 2, label: '已结束' },
  { value: 3, label: '已下线' },
]
const statusOptions = [
  { value: 1, label: '进行中' },
  { value: 2, label: '已结束' },
  { value: 3, label: '已下线' },
]
const accumulateOptions = [
  { value: 1, label: '永久累计' },
  { value: 2, label: '限时累计' },
]
const baseCycleOptions = [
  { value: 0, label: '单轮' },
  { value: 3, label: '循环领取·3轮' },
  { value: 5, label: '循环领取·5轮' },
  { value: 10, label: '循环领取·10轮' },
  { value: -1, label: '循环领取·无限轮' },
]
const cycleOptions = ref([...baseCycleOptions])

const columns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 70 },
  { title: '编码', dataIndex: 'code', key: 'code', width: 280 },
  { title: '集合名称', dataIndex: 'name', key: 'name' },
  { title: '版本', dataIndex: 'version', key: 'version', width: 70, align: 'center' as const },
  { title: '状态', key: 'status', width: 90, align: 'center' as const },
  { title: '累计/循环', key: 'accumulate', width: 110, align: 'center' as const },
  { title: '循环领取', key: 'cycle', width: 140, align: 'center' as const },
  { title: '排序', dataIndex: 'sort', key: 'sort', width: 70, align: 'center' as const },
  { title: '操作', key: 'action', width: 190, align: 'center' as const, fixed: 'right' as const },
]

function statusText(status: number | string): string {
  switch (Number(status)) {
    case 1: return '进行中'
    case 2: return '已结束'
    case 3: return '已下线'
    default: return '-'
  }
}

function statusColor(status: number | string): string {
  switch (Number(status)) {
    case 1: return 'green'
    case 2: return 'orange'
    default: return 'default'
  }
}

function accumulateText(type: number | string): string {
  return Number(type) === 2 ? '限时累计' : '永久累计'
}

function cycleText(cycle?: number | string): string {
  const value = Number(cycle ?? 0)
  if (value < 0) return '循环领取·无限轮'
  if (value === 0) return '单轮'
  return `循环领取·${value}轮`
}

function cycleColor(cycle?: number | string): string {
  const value = Number(cycle ?? 0)
  if (value < 0) return 'green'
  if (value === 0) return 'default'
  return 'blue'
}

async function load() {
  loading.value = true
  try {
    const reply = await codexClient.List({
      keyword: query.keyword.trim(),
      status: query.status,
      page: query.page,
      pageSize: query.pageSize,
    })
    items.value = reply.items ?? []
    total.value = Number(reply.total ?? 0)
  } catch {
    // 错误提示由请求拦截器统一处理
  } finally {
    loading.value = false
  }
}

function search() {
  query.page = 1
  void load()
}

function onPageSizeChange(_current: number, size: number) {
  query.pageSize = size
  query.page = 1
  void load()
}

function newCode(): string {
  const stamp = new Date().toISOString().replace(/[-:.TZ]/g, '').slice(0, 14)
  const rand = Math.random().toString(16).slice(2, 10).toUpperCase()
  return `COL_${stamp}_${rand}`
}

const editOpen = ref(false)
const saving = ref(false)
const editingId = ref(0)
const form = reactive({
  code: '',
  name: '',
  accumulateType: 1,
  cycleType: 0,
  sort: 0,
  status: 1,
  startAt: '',
  endAt: '',
  configJson: '{}',
})

function resetForm() {
  form.code = newCode()
  form.name = ''
  form.accumulateType = 1
  form.cycleType = 0
  form.sort = 0
  form.status = 1
  form.startAt = ''
  form.endAt = ''
  form.configJson = '{}'
  cycleOptions.value = [...baseCycleOptions]
}

function openCreate() {
  editingId.value = 0
  resetForm()
  editOpen.value = true
}

function openEdit(record: CollectionSetItem) {
  editingId.value = record.id
  form.code = record.code
  form.name = record.name
  form.accumulateType = Number(record.accumulateType ?? 1)
  form.cycleType = Number(record.cycleType ?? 0)
  form.sort = Number(record.sort ?? 0)
  form.status = Number(record.status ?? 1)
  form.startAt = record.startAt ?? ''
  form.endAt = record.endAt ?? ''
  form.configJson = record.configJson || '{}'
  cycleOptions.value = baseCycleOptions.some((option) => option.value === form.cycleType)
    ? [...baseCycleOptions]
    : [...baseCycleOptions, { value: form.cycleType, label: cycleText(form.cycleType) }]
  editOpen.value = true
}

async function save() {
  if (!form.code.trim()) {
    message.warning('请输入集合编码')
    return
  }
  if (!form.name.trim()) {
    message.warning('请输入集合名称')
    return
  }
  try {
    JSON.parse(form.configJson || '{}')
  } catch {
    message.warning('配置 JSON 格式不正确')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await codexClient.Update(editingId.value, {
        name: form.name.trim(),
        accumulateType: form.accumulateType,
        cycleType: form.cycleType,
        sort: form.sort,
        status: form.status,
        configJson: form.configJson,
        startAt: form.startAt,
        endAt: form.endAt,
      })
      message.success('图鉴集合已更新')
    } else {
      await codexClient.Create({
        code: form.code.trim(),
        name: form.name.trim(),
        accumulateType: form.accumulateType,
        cycleType: form.cycleType,
        sort: form.sort,
        status: form.status,
        configJson: form.configJson,
        startAt: form.startAt,
        endAt: form.endAt,
      })
      message.success('图鉴集合已创建')
    }
    editOpen.value = false
    await load()
  } catch {
    // 错误提示由请求拦截器统一处理
  } finally {
    saving.value = false
  }
}

const dataOpen = ref(false)
const data = reactive<CollectionSetData>({
  participants: 0,
  completed: 0,
  claims: 0,
  pendingClaims: 0,
  rewards: [],
})

async function openData(record: CollectionSetItem) {
  try {
    const reply = await codexClient.Data(record.id)
    data.participants = Number(reply.participants ?? 0)
    data.completed = Number(reply.completed ?? 0)
    data.claims = Number(reply.claims ?? 0)
    data.pendingClaims = Number(reply.pendingClaims ?? 0)
    data.rewards = reply.rewards ?? []
    dataOpen.value = true
  } catch {
    // 错误提示由请求拦截器统一处理
  }
}

const statusOpen = ref(false)
const statusSaving = ref(false)
const statusValue = ref(1)
const statusTarget = ref<CollectionSetItem | null>(null)

function openStatus(record: CollectionSetItem) {
  statusTarget.value = record
  statusValue.value = Number(record.status ?? 1)
  statusOpen.value = true
}

async function saveStatus() {
  if (!statusTarget.value) return
  statusSaving.value = true
  try {
    await codexClient.SetStatus(statusTarget.value.id, statusValue.value)
    message.success('状态已更新')
    statusOpen.value = false
    await load()
  } catch {
    // 错误提示由请求拦截器统一处理
  } finally {
    statusSaving.value = false
  }
}

onMounted(load)
</script>

<style scoped lang="scss">
.codex-page {
  padding: 14px 16px 18px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}

.page-head h2 {
  margin: 0 0 18px;
  font-size: 20px;
  font-weight: 600;
  color: #1f2329;
}

.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}

.toolbar-input {
  width: 220px;
}

.toolbar-status {
  width: 160px;
}

.notice {
  margin-bottom: 16px;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 18px;
}

.data-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 18px;
}

.data-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px;
  border: 1px solid #f0f0f0;
  border-radius: 8px;
  background: #fafafa;
  color: #646a73;

  strong {
    font-size: 22px;
    color: #1f2329;
  }
}
</style>

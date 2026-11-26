<template>
  <div class="box-page console-list-page">
    <div class="box-toolbar">
      <a-input
        v-model:value="filters.keyword"
        class="filter-keyword"
        placeholder="主题名称 / 宝箱ID"
        allow-clear
        @press-enter="search"
      />
      <a-button class="btn-query" type="primary" :loading="loading" @click="search">查 询</a-button>
      <a-button class="btn-plain" :loading="loading" @click="load">刷新</a-button>
      <a-button v-if="canCreate" class="btn-plain" @click="openCreate">创建无限赏</a-button>
    </div>

    <a-table
      :data-source="items"
      :loading="loading"
      :pagination="false"
      :row-key="(record: BoxPoolSummary) => String(record.poolId ?? '')"
      size="middle"
      :scroll="{ x: 1500 }"
    >
      <a-table-column title="宝箱ID" data-index="poolId" width="150" align="center" />
      <a-table-column title="主题名称" data-index="boxName" min-width="240" align="center" />
      <a-table-column title="价格" width="100" align="center">
        <template #default="{ record }">{{ formatPrice(record.price) }}</template>
      </a-table-column>
      <a-table-column title="期数" width="80" align="center">
        <template #default="{ record }">{{ Number(record.periods ?? 0) }}</template>
      </a-table-column>
      <a-table-column title="活动时间" width="330" align="center">
        <template #default="{ record }">{{ formatRange(record.startAt, record.endAt) }}</template>
      </a-table-column>
      <a-table-column title="折扣" width="80" align="center">
        <template #default="{ record }">{{ Number(record.discount ?? 0) }}%</template>
      </a-table-column>
      <a-table-column title="版本" width="80" align="center">
        <template #default="{ record }">v{{ record.version }}</template>
      </a-table-column>
      <a-table-column title="状态" width="100" align="center">
        <template #default="{ record }">
          <a-tag :color="record.status === 'POOL_STATUS_PUBLISHED' ? 'green' : 'blue'">
            {{ record.status === 'POOL_STATUS_PUBLISHED' ? '已发布' : '草稿' }}
          </a-tag>
        </template>
      </a-table-column>
      <a-table-column title="排序权重" width="100" align="center">
        <template #default="{ record }">{{ Number(record.sortWeight ?? 0) }}</template>
      </a-table-column>
      <a-table-column title="更新时间" width="170" align="center">
        <template #default="{ record }">{{ formatTime(record.updatedAtUnixMillis) }}</template>
      </a-table-column>
      <a-table-column title="操作" width="300" align="center" fixed="right">
        <template #default="{ record }">
          <a-space :size="4">
            <a-button v-if="canWrite" size="small" @click="openEdit(record)">编辑</a-button>
            <a-button
              v-if="canRequest"
              size="small"
              :disabled="record.status === 'POOL_STATUS_PUBLISHED'"
              @click="requestPublication(record)"
            >
              申请发布
            </a-button>
            <a-button
              v-if="canApprove"
              size="small"
              :disabled="record.status === 'POOL_STATUS_PUBLISHED'"
              @click="approvePublication(record)"
            >
              审核通过
            </a-button>
            <a-button
              v-if="canPublish"
              size="small"
              type="primary"
              :disabled="record.status === 'POOL_STATUS_PUBLISHED'"
              @click="publish(record)"
            >
              发布
            </a-button>
          </a-space>
        </template>
      </a-table-column>
    </a-table>

    <div class="box-footer console-list-footer">
      <a-pagination
        :current="page"
        :page-size="pageSize"
        :total="total"
        :page-size-options="pageSizeOptions"
        :locale="{ items_per_page: '条/页' }"
        show-size-changer
        show-less-items
        @change="onPageChange"
        @show-size-change="onPageSizeChange"
      />
    </div>

    <a-modal
      v-if="formOpen"
      v-model:open="formOpen"
      centered
      width="96vw"
      :style="{ maxWidth: '1800px', top: '24px' }"
      :footer="null"
      :destroy-on-close="true"
      :body-style="{ maxHeight: 'calc(100vh - 120px)', overflowY: 'auto' }"
    >
      <template #title>
        <div class="form-title">{{ formInitial?.poolId ? '编辑无限赏' : '创建无限赏' }}</div>
      </template>
      <PlayPoolCreateForm
        :key="formKey"
        :initial="formInitial"
        lock-mode="box"
        @success="onFormSuccess"
        @cancel="formOpen = false"
      />
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message, Modal } from 'ant-design-vue'

import { allows } from '@/access/grants'
import { poolClient, type BoxDraft, type BoxPoolSummary } from '@/api/pool'
import type { PlayPoolCreateInitial, PlayPoolPrize } from '@/api/playPool'
import { useSessionStore } from '@/stores/session'
import PlayPoolCreateForm from '@/views/gacha/PlayPoolCreateForm.vue'

const session = useSessionStore()

const canCreate = computed(() => allows(session.permissionNodes, 'gacha.pool', 'create'))
const canWrite = computed(() => allows(session.permissionNodes, 'gacha.pool', 'write'))
const canRequest = computed(() => allows(session.permissionNodes, 'gacha.pool', 'request_publication'))
const canApprove = computed(() => allows(session.permissionNodes, 'gacha.pool', 'approve_publication'))
const canPublish = computed(() => allows(session.permissionNodes, 'gacha.pool', 'publish'))

const filters = reactive({ keyword: '' })
const items = ref<BoxPoolSummary[]>([])
const page = ref(1)
const pageSize = ref(20)
const pageSizeOptions = ['10', '20', '50', '100']
const total = ref(0)
const loading = ref(false)

const formOpen = ref(false)
const formKey = ref(0)
const formInitial = ref<PlayPoolCreateInitial | null>(null)

async function load() {
  loading.value = true
  try {
    const reply = await poolClient().ListBoxPools({
      page: page.value,
      pageSize: pageSize.value,
      keyword: filters.keyword.trim(),
    })
    items.value = reply.items ?? []
    total.value = Number(reply.total ?? 0)
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

function onPageSizeChange(_current: number, nextPageSize: number) {
  pageSize.value = nextPageSize
  page.value = 1
  void load()
}

function formatPrice(value?: string) {
  return `¥${(Number(value ?? 0) / 100).toFixed(2)}`
}

function formatTime(value?: string) {
  const n = Number(value ?? 0)
  if (!n) return '-'
  return new Date(n).toLocaleString('zh-CN', { hour12: false })
}

function formatRange(startAt?: string, endAt?: string) {
  const start = startAt || '-'
  const end = endAt || '-'
  return `${start} ~ ${end}`
}

function openCreate() {
  formInitial.value = { mode: 'box', name: '' }
  formKey.value += 1
  formOpen.value = true
}

async function openEdit(record: BoxPoolSummary) {
  if (!record.poolId || !record.version) return
  loading.value = true
  try {
    const box = await poolClient().Get({ poolId: String(record.poolId), version: String(record.version) })
    formInitial.value = boxToInitial(box)
    formKey.value += 1
    formOpen.value = true
  } finally {
    loading.value = false
  }
}

function boxToInitial(box: BoxDraft): PlayPoolCreateInitial {
  const flags = {
    isOnSale: box.isOnSale !== false,
    isNew: Boolean(box.isNew),
    isRecommended: Boolean(box.isRecommended),
    isBargain: Boolean(box.isBargain),
    isPresale: Boolean(box.isPresale),
    isMiniapp: Boolean(box.isMiniapp),
    isWelfare: Boolean(box.isWelfare),
    excludeTask: Boolean(box.excludeTask),
    allowBoxInBox: Boolean(box.allowBoxInBox),
  }
  const tags: string[] = []
  if (flags.isOnSale) tags.push('展示')
  if (flags.isNew) tags.push('新品')
  if (flags.isRecommended) tags.push('推荐')
  if (flags.isBargain) tags.push('捡漏')
  if (flags.isPresale) tags.push('预售')
  const prizes: PlayPoolPrize[] = (box.entries ?? []).map((entry, index) => ({
    prizeIndex: index,
    prizeGrade: entry.prizeGrade ?? '',
    prizeType: Number(entry.prizeType ?? 0),
    name: entry.name ?? '',
    skuId: Number(entry.skuId ?? 0),
    weight: Number(entry.weight ?? 1),
    quantity: Number(entry.quantity ?? 1),
    remaining: Number(entry.quantity ?? 1),
    isPrize: entry.isPrize !== false,
    costFen: Number(entry.costFen ?? 0),
    imageUrl: entry.imageUrl ?? '',
  }))
  let coupons: PlayPoolCreateInitial['coupons'] = []
  try {
    const parsed = JSON.parse(box.couponsJson || '[]') as unknown
    if (Array.isArray(parsed)) coupons = parsed as NonNullable<PlayPoolCreateInitial['coupons']>
  } catch {
    coupons = []
  }
  return {
    poolId: String(box.poolId ?? ''),
    mode: 'box',
    name: box.boxName ?? '',
    priceYuan: Number(box.price ?? 0) / 100,
    ipId: box.ipId ? String(box.ipId) : undefined,
    periods: Number(box.periods ?? 1),
    weight: Number(box.sortWeight ?? 0),
    prizes,
    coverImage: box.coverImage ?? '',
    coverThumb: box.coverThumb ?? '',
    startAt: box.startAt ?? '',
    endAt: box.endAt ?? '',
    discount: Number(box.discount ?? 0),
    tags,
    coupons,
    ...flags,
  }
}

function onFormSuccess() {
  formOpen.value = false
  page.value = 1
  void load()
}

async function fetchBox(record: BoxPoolSummary): Promise<BoxDraft> {
  return poolClient().Get({ poolId: String(record.poolId), version: String(record.version) })
}

function requestPublication(record: BoxPoolSummary) {
  Modal.confirm({
    title: '申请发布',
    content: `确认提交「${record.boxName || record.poolId}」的发布申请？`,
    okText: '确认',
    cancelText: '取消',
    onOk: async () => {
      const box = await fetchBox(record)
      await poolClient().RequestPublication({
        poolId: String(record.poolId),
        version: String(record.version),
        expectedConfigHash: box.configHash ?? '',
      })
      message.success('已提交发布申请')
      void load()
    },
  })
}

function approvePublication(record: BoxPoolSummary) {
  Modal.confirm({
    title: '审核通过',
    content: `确认审核通过「${record.boxName || record.poolId}」？审批人不能与申请人相同。`,
    okText: '通过',
    cancelText: '取消',
    onOk: async () => {
      const box = await fetchBox(record)
      await poolClient().ApprovePublication({
        poolId: String(record.poolId),
        version: String(record.version),
        expectedConfigHash: box.configHash ?? '',
      })
      message.success('已审核通过')
      void load()
    },
  })
}

function publish(record: BoxPoolSummary) {
  Modal.confirm({
    title: '发布无限赏',
    content: `确认发布「${record.boxName || record.poolId}」？发布后将物化到线上宝箱。`,
    okText: '发布',
    cancelText: '取消',
    onOk: async () => {
      const box = await fetchBox(record)
      await poolClient().Publish({
        poolId: String(record.poolId),
        version: String(record.version),
        expectedConfigHash: box.configHash ?? '',
      })
      message.success('已发布')
      void load()
    },
  })
}

onMounted(load)
</script>

<style scoped>
.box-page {
  padding: 20px 24px 32px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}

.box-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
}

.filter-keyword {
  width: 260px;
}

.btn-query {
  background: #42b983;
  border-color: #42b983;
}

.btn-query:hover {
  background: #36a374 !important;
  border-color: #36a374 !important;
}

.btn-plain {
  background: #fff;
  border-color: #d9d9d9;
}

.box-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.form-title {
  text-align: center;
  font-size: 16px;
  font-weight: 600;
}
</style>

<template>
  <div>
    <PageHeader :title="$t('nav.catalog')" :show-back="false">
      <template #extra>
        <a-space>
          <a-button :loading="loading" @click="load">刷新</a-button>
          <a-button v-if="canCreate" type="primary" @click="openCreate">新建商品</a-button>
        </a-space>
      </template>
    </PageHeader>

    <a-table
      :data-source="products"
      :loading="loading"
      :pagination="false"
      :row-key="(row: ProductSummaryView) => row.productId"
      size="middle"
      :scroll="{ x: 760 }"
    >
      <a-table-column title="商品 ID" data-index="productId" width="110" />
      <a-table-column title="商品编码" data-index="productCode" width="180" />
      <a-table-column title="名称" data-index="title" />
      <a-table-column title="状态" width="110">
        <template #default="{ record }">
          <a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag>
        </template>
      </a-table-column>
      <a-table-column title="版本" data-index="version" width="90" />
      <a-table-column title="操作" width="160">
        <template #default="{ record }">
          <a-space>
            <router-link :to="`/catalog/products/${record.productId}`">查看</router-link>
            <a v-if="canPublish && isDraft(record.status)" @click="publish(record)">发布</a>
          </a-space>
        </template>
      </a-table-column>
    </a-table>

    <a-space class="pager">
      <a-button :disabled="loading || cursors.length === 1" @click="previous">上一页</a-button>
      <span>第 {{ cursors.length }} 页</span>
      <a-button :disabled="loading || !nextCursor" @click="next">下一页</a-button>
    </a-space>

    <a-modal v-model:open="createOpen" title="新建商品" :confirm-loading="creating" width="640px" @ok="create">
      <a-form layout="vertical">
        <a-row :gutter="16">
          <a-col :xs="24" :md="12">
            <a-form-item label="商品编码" required><a-input v-model:value="form.productCode" placeholder="例如 PLAY-SKU-1" /></a-form-item>
          </a-col>
          <a-col :xs="24" :md="12">
            <a-form-item label="商品名称" required><a-input v-model:value="form.title" /></a-form-item>
          </a-col>
        </a-row>
        <a-divider orientation="left">默认 SKU</a-divider>
        <a-row :gutter="16">
          <a-col :xs="24" :md="12">
            <a-form-item label="SKU 编码" required><a-input v-model:value="form.skuCode" placeholder="例如 PLAY-SKU-1-A" /></a-form-item>
          </a-col>
          <a-col :xs="24" :md="12">
            <a-form-item label="SKU 名称" required><a-input v-model:value="form.skuTitle" /></a-form-item>
          </a-col>
          <a-col :xs="24" :md="12">
            <a-form-item label="履约方式">
              <a-select v-model:value="form.fulfillmentMode" :options="fulfillmentOptions" />
            </a-form-item>
          </a-col>
          <a-col :xs="24" :md="12">
            <a-form-item label="售价（分）"><a-input-number v-model:value="form.saleAmountMinor" :min="0" style="width: 100%" /></a-form-item>
          </a-col>
        </a-row>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { useRouter } from 'vue-router'
import PageHeader from '@/components/PageHeader.vue'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'
import { listConsoleProducts, type ProductSummaryView } from '@/api/console'
import { catalogClient, type CreateProductInput } from '@/api/catalog'

const session = useSessionStore()
const router = useRouter()
const canCreate = computed(() => allows(session.permissionNodes, 'catalog.product', 'create'))
const canPublish = computed(() => allows(session.permissionNodes, 'catalog.product', 'publish'))

const products = ref<ProductSummaryView[]>([])
const cursors = ref([''])
const nextCursor = ref('')
const loading = ref(false)

const createOpen = ref(false)
const creating = ref(false)
const form = reactive({
  productCode: '',
  title: '',
  skuCode: '',
  skuTitle: '',
  fulfillmentMode: 'FULFILLMENT_MODE_PHYSICAL',
  saleAmountMinor: 1000,
})

const fulfillmentOptions = [
  { label: '实物', value: 'FULFILLMENT_MODE_PHYSICAL' },
  { label: '数字', value: 'FULFILLMENT_MODE_DIGITAL' },
  { label: '无需履约', value: 'FULFILLMENT_MODE_NONE' },
]

function statusText(status?: string | number): string {
  const value = String(status ?? '')
  if (value === '2' || value === 'PRODUCT_STATUS_ACTIVE') return '已上架'
  if (value === '3' || value === 'PRODUCT_STATUS_ARCHIVED') return '已归档'
  return '草稿'
}

function statusColor(status?: string | number): string {
  const value = String(status ?? '')
  if (value === '2' || value === 'PRODUCT_STATUS_ACTIVE') return 'green'
  if (value === '3' || value === 'PRODUCT_STATUS_ARCHIVED') return 'default'
  return 'blue'
}

function isDraft(status?: string | number): boolean {
  return statusText(status) === '草稿'
}

async function load() {
  loading.value = true
  try {
    const result = await listConsoleProducts(cursors.value.at(-1))
    products.value = result.products ?? []
    nextCursor.value = result.nextAfterProductId === '0' ? '' : result.nextAfterProductId ?? ''
  } catch {
    products.value = []
    message.error('商品列表加载失败')
  } finally {
    loading.value = false
  }
}

function next() {
  if (!nextCursor.value) return
  cursors.value.push(nextCursor.value)
  void load()
}

function previous() {
  if (cursors.value.length <= 1) return
  cursors.value.pop()
  void load()
}

function openCreate() {
  createOpen.value = true
}

async function create() {
  if (!form.productCode.trim() || !form.title.trim() || !form.skuCode.trim() || !form.skuTitle.trim()) {
    message.warning('请填写商品编码、名称及 SKU 编码、名称')
    return
  }
  creating.value = true
  const payload: CreateProductInput = {
    productCode: form.productCode.trim(),
    title: form.title.trim(),
    options: [],
    skus: [
      {
        skuCode: form.skuCode.trim(),
        title: form.skuTitle.trim(),
        fulfillmentMode: form.fulfillmentMode,
        salePrice: { amountMinor: form.saleAmountMinor, currency: 'CNY' },
        referencePrice: { amountMinor: form.saleAmountMinor, currency: 'CNY' },
        selections: [],
        externalCode: '',
      },
    ],
  }
  try {
    const created = await catalogClient.Create(payload)
    message.success('商品已创建')
    createOpen.value = false
    form.productCode = ''
    form.title = ''
    form.skuCode = ''
    form.skuTitle = ''
    await load()
    if (created.productId) router.push(`/catalog/products/${created.productId}`)
  } catch {
    message.error('创建失败，请检查编码是否重复')
  } finally {
    creating.value = false
  }
}

async function publish(record: ProductSummaryView) {
  try {
    await catalogClient.Publish({ productId: record.productId!, expectedVersion: record.version ?? '1' })
    message.success('商品已发布')
    await load()
  } catch {
    message.error('发布失败，请刷新后重试')
  }
}

onMounted(load)
</script>

<style scoped lang="scss">
.pager {
  margin-top: 16px;
}
</style>

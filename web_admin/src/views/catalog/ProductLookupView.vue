<template>
  <section>
    <h2>{{ $t('catalog.lookupTitle') }}</h2>
    <ProductEditor @changed="refresh" />
    <div v-if="canRead">
      <a-form :model="form" layout="inline" class="toolbar" @finish="openProduct">
        <a-form-item :label="$t('catalog.productId')" name="productId" :rules="[{ required: true, pattern: /^[1-9]\d*$/, message: $t('catalog.validation.invalidId') }]">
          <a-input v-model:value="form.productId" :placeholder="$t('catalog.productIdPlaceholder')" inputmode="numeric" />
        </a-form-item>
        <a-form-item><a-button html-type="submit">{{ $t('catalog.queryById') }}</a-button></a-form-item>
      </a-form>
      <a-alert v-if="error" type="error" show-icon :message="$t('catalog.listFailed')" class="toolbar" />
      <a-table :data-source="products" :loading="loading" :pagination="false" :row-key="(row: ProductSummaryView) => row.productId" :scroll="{ x: 720 }" size="small">
        <a-table-column :title="$t('catalog.productId')" data-index="productId" />
        <a-table-column :title="$t('catalog.productCode')" data-index="productCode" />
        <a-table-column :title="$t('common.name')" data-index="title" />
        <a-table-column :title="$t('common.status')" data-index="status" />
        <a-table-column :title="$t('common.version')" data-index="version" />
        <a-table-column :title="$t('common.action')">
          <template #default="{ record }"><router-link :to="entityRoutes.product(record.productId)">{{ $t('common.view') }}</router-link></template>
        </a-table-column>
      </a-table>
      <a-space class="pager">
        <a-button :disabled="loading || cursors.length === 1" @click="previous">{{ $t('common.previousPage') }}</a-button>
        <span>{{ $t('common.pageN', { n: cursors.length }) }}</span>
        <a-button :disabled="loading || !nextCursor" @click="next">{{ $t('common.nextPage') }}</a-button>
        <a-button :loading="loading" @click="load">{{ $t('common.refresh') }}</a-button>
      </a-space>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import ProductEditor from './ProductEditor.vue'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'
import { useRouter } from 'vue-router'
import { entityRoutes } from '@/navigation/entityRoutes'
import { listConsoleProducts, type ProductSummaryView } from '@/api/console'

const router = useRouter()
const session = useSessionStore()
const canRead = computed(() => allows(session.permissionNodes, 'catalog.product', 'read'))
const form = reactive({ productId: '' })
const products = ref<ProductSummaryView[]>([])
const cursors = ref([''])
const nextCursor = ref('')
const loading = ref(false)
const error = ref(false)

async function load() {
  if (!canRead.value) return
  loading.value = true
  error.value = false
  try {
    const result = await listConsoleProducts(cursors.value.at(-1))
    products.value = result.products ?? []
    nextCursor.value = result.nextAfterProductId === '0' ? '' : result.nextAfterProductId ?? ''
  } catch {
    error.value = true
    products.value = []
    nextCursor.value = ''
  } finally {
    loading.value = false
  }
}
function next() {
  if (nextCursor.value) {
    cursors.value.push(nextCursor.value)
    void load()
  }
}
function previous() {
  if (cursors.value.length > 1) {
    cursors.value.pop()
    void load()
  }
}
function openProduct() {
  router.push(entityRoutes.product(form.productId.trim()))
}
function refresh() {
  cursors.value = ['']
  void load()
}
onMounted(() => {
  void load()
})
</script>

<style scoped>.toolbar { margin-bottom: 16px; }.pager { margin-top: 16px; }</style>

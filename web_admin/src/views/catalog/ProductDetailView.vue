<template>
  <div>
    <PageHeader :title="$t('catalog.productTitle', { id: productId })" />
    <a-spin :spinning="loading">
      <a-alert v-if="error" type="error" show-icon :message="error" class="notice" />
      <template v-if="product">
        <a-descriptions bordered :column="2" size="small" class="notice">
          <a-descriptions-item :label="$t('catalog.productId')">{{ product.productId }}</a-descriptions-item>
          <a-descriptions-item :label="$t('catalog.productCode')">{{ product.productCode || $t('common.none') }}</a-descriptions-item>
          <a-descriptions-item :label="$t('common.name')">{{ product.title }}</a-descriptions-item>
          <a-descriptions-item :label="$t('common.status')">{{ product.status }}</a-descriptions-item>
          <a-descriptions-item :label="$t('common.version')">{{ product.version }}</a-descriptions-item>
        </a-descriptions>
        <h3>{{ $t('catalog.options') }}</h3>
        <a-empty v-if="!product.options?.length" :description="$t('catalog.noOptions')" />
        <a-descriptions v-else bordered :column="1" size="small" class="notice">
          <a-descriptions-item v-for="option in product.options" :key="option.optionId" :label="option.name">
            {{ option.values?.map((value) => value.value).join('、') || $t('common.none') }}
          </a-descriptions-item>
        </a-descriptions>
        <h3>{{ $t('catalog.skus') }}</h3>
        <a-table :data-source="product.skus ?? []" :pagination="false" :row-key="(row: { skuId: string }) => row.skuId" size="small" :scroll="{ x: 600 }">
          <a-table-column title="SKU ID" data-index="skuId" />
          <a-table-column :title="$t('catalog.skuCode')" data-index="skuCode" />
          <a-table-column :title="$t('common.name')" data-index="title" />
          <a-table-column :title="$t('common.status')" data-index="status" />
        </a-table>
      </template>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import PageHeader from '@/components/PageHeader.vue'
import { getConsoleProduct, type ProductView } from '@/api/console'
import { routeParam } from '@/utils/routeParams'

const route = useRoute()
const { t } = useI18n()
const productId = computed(() => routeParam(route, 'productId'))
const product = ref<ProductView | null>(null)
const loading = ref(false)
const error = ref('')
watch(
  productId,
  async (id, _old, onCleanup) => {
    let active = true
    onCleanup(() => {
      active = false
    })
    product.value = null
    error.value = ''
    loading.value = true
    try {
      const result = await getConsoleProduct(id)
      if (active) product.value = result
    } catch {
      if (active) error.value = t('catalog.detailFailed')
    } finally {
      if (active) loading.value = false
    }
  },
  { immediate: true },
)
</script>

<style scoped>.notice { margin-bottom: 16px; }</style>

<template>
  <div>
    <PageHeader :title="$t('catalog.skuTitle', { id: skuId })" />
    <a-alert type="info" show-icon :message="$t('catalog.skuDetailTodo')" />
    <a-space direction="vertical" style="margin-top: 16px">
      <span>
        {{ $t('catalog.belongsTo') }}：
        <EntityLink :kind="$t('entity.spu')" :id="productId" :to="entityRoutes.product(productId)" />
      </span>
      <span>
        {{ $t('catalog.currentSku') }}：
        <EntityLink :kind="$t('entity.sku')" :id="skuId" :to="entityRoutes.sku(productId, skuId)" />
      </span>
    </a-space>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import PageHeader from '@/components/PageHeader.vue'
import EntityLink from '@/components/entity/EntityLink.vue'
import { entityRoutes } from '@/navigation/entityRoutes'
import { routeParam } from '@/utils/routeParams'

const route = useRoute()
const productId = computed(() => routeParam(route, 'productId'))
const skuId = computed(() => routeParam(route, 'skuId'))
</script>

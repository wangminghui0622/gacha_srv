<template>
  <div>
    <PageHeader :title="$t('pool.detailTitle', { id: poolId, version })" />
    <a-spin :spinning="loading">
      <a-alert v-if="error" type="error" show-icon :message="error" class="notice" />
      <template v-if="pool">
        <a-descriptions bordered :column="2" size="small" class="notice">
          <a-descriptions-item :label="$t('pool.poolId')">{{ pool.poolId }}</a-descriptions-item>
          <a-descriptions-item :label="$t('pool.version')">{{ pool.version }}</a-descriptions-item>
          <a-descriptions-item :label="$t('common.status')">{{ pool.status }}</a-descriptions-item>
          <a-descriptions-item :label="$t('box.boxName')">{{ pool.boxName || $t('common.none') }}</a-descriptions-item>
          <a-descriptions-item :label="$t('box.price')">{{ pool.price }}</a-descriptions-item>
        </a-descriptions>
        <h3>{{ $t('pool.entriesTitle') }}</h3>
        <a-table :data-source="pool.entries ?? []" :pagination="false" :row-key="(row: PoolEntryView) => row.entryId" size="small" :scroll="{ x: 680 }">
          <a-table-column :title="$t('pool.entryId')" data-index="entryId" />
          <a-table-column :title="$t('pool.name')" data-index="name" />
          <a-table-column :title="$t('pool.weight')" data-index="weight" />
          <a-table-column :title="$t('pool.quantity')" data-index="quantity" />
          <a-table-column :title="$t('pool.isPrize')" data-index="isPrize" />
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
import { getConsolePoolVersion, type PoolVersionView, type PoolEntryView } from '@/api/console'
import { routeParam } from '@/utils/routeParams'

const route = useRoute()
const { t } = useI18n()
const poolId = computed(() => routeParam(route, 'poolId'))
const version = computed(() => (typeof route.query.version === 'string' ? route.query.version : '1'))
const pool = ref<PoolVersionView | null>(null)
const loading = ref(false)
const error = ref('')
watch(
  [poolId, version],
  async ([id, revision], _old, onCleanup) => {
    let active = true
    onCleanup(() => {
      active = false
    })
    pool.value = null
    error.value = ''
    loading.value = true
    try {
      const result = await getConsolePoolVersion(id, revision)
      if (active) pool.value = result
    } catch {
      if (active) error.value = t('pool.detailFailed')
    } finally {
      if (active) loading.value = false
    }
  },
  { immediate: true },
)
</script>

<style scoped>.notice { margin-bottom: 16px; }</style>

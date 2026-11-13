<template>
  <div class="prize-grade-page">
    <div class="toolbar">
      <a-button class="btn-query" type="primary" :loading="loading" @click="load">{{ $t('prizeGrade.query') }}</a-button>
    </div>

    <a-table
      :data-source="items"
      :loading="loading"
      :pagination="false"
      row-key="code"
      size="middle"
      :scroll="{ x: 900 }"
    >
      <a-table-column :title="$t('prizeGrade.sort')" data-index="sort" width="100" align="center" />
      <a-table-column :title="$t('prizeGrade.name')" data-index="code" align="center" />
      <a-table-column :title="$t('prizeGrade.category')" data-index="category" align="center" />
    </a-table>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { listPrizeGrades, type PrizeGrade } from '@/api/prizeGrade'

const items = ref<PrizeGrade[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const reply = await listPrizeGrades()
    items.value = reply.items ?? []
  } catch {
    items.value = []
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped lang="scss">
.prize-grade-page {
  min-height: calc(100vh - 132px);
  padding: 14px 16px 18px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}

.toolbar {
  display: flex;
  align-items: center;
  margin-bottom: 14px;
}

.btn-query {
  background: #42b983 !important;
  border-color: #42b983 !important;
}

.btn-query:hover {
  background: #36a374 !important;
  border-color: #36a374 !important;
}
</style>

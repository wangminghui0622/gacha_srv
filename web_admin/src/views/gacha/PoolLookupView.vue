<template>
  <section>
    <h2>{{ $t('pool.lookupTitle') }}</h2>
    <PoolEditor @changed="refresh" />
    <BoxManagement />
    <div v-if="canRead">
      <a-form :model="form" layout="inline" class="toolbar" @finish="openPool">
        <a-form-item :label="$t('pool.poolId')" name="poolId" :rules="[{ required: true, pattern: /^[1-9]\d*$/, message: $t('pool.validation.invalidPoolId') }]">
          <a-input v-model:value="form.poolId" :placeholder="$t('pool.poolId')" inputmode="numeric" />
        </a-form-item>
        <a-form-item :label="$t('pool.version')" name="version" :rules="[{ required: true, pattern: /^[1-9]\d*$/, message: $t('pool.validation.invalidVersion') }]">
          <a-input v-model:value="form.version" :placeholder="$t('pool.versionPlaceholder')" inputmode="numeric" />
        </a-form-item>
        <a-form-item><a-button html-type="submit">{{ $t('pool.queryById') }}</a-button></a-form-item>
      </a-form>
      <a-alert v-if="error" type="error" show-icon :message="$t('pool.listFailed')" class="toolbar" />
      <a-table :data-source="versions" :loading="loading" :pagination="false" :row-key="(row: PoolVersionSummaryView) => `${row.poolId}:${row.version}`" :scroll="{ x: 600 }" size="small">
        <a-table-column :title="$t('pool.poolId')" data-index="poolId" />
        <a-table-column :title="$t('pool.version')" data-index="version" />
        <a-table-column :title="$t('common.status')" data-index="status" />
        <a-table-column :title="$t('common.action')">
          <template #default="{ record }"><router-link :to="{ ...entityRoutes.pool(record.poolId), query: { version: record.version } }">{{ $t('common.view') }}</router-link></template>
        </a-table-column>
      </a-table>
      <a-space class="pager">
        <a-button :disabled="loading || cursors.length === 1" @click="previous">{{ $t('common.previousPage') }}</a-button>
        <span>{{ $t('common.pageN', { n: cursors.length }) }}</span>
        <a-button :disabled="loading || !nextCursor.poolId" @click="next">{{ $t('common.nextPage') }}</a-button>
        <a-button :loading="loading" @click="load">{{ $t('common.refresh') }}</a-button>
      </a-space>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import PoolEditor from './PoolEditor.vue'
import BoxManagement from './BoxManagement.vue'
import { allows } from '@/access/grants'
import { useSessionStore } from '@/stores/session'
import { useRouter } from 'vue-router'
import { entityRoutes } from '@/navigation/entityRoutes'
import { listConsolePoolVersions, type PoolVersionSummaryView } from '@/api/console'

type Cursor = { poolId: string; version: string }
const router = useRouter()
const session = useSessionStore()
const canRead = computed(() => allows(session.permissionNodes, 'gacha.pool', 'read'))
const form = reactive({ poolId: '', version: '1' })
const versions = ref<PoolVersionSummaryView[]>([])
const cursors = ref<Cursor[]>([{ poolId: '', version: '' }])
const nextCursor = ref<Cursor>({ poolId: '', version: '' })
const loading = ref(false)
const error = ref(false)

async function load() {
  if (!canRead.value) return
  loading.value = true
  error.value = false
  const cursor = cursors.value.at(-1)!
  try {
    const result = await listConsolePoolVersions(cursor.poolId, cursor.version)
    versions.value = result.versions ?? []
    nextCursor.value = {
      poolId: result.nextAfterPoolId === '0' ? '' : result.nextAfterPoolId ?? '',
      version: result.nextAfterVersion === '0' ? '' : result.nextAfterVersion ?? '',
    }
  } catch {
    error.value = true
    versions.value = []
    nextCursor.value = { poolId: '', version: '' }
  } finally {
    loading.value = false
  }
}
function next() {
  if (nextCursor.value.poolId) {
    cursors.value.push({ ...nextCursor.value })
    void load()
  }
}
function previous() {
  if (cursors.value.length > 1) {
    cursors.value.pop()
    void load()
  }
}
function openPool() {
  router.push({ ...entityRoutes.pool(form.poolId.trim()), query: { version: form.version.trim() } })
}
function refresh() {
  cursors.value = [{ poolId: '', version: '' }]
  void load()
}
onMounted(() => {
  void load()
})
</script>

<style scoped>.toolbar { margin-bottom: 16px; }.pager { margin-top: 16px; }</style>

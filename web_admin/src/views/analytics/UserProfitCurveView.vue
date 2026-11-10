<template>
  <div>
    <PageHeader :title="$t('userProfit.title', { id: userId })">
      <template #extra>
        <EntityLink :kind="$t('entity.user')" :id="userId" :to="entityRoutes.userOverview(userId)" :label="$t('userProfit.overviewLink')" />
      </template>
    </PageHeader>
    <a-form layout="inline" :model="query" @finish="load">
      <a-form-item :label="$t('userProfit.grain')">
        <a-radio-group v-model:value="query.grain" button-style="solid">
          <a-radio-button value="hour">{{ $t('userProfit.grains.hour') }}</a-radio-button>
          <a-radio-button value="day">{{ $t('userProfit.grains.day') }}</a-radio-button>
          <a-radio-button value="month">{{ $t('userProfit.grains.month') }}</a-radio-button>
        </a-radio-group>
      </a-form-item>
      <a-form-item :label="$t('userProfit.timeRange')">
        <a-range-picker v-model:value="query.range" show-time />
      </a-form-item>
      <a-form-item>
        <a-button type="primary" html-type="submit" :loading="loading">{{ $t('common.query') }}</a-button>
      </a-form-item>
    </a-form>
    <a-alert
      v-if="!dataReady"
      type="info"
      show-icon
      style="margin: 16px 0"
      :message="$t('userProfit.notReady')"
      :description="$t('userProfit.notReadyHint')"
    />
    <ProfitCurveChart :points="points" :loading="loading" />
  </div>
</template>

<script setup lang="ts">
import dayjs, { type Dayjs } from 'dayjs'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { fetchUserProfitCurve, type UserProfitCurvePoint } from '@/api/analytics'
import PageHeader from '@/components/PageHeader.vue'
import ProfitCurveChart from '@/components/charts/ProfitCurveChart.vue'
import EntityLink from '@/components/entity/EntityLink.vue'
import { entityRoutes, type TimeGrainKey } from '@/navigation/entityRoutes'
import { useSessionStore } from '@/stores/session'
import { routeParam } from '@/utils/routeParams'

const route = useRoute()
const session = useSessionStore()
const userId = computed(() => Number(routeParam(route, 'userId')))

const query = reactive<{ grain: TimeGrainKey; range: [Dayjs, Dayjs] | null }>({
  grain: (route.query.grain as TimeGrainKey) || 'day',
  range: [dayjs().subtract(7, 'day'), dayjs()],
})

const loading = ref(false)
const points = ref<UserProfitCurvePoint[]>([])
const dataReady = ref(false)

async function load() {
  if (!query.range) {
    return
  }
  loading.value = true
  try {
    const reply = await fetchUserProfitCurve({
      userId: userId.value,
      tenantId: session.tenantId,
      grain: query.grain,
      from: query.range[0].toISOString(),
      to: query.range[1].toISOString(),
    })
    points.value = reply.points ?? []
    dataReady.value = reply.dataReady === true
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void load()
})
</script>

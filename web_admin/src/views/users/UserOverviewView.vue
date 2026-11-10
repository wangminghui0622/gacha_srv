<template>
  <div>
    <PageHeader :title="$t('userOverview.title', { id: userId })" />
    <a-descriptions bordered :column="1" size="small">
      <a-descriptions-item :label="$t('userOverview.userId')">
        <EntityLink :kind="$t('entity.user')" :id="userId" />
      </a-descriptions-item>
      <a-descriptions-item :label="$t('userOverview.shortcuts')">
        <a-space wrap>
          <router-link :to="entityRoutes.userProfit(userId)">{{ $t('userOverview.profitCurve') }}</router-link>
          <router-link :to="{ name: 'task-progress', query: { userId } }">{{ $t('userOverview.taskProgress') }}</router-link>
        </a-space>
      </a-descriptions-item>
    </a-descriptions>
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
const userId = computed(() => routeParam(route, 'userId'))
</script>

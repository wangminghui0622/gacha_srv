<template>
  <div>
    <PageHeader :title="$t('box.boxTitle', { id: boxId })" />
    <BoxManagement :initial-box-id="boxId" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import PageHeader from '@/components/PageHeader.vue'
import BoxManagement from './BoxManagement.vue'
import { routeParam } from '@/utils/routeParams'

const route = useRoute()
const boxId = computed(() => routeParam(route, 'boxId'))
</script>

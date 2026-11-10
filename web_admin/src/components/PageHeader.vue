<template>
  <div class="page-header">
    <a-space align="center">
      <a-button type="text" @click="goBack">
        <template #icon><ArrowLeftOutlined /></template>
        {{ $t('common.back') }}
      </a-button>
      <h2 class="page-title">{{ title }}</h2>
    </a-space>
    <div v-if="$slots.extra" class="page-extra">
      <slot name="extra" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ArrowLeftOutlined } from '@ant-design/icons-vue'
import { usePageBack } from '@/composables/usePageBack'

defineProps<{ title: string }>()
const { goBack } = usePageBack()
</script>

<style scoped lang="scss">
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.page-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}
</style>

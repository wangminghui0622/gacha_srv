<template>
  <div>
    <PageHeader :title="title" :show-back="false" />
    <a-card>
      <a-empty :description="$t('common.modulePending')">
        <template #image><InboxOutlined class="empty-icon" /></template>
      </a-empty>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { InboxOutlined } from '@ant-design/icons-vue'
import PageHeader from '@/components/PageHeader.vue'

const route = useRoute()
const { t } = useI18n()
const title = computed(() => t((route.meta.title as string) ?? 'nav.dashboard'))
</script>

<style scoped lang="scss">
.empty-icon {
  font-size: 48px;
  color: #bfbfbf;
}
</style>

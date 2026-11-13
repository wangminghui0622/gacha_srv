<template>
  <div>
    <PageHeader :title="$t('nav.gachaPools')" />
    <div class="create-page">
      <PlayPoolCreateForm @success="onSuccess" @cancel="router.push('/gacha')" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import PageHeader from '@/components/PageHeader.vue'
import PlayPoolCreateForm from '@/views/gacha/PlayPoolCreateForm.vue'

const router = useRouter()

function onSuccess() {
  router.push('/gacha')
}
</script>

<style scoped lang="scss">
.create-page {
  padding: 20px 24px 32px;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
}
</style>

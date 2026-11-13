<template>
  <div>
    <PageHeader :title="$t('nav.customerUsers')" :show-back="false" />
    <a-row :gutter="16">
      <a-col :xs="24" :md="14">
        <a-card :title="$t('customers.searchTitle')">
          <a-form layout="vertical" @submit.prevent="search">
            <a-form-item :label="$t('customers.customerId')">
              <a-input
                v-model:value="customerId"
                :placeholder="$t('customers.customerIdPlaceholder')"
                allow-clear
                @press-enter="search"
              />
            </a-form-item>
            <a-space wrap>
              <a-button type="primary" @click="search">{{ $t('customers.openOverview') }}</a-button>
              <a-button @click="openProfit">{{ $t('customers.openProfit') }}</a-button>
            </a-space>
          </a-form>
        </a-card>
      </a-col>
      <a-col :xs="24" :md="10">
        <a-card :title="$t('customers.quickTitle')">
          <p class="hint">{{ $t('customers.quickHint') }}</p>
          <a-space direction="vertical" style="width: 100%">
            <a-button block @click="router.push('/catalog')">{{ $t('nav.catalog') }}</a-button>
            <a-button block @click="router.push('/gacha')">{{ $t('nav.gacha') }}</a-button>
          </a-space>
        </a-card>
      </a-col>
    </a-row>
    <a-card :title="$t('customers.aboutTitle')" style="margin-top: 16px">
      <p class="hint">{{ $t('customers.aboutHint') }}</p>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import PageHeader from '@/components/PageHeader.vue'

const router = useRouter()
const { t } = useI18n()
const customerId = ref('')

function normalizedId(): string | null {
  const id = customerId.value.trim()
  if (!/^\d+$/.test(id) || Number(id) <= 0) {
    message.warning(t('customers.invalidId'))
    return null
  }
  return id
}

function search() {
  const id = normalizedId()
  if (id) router.push(`/users/${id}`)
}

function openProfit() {
  const id = normalizedId()
  if (id) router.push(`/analytics/users/${id}/profit`)
}
</script>

<style scoped lang="scss">
.hint {
  color: rgba(0, 0, 0, 0.55);
  margin: 0 0 12px;
}
</style>

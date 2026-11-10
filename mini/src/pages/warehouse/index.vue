<script setup lang="ts">
import { ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as drawApi from '@/services/draw'
import { formatUnix } from '@/utils/format'

import type { WarehouseItem } from '@/services/types'

// 吉仓（中奖物品列表）

const { t } = useI18n()

const loading = ref(true)
const items = ref<WarehouseItem[]>([])
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    items.value = (await drawApi.listWarehouse()).list
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    loading.value = false
  }
}

onLoad(() => {
  void load()
})
</script>

<template>
  <view>
    <view class="page-title">{{ t('warehouse.title') }}</view>

    <view v-if="loading" class="list-empty">{{ t('common.loading') }}</view>
    <view v-else-if="error" class="error-text">{{ error }}</view>
    <view v-else-if="items.length === 0" class="list-empty">{{ t('warehouse.empty') }}</view>

    <view v-if="items.length" class="wh-list">
      <view v-for="item in items" :key="item.itemId" class="gacha-card wh-item">
        <view class="wh-emoji">🏆</view>
        <view class="wh-info">
          <view class="wh-name">{{ item.itemName }}</view>
          <view class="wh-line">{{ t('warehouse.from') }}：{{ item.boxName }}</view>
          <view class="wh-line">{{ t('warehouse.time') }}：{{ formatUnix(item.createdAt) }}</view>
        </view>
      </view>
    </view>
  </view>
</template>

<style scoped lang="scss">
.list-empty {
  text-align: center;
  color: var(--color-text-secondary);
  padding: 48rpx 0;
}

.wh-item {
  display: flex;
  align-items: center;
  gap: 24rpx;

  .wh-emoji {
    font-size: 64rpx;
  }

  .wh-info {
    flex: 1;

    .wh-name {
      font-size: 30rpx;
      font-weight: 600;
      margin-bottom: 8rpx;
    }

    .wh-line {
      font-size: 24rpx;
      color: var(--color-text-secondary);
      line-height: 36rpx;
    }
  }
}
</style>

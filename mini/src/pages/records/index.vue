<script setup lang="ts">
import { ref } from 'vue'
import { onLoad, onReachBottom } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as drawApi from '@/services/draw'
import { formatMoney, formatUnix } from '@/utils/format'

import type { DrawRecordItem } from '@/services/types'

// 抽赏记录：分页 + 上拉加载（规范 07 第 12 条：禁止一次性渲染全量数据）

const { t } = useI18n()

const loading = ref(true)
const loadingMore = ref(false)
const records = ref<DrawRecordItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const error = ref('')

async function load(reset: boolean) {
  if (reset) {
    loading.value = true
    error.value = ''
  } else {
    loadingMore.value = true
  }
  try {
    const res = await drawApi.listRecords(page.value, pageSize)
    records.value = reset ? res.list : [...records.value, ...res.list]
    total.value = res.total
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

onLoad(() => {
  void load(true)
})

// 上拉加载下一页（已拉完则不再请求）
onReachBottom(() => {
  if (loading.value || loadingMore.value || records.value.length >= total.value) return
  page.value += 1
  void load(false)
})
</script>

<template>
  <view>
    <view class="page-title">{{ t('records.title') }}</view>

    <view v-if="loading" class="list-empty">{{ t('common.loading') }}</view>
    <view v-else-if="error" class="error-text">{{ error }}</view>
    <view v-else-if="records.length === 0" class="list-empty">{{ t('records.empty') }}</view>

    <view v-if="records.length" class="record-list">
      <view v-for="item in records" :key="item.requestId" class="gacha-card record-item">
        <view class="record-head">
          <text class="record-box">{{ item.boxName }}</text>
          <text class="record-tag" :class="{ prize: item.isPrize }">
            {{ item.isPrize ? item.entryName : t('records.thanks') }}
          </text>
        </view>
        <view class="record-line">{{ t('records.time') }}：{{ formatUnix(item.createdAt) }}</view>
        <view class="record-line">{{ t('box.price') }}：{{ formatMoney(item.price) }}</view>
      </view>
    </view>

    <view v-if="loadingMore" class="list-empty">{{ t('common.loading') }}</view>
    <view v-else-if="records.length >= total && total > 0" class="list-empty">{{ t('common.empty') }}</view>
  </view>
</template>

<style scoped lang="scss">
.list-empty {
  text-align: center;
  color: var(--color-text-secondary);
  padding: 48rpx 0;
  font-size: 26rpx;
}

.record-item {
  .record-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 12rpx;

    .record-box {
      font-size: 30rpx;
      font-weight: 600;
    }

    .record-tag {
      font-size: 24rpx;
      color: var(--color-text-secondary);
      border: 1rpx solid var(--color-border);
      border-radius: 6rpx;
      padding: 2rpx 12rpx;

      &.prize {
        color: #d46b08;
        border-color: #ffd591;
        background: #fff7e6;
      }
    }
  }

  .record-line {
    font-size: 24rpx;
    color: var(--color-text-secondary);
    line-height: 36rpx;
  }
}
</style>

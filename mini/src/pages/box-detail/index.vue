<script setup lang="ts">
import { ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as drawApi from '@/services/draw'
import { formatMoney } from '@/utils/format'

import type { BoxDetailResponse } from '@/services/types'

// 宝箱详情 + 概率公示（列表化，替代 web 的 antd 表格）

const { t } = useI18n()

const boxId = ref(0)
const loading = ref(true)
const detail = ref<BoxDetailResponse | null>(null)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    detail.value = await drawApi.boxDetail(boxId.value)
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    loading.value = false
  }
}

onLoad((query) => {
  boxId.value = Number(query?.boxId ?? 0)
  void load()
})

function goBuy() {
  if (!detail.value || detail.value.soldOut) {
    uni.showToast({ title: t('box.soldOut'), icon: 'none' })
    return
  }
  uni.navigateTo({ url: `/pages/buy/index?boxId=${boxId.value}` })
}
</script>

<template>
  <view>
    <view v-if="loading" class="list-empty">{{ t('common.loading') }}</view>
    <view v-else-if="error" class="error-text">{{ error }}</view>
    <view v-else-if="!detail" class="list-empty">{{ t('common.empty') }}</view>

    <template v-if="detail">
      <view class="gacha-card detail-card">
        <view class="detail-head">
          <text class="detail-name">{{ detail.boxName }}</text>
          <text v-if="detail.soldOut" class="soldout-tag">{{ t('box.soldOut') }}</text>
        </view>
        <view class="detail-desc">{{ detail.description }}</view>
        <view class="detail-price">
          {{ t('box.price') }}：<text class="money">{{ formatMoney(detail.price) }}</text>
        </view>
        <view
          class="btn-primary"
          :class="{ disabled: detail.soldOut }"
          @click="goBuy"
        >
          {{ detail.soldOut ? t('box.soldOut') : t('box.buy') }}
        </view>
      </view>

      <view class="gacha-card prob-card">
        <view class="prob-title">{{ t('box.probability') }}</view>
        <view class="prob-notice">{{ t('box.notice') }}</view>
        <view class="entry-row entry-head">
          <text class="col-name">{{ t('box.entryName') }}</text>
          <text class="col-prob">{{ t('box.probabilityCol') }}</text>
          <text class="col-remain">{{ t('box.remaining') }}</text>
        </view>
        <view v-for="entry in detail.entries" :key="entry.entryId" class="entry-row">
          <text class="col-name" :class="{ 'prize-name': entry.isPrize }">{{ entry.name }}</text>
          <text class="col-prob">{{ entry.probability }}</text>
          <text v-if="entry.remaining === -1" class="col-remain">{{ t('box.unlimited') }}</text>
          <text v-else class="col-remain" :class="{ 'zero-stock': entry.remaining === 0 }">
            {{ entry.remaining === 0 ? t('box.soldOut') : entry.remaining }}
          </text>
        </view>
      </view>
    </template>
  </view>
</template>

<style scoped lang="scss">
.list-empty {
  text-align: center;
  color: var(--color-text-secondary);
  padding: 48rpx 0;
}

.detail-card {
  .detail-head {
    display: flex;
    align-items: center;
    gap: 16rpx;

    .detail-name {
      font-size: 40rpx;
      font-weight: 700;
    }

    .soldout-tag {
      font-size: 22rpx;
      color: var(--color-text-secondary);
      border: 1rpx solid var(--color-border);
      border-radius: 6rpx;
      padding: 0 8rpx;
    }
  }

  .detail-desc {
    font-size: 26rpx;
    color: var(--color-text-secondary);
    margin: 12rpx 0;
  }

  .detail-price {
    font-size: 30rpx;
    margin-bottom: 24rpx;
  }
}

.prob-card {
  .prob-title {
    font-size: 32rpx;
    font-weight: 600;
    margin-bottom: 8rpx;
  }

  .prob-notice {
    font-size: 24rpx;
    color: var(--color-text-secondary);
    margin-bottom: 20rpx;
  }

  .entry-row {
    display: flex;
    font-size: 26rpx;
    padding: 14rpx 0;
    border-bottom: 1rpx solid var(--color-border);

    &:last-child {
      border-bottom: none;
    }

    .col-name {
      flex: 1;
      min-width: 0;
    }

    .col-prob {
      width: 140rpx;
      text-align: right;
    }

    .col-remain {
      width: 120rpx;
      text-align: right;
      color: var(--color-text-secondary);
    }
  }

  .entry-head {
    color: var(--color-text-secondary);
    font-size: 24rpx;
  }

  .prize-name {
    font-weight: 600;
  }

  .zero-stock {
    color: var(--color-text-secondary);
  }
}
</style>

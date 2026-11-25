<script setup lang="ts">
import { computed, ref } from 'vue'
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

const cover = computed(() => detail.value?.coverImage || detail.value?.coverThumb || '')

const gradeGroups = computed(() => {
  const map = new Map<string, { grade: string; probability: number; count: number }>()
  for (const entry of detail.value?.entries ?? []) {
    if (!entry.isPrize) continue
    const grade = entry.prizeGrade?.trim() || '奖品'
    const p = Number.parseFloat(entry.probability.replace('%', '')) || 0
    const item = map.get(grade) ?? { grade, probability: 0, count: 0 }
    item.probability += p
    item.count += 1
    map.set(grade, item)
  }
  return [...map.values()].sort((a, b) => b.probability - a.probability)
})

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
        <image v-if="cover" class="detail-cover" :src="cover" mode="aspectFill" />
        <view class="detail-head">
          <text class="detail-name">{{ detail.boxName }}</text>
          <text v-if="detail.soldOut" class="soldout-tag">{{ t('box.soldOut') }}</text>
        </view>
        <view class="detail-desc">{{ detail.description }}</view>
        <view class="detail-meta">
          <text v-if="Number(detail.periods ?? 0) > 0">第 {{ detail.periods }} 期</text>
          <text v-if="Number(detail.discount ?? 0) > 0">{{ detail.discount }}% 折扣</text>
        </view>
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

      <view v-if="gradeGroups.length" class="gacha-card grade-card">
        <view class="prob-title">分类概率</view>
        <view class="grade-list">
          <view v-for="group in gradeGroups" :key="group.grade" class="grade-item">
            <text class="grade-name">{{ group.grade }}</text>
            <text class="grade-prob">{{ group.probability.toFixed(2) }}%</text>
          </view>
        </view>
      </view>

      <view class="gacha-card prob-card">
        <view class="prob-title">奖品概览</view>
        <view class="prob-notice">{{ t('box.notice') }}</view>
        <view v-for="entry in detail.entries" :key="entry.entryId" class="entry-card">
          <image v-if="entry.imageUrl" class="entry-image" :src="entry.imageUrl" mode="aspectFill" />
          <view v-else class="entry-image entry-image-empty">吉</view>
          <view class="entry-body">
            <view class="entry-top">
              <text v-if="entry.prizeGrade" class="entry-grade">{{ entry.prizeGrade }}</text>
              <text class="entry-name" :class="{ 'prize-name': entry.isPrize }">{{ entry.name }}</text>
            </view>
            <view class="entry-bottom">
              <text class="entry-prob">{{ entry.probability }}</text>
              <text v-if="entry.remaining === -1" class="entry-remain">{{ t('box.unlimited') }}</text>
              <text v-else class="entry-remain" :class="{ 'zero-stock': entry.remaining === 0 }">
                {{ entry.remaining === 0 ? t('box.soldOut') : `剩余 ${entry.remaining}` }}
              </text>
            </view>
          </view>
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
  .detail-cover {
    width: 100%;
    height: 360rpx;
    border-radius: 16rpx;
    margin-bottom: 20rpx;
    background: #f2f3f5;
  }

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

  .detail-meta {
    display: flex;
    gap: 20rpx;
    font-size: 24rpx;
    color: var(--color-text-secondary);
    margin-bottom: 12rpx;
  }

  .detail-price {
    font-size: 30rpx;
    margin-bottom: 24rpx;
  }
}

.grade-card {
  .grade-list {
    display: flex;
    flex-wrap: wrap;
    gap: 16rpx;
    margin-top: 16rpx;
  }

  .grade-item {
    display: flex;
    align-items: center;
    gap: 12rpx;
    padding: 12rpx 20rpx;
    border-radius: 999rpx;
    background: #f7f8fa;
    border: 1rpx solid var(--color-border);
  }

  .grade-name {
    font-size: 26rpx;
    font-weight: 600;
  }

  .grade-prob {
    font-size: 24rpx;
    color: #ff5a3c;
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

  .entry-card {
    display: flex;
    gap: 20rpx;
    padding: 18rpx 0;
    border-bottom: 1rpx solid var(--color-border);

    &:last-child {
      border-bottom: none;
    }
  }

  .entry-image {
    width: 120rpx;
    height: 120rpx;
    border-radius: 12rpx;
    background: #f2f3f5;
    flex-shrink: 0;
  }

  .entry-image-empty {
    display: flex;
    align-items: center;
    justify-content: center;
    color: #c0c4cc;
    font-size: 36rpx;
  }

  .entry-body {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
  }

  .entry-top {
    display: flex;
    align-items: center;
    gap: 12rpx;
  }

  .entry-grade {
    flex-shrink: 0;
    padding: 2rpx 10rpx;
    font-size: 22rpx;
    color: #ff5a3c;
    border: 1rpx solid #ff5a3c;
    border-radius: 6rpx;
  }

  .entry-name {
    font-size: 28rpx;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .prize-name {
    font-weight: 600;
  }

  .entry-bottom {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 24rpx;
    color: var(--color-text-secondary);
  }

  .entry-prob {
    color: #ff5a3c;
    font-weight: 600;
  }

  .zero-stock {
    color: var(--color-text-secondary);
  }
}
</style>

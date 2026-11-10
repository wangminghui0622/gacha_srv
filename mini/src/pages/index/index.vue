<script setup lang="ts">
import { computed, ref } from 'vue'
import { onShow, onPullDownRefresh } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import RollingMarquee from '@/components/RollingMarquee.vue'
import * as drawApi from '@/services/draw'
import * as userApi from '@/services/user'
import { useAuthStore } from '@/store/authStore'
import { formatMoney } from '@/utils/format'

import type { BoxItem } from '@/services/types'

// 首页（tab）：滚动中奖 marquee + 宝箱列表 + 玩法宫格（设计 M16 D4）
// 进入即拉用户资料（web MainLayout onMounted 同款语义）

const { t } = useI18n()
const auth = useAuthStore()

const loading = ref(true)
const boxes = ref<BoxItem[]>([])
const error = ref('')

// 玩法宫格：M16b 落地全部玩法页
const playEntries = computed(() => [
  { key: 'campaigns', icon: '🏆', label: t('nav.campaigns') },
  { key: 'digBox', icon: '⛏️', label: t('nav.digBox') },
  { key: 'fullCar', icon: '🚗', label: t('nav.fullCar') },
  { key: 'gashapon', icon: '🥚', label: t('nav.gashapon') },
  { key: 'comboChain', icon: '⚡', label: t('nav.comboChain') },
  { key: 'groupBuy', icon: '👥', label: t('nav.groupBuy') },
  { key: 'memoryMatch', icon: '🃏', label: t('nav.memoryMatch') },
  { key: 'tasks', icon: '📋', label: t('nav.tasks') },
])

// 玩法宫格 → 页面路由
const playRoutes: Record<string, string> = {
  campaigns: '/pages/campaigns/index',
  digBox: '/pages/dig/index',
  fullCar: '/pages/full-car/index',
  gashapon: '/pages/gashapon/index',
  comboChain: '/pages/combo-chain/index',
  groupBuy: '/pages/group-buy/index',
  memoryMatch: '/pages/memory-match/index',
  tasks: '/pages/tasks/index',
}

function onPlayTap(key: string) {
  const url = playRoutes[key]
  if (url) {
    uni.navigateTo({ url })
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    boxes.value = (await drawApi.listBoxes()).list
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    loading.value = false
  }
}

// 进入布局即拉取用户资料（余额同步到 store；401 由适配器统一处理）
async function refreshProfile() {
  if (!auth.user) {
    try {
      auth.setUser(await userApi.getProfile())
    } catch {
      // 401 已由响应适配器统一处理（清令牌跳登录）
    }
  }
}

onShow(() => {
  void refreshProfile()
  void load()
})

onPullDownRefresh(async () => {
  await Promise.all([refreshProfile(), load()])
  uni.stopPullDownRefresh()
})

function goDetail(box: BoxItem) {
  uni.navigateTo({ url: `/pages/box-detail/index?boxId=${box.boxId}` })
}
</script>

<template>
  <view>
    <view class="page-title">{{ t('box.title') }}</view>
    <view class="page-subtitle">{{ t('box.subtitle') }}</view>

    <RollingMarquee />

    <view v-if="loading" class="list-empty">{{ t('common.loading') }}</view>
    <view v-else-if="error" class="error-text">{{ error }}</view>
    <view v-else-if="boxes.length === 0" class="list-empty">{{ t('common.empty') }}</view>

    <view v-if="boxes.length" class="box-list">
      <view v-for="box in boxes" :key="box.boxId" class="gacha-card box-card" @click="goDetail(box)">
        <view class="box-emoji">{{ box.boxId === 1 ? '📦' : box.boxId === 2 ? '🏖️' : '👑' }}</view>
        <view class="box-info">
          <view class="box-name-row">
            <text class="box-name">{{ box.boxName }}</text>
            <text v-if="box.soldOut" class="soldout-tag">{{ t('box.soldOut') }}</text>
          </view>
          <view class="box-desc">{{ box.description }}</view>
          <view class="box-footer">
            <text class="money">{{ formatMoney(box.price) }}</text>
            <text class="detail-link">{{ t('box.detail') }} ›</text>
          </view>
        </view>
      </view>
    </view>

    <!-- 玩法宫格（M16b 全玩法入口） -->
    <view class="gacha-card play-grid-card">
      <view class="play-grid">
        <view
          v-for="entry in playEntries"
          :key="entry.key"
          class="play-item"
          @click="onPlayTap(entry.key)"
        >
          <view class="play-icon">{{ entry.icon }}</view>
          <view class="play-label">{{ entry.label }}</view>
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

.box-list {
  display: flex;
  flex-direction: column;
}

.box-card {
  display: flex;
  gap: 20rpx;

  .box-emoji {
    font-size: 72rpx;
    display: flex;
    align-items: center;
  }

  .box-info {
    flex: 1;
    min-width: 0;

    .box-name-row {
      display: flex;
      align-items: center;
      gap: 12rpx;
    }

    .box-name {
      font-size: 32rpx;
      font-weight: 600;
    }

    .soldout-tag {
      font-size: 22rpx;
      color: var(--color-text-secondary);
      border: 1rpx solid var(--color-border);
      border-radius: 6rpx;
      padding: 0 8rpx;
    }

    .box-desc {
      font-size: 24rpx;
      color: var(--color-text-secondary);
      margin: 8rpx 0 16rpx;
    }

    .box-footer {
      display: flex;
      align-items: center;
      justify-content: space-between;
    }

    .detail-link {
      font-size: 24rpx;
      color: var(--color-primary);
    }
  }
}

.play-grid-card {
  padding: 24rpx 12rpx;
}

.play-grid {
  display: flex;
  flex-wrap: wrap;

  .play-item {
    width: 25%;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8rpx;
    padding: 16rpx 0;

    .play-icon {
      font-size: 56rpx;
    }

    .play-label {
      font-size: 24rpx;
      color: var(--color-text-secondary);
    }
  }
}
</style>

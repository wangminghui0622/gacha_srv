<script setup lang="ts">
import { ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { useI18n } from 'vue-i18n'

import * as couponApi from '@/services/coupon'
import { couponValueText } from '@/utils/coupon'
import type { CouponItem, CouponTemplateItem } from '@/services/types'

// 优惠券中心：可领取模板 + 我的券（web Coupons 页同流程）
const { t } = useI18n()

const loading = ref(true)
const error = ref('')
const templates = ref<CouponTemplateItem[]>([])
const coupons = ref<CouponItem[]>([])
const claiming = ref('') // 正在领取的 templateNo

// 券状态 → 标签（1 AVAILABLE 2 LOCKED 3 USED 4 EXPIRED）
const statusMap: Record<number, { text: string; color: string }> = {
  1: { text: 'coupon.statusAvailable', color: 'green' },
  2: { text: 'coupon.statusLocked', color: 'orange' },
  3: { text: 'coupon.statusUsed', color: 'default' },
  4: { text: 'coupon.statusExpired', color: 'red' },
}

// 面值文案（kind 1 FIXED_MINOR=分 / 2 PERCENT_BPS=万分比 / 3 FREE）
function valueText(kind: number, value: number): string {
  return couponValueText(kind, value, t)
}

function minGrossText(minGross: string): string {
  return Number(minGross) > 0 ? t('coupon.minGross', { min: minGross }) : ''
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [tpl, mine] = await Promise.all([couponApi.listTemplates(), couponApi.listUserCoupons()])
    templates.value = tpl.list
    coupons.value = mine.list
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    loading.value = false
  }
}

async function claim(templateNo: string) {
  claiming.value = templateNo
  try {
    await couponApi.claimCoupon(templateNo)
    uni.showToast({ title: t('coupon.claimSuccess'), icon: 'success' })
    await load()
  } catch (err) {
    uni.showToast({ title: (err as Error).message, icon: 'none' })
  } finally {
    claiming.value = ''
  }
}

// 去使用 → 宝箱商城（首页 tab）
function goBuy() {
  uni.switchTab({ url: '/pages/index/index' })
}

onLoad(() => {
  void load()
})
</script>

<template>
  <view>
    <view class="page-title">{{ t('coupon.title') }}</view>
    <view v-if="loading" class="list-empty">{{ t('common.loading') }}</view>
    <view v-else-if="error" class="error-text">{{ error }}</view>
    <template v-else>
      <!-- 可领取券模板 -->
      <view class="section-title section-title-page">{{ t('coupon.templates') }}</view>
      <view v-if="templates.length === 0" class="list-empty">{{ t('coupon.empty') }}</view>
      <view v-for="tpl in templates" :key="tpl.id" class="gacha-card coupon-card">
        <view class="coupon-main">
          <view class="coupon-name">{{ tpl.name }}</view>
          <view class="coupon-value accent">{{ valueText(tpl.kind, tpl.value) }}</view>
          <view v-if="minGrossText(tpl.minGross)" class="coupon-sub">{{ minGrossText(tpl.minGross) }}</view>
        </view>
        <view
          class="btn-primary coupon-btn"
          :class="{ disabled: tpl.claimed || claiming === tpl.templateNo }"
          @click="claim(tpl.templateNo)"
        >
          {{ tpl.claimed ? t('coupon.claimed') : t('coupon.claim') }}
        </view>
      </view>

      <!-- 我的券 -->
      <view class="section-title section-title-page">{{ t('coupon.mine') }}</view>
      <view v-if="coupons.length === 0" class="list-empty">{{ t('coupon.mineEmpty') }}</view>
      <view v-for="c in coupons" :key="c.id" class="gacha-card coupon-card">
        <view class="coupon-main">
          <view class="coupon-name">
            <text>{{ c.name }}</text>
            <text
              class="tag-chip"
              :class="{
                'tag-green': statusMap[c.status]?.color === 'green',
                'tag-orange': statusMap[c.status]?.color === 'orange',
                'tag-red': statusMap[c.status]?.color === 'red',
              }"
            >
              {{ t(statusMap[c.status]?.text ?? 'coupon.statusAvailable') }}
            </text>
          </view>
          <view class="coupon-value accent">{{ valueText(c.kind, c.value) }}</view>
          <view v-if="minGrossText(c.minGross)" class="coupon-sub">{{ minGrossText(c.minGross) }}</view>
          <view v-if="c.expiresAt" class="coupon-sub">{{ t('coupon.expiresAt') }}：{{ c.expiresAt }}</view>
        </view>
        <view v-if="c.usable" class="btn-primary coupon-btn" @click="goBuy">
          {{ t('coupon.goBuy') }}
        </view>
      </view>
    </template>
  </view>
</template>

<style scoped lang="scss">
.section-title-page {
  margin-left: 24rpx;
}

.error-text {
  color: var(--color-danger);
  text-align: center;
}

.coupon-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24rpx;
  margin-bottom: 16rpx;

  .coupon-main {
    flex: 1;
    min-width: 0;

    .coupon-name {
      font-size: 30rpx;
      font-weight: 600;
      display: flex;
      align-items: center;

      .tag-chip {
        margin-left: 12rpx;
      }
    }

    .coupon-value {
      font-size: 36rpx;
      font-weight: 700;
      margin-top: 4rpx;
    }

    .coupon-sub {
      font-size: 24rpx;
      color: var(--color-text-secondary);
      margin-top: 4rpx;
    }
  }

  .coupon-btn {
    line-height: 68rpx;
    padding: 0 32rpx;
    font-size: 26rpx;
    flex-shrink: 0;
  }
}
</style>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

type TabKey = 'home' | 'warehouse' | 'mine'

const props = defineProps<{
  active: TabKey
}>()

const { t } = useI18n()

const tabs = computed(() => [
  { key: 'home' as const, icon: 'home', activeIcon: 'home-filled', label: t('nav.home'), route: '/pages/index/index' },
  { key: 'warehouse' as const, icon: 'shop', activeIcon: 'shop-filled', label: t('nav.warehouse'), route: '/pages/warehouse/index' },
  { key: 'mine' as const, icon: 'person', activeIcon: 'person-filled', label: t('nav.mine'), route: '/pages/mine/index' },
])

function switchTo(tab: (typeof tabs.value)[number]) {
  if (tab.key === props.active) return
  uni.switchTab({
    url: tab.route,
    fail: () => {
      uni.reLaunch({ url: tab.route })
    },
  })
}
</script>

<template>
  <view class="bottom-tabbar">
    <view
      v-for="tab in tabs"
      :key="tab.key"
      class="tab-item"
      :class="{ active: tab.key === active, warehouse: tab.key === 'warehouse' }"
      @click="switchTo(tab)"
    >
      <view class="tab-icon-wrap">
        <uni-icons
          :type="tab.key === active ? tab.activeIcon : tab.icon"
          :size="23"
          :color="tab.key === active && tab.key === 'warehouse' ? '#0b0f0c' : tab.key === active ? '#b6ff3b' : '#8b918b'"
        />
      </view>
      <text class="tab-label">{{ tab.label }}</text>
    </view>
  </view>
</template>

<style scoped lang="scss">
.bottom-tabbar {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 80;
  display: flex;
  align-items: center;
  height: 112rpx;
  height: calc(112rpx + constant(safe-area-inset-bottom));
  height: calc(112rpx + env(safe-area-inset-bottom));
  padding: 0 40rpx;
  padding-bottom: constant(safe-area-inset-bottom);
  padding-bottom: env(safe-area-inset-bottom);
  background: rgba(9, 13, 10, 0.97);
  border-top: 1rpx solid rgba(255, 255, 255, 0.06);
  box-sizing: border-box;
  backdrop-filter: blur(12px);
}

.tab-item {
  flex: 1;
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4rpx;
}

.tab-icon-wrap {
  width: 66rpx;
  height: 52rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 18rpx;
  transition: background 0.2s ease;
}

.tab-label {
  font-size: 22rpx;
  line-height: 1.2;
  color: #8b918b;
}

.tab-item.active .tab-label {
  color: #b6ff3b;
  font-weight: 600;
}

.tab-item.active.warehouse .tab-icon-wrap {
  background: #b6ff3b;
  box-shadow: 0 0 20rpx rgba(182, 255, 59, 0.35);
}
</style>

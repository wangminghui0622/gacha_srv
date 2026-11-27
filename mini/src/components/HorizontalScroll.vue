<template>
  <!-- #ifdef MP-WEIXIN -->
  <scroll-view scroll-x class="horizontal-scroll" :show-scrollbar="false">
    <slot />
  </scroll-view>
  <!-- #endif -->
  <!-- #ifndef MP-WEIXIN -->
  <view class="horizontal-scroll">
    <slot />
  </view>
  <!-- #endif -->
</template>

<style scoped>
.horizontal-scroll {
  width: 100%;
  overflow-x: auto;
  overflow-y: hidden;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
}

.horizontal-scroll::-webkit-scrollbar {
  display: none;
  width: 0;
  height: 0;
}
</style>

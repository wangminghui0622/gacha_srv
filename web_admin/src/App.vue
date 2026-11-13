<template>
  <a-config-provider :locale="antLocale">
    <router-view />
  </a-config-provider>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import enUS from 'ant-design-vue/es/locale/en_US'
import zhCN from 'ant-design-vue/es/locale/zh_CN'

const { locale } = useI18n()
const antLocale = computed(() => (locale.value === 'en-US' ? enUS : zhCN))
</script>

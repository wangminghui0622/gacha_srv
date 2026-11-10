import { createI18n } from 'vue-i18n'

import enUS from './locales/en-US.json'
import zhCN from './locales/zh-CN.json'

// vue-i18n 初始化（规范 06-i18n-styling 第 1 条）：locale 与 fallbackLocale 一致。
// web_admin 双语是相对 server_v2 参考控制台（纯中文硬编码）的刻意差异；
// 新增文案必须同步补齐两个 locale 文件（严禁模板硬编码）。
export const i18n = createI18n({
  legacy: false,
  locale: 'zh-CN',
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': zhCN,
    'en-US': enUS,
  },
})

export default i18n

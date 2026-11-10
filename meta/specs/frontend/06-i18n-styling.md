# i18n 与样式规范

## i18n（vue-i18n）

1. 初始化集中在 `src/i18n/index.ts`，`main.ts` 装配区 `.use(i18n)`。
2. 语言资源 `src/i18n/locales/<语言>.json`；`locale`/`fallbackLocale` 一致；模板内文案经 vue-i18n 插值机制渲染，天然防注入——**严禁 `v-html` 渲染任何含用户输入的文案**。
3. key 用点分命名空间按域组织：`common.*`、`nav.*`、`auth.*`、`game.*`、`payment.*`、`user.*`、`records.*`、`error.*`……新增域先建命名空间再写 key。
4. **所有面向用户的文案必须走 `t()`**，禁止硬编码（含 loading、错误、空态文案）；新增文案同步补齐所有 locale 文件。
5. 动态文案用插值 `t('records.count', { n })` 或 `t(item.labelKey)`，禁止字符串拼接拼句子。

## 样式（Ant Design Vue 令牌 + SCSS）

6. 主题令牌集中两处，互为镜像：Antdv 主题定制收敛在 `App.vue` 的 `ConfigProvider`（`theme.token`）；项目自有色板统一定义在 `src/styles/tokens.scss`（SCSS 变量 + CSS 自定义属性）：

```scss
$color-primary: #2e3142;
:root {
  --color-primary: #{$color-primary};
}
```

7. 类名/组件样式引用令牌（`var(--color-primary)`、`$color-primary`）；需要新颜色**先加令牌再使用**，禁止散落任意十六进制值。
8. 后台页面用 Antdv Layout 组件搭骨架（菜单/头部/内容区）；移动端页面移动优先：主容器 `max-width` 居中、固定 Header / 底部 TabBar、刘海屏适配 `safe-area` 工具类。
9. 条件类名用对象/数组绑定（`:class="{ active: isActive }"`）；动画优先 CSS transition，简单交互不引入 JS 动画库。
10. 全局 reset、滚动条、body 默认样式只写在 `src/styles/`；组件私有样式写在 SFC `<style scoped lang="scss">`；覆盖 Antdv 内置样式一律收敛到全局样式文件并注释原因，禁止组件内 `:deep` 散改。
11. 暗色主题：Antdv `theme.darkAlgorithm` 切换 + `tokens.scss` 定义双主题令牌；组件内禁止写死色值。

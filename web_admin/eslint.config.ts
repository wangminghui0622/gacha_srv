import { globalIgnores } from 'eslint/config'
import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'
import pluginVue from 'eslint-plugin-vue'
import pluginOxlint from 'eslint-plugin-oxlint'

export default defineConfigWithVueTs(
  {
    name: 'app/files-to-lint',
    files: ['**/*.{vue,ts,mts,tsx}'],
  },

  globalIgnores(['**/dist/**', '**/dist-ssr/**', '**/coverage/**', '**/node_modules/**']),

  ...pluginVue.configs['flat/essential'],
  vueTsConfigs.recommended,

  // 视图目录按 feature 建单名文件（views/<feature>/XxxView.vue），
  // 与 web/ 的 pages 命名规则同款，multi-word 规则在此不适用
  {
    name: 'app/pages-naming',
    files: ['src/views/**/*.vue', 'src/layout/**/*.vue'],
    rules: {
      'vue/multi-word-component-names': 'off',
    },
  },

  // create-vue 约定：`_` 前缀 = 有意未使用（解构剔除字段等），不报 unused
  {
    name: 'app/ts-rules',
    rules: {
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_', ignoreRestSiblings: true },
      ],
    },
  },

  ...pluginOxlint.buildFromOxlintConfigFile('.oxlintrc.json'),
)

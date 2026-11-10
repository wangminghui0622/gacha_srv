import { fileURLToPath, URL } from 'node:url'

import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

// https://vite.dev/config/
// 开发代理：/console → VITE_DEV_PROXY_TARGET（默认 admin gateway 9321，见 .env.development）
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd())

  return {
    plugins: [vue(), vueDevTools()],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: {
      // B 端 dev 端口固定 9374（避开 C 端 9373、admin-zero 9210-9216）
      port: 9374,
      proxy: {
        '/console': {
          target: env.VITE_DEV_PROXY_TARGET || 'http://127.0.0.1:9321',
          changeOrigin: true,
        },
      },
    },
  }
})

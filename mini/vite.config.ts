import { defineConfig, loadEnv } from "vite";
import { fileURLToPath } from "node:url";
import uni from "@dcloudio/vite-plugin-uni";

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  // .env 变量（规范 07 第 8 条）：只暴露 VITE_ 前缀；代理目标可配
  const env = loadEnv(mode, process.cwd(), "");
  return {
    plugins: [uni()],
    resolve: {
      alias: {
        "@": fileURLToPath(new URL("./src", import.meta.url)),
      },
    },
    // h5 兜底形态：/gacha 前缀代理到玩家网关（原 web/ 同款约定，web/ 已删除）
    server: {
      port: 9375,
      proxy: {
        "/gacha": {
          target: env.VITE_DEV_PROXY_TARGET || "http://127.0.0.1:9310",
          changeOrigin: true,
        },
      },
    },
  };
});

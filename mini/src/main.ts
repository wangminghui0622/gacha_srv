import { createSSRApp } from "vue";
import App from "./App.vue";
import { i18n } from "@/i18n";
import { pinia } from "@/store/pinia";

export function createApp() {
  const app = createSSRApp(App);
  app.use(pinia);
  app.use(i18n);
  return {
    app,
  };
}

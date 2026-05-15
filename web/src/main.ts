import { createApp } from "vue";
import "./styles/element/index.scss";
import "element-plus/theme-chalk/dark/css-vars.css";
import "./style.scss";
import router from "./router";
import store from "./store";
import App from "./App.vue";
import ElementPlus from "element-plus";
import * as ElementPlusIconsVue from "@element-plus/icons-vue";
import i18n from "./i18n";
import { initTheme } from "./reactives/useTheme";

initTheme();

// console.log("process.env", $env);
const app = createApp(App).use(store).use(router).use(ElementPlus).use(i18n);
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component);
}
app.mount("#app");

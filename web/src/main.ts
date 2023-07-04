import { createApp } from "vue";
import "./style.scss";
import router from "./router";
import store from "./store";
import App from "./App.vue";
import ElementPlus from "element-plus";
import "element-plus/dist/index.css";
import * as ElementPlusIconsVue from "@element-plus/icons-vue";

// console.log("process.env", $env);
const app = createApp(App).use(store).use(router).use(ElementPlus);
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component);
}
app.mount("#app");

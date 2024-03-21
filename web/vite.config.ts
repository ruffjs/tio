import { resolve } from "path";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    //路径别名配置
    alias: {
      "@": resolve(__dirname, ".", "src"),
    },
  },
  define: {
    $env: process.env,
  },
  base: "/web",
  server: {
    hmr: true,
    host: "0.0.0.0",
    port: 3333,
    //  // 反向代理配置，注意rewrite写法，开始没看文档在这里踩了坑
    proxy: {
      "/api": {
        target: "http://localhost:9000/",
        changeOrigin: true,
      },
      "/private/api": {
        target: "http://localhost:9000/",
        changeOrigin: true,
      },
    },
  },
});

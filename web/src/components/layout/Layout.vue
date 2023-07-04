<template>
  <div class="layout">
    <div class="playground" :style="{ zIndex }"><router-view></router-view></div>
    <div class="background">
      <nav class="top-nav-bar">
        <TopNavBar />
      </nav>
      <div class="tool-area">
        <ToolArea />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from "vue";
import { useRoute } from "vue-router";

import TopNavBar from "@/components/layout/TopNavBar.vue";
import ToolArea from "@/components/layout/ToolArea.vue";

const route = useRoute();
const zIndex = ref(0);

watch(
  route,
  () => {
    console.log("route:", route.name, route.meta);
    zIndex.value = route.meta.zIndex;
    if (typeof route.meta.title === "function") {
      document.title = route.meta.title(route);
    } else {
      document.title = route.meta.title || "TIO Playground";
    }
  },
  { immediate: true }
);
</script>

<style scoped lang="scss">
.layout {
  width: 100vw;
  height: 100vh;
  min-width: 1080px;
  min-height: 568px;
  background-color: #e0e0e0;
  .playground {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
  }
  .background {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 0;
    background-color: transparent;
    z-index: 10;
    overflow: visible;

    .top-nav-bar {
      position: fixed;
      top: 0;
      left: 0;
      width: 100vw;
      height: 50px;
      min-width: 1080px;
      overflow: visible;
    }
    .tool-area {
      position: fixed;
      bottom: 0;
      left: 0;
      width: 100vw;
      height: auto;
      min-width: 1080px;
    }
  }
}
</style>

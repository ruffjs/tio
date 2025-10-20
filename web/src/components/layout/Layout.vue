<template>
  <el-container>
    <el-aside class="left">
      <div class="logo-con">
        <div class="nav-logo">
          <div class="nav-logo-tio">
            <el-icon>
              <ArrowLeftBold />
            </el-icon>
            <span>T</span>
            <span>I</span>
            <span>O</span>
            <el-icon>
              <ArrowRightBold />
            </el-icon>
          </div>
          <div class="nav-logo-sub">
            <span v-for="l in 'playground'.split('')">{{ l }}</span>
          </div>
        </div>
      </div>

      <el-menu :default-active="route.path" :router="true" class="menu" background-color="#071927" text-color="#fff">
        <el-menu-item index="/" route="/">
          <el-icon>
            <Grid />
          </el-icon>
          <span>{{ $t('things.navTitle') }}</span>
        </el-menu-item>
        <el-menu-item index="/rules" route="/rules">
          <el-icon>
            <Operation />
          </el-icon>
          <span>{{ $t('rules.navTitle') }}</span>
        </el-menu-item>
      </el-menu>

    </el-aside>
    <el-container class="right">
      <el-header class="top-nav-bar">
        <nav>
          <TopNavBar />
        </nav>
      </el-header>
      <el-main>
        <div class="playground" :style="{ zIndex }">
          <router-view></router-view>
        </div>

        <div class="tool-area">
          <ToolArea />
        </div>
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";

import TopNavBar from "@/components/layout/TopNavBar.vue";
import ToolArea from "@/components/layout/ToolArea.vue";

const route = useRoute();
const zIndex = ref(0);

watch(
  route,
  () => {
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
.left {
  position: fixed;
  z-index: 10;
  height: 100vh;
  width: 180px;

  .menu {
    height: calc(100vh - 50px);
  }

  .logo-con {
    display: block;
    height: 58px;
    margin-top: -8px;
    padding-top: 8px;
    background-color: #071927;
  }

  .nav-logo {
    width: 100px;
    height: 34px;
    margin: 8px 0px 8px 20px;
    text-align: center;
    color: #fff;
    cursor: default;

    .nav-logo-tio {
      display: flex;
      justify-content: space-between;
      align-items: center;
      text-transform: uppercase;
      line-height: 24px;
      font-size: 24px;
      font-weight: 700;
    }

    .nav-logo-sub {
      display: flex;
      justify-content: space-between;
      text-transform: uppercase;
      line-height: 10px;
      font-size: 12px;
      font-weight: 900;
      color: #a0cfff;
    }
  }
}

.right {
  margin-left: 160px;

  .top-nav-bar {
    position: fixed;
    z-index: 10;
    width: calc(100% - 140px);
  }

  .playground {
    margin-top: 40px;
  }

  .tool-area {
    z-index: 10;
    position: fixed;
    bottom: 0;
    width: calc(100% - 180px);
    height: auto;
    min-width: 1080px;
  }
}
</style>

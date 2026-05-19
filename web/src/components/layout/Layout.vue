<template>
  <el-container :class="['layout-shell', isStandalone ? 'standalone' : '']">
    <el-aside v-if="!isStandalone" class="left">
      <div class="logo-con">
        <div class="nav-logo">
          <img class="nav-logo-mark" :src="tioLogoUrl" alt="" aria-hidden="true" />
          <div>
            <div class="nav-logo-title">TIO</div>
            <div class="nav-logo-sub">playground</div>
          </div>
        </div>
      </div>

      <el-menu :default-active="route.path" :router="true" class="menu">
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
      <el-header v-if="!isStandalone" class="top-nav-bar">
        <nav>
          <TopNavBar />
        </nav>
      </el-header>
      <el-main>
        <div class="playground" :style="{ zIndex }">
          <router-view></router-view>
        </div>

        <div v-if="!isStandalone" class="tool-area">
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
const isStandalone = computed(() => Boolean(route.meta.standalone));
const publicPath = import.meta.env.BASE_URL.endsWith("/")
  ? import.meta.env.BASE_URL
  : `${import.meta.env.BASE_URL}/`;
const tioLogoUrl = `${publicPath}tio-logo.svg`;

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
.layout-shell {
  min-height: 100vh;
  background: transparent;

  &.standalone {
    display: block;
  }
}

.left {
  position: fixed;
  z-index: 10;
  display: flex;
  flex-direction: column;
  height: 100vh;
  width: 180px;
  padding: 12px 10px;
  border-right: 1px solid var(--tio-line);
  background: var(--tio-surface-solid);
  box-shadow: none;
  box-sizing: border-box;

  .menu {
    flex: 1;
    min-height: 0;
    border-right: 0;
    background: transparent;
    overflow-y: auto;
    scrollbar-width: none;

    &::-webkit-scrollbar {
      width: 0;
      height: 0;
    }

    :deep(.el-menu-item) {
      height: 40px;
      margin: 5px 0;
      border: 1px solid transparent;
      border-radius: var(--tio-radius);
      color: var(--tio-muted);
      transition: background-color 0.18s ease, border-color 0.18s ease, color 0.18s ease;

      &:hover {
        border-color: var(--tio-line);
        background: var(--tio-surface-soft);
        color: var(--tio-text-strong);
      }

      &.is-active {
        border-color: var(--tio-accent-strong);
        background: var(--tio-accent-soft);
        color: var(--tio-text-strong);
      }
    }
  }

  .logo-con {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    height: 56px;
    margin-bottom: 8px;
    border: 1px solid var(--tio-line);
    border-radius: var(--tio-radius);
    background: var(--tio-surface);
  }

  .nav-logo {
    display: flex;
    align-items: center;
    gap: 9px;
    width: 100%;
    height: 100%;
    padding: 7px 11px;
    color: var(--tio-text-strong);
    cursor: default;

    .nav-logo-mark {
      width: 34px;
      height: 34px;
      flex: 0 0 auto;
    }

    .nav-logo-title {
      text-transform: uppercase;
      line-height: 20px;
      font-size: 20px;
      font-weight: 700;
      letter-spacing: 0.08em;
    }

    .nav-logo-sub {
      text-transform: uppercase;
      line-height: 12px;
      font-size: 10px;
      font-weight: 700;
      letter-spacing: 0.14em;
      color: var(--tio-accent);
    }
  }
}

:global(:root[data-theme="light"]) .left {
  background: var(--tio-surface-solid);
  box-shadow: none;
}

.right {
  margin-left: 180px;

  .top-nav-bar {
    position: fixed;
    z-index: 10;
    width: calc(100% - 180px);
    height: 58px;
    padding: 0;
    border-bottom: 1px solid var(--tio-line);
    background: var(--tio-surface-solid);
    backdrop-filter: none;
  }

  .playground {
    padding: 74px 18px 42px;
    min-height: calc(100vh - 30px);
    background: var(--tio-bg);
  }

  .tool-area {
    z-index: 10;
    position: fixed;
    bottom: 0;
    width: calc(100% - 180px);
    height: auto;
    min-width: 900px;
    margin-left: 0;
  }
}

.standalone .right {
  margin-left: 0;

  .playground {
    min-height: 100vh;
    padding: 0;
  }
}

:global(:root[data-theme="light"]) .right {
  .top-nav-bar {
    background: var(--tio-surface-solid);
  }

  .playground {
    background: var(--tio-bg);
  }
}

.el-main {
  padding: 0;
}
</style>

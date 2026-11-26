<template>
  <el-container :class="['layout-shell', isStandalone ? 'standalone' : '']">
    <el-aside v-if="!isStandalone" class="left">
      <NavMenu />
    </el-aside>
    <el-container class="right">
      <el-header v-if="!isStandalone" class="top-nav-bar">
        <button
          v-if="isMobileShell"
          class="mobile-menu-button"
          type="button"
          title="Menu"
          aria-label="Menu"
          @click="drawerOpen = true"
        >
          <span class="menu-lines" aria-hidden="true">
            <i></i>
            <i></i>
            <i></i>
          </span>
        </button>
        <nav>
          <TopNavBar :compact="isMobileShell" />
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

    <MobileNavDrawer v-if="!isStandalone" v-model="drawerOpen" />
  </el-container>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";

import TopNavBar from "@/components/layout/TopNavBar.vue";
import ToolArea from "@/components/layout/ToolArea.vue";
import NavMenu from "@/components/layout/NavMenu.vue";
import MobileNavDrawer from "@/components/layout/MobileNavDrawer.vue";
import useMediaQuery from "@/reactives/useMediaQuery";

const route = useRoute();
const zIndex = ref(0);
const drawerOpen = ref(false);
const isMobileShell = useMediaQuery("(max-width: 900px)");
const isStandalone = computed(() => Boolean(route.meta.standalone));

watch(
  route,
  () => {
    zIndex.value = route.meta.zIndex;
    drawerOpen.value = false;
    if (typeof route.meta.title === "function") {
      document.title = route.meta.title(route);
    } else {
      document.title = route.meta.title || "TIO Playground";
    }
  },
  { immediate: true }
);

watch(isMobileShell, (mobile) => {
  if (!mobile) drawerOpen.value = false;
});
</script>

<style scoped lang="scss">
.layout-shell {
  min-height: 100vh;
  background: transparent;

  &.standalone {
    display: block;
  }
}

.left,
.top-nav-bar {
  --tio-text: #dbe4f0;
  --tio-text-strong: #f8fafc;
  --tio-muted: #aebdd1;
  --tio-line: #3b4a60;
  --tio-surface: #233047;
  --tio-surface-soft: #2d3d56;
  --tio-accent: #a5b4fc;
  --tio-accent-strong: #818cf8;
  --tio-accent-soft: #303b66;
  --tio-link: #c7d2fe;
}

.top-nav-bar {
  --tio-surface-solid: #243247;
}

.left {
  --tio-surface-solid: #182337;
  position: fixed;
  z-index: 10;
  display: flex;
  flex-direction: column;
  height: 100vh;
  width: 180px;
  overflow: hidden;
  border-right: 1px solid var(--tio-line);
  background: var(--tio-surface-solid);
  box-shadow: none;
}

:global(:root[data-theme="light"]) .left {
  background: var(--tio-surface-solid);
  box-shadow: none;
}

.mobile-menu-button {
  display: none;
  flex: 0 0 44px;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  margin: 0;
  padding: 0;
  border: 0;
  border-radius: var(--tio-radius);
  background: transparent;
  color: var(--tio-text);
  cursor: pointer;
  transition: background-color 0.18s ease, color 0.18s ease;

  &:hover {
    background: var(--tio-surface-soft);
    color: var(--tio-text-strong);
  }

  .menu-lines {
    display: flex;
    flex-direction: column;
    gap: 4px;
    width: 18px;

    i {
      display: block;
      height: 2px;
      border-radius: 1px;
      background: currentColor;
    }
  }
}

.right {
  margin-left: 180px;

  .top-nav-bar {
    position: fixed;
    z-index: 10;
    display: flex;
    align-items: center;
    width: calc(100% - 180px);
    height: 58px;
    padding: 0;
    border-bottom: 1px solid var(--tio-line);
    background: var(--tio-surface-solid);
    backdrop-filter: none;

    nav {
      flex: 1;
      min-width: 0;
    }
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

@media (max-width: 900px) {
  .layout-shell:not(.standalone) {
    flex-direction: column;
  }

  .left {
    display: none;
  }

  .right {
    min-width: 0;
    margin-left: 0;

    .top-nav-bar {
      position: fixed;
      top: 0;
      left: 0;
      width: 100%;
      height: 52px;
      padding: 0 6px;
      gap: 2px;
    }

    .playground {
      min-height: 100dvh;
      padding: 60px 8px calc(46px + env(safe-area-inset-bottom));
    }

    .tool-area {
      left: 0;
      width: 100%;
      min-width: 0;
    }
  }

  .mobile-menu-button {
    display: flex;
  }

  .standalone .right .playground {
    padding: 0;
  }
}
</style>

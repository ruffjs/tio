<template>
  <div :class="['nav-menu', { flat }]">
    <div class="logo-con">
      <div class="nav-logo">
        <img class="nav-logo-mark" :src="tioLogoUrl" alt="" aria-hidden="true" />
        <div>
          <div class="nav-logo-title">TIO</div>
          <div class="nav-logo-sub">playground</div>
        </div>
      </div>
    </div>

    <el-menu
      :default-active="route.path"
      :router="true"
      class="menu"
      @select="emit('navigate')"
    >
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
      <el-menu-item index="/broker" route="/broker">
        <el-icon>
          <Connection />
        </el-icon>
        <span>{{ $t('broker.navTitle') }}</span>
      </el-menu-item>
    </el-menu>
  </div>
</template>

<script setup>
import { useRoute } from "vue-router";

const emit = defineEmits(["navigate"]);
defineProps({
  flat: Boolean,
});
const route = useRoute();
const publicPath = import.meta.env.BASE_URL.endsWith("/")
  ? import.meta.env.BASE_URL
  : `${import.meta.env.BASE_URL}/`;
const tioLogoUrl = `${publicPath}tio-logo.svg`;
</script>

<style scoped lang="scss">
.nav-menu {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 12px 10px;

  .menu {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    border-right: 0;
    background: transparent;

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
        border-color: var(--tio-link);
        background: var(--tio-accent-soft);
        color: var(--tio-link);
      }
    }
  }

  .logo-con {
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

  &.flat {
    .logo-con {
      border-color: transparent;
    }

    .menu {
      :deep(.el-menu-item) {
        &:hover,
        &.is-active {
          border-color: transparent;
        }
      }
    }
  }
}
</style>

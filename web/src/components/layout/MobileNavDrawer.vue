<template>
  <el-drawer
    v-model="open"
    direction="ltr"
    size="min(280px, 82vw)"
    :with-header="false"
    class="mobile-nav-drawer"
  >
    <div class="drawer-inner">
      <div class="drawer-nav">
        <NavMenu flat @navigate="close" />
      </div>

      <div class="drawer-actions">
        <button
          type="button"
          class="drawer-action"
          :title="`Switch to ${nextThemeMode}`"
          @click="toggleTheme"
        >
          <el-icon>
            <Monitor v-if="themeMode === 'auto'" />
            <Sunny v-else-if="themeMode === 'dark'" />
            <Moon v-else />
          </el-icon>
          <span>{{ themeLabel }}</span>
        </button>

        <LanguageSwitcher class="drawer-action" />

        <button type="button" class="drawer-action danger" @click="handleLogOut">
          <el-icon><Avatar /></el-icon>
          <span>{{ $t('nav.logOut') }}</span>
        </button>
      </div>
    </div>
  </el-drawer>
</template>

<script setup>
import { computed } from "vue";
import { useStore } from "vuex";
import { useRouter } from "vue-router";
import LanguageSwitcher from "@/components/common/LanguageSwitcher.vue";
import NavMenu from "./NavMenu.vue";
import useTheme from "@/reactives/useTheme";

const props = defineProps({
  modelValue: Boolean,
});
const emit = defineEmits(["update:modelValue"]);

const store = useStore();
const router = useRouter();
const { themeMode, themeLabel, nextThemeMode, toggleTheme } = useTheme();

const open = computed({
  get: () => props.modelValue,
  set: (value) => emit("update:modelValue", value),
});

const close = () => {
  open.value = false;
};

const handleLogOut = () => {
  localStorage.setItem("$tiopg/user/auth", "");
  store.commit("user/setState", { auth: "" });
  close();
  router.push("/login");
};
</script>

<style scoped lang="scss">
.drawer-inner {
  --tio-text: #dbe4f0;
  --tio-text-strong: #f8fafc;
  --tio-muted: #aebdd1;
  --tio-line: #3b4a60;
  --tio-surface: #233047;
  --tio-surface-soft: #2d3d56;
  --tio-surface-solid: #182337;
  --tio-accent: #a5b4fc;
  --tio-accent-strong: #818cf8;
  --tio-accent-soft: #303b66;
  --tio-link: #c7d2fe;

  display: flex;
  flex-direction: column;
  height: 100%;
  color: var(--tio-text);
  background: var(--tio-surface-solid);
}

.drawer-nav {
  flex: 1;
  min-height: 0;
}

.drawer-actions {
  flex: 0 0 auto;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px 10px calc(8px + env(safe-area-inset-bottom));
  border-top: 1px solid rgba(148, 163, 184, 0.14);
}

.drawer-action {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  height: 40px;
  padding: 0 10px;
  border: 0;
  border-radius: var(--tio-radius);
  background: transparent;
  color: var(--tio-text);
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  text-align: left;
  cursor: pointer;
  transition: background-color 0.18s ease, color 0.18s ease;

  &:hover {
    background: var(--tio-surface-soft);
    color: var(--tio-text-strong);
  }

  .el-icon {
    font-size: 15px;
  }

  &.danger {
    color: var(--tio-danger);
  }
}
</style>

<style lang="scss">
.mobile-nav-drawer .el-drawer__body {
  padding: 0;
}
</style>

<template>
  <div class="nav-container" :class="{ compact }">
    <div class="nav-buttons">
      <template v-if="!compact">
        <LanguageSwitcher />
        <button type="button" class="nav-button theme-button" @click="toggleTheme" :title="`Switch to ${nextThemeMode}`">
          <el-icon>
            <Monitor v-if="themeMode === 'auto'" />
            <Sunny v-else-if="themeMode === 'dark'" />
            <Moon v-else />
          </el-icon>
          {{ themeLabel }}
        </button>
        <button type="button" class="nav-button" @click="handleLogOut">
          <el-icon><Avatar /></el-icon>{{ hasAuth ? $t('nav.logOut') : "" }}
        </button>
      </template>
      <button
        type="button"
        v-if="isList"
        class="nav-button"
        :title="compact ? $t('nav.addThing') : undefined"
        :aria-label="compact ? $t('nav.addThing') : undefined"
        @click="handleShowAddingForm"
      >
        <el-icon><CirclePlusFilled /></el-icon><span v-if="!compact">{{ $t('nav.addThing') }}</span>
      </button>
    </div>
  </div>
  <AddThingForm v-if="isAdding" @close="handleCloseDialogs" />
</template>

<script setup>
import { computed, defineAsyncComponent, ref } from "vue";
import { useStore } from "vuex";
import { useRouter } from "vue-router";
import LanguageSwitcher from "@/components/common/LanguageSwitcher.vue";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import useTheme from "@/reactives/useTheme";

const AddThingForm = defineAsyncComponent(() => import("./AddThingForm.vue"));

defineProps({
  compact: Boolean,
});

const store = useStore();
const router = useRouter();
const { route } = useThingsAndShadows();
const { themeMode, themeLabel, nextThemeMode, toggleTheme } = useTheme();
const hasAuth = computed(() => !!store.state.user.auth);
const isList = computed(() => route.name === "List");

const isAdding = ref(false);
const isSetting = ref(false);

const handleShowAddingForm = () => {
  isAdding.value = true;
  isSetting.value = false;
};

const handleShowSettingPanel = () => {
  isAdding.value = false;
  isSetting.value = true;
};

const handleCloseDialogs = () => {
  isAdding.value = false;
  isSetting.value = false;
};

const handleLogOut = () => {
  localStorage.setItem("$tiopg/user/auth", "");
  store.commit("user/setState", { auth: "" });
  router.push("/login");
};
</script>

<style scoped lang="scss">
.nav-container {
  display: flex;
  justify-content: space-between;
  height: 58px;
  color: var(--tio-text);
  user-select: none;

  .nav-buttons {
    flex: 1;
    display: flex;
    flex-direction: row-reverse;
    align-items: center;
    height: 58px;
    padding: 10px 18px;
    gap: 10px;

    .nav-button {
      display: flex;
      flex-direction: row;
      justify-content: center;
      align-items: center;
      min-height: 34px;
      padding: 0 10px;
      border: 1px solid var(--tio-line);
      border-radius: var(--tio-radius);
      background: transparent;
      color: var(--tio-text);
      font-size: 12px;
      font-weight: 650;
      white-space: nowrap;
      cursor: pointer;
      transition: background-color 0.18s ease, border-color 0.18s ease, color 0.18s ease;

      &:hover {
        border-color: var(--tio-accent-strong);
        background: var(--tio-accent-soft);
        color: var(--tio-text-strong);
      }

      &.danger {
        color: var(--tio-danger);
      }

      .el-icon {
        font-size: 14px;
        margin-right: 4px;
      }
    }
  }

  &.compact {
    height: 52px;

    .nav-buttons {
      height: 52px;
      padding: 4px;
      gap: 0;
    }

    .nav-button {
      width: 44px;
      height: 44px;
      min-height: 44px;
      padding: 0;

      .el-icon {
        margin-right: 0;
        font-size: 18px;
      }
    }
  }
}
</style>

<template>
  <div class="nav-container">
    <div v-if="isThing" class="nav-thing-id">
      <span>{{ thingId }}</span>
    </div>
    <div class="nav-buttons">
      <LanguageSwitcher />
      <div class="nav-button theme-button" @click="toggleTheme" :title="`Switch to ${nextThemeMode}`">
        <el-icon>
          <Monitor v-if="themeMode === 'auto'" />
          <Sunny v-else-if="themeMode === 'dark'" />
          <Moon v-else />
        </el-icon>
        {{ themeLabel }}
      </div>
      <div class="nav-button" @click="handleLogOut">
        <el-icon><Avatar /></el-icon>{{ hasAuth ? $t('nav.logOut') : "" }}
      </div>
      <div v-if="isList" class="nav-button" @click="handleShowAddingForm">
        <el-icon><CirclePlusFilled /></el-icon>{{ $t('nav.addThing') }}
      </div>
    </div>
  </div>
  <AddThingForm v-if="isAdding" @close="handleCloseDialogs" />
</template>

<script setup>
import { computed, ref } from "vue";
import { useStore } from "vuex";
import { useRouter } from "vue-router";
import List from "@/components/views/List.vue";
import Thing from "@/components/views/Thing.vue";
import AddThingForm from "./AddThingForm.vue";
import LanguageSwitcher from "@/components/common/LanguageSwitcher.vue";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import useTheme from "@/reactives/useTheme";

const store = useStore();
const router = useRouter();
const { route } = useThingsAndShadows();
const { themeMode, themeLabel, nextThemeMode, toggleTheme } = useTheme();
const hasAuth = computed(() => !!store.state.user.auth);
const thingId = computed(() => route.params.thingId || "");
const isList = computed(() => route.name === List.name);
const isThing = computed(() => route.name === Thing.name && thingId.value);

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

  .nav-thing-id {
    display: flex;
    align-items: center;
    max-width: 48vw;
    height: 36px;
    margin: 11px 0 11px 14px;
    padding: 0 12px;
    overflow: hidden;
    border: 1px solid var(--tio-line);
    border-left: 3px solid var(--tio-accent);
    border-radius: var(--tio-radius);
    background: transparent;
    color: var(--tio-text-strong);
    line-height: 18px;
    font-size: 18px;
    font-weight: 650;
    text-overflow: ellipsis;
    white-space: nowrap;
    cursor: pointer;
  }

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
}
</style>

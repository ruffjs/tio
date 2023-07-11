<template>
  <div class="nav-container">
    <div class="nav-logo">
      <div class="nav-logo-tio">
        <el-icon><ArrowLeftBold /></el-icon>
        <span>T</span>
        <span>I</span>
        <span>O</span>
        <el-icon><ArrowRightBold /></el-icon>
      </div>
      <div class="nav-logo-sub">
        <span v-for="l in 'playground'.split('')">{{ l }}</span>
      </div>
    </div>
    <div v-if="isThing" class="nav-thing-id">
      <span>{{ thingId }}</span>
    </div>
    <div class="nav-buttons">
      <div class="nav-button" @click="handleLogOut">
        <el-icon><Avatar /></el-icon>{{ hasAuth ? "Log Out" : "" }}
      </div>
      <div v-if="isList" class="nav-button" @click="handleShowAddingForm">
        <el-icon><CirclePlusFilled /></el-icon>Add Thing
      </div>
      <div v-if="isList" class="nav-button" @click="requestUpdateShadowList">
        <el-icon><Refresh /></el-icon>Refresh List
      </div>
      <div
        v-if="isThing && currentShadow.connected"
        class="nav-button danger"
        @click="handleKickOutSelected"
      >
        <el-icon><Scissor /></el-icon>Kick Out
      </div>
      <div v-if="isThing" class="nav-button" @click="updateCurrentShadow">
        <el-icon><Refresh /></el-icon>Refresh Shadow
      </div>
      <!-- <div v-if="isThing" class="nav-button" @click="handleReturnList">
        <el-icon><Menu /></el-icon>Back List
      </div> -->
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
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import { kickOutClient } from "@/apis";

const store = useStore();
const router = useRouter();
const {
  route,
  selectedThingId,
  currentShadow,
  requestUpdateShadowList,
  updateCurrentShadow,
} = useThingsAndShadows();
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

const handleKickOutSelected = async () => {
  try {
    const res = await kickOutClient(selectedThingId.value);
    console.log("handleKickOutSelected", res);
    updateCurrentShadow();
  } catch (error) {
    console.error("error", error);
  }
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
  background-color: #f2f2f2;
  box-shadow: 0 1px 2px rgba($color: #000000, $alpha: 0.1);
  user-select: none;
  .nav-logo {
    width: 100px;
    height: 34px;
    margin: 8px 0px 8px 20px;
    text-align: center;
    background-color: var(--el-text-color-primary);
    color: #f2f2f2;
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
    }
  }

  .nav-thing-id {
    height: 34px;
    margin: 8px 0 8px 3px;
    padding: 4px;
    border: solid 3px var(--el-text-color-primary);
    border-left-width: 3px;
    color: var(--el-text-color-primary);
    line-height: 18px;
    font-size: 24px;
    font-weight: 400;
    cursor: pointer;
  }
  .nav-buttons {
    flex: 1;
    display: flex;
    flex-direction: row-reverse;
    align-items: center;
    height: 50px;
    padding: 10px 20px;

    .nav-button {
      display: flex;
      flex-direction: row;
      justify-content: center;
      align-items: center;

      margin-left: 20px;
      font-size: 12px;
      cursor: pointer;
      &:hover {
        color: var(--el-color-primary);
      }
      &.danger {
        color: red;
      }
      .el-icon {
        font-size: 14px;
        margin-right: 2px;
      }
    }
  }
}
</style>

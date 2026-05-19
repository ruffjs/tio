<template>
  <div class="thing-view">
    <div class="thing-summary">
      <div class="thing-summary-main">
        <el-tooltip :content="$t('things.backToList')" placement="bottom">
          <el-button class="thing-back-button" link circle icon="Back" @click="handleBack2List" />
        </el-tooltip>
        <div class="thing-title-group">
          <div class="thing-title-row">
            <h2>{{ thingId }}</h2>
            <span :class="['thing-status-pill', shadow?.connected ? 'connected' : '']">
              <i></i>
              {{ shadow?.connected ? $t('things.connected') : $t('things.disconnected') }}
            </span>
          </div>
          <div class="thing-meta-strip">
            <span v-if="thing.remoteAddr" class="thing-meta-chip">
              <el-icon>
                <Location />
              </el-icon>
              {{ thing.remoteAddr }}
            </span>
            <span v-if="thing.version" class="thing-meta-chip">
              <el-icon>
                <Document />
              </el-icon>
              {{ $t('things.version') }}: {{ thing.version }}
            </span>
          </div>
        </div>
      </div>
      <div class="thing-summary-actions">
        <el-tooltip :content="$t('nav.kickOut')">
          <el-button v-if="shadow?.connected" type="danger" plain circle icon="RemoveFilled"
            @click="handleKickOutSelected" />
        </el-tooltip>
        <el-tooltip :content="$t('things.reloadData')">
          <el-button icon="RefreshRight" plain circle @click="refreshThingData" />
        </el-tooltip>
      </div>
    </div>

    <div class="thing-workspace">
      <aside class="thing-sidebar">
        <section class="thing-side-section">
          <h3>{{ $t('things.basicInfo') }}</h3>
          <div class="thing-meta-item">
            <div class="thing-meta-label">{{ $t('things.enabled') }}</div>
            <div class="thing-meta-value">
              <el-switch
                size="small"
                v-model="thing.enabled"
                :loading="updatingEnabled"
                @change="handleToggleEnabled"
              />
            </div>
          </div>
          <KeyValueDisplayer :data="thing" :fields="metaFields" />
        </section>

        <section class="thing-side-section">
          <h3>{{ $t('things.actions') }}</h3>
          <div class="thing-action-stack">
            <el-button icon="TopRight" type="primary" plain @click="openPoster('invoke')" block>
              {{ $t('things.requestDirectMethod') }}
            </el-button>
            <MqttClients />
          </div>
        </section>
      </aside>

      <main class="thing-shadow-panel">
        <ShadowTags :data="shadow?.tags"
          @update="(payload) => openPoster('tags', payload || null)" />
        <div class="thing-shadow-state">
          <ShadowData @call="(code) => openPoster(code)" />
        </div>
      </main>
    </div>
  </div>
  <HttpPoster v-if="posterCode" v-model="posterVisible" :key="posterKey" :code="posterCode" :thing-id="thingId" :payload="posterData" @done="updateCurrentShadow"
    @close="closePoster" />
</template>

<script>
export default {
  name: "Thing",
  inheritAttrs: false,
  customOptions: {
    title: (route) => `${route.params.thingId || "(thingId)"} | Thing`,
    zIndex: 0,
    actived: true,
  },
  components: { KeyValueDisplayer, MqttClients },
};
</script>

<script setup>
import { computed, nextTick, onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { getThing, kickOutClient, patchThing } from "@/apis";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import { createMetaFields } from "@/configs/thing";
import KeyValueDisplayer from "@/components/common/KeyValueDisplayer.vue";
import ShadowTags from "@/components/thing/ShadowTags.vue";
import ShadowData from "@/components/thing/ShadowData.vue";
import HttpPoster from "@/components/thing/HttpPoster.vue";
import MqttClients from "@/components/thing/MqttClientsOfThing.vue";
import {
  TSCE_MQTO,
  TSCE_MQTT,
  TSCE_MSGO,
  TSCE_MSGI,
} from "@/utils/event";
import useThingEvent from "@/reactives/useThingEvent";

const router = useRouter();
const { t } = useI18n();
const {
  route,
  selectedThingId: thingId,
  currentShadow: shadow,
  setCurrentShadow,
  updateCurrentShadow,
  updateThings,
} = useThingsAndShadows();
const { onSomethingStatusChange } = useThingEvent();
const isFromList = ref(false);
const thing = reactive({});
const posterCode = ref("");
const posterData = ref(null);
const posterKey = ref(0);
const posterVisible = ref(false);
const updatingEnabled = ref(false);
const metaFields = computed(() => createMetaFields(t).filter(({ key }) => key !== "enabled"));

const openPoster = async (code, payload = null) => {
  posterVisible.value = false;
  posterCode.value = "";
  posterData.value = null;
  await nextTick();
  posterData.value = payload;
  posterKey.value += 1;
  posterCode.value = code;
  posterVisible.value = true;
};

const closePoster = () => {
  posterVisible.value = false;
  posterCode.value = "";
  posterData.value = null;
};

const handleBack2List = () => {
  if (isFromList.value) {
    router.back();
  } else {
    router.replace("/");
  }
  setCurrentShadow(null);
};

const refreshThingData = async () => {
  await Promise.all([
    getBasicInfo(),
    updateCurrentShadow()
  ]);
  ElMessage.success(t('common.success'));
};

const getBasicInfo = async () => {
  try {
    const res = await getThing(thingId.value);
    // console.log("getBasicInfo", res);
    Object.assign(thing, res.data);
  } catch (error) {
    console.error("error", error);
  }
};

const handleToggleEnabled = async (enabled) => {
  const previousEnabled = !enabled;
  try {
    await ElMessageBox.confirm(
      enabled ? t("things.confirmEnableThing") : t("things.confirmDisableThing"),
      t("common.warning"),
      {
        confirmButtonText: t("common.confirm"),
        cancelButtonText: t("common.cancel"),
        type: "warning",
      }
    );
  } catch {
    thing.enabled = previousEnabled;
    return;
  }

  updatingEnabled.value = true;
  try {
    await patchThing(thingId.value, { enabled });
    await updateThings();
    await getBasicInfo();
    ElMessage.success(t("common.success"));
  } catch (error) {
    thing.enabled = previousEnabled;
    console.error("error", error);
    ElMessage.error(t("common.error"));
  } finally {
    updatingEnabled.value = false;
  }
};

onSomethingStatusChange(async ({ thingId: eventThingId, type, about }) => {
  // console.log(eventThingId, type, about);
  if (eventThingId === thingId.value) {
    console.log(type, about);
    switch (type) {
      case TSCE_MQTT:
      case TSCE_MQTO:
      case TSCE_MSGO:
      case TSCE_MSGI:
        updateCurrentShadow();
        break;

      default:
        break;
    }
  } else if (eventThingId === "*" && type === TSCE_MSGO) {
    if (about.topic.startsWith(`$iothub/things/${thingId.value}/`)) {
      await nextTick();
      updateCurrentShadow();
    }
  }
});

const handleKickOutSelected = async () => {
  try {
    const res = await kickOutClient(thingId.value);
    console.log("handleKickOutSelected", res);
    updateCurrentShadow();
  } catch (error) {
    console.error("error", error);
  }
};

onMounted(() => {
  if (shadow.value?.fromList) isFromList.value = true;
  getBasicInfo();
  updateCurrentShadow();
});
</script>

<style scoped lang="scss">
.thing-view {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: transparent;
  color: var(--tio-text);

  .thing-summary {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 18px;
    margin-bottom: 14px;
    padding: 4px 2px 14px;
    border-bottom: 1px solid var(--tio-line);
    background: transparent;

    .thing-summary-main {
      display: flex;
      align-items: flex-start;
      gap: 14px;
      min-width: 0;
    }

    .thing-back-button {
      width: 30px;
      height: 30px;
      margin-top: 2px;
      border: 1px solid var(--tio-line);
      background: var(--tio-surface-soft);
      flex: 0 0 auto;
    }

    .thing-title-group {
      min-width: 0;
    }

    .thing-title-row {
      display: flex;
      align-items: center;
      gap: 10px;
      min-width: 0;

      h2 {
        margin: 0;
        overflow: hidden;
        color: var(--tio-text-strong);
        font-size: 20px;
        font-weight: 750;
        line-height: 1.25;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
    }

    .thing-status-pill {
      display: inline-flex;
      align-items: center;
      gap: 7px;
      height: 24px;
      padding: 0 9px;
      border: 1px solid var(--tio-border);
      border-radius: 999px;
      color: var(--tio-muted);
      background: var(--tio-surface-soft);
      font-size: 12px;
      font-weight: 650;
      white-space: nowrap;

      i {
        width: 7px;
        height: 7px;
        border-radius: 50%;
        background: var(--tio-muted);
      }

      &.connected {
        border-color: rgba(34, 197, 94, 0.26);
        color: var(--tio-success);
        background: var(--tio-success-soft);

        i {
          background: var(--tio-success);
        }
      }
    }

    .thing-meta-strip {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      margin-top: 8px;
    }

    .thing-meta-chip {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      max-width: 360px;
      color: var(--tio-muted);
      font-size: 12px;

      .el-icon {
        flex: 0 0 auto;
      }
    }

    .thing-summary-actions {
      display: flex;
      gap: 8px;
      flex: 0 0 auto;
    }
  }

  .thing-workspace {
    display: grid;
    grid-template-columns: 300px minmax(0, 1fr);
    gap: 14px;
    flex: 1;
    min-height: 0;
    overflow: hidden;

    .thing-sidebar {
      display: flex;
      flex-direction: column;
      gap: 12px;
      min-height: 0;
      padding: 2px 16px 0 0;
      border-right: 1px solid var(--tio-line);
      overflow-y: auto;
    }

    .thing-side-section {
      padding: 0 0 14px;
      border-bottom: 1px solid var(--tio-line);
      background: transparent;

      &:last-child {
        border-bottom: 0;
      }

      h3 {
        margin: 0 0 12px;
        color: var(--tio-text-strong);
        font-size: 13px;
        font-weight: 750;
        letter-spacing: 0.02em;
      }

      :deep(.key-value-item) {
        min-height: 30px;
        margin-top: 0;
        border-width: 0 0 1px;
        border-radius: 0;
        background: transparent;
        padding: 4px 0;

        &:last-child {
          border-bottom: 0;
        }
      }
    }

    .thing-action-stack {
      display: flex;
      flex-direction: column;
      gap: 12px;

      > .el-button {
        width: 100%;
        margin-left: 0;
      }
    }

    .thing-shadow-panel {
      display: flex;
      flex-direction: column;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
      padding-left: 2px;
      background: transparent;

      .thing-shadow-state {
        flex: 1;
        min-height: 0;
        border-top: 1px solid var(--tio-line);
      }
    }
  }
}

.thing-meta-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  min-height: 30px;
  margin-bottom: 4px;
  padding: 4px 0 8px;
  border-bottom: 1px solid var(--tio-border);
  color: var(--tio-text);

  .thing-meta-label {
    margin-right: 10px;
    color: var(--tio-muted);
    font-size: 12px;
    font-weight: 700;
  }

  .thing-meta-value {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    min-height: 24px;
  }
}

@media (max-width: 768px) {
  .thing-view {
    overflow-y: auto;

    .thing-summary {
      flex-direction: column;
      gap: 12px;

      .thing-summary-main {
        flex-direction: column;
        gap: 10px;
      }

      .thing-title-row {
        align-items: flex-start;
        flex-direction: column;
      }

      .thing-summary-actions {
        align-self: flex-end;
      }
    }

    .thing-workspace {
      grid-template-columns: 1fr;
      overflow: visible;

      .thing-sidebar,
      .thing-shadow-panel {
        overflow: visible;
      }

      .thing-sidebar {
        padding-right: 0;
        border-right: 0;
      }

      .thing-shadow-panel {
        padding-left: 0;
      }
    }
  }
}
</style>

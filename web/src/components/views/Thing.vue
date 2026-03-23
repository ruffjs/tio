<template>
  <div class="thing-view">
    <!-- 顶部状态栏 -->
    <div class="thing-view-header">
      <div class="thing-view-header-left">
        <el-button type="text" icon="Back" plain @click="handleBack2List">{{ $t('things.backToList') }}</el-button>
        <div class="thing-status-card">
          <div class="status-indicator">
            <div class="status-dot" :class="{ connected: shadow.connected }"></div>
            <span class="status-text">{{ shadow.connected ? $t('things.connected') : $t('things.disconnected') }}</span>
          </div>
          <div class="status-info">
            <div class="info-item" v-if="thing.remoteAddr">
              <el-icon>
                <Location />
              </el-icon>
              <span>{{ thing.remoteAddr }}</span>
            </div>
            <div class="info-item" v-if="thing.version">
              <el-icon>
                <Document />
              </el-icon>
              <span>{{ $t('things.version') }}: {{ thing.version }}</span>
            </div>
          </div>
        </div>
      </div>
      <div class="thing-view-header-right">
        <el-tooltip :content="$t('nav.kickOut')">
          <el-button v-if="shadow?.connected" type="danger" plain circle icon="RemoveFilled"
            @click="handleKickOutSelected" />
        </el-tooltip>
        <el-tooltip :content="$t('things.reloadData')">
          <el-button icon="RefreshRight" plain circle @click="refreshThingData" />
        </el-tooltip>
      </div>
    </div>

    <!-- 主要内容区域 -->
    <div class="thing-view-content">
      <div class="thing-view-left">
        <div class="thing-info-card">
          <div class="card-header">
            <h3>{{ $t('things.basicInfo') }}</h3>
          </div>
          <div class="card-content">
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
          </div>
        </div>

        <div class="thing-actions-card">
          <div class="card-header">
            <h3>{{ $t('things.actions') }}</h3>
          </div>
          <div class="card-content">
            <el-button icon="TopRight" type="primary" plain @click="(posterCode = 'invoke'), (posterData = null)" block>
              {{ $t('things.requestDirectMethod') }}
            </el-button>
            <MqttClients />
          </div>
        </div>
      </div>

      <div class="thing-view-right">
        <!-- <ShadowProps :shadow="shadow" /> -->
        <ShadowTags :data="shadow?.tags"
          @update="(payload) => ((posterCode = 'tags'), (posterData = payload || null))" />
        <div class="thing-view-state">
          <ShadowData @call="(code) => ((posterCode = code), (posterData = null))" />
        </div>
      </div>
    </div>
  </div>
  <HttpPoster :code="posterCode" :thing-id="thingId" :payload="posterData" @done="updateCurrentShadow"
    @close="posterCode = ''" />
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
import ShadowProps from "@/components/thing/ShadowProps.vue";
import ShadowTags from "@/components/thing/ShadowTags.vue";
import ShadowData from "@/components/thing/ShadowData.vue";
import HttpPoster from "@/components/thing/HttpPoster.vue";
import MqttClients from "@/components/thing/MqttClientsOfThing.vue";
import dayjs from "dayjs";
import {
  TH_STATUS_CHG_EVT,
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
const updatingEnabled = ref(false);
const metaFields = computed(() => createMetaFields(t).filter(({ key }) => key !== "enabled"));

const formatTime = (time) => {
  return time ? dayjs(time).format("YYYY-MM-DD HH:mm:ss") : "-";
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
  if (shadow.value.fromList) isFromList.value = true;
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
  background-color: #f5f7fa;
  border-bottom-left-radius: 6px;
  border-bottom-right-radius: 6px;

  .thing-view-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 20px;
    background-color: white;
    border-bottom: 1px solid #e4e7ed;
    border-top-left-radius: 6px;
    border-top-right-radius: 6px;
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05);

    .thing-view-header-left {
      display: flex;
      align-items: center;
      gap: 16px;

      .thing-status-card {
        display: flex;
        align-items: center;
        gap: 16px;
        padding: 4px 8px;
        background-color: #f8f9fa;
        border-radius: 8px;
        border: 1px solid #e9ecef;

        .status-indicator {
          display: flex;
          align-items: center;
          gap: 8px;

          .status-dot {
            width: 8px;
            height: 8px;
            border-radius: 50%;
            background-color: #dcdfe6;

            &.connected {
              background-color: #67c23a;
            }
          }

          .status-text {
            font-weight: 500;
            color: #303133;
          }
        }

        .status-info {
          display: flex;
          align-items: center;
          gap: 16px;

          .info-item {
            display: flex;
            align-items: center;
            gap: 4px;
            font-size: 13px;
            color: #606266;
          }
        }
      }
    }
  }

  .thing-view-content {
    display: flex;
    flex: 1;
    gap: 16px;
    padding: 16px 20px;
    overflow: hidden;

    .thing-view-left {
      width: 280px;
      display: flex;
      flex-direction: column;
      gap: 16px;
      overflow-y: auto;

      .thing-info-card,
      .thing-actions-card {
        background-color: white;
        border-radius: 12px;
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
        border: 1px solid #e4e7ed;
        overflow: hidden;

        .card-header {
          display: flex;
          align-items: center;
          justify-content: space-between;
          padding: 16px 20px;
          border-bottom: 1px solid #f0f0f0;
          background-color: #fafafa;

          h3 {
            margin: 0;
            font-size: 16px;
            font-weight: 600;
            color: #303133;
          }
        }

        .card-content {
          padding: 16px 20px;
        }
      }

      .thing-actions-card {
        .card-content {
          display: flex;
          flex-direction: column;
          gap: 12px;
        }
      }
    }

    .thing-view-right {
      flex: 1;
      display: flex;
      flex-direction: column;
      gap: 16px;
      overflow-y: auto;

      .thing-view-state {
        flex: 1;
        min-height: 0;
      }
    }
  }
}

.thing-meta-item {
  display: flex;
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
  width: 100%;
  min-height: 28px;
  margin-top: 2px;
  margin-bottom: 2px;
  padding: 1px 5px;
  border: solid 1px rgba(0, 0, 0, 0.1);
  border-radius: 5px 5px 2px 2px;

  .thing-meta-label {
    margin-right: 10px;
    font-size: 12px;
    font-weight: 700;
    color: #555;
  }

  .thing-meta-value {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    min-height: 24px;
  }
}

// 响应式设计
@media (max-width: 768px) {
  .thing-view {
    .thing-view-header {
      flex-direction: column;
      gap: 12px;
      align-items: stretch;

      .thing-view-header-left {
        flex-direction: column;
        gap: 12px;
        align-items: stretch;

        .thing-status-card {
          flex-direction: column;
          gap: 12px;
          align-items: stretch;

          .status-info {
            flex-direction: column;
            gap: 8px;
            align-items: flex-start;
          }
        }
      }
    }

    .thing-view-content {
      flex-direction: column;
      padding: 12px;

      .thing-view-left {
        width: 100%;
      }
    }
  }
}
</style>

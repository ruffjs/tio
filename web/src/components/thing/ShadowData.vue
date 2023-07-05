<template>
  <el-card class="shadow-state-card" shadow="never">
    <template #header>
      <div class="shadow-state-card-header">
        <span>Shadow Data</span>
        <div class="shadow-state-card-buttons">
          <!-- <el-button
            icon="View"
            size="small"
            @click="viewObject(currentShadow, 'Shadow Raw', false)"
            >View Raw</el-button
          > -->
          <el-button size="small" @click="handleCompareState">Compare State</el-button>
          <el-button size="small" @click="handleCheckDelta">Check Delta</el-button>
          <el-divider direction="vertical" />
          <el-button size="small" @click="emit('call', 'desire')">Set Desired</el-button>
          <el-button size="small" @click="handleSetReported">Set Reported</el-button>
        </div>
      </div>
    </template>
    <div class="shadow-state-card-main">
      <JSONEditor
        mode="tree"
        :model-value="JSON.stringify(currentShadow)"
        read-only
        class="shadow-state-card-code"
      />
    </div>
  </el-card>
  <ObjectViewer
    :visible="!!objectToBeView"
    :data="objectToBeView"
    :type="titleOfViewer"
    :as-tree="viewObjectAsTree"
    @close="handleCloseViewers"
  />
  <StateViewer
    :visible="!!selectedState"
    :data="selectedState"
    :type="titleOfViewer"
    @close="handleCloseViewers"
  />
  <MqttPublish
    :visible="isMqttPublishShown"
    :conn-config="mqttPublishConnConf"
    :type="mqttPublishTitle"
    :topic="mqttPublishTopic"
    :payload="mqttPublishPayload"
    :paytype="mqttPublishPaytype"
    @close="handleCloseMqttPublish"
    @done="handleDoneMqttPublish"
  />
</template>

<script setup>
import { computed, ref, shallowRef } from "vue";
import { ElMessageBox } from "element-plus";
import { diffState } from "@/utils/shadow";
import { genMqttClientToken } from "@/utils/generators";
import ObjectViewer from "@/components/common/ObjectViewer.vue";
import StateViewer from "@/components/common/StateViewer.vue";
import MqttPublish from "@/components/thing/MqttPublishForThingView.vue";
import JSONEditor from "../common/JSONEditor.vue";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import useLayout from "@/reactives/useLayout";
import useObjectViewer from "@/reactives/useObjectViewer";

import useMqtt from "@/reactives/useMqtt";
import { TH_STATUS_CHG_EVT, TSCE_MQTT } from "@/utils/event";
import { genConnectedCallbackToken } from "@/utils/generators";
import useThingEvent from "@/reactives/useThingEvent";

const emit = defineEmits(["call"]);
const ccbt = shallowRef("");
const { currentShadow, updateCurrentShadow } = useThingsAndShadows();
const {
  delegateSharedStates,
  getConnConfigsByClientId,
  selectConnection,
  connect,
} = useMqtt();
const { activeToolKey, switchActiveTool, showMqttConnForm } = useLayout();
const {
  objectToBeView,
  titleOfViewer,
  viewObjectAsTree,
  viewObject,
  handleCloseViewer,
} = useObjectViewer();
const { onSomethingStatusChange } = useThingEvent();

const selectedState = ref(null);

const isMqttPublishShown = ref(false);
const mqttPublishConnConf = ref(null);
const mqttPublishTitle = ref("");
const mqttPublishTopic = ref("");
const mqttPublishPayload = ref("");
const mqttPublishPaytype = ref("");

const handleCompareState = () => {
  selectedState.value = currentShadow.value.state;
  objectToBeView.value = null;
  titleOfViewer.value = "both";
};
const handleCheckDelta = () => {
  selectedState.value = null;
  const [hasDelta, delta] = diffState(currentShadow.value.state);
  if (hasDelta) {
    viewObject(delta, "Delta", true);
  } else {
    ElMessageBox.alert("These is no delta property", "No Delta", {
      confirmButtonText: "OK",
    });
  }
};
const handleCloseViewers = () => {
  selectedState.value = null;
  handleCloseViewer();
};
const handleCloseMqttPublish = () => {
  isMqttPublishShown.value = false;
  mqttPublishConnConf.value = null;
  mqttPublishTitle.value = "";
  mqttPublishTopic.value = "";
  mqttPublishPayload.value = "";
  mqttPublishPaytype.value = "";
};
const handleDoneMqttPublish = (err) => {
  console.log("set reported error", err);
  if (!err) handleCloseMqttPublish();
};
const showSetReportedForm = (config) => {
  ccbt.value = "";
  selectConnection(config);
  mqttPublishConnConf.value = config;
  mqttPublishTitle.value = "Set Reported";
  mqttPublishTopic.value = `$iothub/things/${currentShadow.value.thingId}/shadows/name/default/update`;
  mqttPublishPayload.value = JSON.stringify(
    {
      state: {
        reported: {},
      },
      clientToken: genMqttClientToken(),
      version: 0,
    },
    null,
    2
  );
  mqttPublishPaytype.value = "JSON";
  isMqttPublishShown.value = true;
};

const confirmForCreateMqttClient = () => {
  ElMessageBox.confirm(
    "<p>No matching client yet, would you like to create one?</p><p>It will subscribe all suggested topics after connected.</p>",
    "Confirm",
    {
      // distinguishCancelAndClose: true,
      dangerouslyUseHTMLString: true,
      confirmButtonText: "Create",
      cancelButtonText: "Cancel",
    }
  )
    .then(() => {
      ccbt.value = showMqttConnForm(null, currentShadow.value.thingId);
    })
    .catch((action) => {
      console.log("Cancel to set reported", action);
    });
};
const confirmForConnectMqttClient = (config) => {
  ElMessageBox.confirm(
    "<p>No connected matching client yet, would you like to connect the first one of matching clients?</p><p>It will subscribe all suggested topics after connected.</p>",
    "Confirm",
    {
      // distinguishCancelAndClose: true,
      dangerouslyUseHTMLString: true,
      confirmButtonText: "Connect",
      cancelButtonText: "Cancel",
    }
  )
    .then(() => {
      ccbt.value = genConnectedCallbackToken();
      selectConnection(config);
      connect(config, ccbt.value);
    })
    .catch((action) => {
      console.log("Cancel to set reported", action);
    });
};
const handleSetReported = () => {
  const conns = getConnConfigsByClientId(currentShadow.value.thingId);
  if (conns.length > 0) {
    const activedConnConf = conns.find(
      (config) => delegateSharedStates.value[config.id]?.client?.connected
    );
    if (activedConnConf) {
      showSetReportedForm(activedConnConf);
    } else {
      // console.log("No actived Conn");
      confirmForConnectMqttClient(conns[0]);
    }
  } else {
    confirmForCreateMqttClient();
  }
};
</script>

<style scoped lang="scss">
.shadow-state-card {
  width: 100%;
  margin-top: 10px;

  .shadow-state-card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;

    .el-divider--vertical {
      margin: 0 14px;
    }
  }

  .shadow-state-card-main {
    .shadow-state-card-code {
      position: relative;
      width: 100%;
      height: auto;
      overflow: hidden;
    }
  }
}
</style>

<style lang="scss">
.shadow-state-card {
  .el-card__header {
    padding: 10px var(--el-card-padding);
  }

  .el-card__body {
    padding: 0;
    .shadow-state-card-main {
      .shadow-state-card-code {
        .jse-main {
          position: relative;
          height: auto;
          max-height: 450px;
          .jse-tree-mode {
            border: none;
            .jse-contents {
              border: none;
            }
          }
        }
      }
    }
  }
}
</style>

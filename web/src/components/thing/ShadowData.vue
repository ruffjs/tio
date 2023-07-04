<template>
  <el-card class="shadow-state-card" shadow="never">
    <template #header>
      <div class="shadow-state-card-header">
        <span>Shadow Data</span>
        <div class="shadow-state-card-buttons">
          <el-button size="small" @click="handleCompareState">Compare State</el-button>
          <el-button size="small" @click="handleCheckDelta">Check Delta</el-button>
          <el-button size="small" @click="emit('call', 'desire')">Set Desired</el-button>
          <el-button size="small" @click="handleSetReported">Set Reported</el-button>
        </div>
      </div>
    </template>
    <div class="shadow-state-card-main">
      <pre class="shadow-state-card-code">{{
        JSON.stringify(states[0].data, null, 2)
      }}</pre>
      <!-- <el-collapse v-model="activeNames">
        <el-collapse-item
          v-for="state in states"
          :title="state.title"
          :name="state.key"
          :key="state.key"
        >
          <pre class="shadow-state-card-code">{{
            JSON.stringify(state.data, null, 2)
          }}</pre>
        </el-collapse-item>
      </el-collapse> -->
    </div>
  </el-card>
  <ObjectViewer
    :visible="!!selectedObject"
    :data="selectedObject"
    :type="selectedType"
    @close="handleCloseViewers"
  />
  <StateViewer
    :visible="!!selectedState"
    :data="selectedState"
    :type="selectedType"
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
import { computed, onMounted, onUnmounted, ref, shallowRef } from "vue";
import { ElMessageBox } from "element-plus";
import { diffState } from "@/utils/shadow";
import { genMqttClientToken } from "@/utils/generators";
import ObjectViewer from "@/components/common/ObjectViewer.vue";
import StateViewer from "@/components/common/StateViewer.vue";
import MqttPublish from "@/components/thing/MqttPublishForThingView.vue";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import useLayout from "@/reactives/useLayout";
import useObjectViewer from "@/reactives/useObjectViewer";

import useMqtt from "@/reactives/useMqtt";
import { TH_STATUS_CHG_EVT, TSCE_MQTT } from "@/utils/event";
import { genConnectedCallbackToken } from "@/utils/generators";

const emit = defineEmits(["call"]);
const { currentShadow, updateCurrentShadow } = useThingsAndShadows();
const {
  delegateSharedStates,
  getConnConfigsByClientId,
  selectConnection,
  connect,
} = useMqtt();
const { activeToolKey, switchActiveTool, showMqttForm } = useLayout();
const activeNames = ["shadow"];
const states = computed(() => {
  try {
    const {
      state: { desired, reported },
      metadata,
    } = currentShadow.value;
    return [
      {
        key: "shadow",
        title: "Whole Shadow",
        data: currentShadow.value,
      },
      {
        key: "desired",
        title: "Desired",
        data: { state: { desired } },
      },
      {
        key: "reported",
        title: "Reported",
        data: { state: { reported } },
      },
      {
        key: "metadata",
        title: "Meta Data",
        data: { metadata },
      },
    ];
  } catch (error) {
    return [
      {
        key: "shadow",
        title: "Whole Shadow",
        data: currentShadow.value,
      },
    ];
  }
});
const { selectedObject, selectedType, viewObject, handleCloseViewer } = useObjectViewer();
const selectedState = ref(null);
const ccbt = shallowRef("");
const isMqttPublishShown = ref(false);
const mqttPublishConnConf = ref(null);
const mqttPublishTitle = ref("");
const mqttPublishTopic = ref("");
const mqttPublishPayload = ref("");
const mqttPublishPaytype = ref("");

const handleCompareState = () => {
  selectedState.value = currentShadow.value.state;
  selectedObject.value = null;
  selectedType.value = "both";
};
const handleCheckDelta = () => {
  selectedState.value = null;
  const [hasDelta, delta] = diffState(currentShadow.value.state);
  if (hasDelta) {
    viewObject(delta, "Delta");
  } else {
    ElMessageBox.alert("This is no delta property", "No Delta", {
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
const onSomethingStatusChange = (message) => {
  const { thingId, type, about } = message.detail;
  if (
    thingId === currentShadow.value.thingId &&
    about.connectedToken &&
    about.connectedToken === ccbt.value &&
    type === TSCE_MQTT
  ) {
    showSetReportedForm(about.connConfig);
  }
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
      ccbt.value = showMqttForm(null, currentShadow.value.thingId);
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
onMounted(() => {
  window.addEventListener(TH_STATUS_CHG_EVT, onSomethingStatusChange);
});
onUnmounted(() => {
  window.removeEventListener(TH_STATUS_CHG_EVT, onSomethingStatusChange);
});
</script>

<style scoped lang="scss">
.shadow-state-card {
  width: 100%;
  margin-top: 10px;

  .shadow-state-card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .shadow-state-card-main {
    .shadow-state-card-code {
      width: 100%;
      height: auto;
      margin: 0;
      padding: 2px 5px;
      border-radius: 2px;
      background-color: #444;
      color: white;
      font-size: 13px;
      line-height: 16px;
      overflow-x: auto;
      overflow-y: hidden;
    }
  }
}
</style>

<style lang="scss">
.shadow-state-card {
  .el-card__header {
    padding: 10px var(--el-card-padding);
  }
}
</style>

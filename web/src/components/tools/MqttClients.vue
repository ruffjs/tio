<template>
  <div
    class="mqtt-clients-panel"
    v-loading="
      connecting
        ? {
            text: retryTimes ? 'Retry: ' + retryTimes : 'Connecting...',
          }
        : false
    "
  >
    <div class="mqtt-clients-conns">
      <div class="mqtt-clients-conns-header">
        <div class="mqtt-clients-title">{{ $t('mqtt.clientConnections') }}</div>
        <div class="mqtt-clients-add">
          <el-tooltip :content="$t('mqtt.clickToCreateMqttClient')" placement="top">
            <el-button
              type="default"
              icon="Plus"
              size="small"
              @click="showMqttConnForm()"
            />
          </el-tooltip>
          <el-tooltip :content="$t('mqtt.viewBrokerStatsTip')" placement="top">
            <el-button
              type="default"
              icon="Odometer"
              size="small"
              @click="handleCheckStats"
            />
          </el-tooltip>
        </div>
      </div>
      <div class="mqtt-clients-conns-list">
        <el-button
          v-for="c in connections"
          :key="c.id"
          icon="SwitchButton"
          :type="c.client?.connected ? 'success' : ''"
          :plain="c.id === currentConnId"
          :text="c.id !== currentConnId"
          :bg="c.id !== currentConnId"
          @click="selectConnection(c.config)"
          class="mqtt-clients-conn"
        >
          {{ c.name }}
        </el-button>
      </div>
    </div>
    <div class="mqtt-clients-curr">
      <div class="mqtt-clients-curr-header">
        <template v-if="currentConnId">
          <div :class="['mqtt-clients-curr-name', isCurrentConnected ? 'active' : '']">
            {{ selectedConn.name }}
            <el-tag :type="isCurrentConnected ? 'success' : 'info'" size="small">{{
              selectedConn.userrole === "thing" ? "Thing" : "Server"
            }}</el-tag>
          </div>
          <div class="mqtt-clients-curr-opts">
            <SwitchSizeButton />
            <el-button
              v-for="op in opts"
              :content="op.tip"
              :key="op.key"
              :type="op.type"
              :icon="op.icon"
              :disabled="op.disabled"
              size="small"
              plain
              @click="handleOpt(op.key)"
              >{{ op.label }}</el-button
            >
          </div>
        </template>
        <div v-else class="mqtt-clients-curr-name">No Selected Connection</div>
      </div>
      <MqttClientDetail @request-add-conn="showMqttConnForm()" />
    </div>
  </div>
  <MqttConnForm @close="handleCreateOrEditCancel" @done="handleCreateOrEditDone" />

  <ObjectViewer
    :visible="!!objectToBeView"
    :data="objectToBeView"
    :type="titleOfViewer"
    as-tree
    @close="handleCloseViewer"
  />
</template>

<script setup>
import { computed, ref } from "vue";
import ObjectViewer from "@/components/common/ObjectViewer.vue";
import MqttConnForm from "@/components/mqtt/MqttConnForm.vue";
import MqttClientDetail from "@/components/mqtt/MqttClientDetail.vue";
import SwitchSizeButton from "@/components/common/SwitchSizeButton.vue";
import useMqtt from "@/reactives/useMqtt";
import useObjectViewer from "@/reactives/useObjectViewer";
import useLayout from "@/reactives/useLayout";
import { getBrokerStats } from "@/apis";
import { getSuggestedTopicsForThing } from "@/utils/subs";
import { useI18n } from "vue-i18n";

const { t } = useI18n();
const {
  connecting,
  retryTimes,
  connections,
  currentConnId,
  selectedConn,
  setConnConfig,
  selectConnection,
  removeConnection,
  connect,
  disconn,
} = useMqtt();
const { isForCreate, showMqttConnForm, hideMqttConnForm } = useLayout();
const isCurrentConnected = computed(() => {
  return selectedConn.value?.client?.connected;
});
const opts = computed(() => {
  const ops = [
    {
      key: "delete",
      label: t('common.delete'),
      icon: "Delete",
      type: "danger",
      disabled: isCurrentConnected.value,
    },
    {
      key: "edit",
      label: t('common.edit'),
      icon: "EditPen",
      type: "info",
      disabled: isCurrentConnected.value,
    },
  ];
  if (isCurrentConnected.value) {
    ops.push({
      key: "disconn",
      label: t('mqtt.disconnect'),
      icon: "SwitchButton",
      type: "warning",
    });
  } else {
    ops.push({
      key: "connect",
      label: t('mqtt.connect'),
      icon: "SwitchButton",
      type: "success",
    });
  }
  return ops;
});

const handleOpt = (key) => {
  switch (key) {
    case "connect":
      connect(selectedConn.value.config);
      break;

    case "disconn":
      disconn(selectedConn.value.config);
      break;

    case "edit":
      showMqttConnForm(currentConnId.value);
      break;

    case "delete":
      removeConnection(currentConnId.value);
      break;
  }
};

const handleCreateOrEditCancel = () => {
  hideMqttConnForm();
};

const handleCreateOrEditDone = (config, connectedToken) => {
  if (isForCreate) {
    selectConnection(config);
    if (config.userrole === "thing") {
      config.subscriptions = JSON.parse(
        JSON.stringify(getSuggestedTopicsForThing(config.username, config))
      );
      setConnConfig(config.id, config);
    }
  }
  handleCreateOrEditCancel();
  connect(selectedConn.value.config, isForCreate ? connectedToken : "");
};

const {
  objectToBeView,
  titleOfViewer,
  viewObject,
  handleCloseViewer,
} = useObjectViewer();
const handleCheckStats = async () => {
  try {
    viewObject((await getBrokerStats()).data, "Embeded MQTT Broker Stats");
  } catch (error) {
    console.error("error", error);
  }
};
</script>

<style scoped lang="scss">
.mqtt-clients-panel {
  display: flex;
  flex-direction: row;

  width: 100%;
  height: 100%;
  background: var(--tio-surface-solid);
  color: var(--tio-text);

  .mqtt-clients-conns-header,
  .mqtt-clients-curr-header {
    display: flex;
    flex-direction: row;
    justify-content: space-between;
    align-items: center;

    width: 100%;
    height: 32px;
    background: var(--tio-surface-soft);
    border-top: 1px solid var(--tio-border);
    border-bottom: 1px solid var(--tio-border);
  }

  .mqtt-clients-conns {
    width: 240px;
    height: 100%;
    border-right: 1px solid var(--tio-border);

    .mqtt-clients-title {
      padding-left: 5px;
      font-size: 16px;
      text-transform: uppercase;
      letter-spacing: 1px;
      overflow: hidden;
    }

    .mqtt-clients-add {
      display: flex;
      flex-direction: row-reverse;
      justify-content: start;
      align-items: center;

      height: 28px;
      padding-right: 5px;
      overflow: hidden;

      .el-button + .el-button {
        margin-left: 0;
        margin-right: 5px;
      }
    }

    .mqtt-clients-conns-list {
      width: 100%;
      height: calc(100% - 32px);
      padding: 5px 0;
      overflow-x: hidden;
      overflow-y: auto;
      .mqtt-clients-conn {
        justify-content: start;
        width: calc(100% - 10px);
        margin: 2px 5px;
        > span {
          display: inline-block;
          overflow: hidden;
          text-overflow: ellipsis;
        }
      }
    }
  }
  .mqtt-clients-curr {
    flex: 1;
    width: 0;
    height: 100%;
    text-align: left;

    .mqtt-clients-curr-header {
      padding: 0 5px;
      .mqtt-clients-curr-name {
        line-height: 30px;
        font-size: 16px;
        font-weight: 700;
        color: var(--tio-muted);
        &.active {
          color: var(--el-color-success);
        }
      }
      .mqtt-clients-curr-opts {
        display: flex;
        flex-direction: row-reverse;
        justify-content: start;
        align-items: center;
        height: 28px;
        padding: 2px;
        .el-button + .el-button {
          margin-left: 0;
          margin-right: 10px;
        }
      }
    }
  }
}
</style>

<style lang="scss">
.mqtt-clients-panel {
  .mqtt-clients-conns {
    .mqtt-clients-conns-list {
      .mqtt-clients-conn {
        > span {
          display: inline-block;
          overflow: hidden;
          text-overflow: ellipsis;
        }
      }
    }
  }
}
</style>

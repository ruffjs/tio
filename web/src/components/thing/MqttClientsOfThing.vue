<template>
  <div class="thing-mqtt-clients">
    <div class="thing-mqtt-clients-title">MQTT Clients of Thing</div>
    <el-collapse v-if="clients.length" v-model="activeName" accordion>
      <el-collapse-item v-for="(c, index) in clients" :name="c.id">
        <template #title>
          <div
            :class="{
              'thing-mqtt-client-header': true,
              connected: c.client.connected,
            }"
          >
            <el-icon><SwitchButton /></el-icon>
            <span>{{ c.name }}</span>
          </div>
        </template>
        <div class="thing-mqtt-innerbtns">
          <!-- <pre>{{ JSON.stringify(client, null, 2) }}</pre> -->
          <el-row :gutter="10">
            <el-col :span="24">
              <el-button
                v-if="c.client.connected"
                class="thing-mqtt-innerbtn"
                type="success"
                size="small"
                plain
                @click="handleDisconnectMqttClient(c.config)"
                >Disconnect</el-button
              >
              <el-button
                v-else
                class="thing-mqtt-innerbtn"
                size="small"
                @click="handleConnectMqttClient(c.config)"
                >Connect and Subscribe</el-button
              >
            </el-col>
            <el-col :span="24">
              <el-button
                :disabled="!c.client.connected"
                size="small"
                class="thing-mqtt-innerbtn subscription-stats"
                @click="handleShowSubscriptions(c.config)"
                ><span>Subscription</span
                ><span
                  >{{ getSubscribedSubs(c.subs).length }} /
                  {{ c.subscriptions.length }}</span
                ></el-button
              >
            </el-col>
            <el-col :span="12">
              <el-button
                :disabled="!c.client.connected"
                type="primary"
                size="small"
                class="thing-mqtt-innerbtn"
                plain
                @click="handleSubscribeAll(c.config, c.subs)"
                >Subs All</el-button
              >
            </el-col>
            <el-col :span="12">
              <el-button
                :disabled="!c.client.connected"
                type="warning"
                size="small"
                class="thing-mqtt-innerbtn"
                plain
                @click="handleUnsubscribeAll(c.config, c.subs)"
                >Unsub All</el-button
              >
            </el-col>
            <el-col :span="24">
              <el-badge
                :value="c.unreadMessageCount || 0"
                :max="999"
                :hidden="c.unreadMessageCount == 0"
                class="thing-mqtt-badge"
                type="primary"
              >
                <el-button
                  class="thing-mqtt-innerbtn"
                  size="small"
                  @click="handleShowToolPanel(c.config)"
                >
                  Show in tool-panel
                </el-button>
              </el-badge>
            </el-col>
          </el-row>
        </div>
      </el-collapse-item>
    </el-collapse>
    <template v-else>
      <el-tooltip
        content="Click to create and connect a mqtt client for this thing, and subscribe all suggested topics."
        placement="right"
      >
        <el-button
          icon="Connection"
          class="thing-mqtt-bigbtn"
          @click="handleCreateMqttClient(false)"
          >Create Client</el-button
        ></el-tooltip
      >
    </template>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref, shallowRef, watch } from "vue";
import useMqtt from "@/reactives/useMqtt";
import useLayout from "@/reactives/useLayout";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import { TH_STATUS_CHG_EVT } from "@/utils/event";
import { TSCE_MQTT } from "@/utils/event";
import { genConnectedCallbackToken } from "@/utils/generators";
import {
  convertSubsConfigSubMap,
  getSubscribedSubs,
  subscribeAll,
  unsubscribeAll,
} from "@/utils/subs";

const { selectedThingId } = useThingsAndShadows();
const {
  connections,
  delegateSharedStates,
  getConnConfigsByClientId,
  selectConnection,
  connect,
  disconn,
  subscribe,
  unsubscribe,
} = useMqtt();
const { activeToolKey, switchActiveTool, showMqttForm, showMqttSubs } = useLayout();
const clients = ref([]);
const activeName = ref("");
const ccbt = shallowRef("");

const onSomethingStatusChange = (message) => {
  const { thingId, type, about } = message.detail;
  if (
    thingId === selectedThingId.value &&
    about.connectedToken &&
    about.connectedToken === ccbt.value &&
    type === TSCE_MQTT
  ) {
    ccbt.value = "";
    // console.log("connected");
    const configs = about.connConfig.subscriptions.filter((sub) => sub.keep);
    const subMap = convertSubsConfigSubMap(configs);
    // console.log("subMap", subMap);
    subscribe(about.connConfig, { topic: subMap, multiple: true, configs });
  }
};

const handleDisconnectMqttClient = (config) => {
  disconn(config);
};
const handleConnectMqttClient = (config) => {
  ccbt.value = genConnectedCallbackToken();
  connect(config, ccbt.value);
};
const handleSubscribeAll = (config, subs) => {
  subscribeAll({
    config,
    subMap: subs || {},
    subscribe,
  });
};
const handleUnsubscribeAll = (config, subs) => {
  unsubscribeAll({
    config,
    subMap: subs || {},
    unsubscribe,
  });
};
const handleShowSubscriptions = (config) => {
  if (
    delegateSharedStates.value[config.id] &&
    delegateSharedStates.value[config.id].client?.connected
  ) {
    showMqttSubs(config);
  }
};
const handleShowToolPanel = (config) => {
  if (activeToolKey.value !== "mqtt") {
    switchActiveTool({ key: "mqtt" });
  }
  selectConnection(config);
};

const handleCreateMqttClient = () => {
  ccbt.value = showMqttForm(null, selectedThingId.value);
};

watch(
  [selectedThingId, connections, delegateSharedStates],
  () => {
    clients.value = getConnConfigsByClientId(selectedThingId.value).map((config) => {
      const {
        id,
        clientId,
        name,
        mqttVersion,
        protocol,
        host,
        port,
        subscriptions,
      } = config;
      const { client, subs, messages, unreadMessageCount } =
        delegateSharedStates.value[id] || {};
      if (activeName.value === "" && client?.connected) {
        activeName.value = id;
      }
      return {
        config,
        id,
        clientId,
        name,
        mqttVersion,
        protocol,
        host,
        port,
        client: client || { connected: false },
        subs: subs || {},
        subscriptions: subscriptions || [],
        messages: messages || [],
        unreadMessageCount: unreadMessageCount || 0,
      };
    });
  },
  { immediate: true }
);

onMounted(() => {
  window.addEventListener(TH_STATUS_CHG_EVT, onSomethingStatusChange);
});
onUnmounted(() => {
  window.removeEventListener(TH_STATUS_CHG_EVT, onSomethingStatusChange);
});
</script>

<style scoped lang="scss">
.thing-mqtt-clients {
  width: 100%;
  margin-top: 14px;
  padding-bottom: 10px;
  border-top: solid 2px rgba($color: #000000, $alpha: 0.1);
  border-radius: 5px;
  text-align: center;

  .thing-mqtt-clients-title {
    width: 100%;
    height: 40px;
    line-height: 40px;
    text-align: center;
    font-size: 13px;
    font-weight: 700;
  }
  .thing-mqtt-client-header {
    display: flex;
    flex-direction: row;
    justify-content: start;
    align-items: center;
    gap: 5px;
    color: #666;
    &.connected {
      color: var(--el-color-success);
    }
  }
  .thing-mqtt-bigbtn.el-button {
    width: 100%;
    height: 48px;
    border-left: none;
    border-right: none;
    border-radius: 0;
    font-weight: 300;
    font-size: 12px;
  }

  .thing-mqtt-innerbtns {
    width: 100%;
    padding: 0 10px;

    .thing-mqtt-badge {
      margin-top: 4px;
      width: 100%;
    }
    .thing-mqtt-innerbtn.el-button {
      width: 100%;
      margin-bottom: 8px;
    }
  }
}
</style>

<style lang="scss">
.thing-mqtt-clients {
  .el-collapse-item__content {
    padding-bottom: 5px;
  }
  .thing-mqtt-innerbtns {
    .thing-mqtt-innerbtn.subscription-stats.el-button {
      > span {
        display: flex;
        flex-direction: row;
        justify-content: space-between;
        align-items: center;
        width: 100%;
      }
    }
  }
}
</style>
@/utils/subs

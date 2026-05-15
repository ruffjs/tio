<template>
  <div class="thing-mqtt-clients">
    <div class="thing-mqtt-clients-title">
      <span class="thing-mqtt-title-label">
        {{ $t('things.mqttClientsOfThing') }}
        <b v-if="totalCount" class="thing-mqtt-count">{{ connectedCount }}/{{ totalCount }}</b>
      </span>
    </div>
    <div v-if="clients.length" class="thing-mqtt-list">
      <div
        v-for="c in clients"
        :key="c.id"
        :class="['thing-mqtt-client', c.client.connected ? 'connected' : '']"
      >
        <div class="thing-mqtt-client-main">
          <i class="thing-mqtt-status-dot"></i>
          <el-tooltip effect="dark" :content="c.name" placement="top-start">
            <span class="thing-mqtt-client-name">{{ c.name }}</span>
          </el-tooltip>
        </div>
        <div class="thing-mqtt-client-meta">
          <span class="thing-mqtt-sub-count">
            {{ getSubscribedSubs(c.subs).length }}/{{ c.subscriptions.length }}
          </span>
          <span v-if="c.unreadMessageCount" class="thing-mqtt-unread">
            {{ c.unreadMessageCount > 99 ? "99+" : c.unreadMessageCount }}
          </span>
          <el-dropdown
            trigger="click"
            popper-class="thing-mqtt-clients-popper"
            @command="(command) => handleClientCommand(command, c)"
          >
            <el-button class="thing-mqtt-more" link icon="MoreFilled" />
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item v-if="c.client.connected" command="disconnect">
                  {{ $t('mqtt.disconnect') }}
                </el-dropdown-item>
                <el-dropdown-item v-else command="connect">
                  {{ $t('mqtt.connectAndSubscribe') }}
                </el-dropdown-item>
                <el-dropdown-item v-if="c.client.connected" command="subscribe" divided>
                  {{ $t('mqtt.subsAll') }}
                </el-dropdown-item>
                <el-dropdown-item v-if="c.client.connected" command="unsubscribe">
                  {{ $t('mqtt.unsubAll') }}
                </el-dropdown-item>
                <el-dropdown-item command="panel" :divided="c.client.connected">
                  {{ $t('mqtt.showInToolPanel') }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>
    </div>
    <div v-else class="thing-mqtt-empty">
      <p>{{ $t('mqtt.createClientTooltip') }}</p>
      <el-button
        icon="Connection"
        size="small"
        plain
        @click="handleCreateMqttClient(false)"
      >
        {{ $t('mqtt.createClient') }}
      </el-button>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, shallowRef, watch } from "vue";
import { TSCE_MQTT } from "@/utils/event";
import { genConnectedCallbackToken } from "@/utils/generators";
import {
  convertSubsConfigSubMap,
  getSubscribedSubs,
  subscribeAll,
  unsubscribeAll,
} from "@/utils/subs";
import useMqtt from "@/reactives/useMqtt";
import useLayout from "@/reactives/useLayout";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import useThingEvent from "@/reactives/useThingEvent";

const ccbt = shallowRef("");
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
const {
  activeToolKey,
  switchActiveTool,
  showMqttConnForm,
} = useLayout();
const { onSomethingStatusChange } = useThingEvent();
const clients = ref([]);
const totalCount = computed(() => clients.value.length);
const connectedCount = computed(
  () => clients.value.filter((item) => item.client.connected).length
);

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
const handleShowToolPanel = (config) => {
  if (activeToolKey.value !== "mqtt") {
    switchActiveTool({ key: "mqtt" });
  }
  selectConnection(config);
};

const handleCreateMqttClient = () => {
  ccbt.value = showMqttConnForm(null, selectedThingId.value);
};
const handleClientCommand = (command, item) => {
  switch (command) {
    case "connect":
      handleConnectMqttClient(item.config);
      break;
    case "disconnect":
      handleDisconnectMqttClient(item.config);
      break;
    case "subscribe":
      handleSubscribeAll(item.config, item.subs);
      break;
    case "unsubscribe":
      handleUnsubscribeAll(item.config, item.subs);
      break;
    case "panel":
      handleShowToolPanel(item.config);
      break;
  }
};

watch(
  [selectedThingId, connections, delegateSharedStates],
  () => {
    clients.value = getConnConfigsByClientId(selectedThingId.value).map((config) => {
      const { id, name, subscriptions } = config;
      const { client, subs, unreadMessageCount } =
        delegateSharedStates.value[id] || {};
      return {
        config,
        id,
        name,
        client: client || { connected: false },
        subs: subs || {},
        subscriptions: subscriptions || [],
        unreadMessageCount: unreadMessageCount || 0,
      };
    });
  },
  { immediate: true }
);

onSomethingStatusChange(({ thingId, type, about }) => {
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
});
</script>

<style scoped lang="scss">
.thing-mqtt-clients {
  width: 100%;
  margin-top: 4px;
  text-align: left;

  .thing-mqtt-clients-title {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    margin-bottom: 8px;
    color: var(--tio-muted);
    font-size: 13px;
    font-weight: 700;
  }

  .thing-mqtt-count {
    display: inline-flex;
    align-items: center;
    margin-left: 6px;
    padding: 1px 6px;
    border-radius: 999px;
    background: var(--tio-surface-soft);
    color: var(--tio-muted);
    font-size: 12px;
    font-weight: 650;
  }

  .thing-mqtt-list {
    border-top: 1px solid var(--tio-border);
  }

  .thing-mqtt-client {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 10px;
    width: 100%;
    min-height: 38px;
    border-bottom: 1px solid var(--tio-border);
    color: var(--tio-muted);

    &.connected {
      .thing-mqtt-status-dot {
        background: var(--tio-success);
      }
    }
  }

  .thing-mqtt-client-main {
    display: flex;
    align-items: center;
    gap: 7px;
    min-width: 0;
  }

  .thing-mqtt-status-dot {
    width: 7px;
    height: 7px;
    flex: 0 0 auto;
    border-radius: 50%;
    background: var(--tio-muted);
  }

  .thing-mqtt-client-name {
    overflow: hidden;
    color: var(--tio-text);
    font-size: 12px;
    font-weight: 650;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .thing-mqtt-client-meta {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 0 0 auto;
  }

  .thing-mqtt-sub-count {
    color: var(--tio-muted);
    font-size: 12px;
  }

  .thing-mqtt-unread {
    min-width: 18px;
    height: 18px;
    padding: 0 5px;
    border-radius: 999px;
    background: var(--tio-accent-soft);
    color: var(--tio-accent);
    font-size: 11px;
    font-weight: 750;
    line-height: 18px;
    text-align: center;
  }

  .thing-mqtt-more.el-button {
    padding: 0;
    color: var(--tio-muted);
  }

  .thing-mqtt-empty {
    padding: 10px 0 2px;
    border-top: 1px solid var(--tio-border);

    p {
      margin: 0 0 10px;
      color: var(--tio-muted);
      font-size: 12px;
      line-height: 1.45;
    }

    .el-button {
      width: 100%;
    }
  }
}
</style>

<style lang="scss">
.thing-mqtt-clients-popper {
  min-width: 150px;
}
</style>

<template>
  <div v-if="currentConnId" class="mqtt-client-detail">
    <div class="mqtt-client-detail-left">
      <div id="mqtt-client-message-panel" class="mqtt-client-detail-message"></div>
      <div class="mqtt-client-detail-publish">
        <MqttPublish :conn-config="selectedConn.config" />
      </div>
    </div>
    <div class="mqtt-client-detail-right">
      <div class="mqtt-client-detail-infos">
        <KeyValueDisplayer :data="baseInfo" :fields="mqttFields" />
      </div>
      <div class="mqtt-client-detail-subs">
        <el-row :gutter="10">
          <el-col :span="24">
            <el-button
              :disabled="!selectedConn.client.connected"
              type="success"
              class="subscription-stats"
              plain
              @click="handleShowSubscriptions"
              ><span>Subscription</span
              ><span>
                {{ getSubscribedSubs(subscribed).length }} /
                {{ selectedConn.subscriptions.length }}</span
              ></el-button
            >
          </el-col>
          <el-col :span="12">
            <el-button
              :disabled="!selectedConn.client.connected"
              type="primary"
              plain
              @click="handleSubscribeAll"
              >Subscribe All</el-button
            >
          </el-col>
          <el-col :span="12">
            <el-button
              :disabled="!selectedConn.client.connected"
              type="warning"
              plain
              @click="handleUnsubscribeAll"
              >Unsubscribe All</el-button
            >
          </el-col>
        </el-row>
      </div>
    </div>
  </div>
  <div v-else class="mqtt-client-detail">
    <el-empty :image-size="100">
      <template #description>
        <p>There is no selectable connection in list.</p>
      </template>
      <el-button type="default" icon="plus" @click="emit('request-add-conn')"
        >Add One</el-button
      >
    </el-empty>
  </div>
  <MqttMessage />
</template>

<script setup>
import { computed } from "vue";
import { mqttFields } from "@/configs/tool";
import KeyValueDisplayer from "@/components/common/KeyValueDisplayer.vue";
import MqttMessage from "./MqttMessage.vue";
import MqttPublish from "./MqttPublish.vue";
import {
  serverSubTopics,
  subscribeAll,
  unsubscribeAll,
  thingSubTopics,
  getSubscribedSubs,
} from "@/utils/subs";
import useMqtt from "@/reactives/useMqtt";
import useLayout from "@/reactives/useLayout";

const emit = defineEmits(["request-add-conn"]);
const { currentConnId, selectedConn, subscribe, unsubscribe } = useMqtt();
const { showMqttSubs } = useLayout();

const baseInfo = computed(() => {
  if (currentConnId.value) {
    const {
      userrole,
      name,
      clientId,
      clean,
      host,
      protocol,
      port,
      keepalive,
      username,
      password,
      mqttVersion,
    } = selectedConn.value;
    return {
      userrole,
      name,
      clientId,
      clean,
      mqttVersion,
      keepalive,
      username,
      password,
      broker: `${protocol}://${host}:${port}`,
    };
  }
  return {};
});
const subscribed = computed(() => selectedConn.value.subs || {});

const handleShowSubscriptions = () => {
  if (selectedConn.value.client?.connected) {
    showMqttSubs(selectedConn.value.config);
  }
};

const handleSubscribeAll = () => {
  subscribeAll({
    config: selectedConn.value.config,
    subMap: selectedConn.value.subs || {},
    subscribe,
  });
};
const handleUnsubscribeAll = () => {
  unsubscribeAll({
    config: selectedConn.value.config,
    subMap: selectedConn.value.subs || {},
    unsubscribe,
  });
};
</script>

<style scoped lang="scss">
.mqtt-client-detail {
  display: flex;
  flex-direction: row;
  justify-content: center;
  align-items: center;
  gap: 5px;

  width: 100%;
  height: 100%;
  padding: 5px;

  .mqtt-client-detail-left {
    width: 50vw;
    height: 278px;
    min-width: 540px;

    .mqtt-client-detail-message {
      width: 100%;
      height: 90px;
      border: solid 1px rgba(0, 0, 0, 0.05);
      border-radius: 2px;
    }

    .mqtt-client-detail-publish {
      width: 100%;
      height: 180px;
      margin-top: 8px;
      border: solid 1px rgba(0, 0, 0, 0.05);
      border-radius: 2px;
    }
  }

  .mqtt-client-detail-right {
    flex: 1;
    width: 0;
    height: 278px;

    .mqtt-client-detail-infos {
      width: 100%;
      height: 178px;
    }
    .mqtt-client-detail-subs {
      width: 100%;
      height: 90px;
      margin-top: 10px;
      padding: 9px 10px 0;
      border-radius: 4px;
      background-color: #f7f7f7;

      .el-button {
        width: 100%;
        margin-bottom: 8px;
      }
    }
  }
}
</style>

<style lang="scss">
.mqtt-client-detail {
  .mqtt-client-detail-right {
    .mqtt-client-detail-subs {
      .subscription-stats.el-button {
        > span {
          display: flex;
          flex-direction: row;
          justify-content: space-around;
          align-items: center;
          width: 100%;
        }
      }
    }
  }
}
</style>
@/utils/subs

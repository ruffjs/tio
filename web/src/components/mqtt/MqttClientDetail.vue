<template>
  <div v-if="currentConnId" class="mqtt-client-detail">
    <div class="mqtt-client-detail-left">
      <div id="mqtt-client-message-panel" class="mqtt-client-detail-message"></div>
      <div class="mqtt-client-detail-publish">
        <MqttPublish :conn-config="selectedConn.config" />
      </div>
    </div>
    <div class="mqtt-client-detail-right">
      <!-- <div class="mqtt-client-detail-infos">
        <KeyValueDisplayer :data="baseInfo" :fields="mqttFields" />
      </div> -->
      <div class="mqtt-client-detail-sublist">
        <Subcriptions
          ref="subsRef"
          :conn="selectedConn"
          :connected="selectedConn.client.connected"
          v-model:filter-topic="filterTopic"
        />
      </div>
      <div class="mqtt-client-detail-subctrl">
        <el-row :gutter="10">
          <el-col :span="16">
            <el-button :disabled="true" type="success" class="subscription-stats" plain
              ><span>Subscription</span
              ><span>
                {{ getSubscribedSubs(subscribed).length }} /
                {{ selectedConn.subscriptions.length }}</span
              ></el-button
            >
          </el-col>
          <el-col :span="8">
            <el-button
              :disabled="!selectedConn.client.connected"
              type="primary"
              icon="Plus"
              plain
              @click="handleCreateSubscription"
              >Add</el-button
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
  <MqttMessage :filter-topic="filterTopic" />
  <SubscriptionForm @submit="handleSubmitSubsForm" />
</template>

<script setup>
import { computed, ref } from "vue";
import { mqttFields } from "@/configs/tool";
import KeyValueDisplayer from "@/components/common/KeyValueDisplayer.vue";
import MqttMessage from "./MqttMessage.vue";
import MqttPublish from "./MqttPublish.vue";
import Subcriptions from "./Subcriptions.vue";
import SubscriptionForm from "./SubscriptionForm.vue";
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
const { showMqttSubsForm } = useLayout();

const subsRef = ref();
const filterTopic = ref("");
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

const handleCreateSubscription = () => {
  if (selectedConn.value.client?.connected) {
    showMqttSubsForm(selectedConn.value.config, null);
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

const handleSubmitSubsForm = (data) => {
  try {
    subsRef.value?.submitForm(data);
  } catch (error) {}
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
    height: 328px;
    min-width: 540px;

    .mqtt-client-detail-message {
      width: 100%;
      height: 150px;
      border: solid 1px rgba(0, 0, 0, 0.05);
      border-radius: 2px;
    }

    .mqtt-client-detail-publish {
      width: 100%;
      height: 172px;
      margin-top: 6px;
      border: solid 1px rgba(0, 0, 0, 0.05);
      border-radius: 2px;
    }
  }

  .mqtt-client-detail-right {
    flex: 1;
    width: 0;
    height: 328px;

    .mqtt-client-detail-infos {
      width: 100%;
      height: 178px;
    }
    .mqtt-client-detail-sublist {
      width: 100%;
      height: 240px;
    }
    .mqtt-client-detail-subctrl {
      width: 100%;
      height: 82px;
      margin-top: 6px;
      padding: 6px 10px 0;
      border-radius: 4px;
      background-color: #f7f7f7;

      .el-button {
        width: 100%;
        margin-bottom: 6px;
      }
    }
  }
}
</style>

<style lang="scss">
.mqtt-client-detail {
  .mqtt-client-detail-left {
    .mqtt-client-detail-publish {
      --pub-json-edit-height: 114px;
    }
  }
  .mqtt-client-detail-right {
    .mqtt-client-detail-subctrl {
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

<template>
  <div v-if="currentConnId" class="mqtt-client-detail">
    <div class="mqtt-client-detail-left">
      <div id="mqtt-client-message-panel" class="mqtt-client-detail-message"></div>
      <div class="mqtt-client-detail-publish">
        <MqttPublish :conn-config="selectedConn.config" />
      </div>
    </div>
    <div class="mqtt-client-detail-right">
      <div class="mqtt-client-detail-sublist">
        <Subcriptions
          ref="subsRef"
          :conn="selectedConn"
          :connected="selectedConn.client.connected"
          v-model:filter-topic="filterTopic"
        />
      </div>
      <div class="mqtt-client-detail-subctrl">
        <div class="subscription-stats">
          <span>{{ $t('mqtt.subscription') }}</span>
          <strong>{{ getSubscribedSubs(subscribed).length }} / {{ selectedConn.subscriptions.length }}</strong>
        </div>
        <div class="subscription-actions">
          <el-button
            :disabled="!selectedConn.client.connected"
            type="primary"
            icon="Plus"
            plain
            @click="handleCreateSubscription"
            >{{ $t('mqtt.addSubscription') }}</el-button
          >
          <el-dropdown
            trigger="click"
            popper-class="mqtt-subctrl-popper"
            @command="handleBulkCommand"
          >
            <el-button
              :disabled="!selectedConn.client.connected"
              icon="MoreFilled"
              plain
            />
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="subscribe">
                  {{ $t('mqtt.subsAll') }}
                </el-dropdown-item>
                <el-dropdown-item command="unsubscribe">
                  {{ $t('mqtt.unsubAll') }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
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
import MqttMessage from "./MqttMessage.vue";
import MqttPublish from "./MqttPublish.vue";
import Subcriptions from "./Subcriptions.vue";
import SubscriptionForm from "./SubscriptionForm.vue";
import {
  subscribeAll,
  unsubscribeAll,
  getSubscribedSubs,
} from "@/utils/subs";
import useMqtt from "@/reactives/useMqtt";
import useLayout from "@/reactives/useLayout";

const emit = defineEmits(["request-add-conn"]);
const { currentConnId, selectedConn, subscribe, unsubscribe } = useMqtt();
const { showMqttSubsForm } = useLayout();

const subsRef = ref();
const filterTopic = ref("");
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
const handleBulkCommand = (command) => {
  switch (command) {
    case "subscribe":
      handleSubscribeAll();
      break;
    case "unsubscribe":
      handleUnsubscribeAll();
      break;
  }
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
  height: calc(100% - 62px);
  padding: 5px;

  overflow: hidden;

  .mqtt-client-detail-left {
    width: 50vw;
    height: 100%;
    min-width: 540px;

    .mqtt-client-detail-message {
      width: 100%;
      height: calc(100% - 178px);
      border: 1px solid var(--tio-border);
      border-radius: var(--tio-radius);
    }

    .mqtt-client-detail-publish {
      width: 100%;
      height: 172px;
      margin-top: 6px;
      border: 1px solid var(--tio-border);
      border-radius: var(--tio-radius);
    }
  }

  .mqtt-client-detail-right {
    flex: 1;
    width: 0;
    height: 100%;

    // .mqtt-client-detail-infos {
    //   width: 100%;
    //   height: 178px;
    // }
    .mqtt-client-detail-sublist {
      width: 100%;
      height: calc(100% - 48px);
    }
    .mqtt-client-detail-subctrl {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 10px;
      width: 100%;
      height: 42px;
      margin-top: 6px;
      padding: 6px 8px;
      border: 1px solid var(--tio-border);
      border-radius: var(--tio-radius);
      background: var(--tio-surface);

      .subscription-stats {
        display: flex;
        align-items: center;
        gap: 8px;
        min-width: 0;
        color: var(--tio-muted);
        font-size: 12px;
        white-space: nowrap;

        strong {
          color: var(--tio-text-strong);
          font-weight: 750;
        }
      }

      .subscription-actions {
        display: flex;
        align-items: center;
        gap: 6px;
        flex: 0 0 auto;
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
}

.mqtt-subctrl-popper {
  min-width: 140px;
}
</style>

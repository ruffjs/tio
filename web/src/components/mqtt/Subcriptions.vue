<template>
  <el-drawer
    :model-value="isSubscriptionsVisible"
    direction="ltr"
    size="30vw"
    title="Subcriptions"
    class="subscriptions-manager"
    modal-class="subscriptions-manager-mask"
    append-to-body
    @close="hideMqttSubs"
  >
    <div class="subscriptions-list" v-loading="loading">
      <div class="subscriptions-list-add">
        <el-button type="primary" icon="Plus" round plain @click="handleCreate()"
          >Add Subcription</el-button
        >
      </div>
      <div
        v-for="sub in subscriptions"
        :class="['subscriptions-list-item', sub.subscribed ? 'subscribed-item' : '']"
      >
        <div class="subscriptions-list-topic">
          <el-tag v-if="sub.name" size="small">{{ sub.name }}</el-tag>
          <span>{{ sub.topic }}</span>
        </div>
        <div class="subscriptions-list-btns">
          <el-button
            v-if="sub.subscribed"
            type="primary"
            size="small"
            class="subscriptions-list-btns-left"
            plain
            @click="handleUnsubscribe(sub)"
            >Unsubscribe</el-button
          >
          <el-button
            v-else
            type="info"
            size="small"
            class="subscriptions-list-btns-left"
            plain
            @click="handleSubscribe(sub)"
            >Subscribe</el-button
          >
          <div class="subscriptions-list-btns-right">
            <el-button
              :disabled="sub.subscribed"
              type="primary"
              icon="Edit"
              size="small"
              circle
              plain
              @click="handleEdit(sub)"
            />
            <el-button
              :disabled="sub.keep || sub.subscribed"
              type="danger"
              icon="Delete"
              size="small"
              circle
              plain
              @click="handleDelete(sub)"
            />
          </div>
        </div>
      </div>
    </div>
    <SubscriptionForm
      :type="subsFormType"
      :data="subsFormData"
      :conn-config="subscriptionsConnConfig"
      @cancel="handleHideForm"
      @submit="handleSubmitForm"
    />
  </el-drawer>
</template>

<script setup>
import { ref, watch } from "vue";
import useLayout from "@/reactives/useLayout";
import useMqtt from "@/reactives/useMqtt";
import SubscriptionForm from "./SubscriptionForm.vue";
import { ElNotification } from "element-plus";
import { notifyDone, notifyFail } from "@/utils/layout";

const {
  delegateSharedStates,
  subscribe,
  unsubscribe,
  removeTopic,
  setConnConfig,
} = useMqtt();
const { isSubscriptionsVisible, subscriptionsConnConfig, hideMqttSubs } = useLayout();
const subscriptions = ref([]);
const subsFormType = ref("");
const subsFormData = ref(null);
const loading = ref(false);

const updateSubscriptions = () => {
  // console.log(subscriptionsConnConfig.value);
  if (subscriptionsConnConfig.value) {
    const subs =
      (delegateSharedStates.value[subscriptionsConnConfig.value.id] &&
        delegateSharedStates.value[subscriptionsConnConfig.value.id].subs) ||
      {};
    subscriptions.value = subscriptionsConnConfig.value.subscriptions
      .map((sub) => {
        //   console.log(sub, delegateSharedStates.value[subscriptionsConnConfig.value.id]);
        const { id, topic, opts, keep, name } = sub;
        return {
          id,
          name,
          topic,
          opts,
          keep,
          subscribed: subs[topic] || false,
        };
      })
      .sort((a, b) => Number(b.subscribed) - Number(a.subscribed));
  } else {
    subscriptions.value = [];
  }
  loading.value = false;
};

const handleUnsubscribe = (sub) => {
  loading.value = true;
  if (sub.subscribed) {
    unsubscribe(subscriptionsConnConfig.value, sub.topic, (err) => {
      if (err) {
        notifyFail("Unsubscribe operation Failed");
      } else {
        notifyDone("Topic Unsubscribed");
        updateSubscriptions();
      }
    });
  }
};
const handleSubscribe = (sub) => {
  if (!sub.subscribed) {
    if (sub.id === subsFormData.value?.id) {
      handleHideForm();
    }
    loading.value = true;
    subscribe(
      subscriptionsConnConfig.value,
      {
        topic: sub.topic,
        opts: sub.opts,
        configs: subscriptionsConnConfig.value.subscriptions || [],
      },
      (err) => {
        if (err) {
          notifyFail("Subscribe operation Failed");
        } else {
          notifyDone("Topic Subscribed");
          updateSubscriptions();
        }
      }
    );
  }
};
const handleCreate = () => {
  subsFormType.value = "create";
  subsFormData.value = null;
};
const handleEdit = (sub) => {
  subsFormType.value = "edit";
  subsFormData.value = sub;
};

const handleDelete = (sub) => {
  if (sub.id === subsFormData.value?.id) {
    handleHideForm();
  }
  if (sub.subscribed) {
    unsubscribe(subscriptionsConnConfig.value, sub.topic);
  }
  const index = subscriptionsConnConfig.value.subscriptions.findIndex(
    (s) => s.id === sub.id
  );
  if (index > -1) {
    subscriptionsConnConfig.value.subscriptions.splice(index, 1);
    setConnConfig(subscriptionsConnConfig.value.id, subscriptionsConnConfig.value);
    removeTopic(sub.topic);
    notifyDone("Delete Subscription");
    updateSubscriptions();
  }
};

const handleHideForm = () => {
  subsFormType.value = "";
  subsFormData.value = null;
};
const handleSubmitForm = (data) => {
  const sameTopicSub = subscriptionsConnConfig.value.subscriptions.find(
    (s) => s.topic === data.topic
  );
  if (sameTopicSub) {
    if (!data.id || data.id !== sameTopicSub.id) {
      ElNotification({
        title: "Conflict",
        message: "The subscription with same topic already exists",
      });
      return;
    }
  }
  handleHideForm();
  const opts = {
    qos: data.qos,
  };
  if (data.nl) {
    opts.nl = data.nl;
  }
  if (data.rap) {
    opts.rap = data.rap;
  }
  if (data.rh) {
    opts.rh = data.rh;
  }
  if (data.subscriptionIdentifier) {
    opts.properties = {
      subscriptionIdentifier: data.subscriptionIdentifier,
    };
  }
  if (data.id) {
    const sub = subscriptionsConnConfig.value.subscriptions.find((s) => s.id === data.id);
    if (sub) {
      const oldTopic = sub.topic;
      sub.name = data.name;
      sub.topic = data.topic;
      sub.opts = opts;
      subscribe(
        subscriptionsConnConfig.value,
        {
          topic: sub.topic,
          opts,
          configs: [sub],
        },
        (err) => {
          if (err) {
            notifyFail("Subscribe operation Failed");
          } else {
            notifyDone("Update subscription and subscribe it.");
            updateSubscriptions();
          }
        }
      );
      if (sub.topic !== oldTopic) {
        removeTopic(subscriptionsConnConfig.value, oldTopic);
      }
    }
  } else {
    subscribe(
      subscriptionsConnConfig.value,
      {
        topic: data.topic,
        opts,
        configs: [
          {
            name: data.name,
            keep: false,
            opts,
            topic: data.topic,
          },
        ],
      },
      (err) => {
        if (err) {
          notifyFail("Subscribe operation Failed");
        } else {
          notifyDone("Add subscription and subscribe it.");
          updateSubscriptions();
        }
      }
    );
  }
};

watch(
  [isSubscriptionsVisible, subscriptionsConnConfig, delegateSharedStates],
  () => updateSubscriptions(),
  {
    immediate: true,
  }
);
</script>

<style scoped lang="scss">
.subscriptions-manager {
  .subscriptions-list-add {
    text-align: center;
    margin-bottom: 16px;
  }
  .subscriptions-list {
    position: relative;
    width: 100%;
    height: 100%;
    padding: 0 var(--el-drawer-padding-primary) 10px;
    background-color: white;
    overflow-x: hidden;
    overflow-y: auto;
    z-index: 11;
    .subscriptions-list-item {
      width: 100%;
      height: auto;

      margin-bottom: 10px;
      padding: 3px 5px 8px;
      border-left: solid 4px rgba($color: #000000, $alpha: 0.1);
      border-radius: 4px;
      background-color: rgba($color: #000000, $alpha: 0.05);
      cursor: pointer;

      &.subscribed-item {
        border-left-color: var(--el-color-success);
      }

      .subscriptions-list-topic {
        width: 100%;
        height: auto;
        padding: 5px 0;
        line-height: 15px;
        font-size: 12px;
        font-weight: 600;
        color: #888;
        word-break: break-all;
        word-wrap: break-word;
        .el-tag {
          float: right;
          margin-left: 5px;
        }
      }

      .subscriptions-list-btns {
        display: flex;
        flex-direction: row;
        justify-content: space-between;
        align-items: center;
      }
    }
  }
}
</style>
<style lang="scss">
.el-overlay.subscriptions-manager-mask {
  background-color: rgba($color: #000000, $alpha: 0.1);
}
.subscriptions-manager {
  .el-drawer__header {
    margin-bottom: 10px;
    .el-drawer__title {
      font-weight: 700;
    }
  }
  .el-drawer__body {
    padding: 0;
  }
}
</style>

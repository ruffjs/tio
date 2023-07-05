<template>
  <div class="subscriptions-list" v-loading="loading">
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
          :disabled="!conn.client?.connected"
          type="primary"
          size="small"
          class="subscriptions-list-btns-left"
          plain
          @click="handleUnsubscribe(sub)"
          >Unsubscribe</el-button
        >
        <el-button
          v-else
          :disabled="!conn.client?.connected"
          type="info"
          size="small"
          class="subscriptions-list-btns-left"
          @click="handleSubscribe(sub)"
          >Subscribe</el-button
        >
        <div class="subscriptions-list-btns-right">
          <el-button
            :type="filterTopic === sub.topic ? 'primary' : ''"
            :plain="filterTopic !== sub.topic"
            icon="Filter"
            size="small"
            circle
            plain
            @click="handleToggleFilter(sub)"
          />
          <el-button
            :disabled="!conn.client?.connected || sub.subscribed"
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
</template>

<script setup>
import { ref, watch } from "vue";
import { ElNotification } from "element-plus";
import { notifyDone, notifyFail } from "@/utils/layout";
import useMqtt from "@/reactives/useMqtt";
import useLayout from "@/reactives/useLayout";

const emit = defineEmits(["update:filterTopic"]);
const props = defineProps({
  conn: {
    type: [Object, null],
    required: true,
  },
  subs: Object,
  filterTopic: {
    type: String,
    default: "",
  },
});
const { subscribe, unsubscribe, removeTopic, setConnConfig } = useMqtt();
const { subsFormData, showMqttSubsForm, hideMqttSubsForm } = useLayout();

const subscriptions = ref([]);
const loading = ref(false);

const updateSubscriptions = () => {
  if (props.conn.config) {
    const subscribed = props.conn.subs || {};
    subscriptions.value = props.conn.subscriptions
      .map((sub) => {
        const { id, topic, opts, keep, name } = sub;
        return {
          id,
          name,
          topic,
          opts,
          keep,
          subscribed: subscribed[topic] || false,
        };
      })
      .sort((a, b) => Number(b.subscribed) - Number(a.subscribed));
  } else {
    subscriptions.value = [];
  }
};

const handleUnsubscribe = (sub) => {
  loading.value = true;
  if (sub.subscribed) {
    unsubscribe(props.conn.config, sub.topic, (err) => {
      loading.value = false;
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
      hideMqttSubsForm();
    }
    loading.value = true;
    subscribe(
      props.conn.config,
      {
        topic: sub.topic,
        opts: sub.opts,
        configs: props.conn.subscriptions || [],
      },
      (err) => {
        loading.value = false;
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
const handleEdit = (sub) => {
  if (props.conn.client?.connected) {
    showMqttSubsForm(props.conn.config, sub);
  }
};

const handleToggleFilter = (sub) => {
  emit("update:filterTopic", props.filterTopic === sub.topic ? "" : sub.topic);
};

const handleDelete = (sub) => {
  if (sub.id === subsFormData.value?.id) {
    hideMqttSubsForm();
  }
  if (props.conn.client?.connected && sub.subscribed) {
    unsubscribe(props.conn.config, sub.topic);
  }
  const index = props.conn.subscriptions.findIndex((s) => s.id === sub.id);
  if (index > -1) {
    props.conn.subscriptions.splice(index, 1);
    setConnConfig(props.conn.id, props.conn.config);
    removeTopic(sub.topic);
    notifyDone("Delete Subscription");
    updateSubscriptions();
  }
};

defineExpose({
  submitForm: (data) => {
    const sameTopicSub = props.conn.subscriptions.find((s) => s.topic === data.topic);
    if (sameTopicSub) {
      if (!data.id || data.id !== sameTopicSub.id) {
        ElNotification({
          title: "Conflict",
          message: "The subscription with same topic already exists",
        });
        return;
      }
    }
    hideMqttSubsForm();
    loading.value = true;
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
      const sub = props.conn.subscriptions.find((s) => s.id === data.id);
      if (sub) {
        const oldTopic = sub.topic;
        sub.name = data.name;
        sub.topic = data.topic;
        sub.opts = opts;
        subscribe(
          props.conn.config,
          {
            topic: sub.topic,
            opts,
            configs: [sub],
          },
          (err) => {
            loading.value = false;
            if (err) {
              notifyFail("Subscribe operation Failed");
            } else {
              notifyDone("Update subscription and subscribe it.");
              updateSubscriptions();
            }
          }
        );
        if (sub.topic !== oldTopic) {
          removeTopic(props.conn.config, oldTopic);
        }
      }
    } else {
      subscribe(
        props.conn.config,
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
          loading.value = false;
          if (err) {
            notifyFail("Subscribe operation Failed");
          } else {
            notifyDone("Add subscription and subscribe it.");
            updateSubscriptions();
          }
        }
      );
    }
  },
});

watch(
  () => props.conn,
  () => updateSubscriptions(),
  {
    immediate: true,
  }
);
</script>

<style scoped lang="scss">
.subscriptions-list {
  position: relative;
  width: 100%;
  height: 100%;
  padding: 0;
  border-radius: 4px;
  background-color: white;
  overflow-x: hidden;
  overflow-y: auto;
  z-index: 11;
  .subscriptions-list-item {
    width: 100%;
    height: auto;

    margin-bottom: 6px;
    padding: 3px 5px 8px;
    border-left: solid 4px rgba($color: #000000, $alpha: 0.1);
    border-radius: 4px;
    background-color: rgba($color: #000000, $alpha: 0.05);
    cursor: pointer;

    &:last-child {
      margin-bottom: 0;
    }

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
</style>

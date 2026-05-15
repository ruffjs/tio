<template>
  <div class="subscriptions-list" v-loading="loading">
    <div
      v-for="sub in subscriptions"
      :key="sub.id"
      :class="['subscriptions-list-item', sub.subscribed ? 'subscribed-item' : '']"
    >
      <span class="subscriptions-list-status-dot"></span>
      <div class="subscriptions-list-topic">
        <div class="subscriptions-list-topic-text">{{ sub.topic }}</div>
        <div v-if="sub.name || filterTopic === sub.topic" class="subscriptions-list-topic-meta">
          <el-tag v-if="sub.name" size="small">{{ sub.name }}</el-tag>
          <span v-if="filterTopic === sub.topic" class="subscriptions-list-filtered">
            {{ $t('mqtt.filterMessages') }}
          </span>
        </div>
      </div>
      <div class="subscriptions-list-actions">
        <el-dropdown
          trigger="click"
          popper-class="subscriptions-list-actions-popper"
          @command="(command) => handleSubscriptionCommand(command, sub)"
        >
          <el-button class="subscriptions-list-more" link icon="MoreFilled" />
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item
                command="toggle"
                :disabled="!conn.client?.connected"
              >
                {{ sub.subscribed ? $t('mqtt.unsubscribe') : $t('mqtt.subscribe') }}
              </el-dropdown-item>
              <el-dropdown-item command="filter" divided>
                {{ filterTopic === sub.topic ? $t('mqtt.clearMessageFilter') : $t('mqtt.filterMessages') }}
              </el-dropdown-item>
              <el-dropdown-item
                command="edit"
                :disabled="!conn.client?.connected || sub.subscribed"
                divided
              >
                {{ $t('common.edit') }}
              </el-dropdown-item>
              <el-dropdown-item
                command="delete"
                :disabled="sub.keep || sub.subscribed"
              >
                {{ $t('common.delete') }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
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
const handleSubscriptionCommand = (command, sub) => {
  switch (command) {
    case "toggle":
      if (sub.subscribed) {
        handleUnsubscribe(sub);
      } else {
        handleSubscribe(sub);
      }
      break;
    case "filter":
      handleToggleFilter(sub);
      break;
    case "edit":
      handleEdit(sub);
      break;
    case "delete":
      handleDelete(sub);
      break;
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
  border-radius: var(--tio-radius);
  background: var(--tio-surface-solid);
  color: var(--tio-text);
  overflow-x: hidden;
  overflow-y: auto;
  z-index: 11;
  .subscriptions-list-item {
    display: flex;
    align-items: center;
    gap: 7px;
    width: 100%;
    height: auto;
    min-height: 40px;
    margin-bottom: 4px;
    padding: 6px 4px 6px 9px;
    border-left: 4px solid var(--tio-border);
    border-radius: var(--tio-radius);
    background: var(--tio-surface-soft);
    transition: background-color 0.16s ease, border-color 0.16s ease;

    &:last-child {
      margin-bottom: 0;
    }

    &:hover,
    &:focus-within {
      border-left-color: var(--tio-accent);
      background: var(--tio-surface);

      .subscriptions-list-actions {
        opacity: 1;
        pointer-events: auto;
      }
    }

    &.subscribed-item {
      border-left-color: var(--tio-success);

      .subscriptions-list-status-dot {
        background: var(--tio-success);
      }
    }

    .subscriptions-list-status-dot {
      width: 7px;
      height: 7px;
      flex: 0 0 auto;
      border-radius: 50%;
      background: var(--tio-muted);
    }

    .subscriptions-list-topic {
      flex: 1;
      min-width: 0;
      width: 100%;
      height: auto;
      padding: 0;
      line-height: 1.35;
      font-size: 12px;
      font-weight: 600;
      color: var(--tio-text);
    }

    .subscriptions-list-topic-meta {
      display: flex;
      align-items: center;
      gap: 6px;
      min-height: 18px;
      margin-top: 3px;

      .el-tag {
        max-width: 120px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
    }

    .subscriptions-list-topic-text {
      overflow: hidden;
      color: var(--tio-text);
      font-family: var(--tio-mono);
      font-size: 12px;
      font-weight: 600;
      line-height: 18px;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .subscriptions-list-filtered {
      color: var(--tio-muted);
      font-size: 11px;
      font-weight: 650;
      white-space: nowrap;
    }

    .subscriptions-list-filtered {
      color: var(--tio-accent);
    }

    .subscriptions-list-actions {
      display: inline-flex;
      align-items: center;
      flex: 0 0 auto;
      opacity: 0;
      pointer-events: none;
      transition: opacity 0.16s ease;
    }

    .subscriptions-list-more.el-button {
      padding: 0;
      color: var(--tio-muted);
    }
  }
}

@media (hover: none) {
  .subscriptions-list .subscriptions-list-item .subscriptions-list-actions {
    opacity: 1;
    pointer-events: auto;
  }
}
</style>

<style lang="scss">
.subscriptions-list-actions-popper {
  min-width: 150px;
}
</style>

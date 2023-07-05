<template>
  <Teleport :to="teleport">
    <div class="mqtt-messages">
      <div v-if="messages.length" class="mqtt-messages-list">
        <div
          v-for="message in messages"
          class="mqtt-message"
          :class="['mqtt-message', message.out ? 'right' : 'left']"
        >
          <div class="mqtt-message-box">
            <div class="mqtt-message-meta">
              <span class="mqtt-message-meta-label">Topic:</span>
              <span class="mqtt-message-meta-value">{{ message.topic }}</span>
              <span class="mqtt-message-meta-label qos">QoS:</span>
              <span class="mqtt-message-meta-value">{{ message.qos }}</span>
            </div>
            <div class="mqtt-message-data">{{ String(message.payload) }}</div>
          </div>
          <div class="mqtt-message-time">{{ message.createAt }}</div>
        </div>
      </div>
      <div v-else class="mqtt-messages-empty"></div>
      <div class="mqtt-messages-filters">
        <el-radio-group v-model="filters.type" size="small">
          <el-radio
            v-for="item in typeOptions"
            :key="item.value"
            :label="item.value"
            :value="item.value"
            >{{ item.label }}</el-radio
          >
        </el-radio-group>
      </div>
      <div class="mqtt-messages-btns">
        <el-button icon="Delete" circle size="small" @click="handleClearMessages" />
        <el-button
          :icon="isGlobal ? 'Bottom' : 'Top'"
          circle
          size="small"
          @click="switchTeleport(!isGlobal)"
        />
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from "vue";
import useMqtt from "@/reactives/useMqtt";
import { teleports } from "@/reactives/useLayout";
import { matchTopicMethod } from "@/utils/subs";

const typeOptions = [
  {
    value: "all",
    label: "All",
  },
  {
    value: "in",
    label: "Received",
  },
  {
    value: "out",
    label: "Published",
  },
];

const props = defineProps({
  filterTopic: {
    type: String,
    default: "",
  },
});
const { currentConnId, selectedConn, clearMessages } = useMqtt();
const teleport = ref(teleports.G);
const isGlobal = computed(() => teleport.value === teleports.G);
const filters = reactive({
  type: "all",
});
const messages = computed(() => {
  if (currentConnId.value) {
    return (
      selectedConn.value.messages?.filter((message) => {
        if (filters.type === "in" && message.out) {
          return false;
        }
        if (filters.type === "out" && !message.out) {
          return false;
        }
        if (props.filterTopic === "") {
          return true;
        }
        if (message.out) {
          return false;
        }
        return matchTopicMethod(message.topic, props.filterTopic);
      }) || []
    );
  }
  return [];
});
const handleClearMessages = () => {
  clearMessages(selectedConn.value.config);
};
const switchTeleport = (toGlobal = false) => {
  const globalTeleport = document.querySelector(teleports.G);
  if (toGlobal) {
    globalTeleport.style.visibility = "visible";
    globalTeleport.style.height = "66vh";
    teleport.value = teleports.G;
  } else {
    globalTeleport.style.visibility = "hidden";
    globalTeleport.style.height = "100px";
    if (currentConnId.value) {
      teleport.value = teleports.D;
    } else {
      teleport.value = teleports.G;
    }
  }
};
const scrollToBottom = async () => {
  await nextTick();
  const list = document.querySelector(".mqtt-messages-list");
  if (list) {
    list.scrollTo({
      top: list.scrollHeight - list.clientHeight,
    });
  }
};

watch(teleport, scrollToBottom);
watch(currentConnId, () => {
  switchTeleport(false);
});
watch(
  selectedConn,
  () => {
    filters.type = "all";
    scrollToBottom();
  },
  { deep: true, immediate: true }
);
onMounted(() => {
  switchTeleport(false);
});
onUnmounted(() => {
  switchTeleport(false);
});
</script>

<style scoped lang="scss">
.mqtt-messages {
  position: relative;
  width: 100%;
  height: 100%;

  .mqtt-messages-list {
    width: 100%;
    height: 100%;
    padding: 10px;
    padding-top: 32px;
    overflow-x: hidden;
    overflow-y: auto;

    .mqtt-message {
      width: 100%;
      margin-bottom: 10px;
      .mqtt-message-box {
        width: 85%;
        height: auto;
        padding: 2px 5px;
        background-color: #f2f2f2;
        border-radius: 5px;
        font-size: 13px;
        overflow: hidden;

        .mqtt-message-meta {
          color: #666;
          .mqtt-message-meta-label {
            margin-right: 6px;
            font-weight: 600;
            &.qos {
              margin-left: 16px;
            }
          }
          .mqtt-message-meta-value {
            font-weight: 400;
          }
        }

        .mqtt-message-data {
          width: 100%;
          height: auto;
          word-break: break-word;
          word-wrap: break-word;
          overflow: hidden;
        }
      }
      .mqtt-message-time {
        line-height: 20px;
        font-size: 12px;
        font-weight: 500;
        color: #999;
      }

      &.left {
        .mqtt-message-box {
          margin-right: 15%;
          border-left: solid 5px var(--el-color-primary);
        }
        .mqtt-message-time {
          text-align: left;
        }
      }

      &.right {
        .mqtt-message-box {
          margin-left: 15%;
          border-right: solid 5px var(--el-color-success);
        }
        .mqtt-message-time {
          text-align: right;
        }
      }
    }
  }

  .mqtt-messages-filters {
    position: absolute;
    top: 2px;
    left: 5px;
    .el-radio-group {
      // display: flex;
      .el-radio {
        margin-right: 12px;
      }
    }
  }

  .mqtt-messages-btns {
    position: absolute;
    top: 2px;
    right: 5px;

    .el-button + .el-button {
      margin-left: 5px;
    }
  }
}
</style>

<template>
  <el-drawer
    :model-value="visible"
    size="calc(50vw - 250px)"
    title="Suggested Topics"
    class="publish-topic-suggestions"
    modal-class="publish-topic-suggestions-mask"
    append-to-body
    @close="emit('close')"
  >
    <div class="publish-topic-suggestions-list">
      <div
        v-for="suggestion in suggestions"
        class="publish-topic-suggestions-item"
        @click="handleSelect(suggestion)"
      >
        <div class="publish-topic-suggestions-name">
          {{ suggestion.name }}
          <el-tag size="small"> {{ suggestion.payloadType }}</el-tag>
        </div>
        <div class="publish-topic-suggestions-code">{{ suggestion.code }}</div>
      </div>
    </div>
  </el-drawer>
</template>

<script setup>
import { computed } from "vue";
import { serverPubTopics, thingPubTopics } from "@/utils/subs";

const emit = defineEmits(["close", "select"]);
const props = defineProps({
  visible: Boolean,
  connConfig: {
    type: [Object, null],
    required: true,
  },
});
const isThing = computed(() => props.connConfig?.userrole === "thing");
const suggestions = computed(() => {
  if (isThing.value) {
    return thingPubTopics;
  }
  return serverPubTopics;
});

const handleSelect = (suggestion) => {
  let thingId = "+";
  if (isThing.value) {
    thingId = props.connConfig?.username;
  }
  emit("select", {
    topic: suggestion.topicResolver({ thingId }),
    paytype: suggestion.payloadType,
    payload: suggestion.payloadResolver({ thingId }),
  });
};
</script>

<style scoped lang="scss">
.publish-topic-suggestions {
  .publish-topic-suggestions-list {
    width: 100%;
    height: auto;
    .publish-topic-suggestions-item {
      width: 100%;
      height: auto;

      margin-bottom: 10px;
      padding: 3px 5px;
      border-radius: 4px;
      background-color: rgba($color: #000000, $alpha: 0.05);
      cursor: pointer;
      .publish-topic-suggestions-name {
        display: flex;
        flex-direction: row;
        justify-content: space-between;
        align-items: center;

        width: 100%;
        height: 28px;
        padding-bottom: 3px;
        border-bottom: solid 1px rgba($color: #ffffff, $alpha: 1);
        line-height: 24px;
        font-size: 13px;
        font-weight: 600;
      }
      .publish-topic-suggestions-code {
        width: 100%;
        height: auto;
        padding: 5px 0;
        line-height: 15px;
        font-size: 12px;
        font-weight: 600;
        color: #888;
        word-break: break-all;
        word-wrap: break-word;
      }
    }
  }
}
</style>
<style lang="scss">
.el-overlay.publish-topic-suggestions-mask {
  background-color: rgba($color: #000000, $alpha: 0.1);
}
.publish-topic-suggestions {
  .el-drawer__header {
    margin-bottom: 10px;
    .el-drawer__title {
      font-weight: 700;
    }
  }
  .el-drawer__body {
    padding: 0 var(--el-drawer-padding-primary) 10px;
  }
}
</style>

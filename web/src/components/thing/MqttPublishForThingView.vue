<template>
  <el-dialog
    :model-value="visible"
    :title="type"
    @close="emit('close')"
    width="60vw"
    class="thing-mqtt-publish-dialog"
    append-to-body
  >
    <div class="thing-mqtt-publish">
      <MqttPublish
        :conn-config="connConfig"
        :topic="topic"
        :payload="payload"
        :paytype="paytype"
        @publish="(err) => emit('done', err)"
      />
    </div>
  </el-dialog>
</template>

<script setup>
import MqttPublish from "@/components/mqtt/MqttPublish.vue";

const emit = defineEmits(["close", "done"]);
const props = defineProps({
  visible: Boolean,
  connConfig: {
    type: [Object, null],
    required: true,
  },
  type: {
    type: String,
    default: "MQTT Publish",
  },
  topic: String,
  payload: String,
  paytype: {
    type: String,
    default: "JSON",
  },
});
</script>

<style lang="scss">
.thing-mqtt-publish-dialog {
  .el-dialog__body {
    padding: 0 var(--el-dialog-padding-primary) 18px;
  }
  .thing-mqtt-publish {
    height: 240px;
    border: solid 1px rgba(0, 0, 0, 0.05);
    border-radius: 2px;
  }
}
</style>

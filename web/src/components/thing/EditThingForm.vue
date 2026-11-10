<template>
  <el-dialog
    :model-value="visible"
    :title="t('things.editThing')"
    width="480"
    append-to-body
    @close="emit('close')"
  >
    <el-form :model="form" label-width="100px">
      <el-form-item :label="t('things.enabled')">
        <el-switch v-model="form.enabled" />
      </el-form-item>
      <DeviceProfileFields
        v-model:protocol="form.messageProtocol"
        v-model:encoding="form.payloadEncoding"
        label-width="100px"
      />
    </el-form>
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="emit('close')">{{ t("common.cancel") }}</el-button>
        <el-button type="primary" @click="emit('save', { ...form })">{{ t("common.confirm") }}</el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { reactive, watch } from "vue";
import { useI18n } from "vue-i18n";
import DeviceProfileFields from "./DeviceProfileFields.vue";

const emit = defineEmits(["close", "save"]);
const props = defineProps({
  visible: Boolean,
  model: Object,
});

const { t } = useI18n();
const form = reactive({
  enabled: false,
  messageProtocol: "",
  payloadEncoding: "",
});

watch(
  () => props.visible,
  () => {
    if (props.visible && props.model) {
      Object.assign(form, {
        enabled: props.model.enabled,
        messageProtocol: props.model.messageProtocol || "",
        payloadEncoding: props.model.payloadEncoding || "",
      });
    }
  },
  { immediate: true }
);
</script>

<style scoped>
.dialog-footer button:first-child {
  margin-right: 10px;
}
</style>

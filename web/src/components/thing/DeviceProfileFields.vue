<template>
  <el-form-item :label="t('things.messageProtocol')" :label-width="labelWidth">
    <el-select
      :model-value="protocol"
      :disabled="profileDisabled"
      :placeholder="t('things.followDeployDefault')"
      style="width: 300px"
      @update:model-value="emit('update:protocol', $event)"
    >
      <el-option :label="t('things.followDeployDefault')" value="" />
      <el-option label="legacy" value="legacy" />
      <el-option label="simple" value="simple" />
    </el-select>
  </el-form-item>
  <el-form-item :label="t('things.payloadEncoding')" :label-width="labelWidth">
    <el-select
      :model-value="encoding"
      :placeholder="t('things.followDeployDefault')"
      style="width: 300px"
      @update:model-value="emit('update:encoding', $event)"
    >
      <el-option :label="t('things.followDeployDefault')" value="" />
      <el-option label="json" value="json" />
      <el-option label="cbor" value="cbor" />
    </el-select>
  </el-form-item>
  <el-form-item v-if="profileDisabled" :label-width="labelWidth">
    <span class="profile-mode-hint">{{ t('things.protocolFixedModeHint', { mode }) }}</span>
  </el-form-item>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { useStore } from "vuex";

defineProps({
  protocol: { type: String, default: "" },
  encoding: { type: String, default: "" },
  labelWidth: { type: String, default: "100px" },
});

const emit = defineEmits(["update:protocol", "update:encoding"]);

const { t } = useI18n();
const store = useStore();
// 设备级协议在 dual 模式下生效；编码可在所有模式下选择。
const mode = computed(() => store.state.app.tioConfig?.protocol?.mode || "");
const profileDisabled = computed(() => mode.value !== "dual");
</script>

<style scoped>
.profile-mode-hint {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.4;
}
</style>

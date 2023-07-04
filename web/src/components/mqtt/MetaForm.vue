<template>
  <el-dialog
    :model-value="visible"
    title="Meta Form"
    width="50vw"
    class="meta-form-dialog"
    append-to-body
    @close="handleClose"
  >
    <el-form label-width="185px" label-position="left" :model="form">
      <el-row :gutter="20">
        <el-col :span="24">
          <KeyValueEditor
            title="User Properties"
            v-model="form.userProperties"
            maxHeight="140px"
            style="margin-bottom: 17px"
          />
        </el-col>
        <el-col :span="24">
          <el-form-item label="Content Type" prop="contentType">
            <el-input size="small" v-model="form.contentType"></el-input>
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item label="Payload Format Indicator" prop="payloadFormatIndicator">
            <el-checkbox
              style="width: 100%"
              size="small"
              v-model="form.payloadFormatIndicator"
              border
              >{{ form.payloadFormatIndicator ? "true" : "false" }}</el-checkbox
            >
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item label="Message Expiry Interval(s)" prop="messageExpiryInterval">
            <el-input
              v-model.number="form.messageExpiryInterval"
              size="small"
              :min="0"
              type="number"
            />
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item label="Topic Alias" prop="topicAlias">
            <el-input
              v-model.number="form.topicAlias"
              size="small"
              :min="1"
              type="number"
            />
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item label="Response Topic" prop="responseTopic">
            <el-input
              placeholder="Response Topic"
              size="small"
              v-model="form.responseTopic"
              type="text"
            />
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item label="Correlation Data" prop="correlationData">
            <el-input
              placeholder="Correlation Data"
              size="small"
              v-model="form.correlationData"
              type="text"
            />
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item label="Subscription Identifier" prop="subscriptionIdentifier">
            <el-input
              size="small"
              type="number"
              v-model.number="form.subscriptionIdentifier"
            >
            </el-input>
          </el-form-item>
        </el-col>
      </el-row>
    </el-form>
    <template #footer>
      <span class="meta-form-dialog-footer">
        <el-button size="small" @click="handleClose">Cancel</el-button>
        <el-button type="primary" size="small" @click="handleSave">Save</el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script setup>
import { reactive, watch } from "vue";
import { getInitMeatModel } from "@/utils/mqtt";
import KeyValueEditor from "@/components/common/KeyValueEditor.vue";

const emit = defineEmits(["close", "save"]);
const props = defineProps({
  visible: Boolean,
  model: Object,
});

const form = reactive(getInitMeatModel());
const handleClose = () => {
  emit("close");
};
const handleSave = () => {
  emit("save", form);
};

watch(
  () => props.visible,
  () => {
    if (props.visible && props.model) {
      Object.assign(form, { ...props.model });
      form.userProperties = { ...form.userProperties };
    }
  },
  { immediate: true }
);
</script>

<style scoped lang="scss">
.meta-form-dialog {
  .el-form {
    .el-form-item {
      margin-bottom: 13px;
    }
  }
  .meta-form-dialog-footer {
    button:first-child {
      margin-right: 10px;
    }
  }
}
</style>

<template>
  <el-dialog
    :model-value="visible"
    :title="isCreate ? 'Add Subcription' : 'Edit Subcription'"
    width="50vw"
    class="subscriptions-form"
    append-to-body
    @close="hideMqttSubsForm"
  >
    <template #footer>
      <span class="subscriptions-form-footer">
        <el-button @click="hideMqttSubsForm">Cancel</el-button>
        <el-button type="primary" @click="handleSubscribe">Subscribe</el-button>
      </span>
    </template>
    <el-form ref="formRef" :model="form" :rules="rules" class="subscriptions-form-main">
      <el-row :gutter="20">
        <el-col :span="24">
          <el-form-item :label-width="formLabelWidth" label="Alias">
            <el-input v-model.trim="form.name" :disabled="form.keep" size="small" />
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item
            :label-width="formLabelWidth"
            label="Topic"
            prop="topic"
            :class="['subscriptions-form-topic', form.keep ? 'for-keep-subs' : '']"
          >
            <el-input
              v-model.trim="form.topic"
              :disabled="form.keep"
              type="textarea"
              placeholder="$iothub/things/#"
              size="small"
            >
            </el-input>
            <div class="mqtt-tpls-btn">
              <el-button
                :disabled="form.keep"
                size="small"
                plain
                @click="toggleTplsCardVisable"
                >Suggestions</el-button
              >
            </div>
            <div v-if="isTplsCardVisable" class="mqtt-tpls-list">
              <div
                v-for="t in topics"
                class="mqtt-tpls-item"
                @click="handleSelectTopic(t)"
              >
                <el-tag class="mqtt-tpls-name">{{ t.name }}</el-tag>
                <span class="mqtt-tpls-code">{{ t.topic }}</span>
              </div>
            </div>
          </el-form-item>
        </el-col>
        <el-col :span="24">
          <el-form-item :label-width="formLabelWidth" label="QoS" prop="qos">
            <el-select class="qos-select" v-model="form.qos" size="small">
              <el-option
                v-for="qos in qosOptions"
                :key="qos.value"
                :label="qos.label"
                :value="qos.value"
              >
                <span style="float: left">{{ qos.value }}</span>
                <span style="float: right; color: #8492a6; margin-left: 12px">{{
                  qos.label
                }}</span>
              </el-option>
            </el-select>
          </el-form-item>
        </el-col>

        <!-- MQTT 5.0 -->
        <template v-if="connConfig?.mqttVersion === '5.0'">
          <el-col :span="24">
            <el-form-item
              :label-width="formLabelWidthMqtt5"
              label="Subscription Identifier"
              prop="subscriptionIdentifier"
            >
              <el-input
                size="small"
                type="number"
                v-model.number="form.subscriptionIdentifier"
              >
              </el-input>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item
              :label-width="formLabelWidthMqtt5"
              label="No Local flag"
              prop="nl"
            >
              <el-radio-group v-model="form.nl">
                <el-radio :label="true">true</el-radio>
                <el-radio :label="false">false</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item
              :label-width="formLabelWidthMqtt5"
              label="Retain as Published flag"
              prop="rap"
            >
              <el-radio-group v-model="form.rap">
                <el-radio :label="true">true</el-radio>
                <el-radio :label="false">false</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item
              :label-width="formLabelWidthMqtt5"
              label="Retain Handling"
              prop="rh"
            >
              <el-select v-model="form.rh" size="small">
                <el-option
                  v-for="retainOps in [0, 1, 2]"
                  :key="retainOps"
                  :label="retainOps"
                  :value="retainOps"
                >
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
        </template>
      </el-row>
    </el-form>
  </el-dialog>
</template>

<script setup>
import { computed, ref, reactive, watch } from "vue";
import { qosOptions } from "@/configs/tool";
import { serverSubTopics, thingSubTopics } from "@/utils/subs";
import useLayout from "@/reactives/useLayout";

const formLabelWidth = "80px";
const formLabelWidthMqtt5 = "180px";
const initModel = {
  keep: false,
  id: "",
  topic: "",
  qos: 0,
  name: "",
  subscriptionIdentifier: undefined,
  nl: undefined,
  rap: undefined,
  rh: undefined,
};
const rules = reactive({
  topic: [{ required: true, message: "Please Input" }],
  qos: [{ required: true, message: "Please Select" }],
});
const emit = defineEmits(["submit"]);
const {
  isSubsFormVisible: visible,
  subsConnConfig: connConfig,
  subsFormData: formData,
  hideMqttSubsForm,
} = useLayout();
const formRef = ref();
const form = reactive({ ...initModel });

const isEdit = computed(() => formData.value !== null);
const isCreate = computed(() => formData.value === null);
const isThing = computed(() => connConfig.value?.userrole === "thing");
const topics = computed(() => {
  if (isThing.value) {
    return thingSubTopics.map(({ name, topicResolver }) => ({
      name,
      topic: topicResolver({ thingId: connConfig.value?.username }),
    }));
  }
  return serverSubTopics.map(({ name, topicResolver }) => ({
    name,
    topic: topicResolver({ thingId: "+" }),
  }));
});

const isTplsCardVisable = ref(false);
const toggleTplsCardVisable = () => {
  isTplsCardVisable.value = !isTplsCardVisable.value;
};

const handleSelectTopic = (t) => {
  isTplsCardVisable.value = false;
  form.name = t.name;
  form.topic = t.topic;
};

const handleSubscribe = async () => {
  if (!formRef.value) return;
  try {
    const valid = await formRef.value.validate();
    if (valid) {
      emit("submit", JSON.parse(JSON.stringify(form)));
    }
  } catch (error) {
    console.error("validate error!", error);
  }
};

watch(
  [visible, connConfig, formData],
  () => {
    if (visible.value) {
      if (isCreate.value) {
        Object.assign(form, initModel);
      } else if (isEdit.value && formData.value) {
        const { id, keep, topic, opts, name } = formData.value;
        Object.assign(form, {
          id,
          topic,
          qos: opts.qos,
          name: name || "",
          keep: keep || false,
          nl: opts.qos || undefined,
          rap: opts.qos || undefined,
          rh: opts.qos || undefined,
          subscriptionIdentifier: opts.properties?.subscriptionIdentifier || undefined,
        });
      }
    }
  },
  { immediate: true, deep: true }
);
</script>

<style scoped lang="scss">
.subscriptions-form {
  position: fixed;
  top: 30vh;
  right: 110vw;
  width: 44vw;
  // height: 60vh;
  opacity: 0;
  box-shadow: var(--el-box-shadow-dark);
  transition: right ease-in-out 0.2s, top ease-in-out 0.2s, opacity ease-in-out 0.2s;
  z-index: 10;

  .el-form.subscriptions-form-main {
    width: 100%;
    .el-form-item {
      margin-bottom: 14px;
    }

    .el-select,
    .el-input-number {
      width: 100%;
    }

    .el-form-item.subscriptions-form-topic {
      .mqtt-tpls-btn {
        position: absolute;
        line-height: 18px;
        top: 3px;
        right: 5px;
        opacity: 0;
        transition: opacity ease-in-out 0.2s;
        .el-button--small {
          --el-button-size: 18px;
          padding: 2px 5px;
        }
      }
      .mqtt-tpls-list {
        position: absolute;
        top: 26px;
        right: 5px;
        width: 34vw;
        height: 40vh;
        padding: 5px 10px;
        background-color: white;
        overflow-x: hidden;
        overflow-y: auto;
        box-shadow: 0 0 3px rgba($color: #000000, $alpha: 0.2);
        z-index: 100;

        .mqtt-tpls-item {
          line-height: 24px;
          margin: 3px 0;
          padding: 5px 0;
          border-bottom: solid 1px rgba($color: #000000, $alpha: 0.05);
          cursor: pointer;
          .mqtt-tpls-name {
            float: left;
            margin-right: 6px;
            padding: 1px 6px;
            line-height: 20px;
          }
          .mqtt-tpls-code {
            word-break: break-all;
            word-wrap: break-word;
            font-size: 13px;
            font-weight: 600;
            color: #888;
          }
        }
      }
    }

    .el-form-item.subscriptions-form-topic:hover,
    .el-form-item.subscriptions-form-topic:has(textarea:focus) {
      .mqtt-tpls-btn {
        opacity: 1;
      }
    }
    .el-form-item.subscriptions-form-topic.for-keep-subs:hover {
      .mqtt-tpls-btn {
        opacity: 0;
      }
    }
  }

  .subscriptions-form-footer {
    button:first-child {
      margin-right: 10px;
    }
  }
}
// }
</style>

<style lang="scss">
.subscriptions-form {
  .el-form.subscriptions-form-main {
    .el-form-item.subscriptions-form-topic {
      .el-textarea {
        textarea.el-textarea__inner {
          padding-right: 85px;
        }
      }
      &.for-keep-subs {
        .el-textarea {
          textarea.el-textarea__inner {
            padding-right: 11px;
          }
        }
      }
    }
  }
}
</style>

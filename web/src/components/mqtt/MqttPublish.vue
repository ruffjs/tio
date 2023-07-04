<template>
  <div class="mqtt-publish">
    <div class="mqtt-publish-header">
      <div class="mqtt-publish-metadata">
        <span class="mqtt-publish-label">Payload: </span>
        <el-select
          v-model="payloadType"
          :disabled="Boolean(paytype)"
          size="small"
          class="mqtt-publish-select"
        >
          <el-option v-for="(type, index) in payloadOptions" :key="index" :value="type">
          </el-option>
        </el-select>
        <span class="mqtt-publish-label">QoS: </span>
        <el-select
          v-model="form.qos"
          :disabled="connConfig?.userrole === 'thing'"
          size="small"
          class="mqtt-publish-select"
        >
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
        <el-checkbox
          v-model="form.retain"
          label="Retain"
          border
          size="small"
          class="mqtt-publish-checkbox retain-block"
        ></el-checkbox>
        <el-tooltip
          :disabled="mqtt5PropsEnable"
          persistent
          placement="top"
          effect="dark"
          content="Enabled only with MQTT 5.0"
          popper-class="tooltip-box"
        >
          <el-badge :is-dot="hasMqtt5Props" class="mqtt-publish-badge">
            <el-button
              :disabled="!mqtt5PropsEnable"
              :class="['meta-block', isMetaFormShown ? 'meta-block-active' : '']"
              plain
              type=""
              label="Meta"
              size="small"
              @click="toggleMetaFormVisable"
            >
              Meta
            </el-button>
          </el-badge>
        </el-tooltip>
      </div>
      <el-input
        v-model="form.topic"
        :disabled="Boolean(topic)"
        placeholder="Topic"
        size="small"
        class="mqtt-publish-topic-input"
        @focus="handleInputFoucs"
      >
      </el-input>
      <div v-if="!topic" class="mqtt-tpls-btn">
        <el-button :disabled="false" size="small" plain @click="toggleTplsCardVisable"
          >Suggestions</el-button
        >
      </div>
    </div>
    <div class="mqtt-publish-editor">
      <!-- <Editor
          ref="payloadEditor"
          id="payload"
          :lang="payloadLang"
          v-model="form.payload"
          :useShadows="true"
          @enter-event="send"
          @format="formatJsonValue"
        /> -->
      <el-input v-model="form.payload" type="textarea" placeholder="Please input" />
      <div class="mqtt-send-btn">
        <el-button icon="Promotion" circle size="small" type="primary" @click="send" />
      </div>
    </div>
  </div>
  <MetaForm
    :visible="isMetaFormShown"
    :model="mqtt5Props"
    @close="isMetaFormShown = false"
    @save="handleSaveMeta"
  />
  <PublishTopicSuggestions
    :visible="isTplsListShown"
    :conn-config="connConfig"
    @select="handleSelectTopic"
    @close="isTplsListShown = false"
  />
</template>

<script setup>
import { computed, reactive, ref, shallowRef, watch } from "vue";
import { getInitMeatModel } from "@/utils/mqtt";
import { qosOptions } from "@/configs/tool";
import useMqtt from "@/reactives/useMqtt";
import MetaForm from "./MetaForm.vue";
import PublishTopicSuggestions from "./PublishTopicSuggestions.vue";

const payloadOptions = ["Plaintext", "Base64", "JSON", "Hex"];
const emit = defineEmits(["publish"]);
const props = defineProps({
  connConfig: {
    type: [Object, null],
    required: true,
  },
  topic: String,
  payload: String,
  paytype: String,
});

const payloadLang = ref("json"); //'plaintext'
const payloadType = ref("JSON");
const form = reactive({
  qos: 0,
  retain: false,
  topic: "",
  payload: "",
});
const mqtt5Props = reactive(getInitMeatModel());
const { setConnConfig, publish } = useMqtt();
const mqtt5PropsEnable = ref(false);
const hasMqtt5Props = ref(false);
const isMetaFormShown = ref(false);
const isTplsListShown = ref(false);

const isNotEmptyObject = (value) => {
  if (typeof value === "object") {
    return value !== null && JSON.stringify(value) !== "{}";
  }
  return true;
};
const calcHasProps = (props) => {
  return Object.values(props).some(
    (value) =>
      isNotEmptyObject(value) && value !== undefined && value !== false && value !== ""
  );
};

const handleInputFoucs = () => {
  console.log("handleInputFoucs");
};
const toggleMetaFormVisable = () => {
  isMetaFormShown.value = !isMetaFormShown.value;
};
const toggleTplsCardVisable = () => {
  isTplsListShown.value = !isTplsListShown.value;
};
const handleSaveMeta = (newProps) => {
  Object.assign(mqtt5Props, {
    userProperties: calcHasProps(newProps.userProperties) ? newProps.userProperties : {},
    contentType: newProps.contentType || undefined,
    responseTopic: newProps.responseTopic || undefined,
    payloadFormatIndicator: newProps.payloadFormatIndicator || false,
    messageExpiryInterval: newProps.messageExpiryInterval || undefined,
    correlationData: newProps.correlationData || undefined,
    topicAlias: undefined,
    subscriptionIdentifier: undefined,
  });
  isMetaFormShown.value = false;
  hasMqtt5Props.value = calcHasProps(mqtt5Props);
};
const handleSelectTopic = ({ topic, paytype, payload }) => {
  form.topic = topic;
  form.payload = payload;
  if (paytype) {
    payloadType.value = paytype;
    if (paytype === "JSON") {
      payloadLang.value = "json";
    } else {
      payloadLang.value = "plaintext";
    }
  }
};
const send = () => {
  if (props.connConfig.id) {
    const properties = hasMqtt5Props.value ? mqtt5Props : undefined;
    publish(
      props.connConfig,
      {
        ...form,
        paytype: payloadType.value,
        properties,
      },
      (err) => {
        emit("publish", err);
      }
    );
  }
};
watch(
  () => props.topic,
  () => {
    form.topic = props.topic || "";
  },
  { deep: true, immediate: true }
);
watch(
  [() => props.paytype, () => props.payload],
  () => {
    if (props.paytype) {
      payloadType.value = props.paytype;
      if (props.paytype === "JSON") {
        payloadLang.value = "json";
        form.payload = props.payload || "{}";
      } else {
        payloadLang.value = "plaintext";
        form.payload = props.payload || "";
      }
    } else {
      payloadType.value = "JSON";
      form.payload = props.payload || "{}";
    }
  },
  { deep: true, immediate: true }
);
watch(
  () => props.connConfig,
  (config, old) => {
    if (config && config.id !== old?.id) {
      const { userrole, pushProps, mqttVersion, will, properties } = config;

      form.qos = userrole === "server" ? pushProps?.qos || 0 : 0;
      form.retain = pushProps?.retain || false;

      if (mqttVersion === "5.0") {
        mqtt5PropsEnable.value = true;
        Object.assign(mqtt5Props, {
          userProperties: properties || {},
          contentType: will.properties?.contentType || undefined,
          responseTopic: will.properties?.responseTopic || undefined,
          payloadFormatIndicator: will.properties?.payloadFormatIndicator || false,
          messageExpiryInterval: will.properties?.messageExpiryInterval || undefined,
          correlationData: will.properties?.correlationData || undefined,
          topicAlias: undefined,
          subscriptionIdentifier: undefined,
        });
        hasMqtt5Props.value = calcHasProps(mqtt5Props);
      } else {
        mqtt5PropsEnable.value = false;
        hasMqtt5Props.value = false;
        Object.assign(mqtt5Props, getInitMeatModel());
      }
    }
  },
  { deep: true, immediate: true }
);
</script>

<style scoped lang="scss">
.mqtt-publish {
  width: 100%;
  height: 100%;

  .mqtt-publish-header {
    height: 56px;
    font-size: 0;
    .mqtt-publish-metadata {
      display: flex;
      flex-direction: row;
      justify-content: start;
      align-items: center;

      width: 100%;
      height: 30px;
      padding: 1px 7px;
      .mqtt-publish-label {
        line-height: 24px;
        font-size: 12px;
        margin-left: 12px;
        margin-right: 4px;
        &:first-child {
          margin-left: 0;
        }
      }

      .mqtt-publish-select {
        width: 120px;
      }

      .mqtt-publish-checkbox.retain-block {
        margin: 0 12px;
      }

      .mqtt-publish-badge {
        height: 24px;
        padding: 0;
        font-size: 0;
      }
    }
    .mqtt-publish-topic-input {
      width: 100%;
      height: 26px;
    }
    .mqtt-tpls-btn {
      position: absolute;
      bottom: 4px;
      right: 2px;
      * {
        overflow: visible;
      }
      .el-button--small {
        --el-button-size: 18px;
        padding: 2px 5px;
      }
    }
  }
  .mqtt-publish-editor {
    position: relative;
    height: calc(100% - 56px);
    .mqtt-send-btn {
      position: absolute;
      bottom: 5px;
      right: 5px;
    }
  }
}
</style>

<style lang="scss">
.mqtt-publish {
  .mqtt-publish-header {
    position: relative;
    .mqtt-publish-topic-input {
      .el-input__wrapper {
        border-top: solid 1px rgba(0, 0, 0, 0.05);
        border-bottom: solid 1px rgba(0, 0, 0, 0.02);
        border-radius: 0;
        box-shadow: none;
      }
    }
  }
  .mqtt-publish-editor {
    .el-textarea {
      height: 100%;
      textarea.el-textarea__inner {
        width: 100%;
        height: 100%;
        padding: 3px 7px;
        line-height: 20px;
        font-size: 12px;
        border: none;
        outline: none;
        resize: none;
        border-radius: 0;
        box-shadow: none;
      }
    }
  }
  .mqtt-tpls-btn {
    overflow: visible;
    * {
      overflow: visible;
    }
  }
}
</style>

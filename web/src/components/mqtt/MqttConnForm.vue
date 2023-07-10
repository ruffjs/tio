<template>
  <el-drawer
    :model-value="isConnFormVisible"
    size="640"
    title="MQTT Connection Config"
    class="mqtt-connection-form"
    append-to-body
    @close="emit('close')"
  >
    <template #footer>
      <div style="flex: auto">
        <el-button :loading="connecting" @click="handleConnect(true)"
          >Test Connection</el-button
        >
        <el-button :loading="connecting" type="primary" @click="handleConnect(false)"
          >Connect</el-button
        >
      </div>
    </template>
    <el-form
      v-loading="
        connecting
          ? {
              text: 'Connecting...',
            }
          : false
      "
      :model="form"
      :rules="rules"
      ref="formRef"
    >
      <div class="el-collapse-item__header" style="font-size: 13px">General</div>
      <el-row :gutter="10">
        <el-col :span="23">
          <el-form-item :label-width="formLabelWidth" label="Role" prop="userrole">
            <el-select
              v-model="form.userrole"
              size="small"
              :disabled="Boolean(editConnId || createThingId)"
            >
              <el-option label="As A Thing" value="thing" />
              <el-option label="As A Biz Server" value="server" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="1"></el-col>
        <el-col :span="23">
          <el-form-item :label-width="formLabelWidth" label="Name" prop="name">
            <el-input v-model="form.name" size="small" clearable />
          </el-form-item>
        </el-col>
        <el-col :span="1">
          <el-tooltip
            v-if="isForCreate"
            effect="dark"
            content="Quick selection of created connection configurations"
            placement="top-end"
            popper-class="tooltip-box"
          >
            <el-icon><Warning /></el-icon>
          </el-tooltip>
        </el-col>
        <el-col :span="23">
          <el-form-item :label-width="formLabelWidth" label="Client ID" prop="clientId">
            <el-input v-model="form.clientId" disabled size="small" />
          </el-form-item>
        </el-col>
        <el-col :span="1">
          <el-icon v-if="form.userrole === 'server'" @click="resetClientID">
            <RefreshRight />
          </el-icon>
        </el-col>
        <el-col :span="23">
          <el-form-item :label-width="formLabelWidth" label="Host" prop="host">
            <el-col :span="6">
              <el-select v-model="form.protocol" size="small">
                <el-option label="ws://" value="ws" :disabled="form.ssl" />
                <el-option label="wss://" value="wss" />
              </el-select>
            </el-col>
            <el-col :span="18">
              <el-input v-model.trim="form.host" size="small" clearable />
            </el-col>
          </el-form-item>
        </el-col>
        <el-col :span="1"></el-col>
        <el-col :span="23">
          <el-form-item :label-width="formLabelWidth" label="Port" prop="port">
            <el-input-number
              v-model="form.port"
              :min="0"
              :max="65535"
              type="number"
              size="small"
              controls-position="right"
            >
            </el-input-number>
          </el-form-item>
        </el-col>
        <el-col :span="23">
          <el-form-item :label-width="formLabelWidth" label="Path" prop="path">
            <el-input v-model="form.path" size="small" clearable />
          </el-form-item>
        </el-col>
        <el-col :span="1"></el-col>

        <el-col :span="23">
          <el-form-item :label-width="formLabelWidth" label="Username" prop="username">
            <el-autocomplete
              v-if="form.userrole === 'thing'"
              v-model.trim="form.username"
              :fetch-suggestions="filterUsername"
              :disabled="Boolean(editConnId || createThingId)"
              clearable
              size="small"
              placeholder="Please select or input"
              autocomplete="off"
              @select="handleUsernameSelect"
            />
            <el-input v-else v-model.trim="form.username" size="small" clearable>
              <template #prepend>$</template>
            </el-input>
          </el-form-item>
        </el-col>
        <el-col :span="1"></el-col>
        <el-col :span="23">
          <el-form-item :label-width="formLabelWidth" label="Password" prop="password">
            <el-input
              v-model.trim="form.password"
              type="password"
              size="small"
              show-password
              clearable
              :disabled="
                (editConnId && form.userrole === 'thing') || Boolean(createThingId)
              "
            />
          </el-form-item>
        </el-col>
        <el-col :span="1"></el-col>
        <el-col :span="23">
          <el-form-item :label-width="formLabelWidth" label="SSL/TLS" prop="ssl">
            <el-switch
              v-model="form.ssl"
              active-color="#13ce66"
              inactive-color="#A2A9B0"
              @change="handleSSL"
            >
            </el-switch>
          </el-form-item>
        </el-col>
        <el-col :span="1"></el-col>

        <template v-if="form.ssl">
          <el-col :span="23">
            <el-form-item
              class="item-secure"
              label="SSL Secure"
              :label-width="formLabelWidth"
              prop="rejectUnauthorized"
            >
              <el-switch
                v-model="form.rejectUnauthorized"
                active-color="#13ce66"
                inactive-color="#A2A9B0"
              >
              </el-switch>
            </el-form-item>
          </el-col>
          <el-col :span="1">
            <el-tooltip
              effect="dark"
              content="Whether a client verifies the server's certificate chain and host name"
              placement="top-end"
              popper-class="tooltip-box"
            >
              <el-icon><Warning /></el-icon>
            </el-tooltip>
          </el-col>
          <el-col :span="1"> </el-col>
        </template>
      </el-row>
      <el-collapse v-model="activeNames">
        <el-collapse-item title="Advanced" name="Advanced">
          <el-row :gutter="10">
            <el-col :span="22">
              <el-form-item
                :label-width="formLabelWidthAdvanced"
                label="Connect Timeout"
                prop="connectTimeout"
              >
                <el-input-number
                  size="small"
                  type="number"
                  :min="0"
                  v-model="form.connectTimeout"
                  controls-position="right"
                >
                </el-input-number>
              </el-form-item>
            </el-col>
            <el-col :span="2"><div class="unit">(s)</div></el-col>
            <el-col :span="22">
              <el-form-item
                :label-width="formLabelWidthAdvanced"
                label="Keep Alive"
                prop="keepalive"
              >
                <el-input-number
                  size="small"
                  type="number"
                  :min="0"
                  v-model="form.keepalive"
                  controls-position="right"
                >
                </el-input-number>
              </el-form-item>
            </el-col>
            <el-col :span="2"><div class="unit">(s)</div></el-col>
            <el-col :span="22">
              <el-form-item
                :label-width="formLabelWidthAdvanced"
                label="Clean Session"
                prop="clean"
              >
                <el-radio-group v-model="form.clean">
                  <el-radio :label="true"></el-radio>
                  <el-radio :label="false"></el-radio>
                </el-radio-group>
              </el-form-item>
            </el-col>
            <el-col :span="2"></el-col>
            <el-col :span="22">
              <el-form-item
                :label-width="formLabelWidthAdvanced"
                label="Auto Reconnect"
                prop="reconnect"
              >
                <el-radio-group v-model="form.reconnect">
                  <el-radio :label="true"></el-radio>
                  <el-radio :label="false"></el-radio>
                </el-radio-group>
              </el-form-item>
            </el-col>
            <el-col :span="2"></el-col>
            <template v-if="form.reconnect">
              <el-col :span="22">
                <el-form-item
                  :label-width="formLabelWidthAdvanced"
                  label="Reconnect Period"
                  prop="reconnectPeriod"
                >
                  <el-input-number
                    size="small"
                    type="number"
                    :min="1"
                    v-model="form.reconnectPeriod"
                    controls-position="right"
                  >
                  </el-input-number>
                </el-form-item>
              </el-col>
              <el-col :span="2">
                <div class="unit">(ms)</div>
              </el-col>
            </template>
            <el-col :span="22">
              <el-form-item
                :label-width="formLabelWidthAdvanced"
                label="MQTT Version"
                prop="mqttVersion"
              >
                <el-select size="small" v-model="form.mqttVersion">
                  <el-option value="3.1.1" label="3.1.1"></el-option>
                  <el-option value="5.0" label="5.0"></el-option>
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="2"></el-col>

            <!-- MQTT v5.0 -->
            <template v-if="form.mqttVersion === '5.0'">
              <el-col :span="22">
                <el-form-item
                  :label-width="formLabelWidthAdvanced"
                  label="Session Expiry Interval"
                  prop="sessionExpiryInterval"
                >
                  <el-input
                    size="small"
                    type="number"
                    :min="1"
                    v-model.number="form.properties.sessionExpiryInterval"
                  >
                  </el-input>
                </el-form-item>
              </el-col>
              <el-col :span="2">
                <div class="unit">(s)</div>
              </el-col>
              <el-col :span="22">
                <el-form-item
                  :label-width="formLabelWidthAdvanced"
                  label="Receive Maximum"
                  prop="receiveMaximum"
                >
                  <el-input
                    size="small"
                    type="number"
                    :min="1"
                    v-model.number="form.properties.receiveMaximum"
                  >
                  </el-input>
                </el-form-item>
              </el-col>
              <el-col :span="2"></el-col>
              <el-col :span="22">
                <el-form-item
                  :label-width="formLabelWidthAdvanced"
                  label="Maximum Packet Size"
                  prop="maximumPacketSize"
                >
                  <el-input
                    size="small"
                    type="number"
                    :min="100"
                    v-model.number="form.properties.maximumPacketSize"
                  >
                  </el-input>
                </el-form-item>
              </el-col>
              <el-col :span="2"></el-col>
              <el-col :span="22">
                <el-form-item
                  :label-width="formLabelWidthAdvanced"
                  label="Topic Alias Maximum"
                  prop="topicAliasMaximum"
                >
                  <el-input
                    size="small"
                    type="number"
                    :min="1"
                    v-model.number="form.properties.topicAliasMaximum"
                  >
                  </el-input>
                </el-form-item>
              </el-col>
              <el-col :span="2"></el-col>
              <el-col :span="22">
                <el-form-item
                  :label-width="formLabelWidthAdvanced"
                  label="Request Response Info"
                  prop="requestResponseInformation"
                >
                  <el-radio-group v-model="form.properties.requestResponseInformation">
                    <el-radio :label="true"></el-radio>
                    <el-radio :label="false"></el-radio>
                  </el-radio-group>
                </el-form-item>
              </el-col>
              <el-col :span="2"></el-col>
              <el-col :span="22">
                <el-form-item
                  :label-width="formLabelWidthAdvanced"
                  label="Request Problem Info"
                  prop="requestProblemInformation"
                >
                  <el-radio-group v-model="form.properties.requestProblemInformation">
                    <el-radio :label="true"></el-radio>
                    <el-radio :label="false"></el-radio>
                  </el-radio-group>
                </el-form-item>
              </el-col>
              <el-col :span="2"></el-col>
            </template>
          </el-row>
          <KeyValueEditor
            title="User Properties"
            v-if="form.mqttVersion === '5.0'"
            v-model="form.properties.userProperties"
          />
        </el-collapse-item>
        <el-collapse-item title="Last Will and Testament" name="WillMessage">
          <el-row :gutter="10">
            <el-col :span="22">
              <el-form-item
                :label-width="formLabelWidthAdvanced"
                label="Last-Will Topic"
                prop="will.lastWillTopic"
              >
                <el-input size="small" v-model="form.will.lastWillTopic"></el-input>
              </el-form-item>
            </el-col>
            <el-col :span="2"></el-col>
            <el-col :span="22">
              <el-form-item
                :label-width="formLabelWidthAdvanced"
                label="Last-Will QoS"
                prop="will.lastWillQos"
              >
                <el-radio-group v-model="form.will.lastWillQos">
                  <el-radio :label="0"></el-radio>
                  <el-radio :label="1"></el-radio>
                  <el-radio :label="2"></el-radio>
                </el-radio-group>
              </el-form-item>
            </el-col>
            <el-col :span="2"></el-col>
            <el-col :span="22">
              <el-form-item
                :label-width="formLabelWidthAdvanced"
                label="Last-Will Retain"
                prop="will.lastWillRetain"
              >
                <el-radio-group v-model="form.will.lastWillRetain">
                  <el-radio :label="true"></el-radio>
                  <el-radio :label="false"></el-radio>
                </el-radio-group>
              </el-form-item>
            </el-col>
            <el-col :span="2"></el-col>
            <el-col :span="22">
              <el-form-item
                class="will-payload-box"
                :label-width="formLabelWidthAdvanced"
                label="Last-Will Payload"
                prop="will.lastWillPayload"
              >
                <!-- <div class="last-will-payload">
                  <Editor
                    ref="lastWillPayload"
                    id="lastWillPayload"
                    :lang="payloadType"
                    :fontSize="12"
                    v-model="form.will.lastWillPayload"
                    scrollbar-status="auto"
                  />
                </div>
                <div class="lang-type">
                  <el-radio-group v-model="payloadType">
                    <el-radio label="json">JSON</el-radio>
                    <el-radio label="plaintext">Plaintext</el-radio>
                  </el-radio-group>
                </div> -->
                <el-input
                  v-model="form.will.lastWillPayload"
                  :autosize="{ minRows: 3 }"
                  type="textarea"
                  placeholder="Please input"
                />
              </el-form-item>
            </el-col>
            <el-col :span="2"></el-col>

            <!-- MQTT v5.0 -->
            <template v-if="form.mqttVersion === '5.0'">
              <el-col :span="22">
                <el-form-item
                  :label-width="formLabelWidthAdvanced"
                  label="Payload Format Indicator"
                  prop="payloadFormatIndicator"
                >
                  <el-radio-group v-model="form.will.properties.payloadFormatIndicator">
                    <el-radio :label="true"></el-radio>
                    <el-radio :label="false"></el-radio>
                  </el-radio-group>
                </el-form-item>
              </el-col>
              <el-col :span="2"></el-col>
              <el-col :span="22">
                <el-form-item
                  :label-width="formLabelWidthAdvanced"
                  label="Will Delay Interval"
                  prop="willDelayInterval"
                >
                  <el-input
                    size="small"
                    type="number"
                    :min="0"
                    v-model.number="form.will.properties.willDelayInterval"
                  >
                  </el-input>
                </el-form-item>
              </el-col>
              <el-col :span="2"><div class="unit">(s)</div></el-col>
              <el-col :span="22">
                <el-form-item
                  :label-width="formLabelWidthAdvanced"
                  label="Message Expiry Interval"
                  props="messageExpiryInterval"
                >
                  <el-input
                    size="small"
                    type="number"
                    :min="0"
                    v-model.number="form.will.properties.messageExpiryInterval"
                  >
                  </el-input>
                </el-form-item>
              </el-col>
              <el-col :span="2"><div class="unit">(s)</div></el-col>
              <el-col :span="22">
                <el-form-item
                  class="content-type-item"
                  :label-width="formLabelWidthAdvanced"
                  label="Content Type"
                  prop="contentType"
                >
                  <el-input size="small" v-model="form.will.properties.contentType">
                  </el-input>
                </el-form-item>
              </el-col>
              <el-col :span="2"></el-col>
              <el-col :span="22">
                <el-form-item
                  class="content-type-item"
                  :label-width="formLabelWidthAdvanced"
                  label="Response Topic"
                  prop="responseTopic"
                >
                  <el-input size="small" v-model="form.will.properties.responseTopic">
                  </el-input>
                </el-form-item>
              </el-col>
              <el-col :span="2"></el-col>
              <el-col :span="22">
                <el-form-item
                  class="content-type-item"
                  :label-width="formLabelWidthAdvanced"
                  label="Correlation Data"
                  prop="correlationData"
                >
                  <el-input size="small" v-model="form.will.properties.correlationData">
                  </el-input>
                </el-form-item>
              </el-col>
              <el-col :span="2"></el-col>
            </template>
          </el-row>
        </el-collapse-item>
      </el-collapse>
    </el-form>
  </el-drawer>
</template>

<script setup>
import { reactive, ref, watch, nextTick, computed } from "vue";
// import { useStore } from "vuex";
import { createClient, getDefaultForm } from "@/utils/mqtt";
import { Warning, RefreshRight } from "@element-plus/icons-vue";

import dayjs from "dayjs";
import { ElNotification } from "element-plus";
import KeyValueEditor from "@/components/common/KeyValueEditor.vue";
import useMqtt from "@/reactives/useMqtt";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import useLayout from "@/reactives/useLayout";
import { genClientIdSuffix } from "@/utils/generators";

let oldName = "";
let connectedToken = "";
const formLabelWidth = "100px";
const formLabelWidthAdvanced = "180px";
const rules = {
  name: [
    { required: true, message: "Please input", trigger: "blur" },
    { min: 3, max: 50, message: "Length should be 3 to 50", trigger: "blur" },
    {
      validator: (_rule, name, callBack) => {
        for (const connection of connections.value) {
          if (isForCreate.value && connection.name === name) {
            callBack("Duplicate name. Please rename it!");
            return;
          } else if (!isForCreate.value && name !== oldName && connection.name === name) {
            callBack("Duplicate name. Please rename it!");
            return;
          }
        }
        callBack();
      },
      trigger: "blur",
    },
  ],
  username: [{ required: true, message: "Please select or input", trigger: "blur" }],
  password: [{ required: true, message: "Please input", trigger: "blur" }],
  path: [{ required: true, message: "Please input" }],
  host: [{ required: true, message: "Please input" }],
  port: [{ required: true, message: "Please input" }],
  certType: [{ required: true, message: "Please select" }],
  ca: [{ required: true, message: "Please input" }],
};
const emit = defineEmits(["close", "done"]);
const { things } = useThingsAndShadows();
const { connections, setConnConfig, getConnConfig } = useMqtt();
const {
  isConnFormVisible,
  editConnId,
  isForCreate,
  createThingId,
  connectedCbT,
} = useLayout();
const formRef = ref();
const form = reactive(getDefaultForm());
const activeNames = ref([]);
const connecting = ref(false);
const handleSSL = (val) => {
  const { protocol } = form;
  changeProtocol(protocol, val);
  if (!val) {
    form.certType = "";
  }
};

const changeProtocol = (protocol, isSSL) => {
  if (!protocol) {
    return false;
  }
  if (/ws/gi.test(protocol)) {
    form.protocol = isSSL ? "wss" : "ws";
  }
};

const emptyToNull = (data) => {
  Object.entries(data).forEach((entry) => {
    const [key, value] = entry;
    if (value === "") {
      data[key] = null;
    }
  });
  return data;
};

let testClient = null;
const onSuccess = () => {
  connecting.value = false;
  testClient?.end(true);
  ElNotification({
    title: "Connected",
    message: `Test client <${form.name}> connected`,
    type: "success",
  });
};
const onFailed = (error) => {
  connecting.value = false;
  testClient?.end(true, (err) => {
    if (err) {
    } else {
      const message = error?.toString() ? error.toString() : `Cconnect failed`;
      ElNotification({
        title: "Failed",
        message,
        type: "error",
      });
    }
  });
};
const onClose = () => {
  console.log("test client close");
  connecting.value = false;
  testClient?.end(true);
};
const onEnd = () => {
  console.log("test client end");
  connecting.value = false;
  testClient = null;
};
const connect = (data) => {
  if (connecting.value || testClient?.connected) return false;
  connecting.value = true;
  // new client
  const { curConnectClient } = createClient(data);
  curConnectClient.once("connect", onSuccess);
  curConnectClient.once("error", onFailed);
  curConnectClient.once("close", onClose);
  curConnectClient.once("end", onEnd);

  testClient = curConnectClient;
};

const handleConnect = async (test = false) => {
  if (!formRef.value) return;
  try {
    const valid = await formRef.value.validate();
    if (valid) {
      const data = { ...form };
      data.properties = emptyToNull(data.properties);
      if (data.userrole === "server") {
        data.username = `\$${data.username}`;
      }
      // console.log("data", data);
      if (test) {
        return connect(data);
      }
      let res = null;
      if (isForCreate.value) {
        // create a new connection
        res = setConnConfig(null, {
          ...data,
          createAt: dayjs().format("YYYY-MM-DD HH:mm:ss:SSS"),
          updateAt: dayjs().format("YYYY-MM-DD HH:mm:ss:SSS"),
        });
      } else {
        // update a exisit connection
        if (data.id) {
          res = setConnConfig(data.id, {
            ...data,
            updateAt: dayjs().format("YYYY-MM-DD HH:mm:ss:SSS"),
          });
        }
      }
      emit("done", res, connectedToken);
    } else {
      console.log("error submit!");
      return false;
    }
  } catch (error) {
    console.error("validate error!", error);
  }
};

const filterUsername = (qs, cb) => {
  if (createThingId.value) {
    const thing = things.value.find(({ thingId }) => thingId === createThingId.value);
    if (thing) {
      cb([
        {
          value: thing.thingId,
          clientId: thing.thingId,
          password: thing.authValue,
        },
      ]);
    } else {
      cb([]);
    }
  } else {
    // form.password = "";
    const results = qs
      ? things.value
          .filter(({ thingId }) => thingId.toLowerCase().indexOf(qs.toLowerCase()) > -1)
          .slice(0, 1000)
      : things.value.slice(0, 1000);
    cb(
      results.map((thing) => ({
        value: thing.thingId,
        clientId: thing.thingId,
        password: thing.authValue,
      }))
    );
  }
};

const handleUsernameSelect = (item) => {
  form.clientId = item.clientId;
  form.password = item.password;
};

watch(isConnFormVisible, async () => {
  oldName = "";
  connectedToken = "";
  if (editConnId.value) {
    const config = getConnConfig(editConnId.value);
    form.userrole = config.userrole;
    oldName = config.name;
    await nextTick();
    let username = config.username;
    if (config.userrole === "server") {
      username = config.username.slice(1);
    }
    // console.log(conn.userrole, username);
    Object.assign(form, { ...config, username });
  } else if (createThingId.value) {
    const thing = things.value.find(({ thingId }) => thingId === createThingId.value);
    if (thing) {
      connectedToken = connectedCbT.value;
      form.userrole = "thing";
      form.name = `Client For ${createThingId.value} (default)`;
      form.username = createThingId.value;
      form.clientId = createThingId.value;
      form.password = thing.authValue;
      form.mqttVersion = "3.1.1";
    }
  } else {
    Object.assign(form, getDefaultForm());
  }
});

const resetClientID = () => {
  if (form.username && form.userrole === "server") {
    form.clientId = `\$${form.username}_${genClientIdSuffix()}`;
  } else {
    form.clientId = "";
  }
};

watch(
  () => form.userrole,
  () => {
    if (createThingId.value) return;
    form.username = "";
  }
);

watch(
  () => form.username,
  () => {
    if (createThingId.value) return;
    if (form.username) {
      if (form.userrole === "server") {
        form.clientId = `\$${form.username}_${genClientIdSuffix()}`;
      }
    } else {
      form.clientId = "";
    }
  }
);
</script>

<style lang="scss">
.mqtt-connection-form {
  .el-form {
    .el-form-item {
      margin-bottom: 13px;
    }

    .el-col-1 {
      padding-top: 5px;
      text-align: right;
    }
    .el-col-2 {
      padding-top: 3px;
    }

    .el-col-6 {
      padding-left: 0 !important;
    }
    .el-col-18 {
      padding-right: 0 !important;
    }
    .el-col-22,
    .el-col-23 {
      .el-select,
      .el-autocomplete,
      .el-input-number {
        width: 100%;
      }
    }
  }
}
</style>

<style lang="scss">
.mqtt-connection-form {
  .el-drawer__header {
    margin-bottom: 20px;
    .el-drawer__title {
      font-weight: 700;
    }
  }
  .el-drawer__body {
    padding: 0 var(--el-drawer-padding-primary) 10px;
  }
  .el-form {
    .el-form-item {
      .el-form-item__error {
        margin-top: -2px;
        padding-top: 0;
      }
    }
    .el-col-1 {
      .el-icon {
        cursor: pointer;
      }
    }
    .el-col-22,
    .el-col-23 {
      .el-input-number {
        .el-input__inner {
          text-align: left;
        }
      }
    }
  }
}
</style>

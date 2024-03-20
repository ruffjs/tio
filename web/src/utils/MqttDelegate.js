import dayjs from "dayjs";
import store from "@/store";
import { genDelegateId, genMessageId } from "@/utils/generators";
import { convertPayloadByType, createClient } from "@/utils/mqtt";
import {
  TSCE_MQTO,
  TSCE_MQTT,
  TSCE_MSGI,
  TSCE_MSGO,
  notifyThingStateChange,
} from "@/utils/event";
import { notifyDone, notifyFail, notifyWarn } from "./layout";

const delegateSharedStates = {};

class MqttDelegate {
  constructor(connConfig) {
    this._self_id = genDelegateId();
    this.id = connConfig.id;
    this.role = connConfig.userrole;
    this.thingId = this.role === "thing" ? connConfig.username : "+";
    this.retryTimes = 0;
    this.subs = {};
    this.messages = [];
    this.unreadMessageCount = 0;
    this.receivedMsgType = "Plaintext";
  }

  destroy() {
    this.disconnect(() => {
      delete delegateSharedStates[this.id];
    });
  }

  // 当前 MQTT Client 是否为连接状态
  get isConnected() {
    // console.log(this.client);
    return this.client?.connected || false;
  }

  // 当前 MQTT Client 是否为连接中或断开连接中
  get isConnectingOrDisconnecting() {
    // console.log(this.client);
    return (
      this.client?.connecting ||
      this.client?.reconnecting ||
      this.client?.disconnecting ||
      false
    );
  }

  get isConnActive() {
    return (
      this.client?.connected ||
      this.client?.connecting ||
      this.client?.reconnecting ||
      this.client?.disconnecting ||
      false
    );
  }

  // 当前 MQTT Client 是否为活动窗口
  get isActive() {
    return (
      store.state.layout.activeToolKey === "mqtt" &&
      this.id === store.state.mqtt.currentConnId
    );
  }

  // 当前 Client 所关联的 Thing 是否正处于详情页面
  get isOfCurrentThing() {
    const currentId = store.state.app.currentShadow?.thingId;
    if (currentId) return this.isClientOfThing(currentId);
    return false;
  }

  isClientOfThing(thingId) {
    const isThing = this.role === "thing";
    if (isThing && thingId) {
      return this.thingId === thingId;
    }
    return isThing;
  }

  updateStore() {
    if (this.client) {
      store.dispatch("mqtt/updateDelegateStates", {
        id: this.id,
        client: this.client,
        subs: this.subs || {},
        messages: this.messages,
        unreadMessageCount: this.unreadMessageCount,
        receivedMsgType: this.receivedMsgType,
      });
    } else {
      store.dispatch("mqtt/updateDelegateStates", {
        id: this.id,
        client: {
          connected: false,
        },
      });
    }
  }

  removeTopic(topic) {
    delete this.subs[topic];
    if (this.isActive || this.isOfCurrentThing) {
      this.updateStore();
    }
  }

  cancel() {
    this.disconnect();
  }
  disconnect(cb) {
    if (this.client?.end) {
      this.client.end((err) => {
        if (err) {
          console.error("error", err);
          cb && cb(err);
        } else {
          this.retryTimes = 0;
          this.subs = {};
          if (typeof cb === "function") {
            cb();
          } else {
            this.updateStore();
          }
        }
      });
    } else {
      cb && cb();
    }
  }
  connect(connConfig, connectedToken = "") {
    this.disconnect((err) => {
      if (err) {
        console.log("connect -> disconnect -> err", err);
        notifyFail("Connect Failure");
      } else {
        this.retryTimes = 0;
        store.commit("mqtt/setState", {
          connecting: true,
          retryTimes: this.retryTimes,
        });
        const { curConnectClient } = createClient(connConfig);
        this.client = curConnectClient;

        curConnectClient.on("connect", () => {
          this.onconnect(connConfig, connectedToken);
          connectedToken = "";
        });
        curConnectClient.on("reconnect", this.onreconnect.bind(this));
        curConnectClient.on("error", this.onerror.bind(this));
        curConnectClient.on("close", this.onclose.bind(this));
        curConnectClient.on("message", this.onmessage.bind(this));
        curConnectClient.once("end", this.onend.bind(this));
      }
    });
  }

  onconnect(connConfig, connectedToken = "") {
    // console.log("connected", this.id, this._self_id);
    notifyDone("Connected");
    this.updateStore();
    if (this.isClientOfThing()) {
      notifyThingStateChange(this.thingId, TSCE_MQTT, {
        connConfig,
        connectedToken,
      });
    }
  }
  onreconnect() {
    // console.log("connected", this.id, this._self_id);
    if (this.retryTimes < 3) {
      console.log(this.id, "is reconnecting...", this.retryTimes);
      this.retryTimes++;
      store.commit("mqtt/setState", {
        connecting: true,
        retryTimes: this.retryTimes,
      });
      notifyWarn("reconnecting...");
    } else {
      this.disconnect(() => {
        this.retryTimes = 0;
        this.updateStore();
      });
    }
  }
  onerror(err) {
    console.error("onerror", err);
    notifyFail(err?.toString() || "Connection encounter error");
    this.updateStore();
  }
  onclose(reason) {
    if (reason) {
      notifyFail(reason.toString());
    }
    this.updateStore();
  }
  onend() {
    this.updateStore();
    if (this.isClientOfThing()) {
      notifyThingStateChange(this.thingId, TSCE_MQTO, {});
    }
    notifyDone("Disconnected");
  }
  onmessage(topic, payload, packet) {
    const { qos, retain, properties } = packet;
    const convertPayload = convertPayloadByType(
      payload,
      this.receivedMsgType,
      "receive"
    );
    const receivedMessage = {
      id: genMessageId(),
      out: false,
      createAt: dayjs().format("YYYY-MM-DD HH:mm:ss:SSS"),
      topic,
      payload: convertPayload,
      qos,
      retain,
      properties,
    };
    this.push(receivedMessage);
  }

  push(message) {
    // console.log(message.out ? "messageOut" : "messageIn", message);
    this.messages.push(message);
    // console.log(this.isActive, this.messages);
    if (!this.isActive && !message.out) {
      this.a += 1;
    } else {
      this.unreadMessageCount = 0;
    }
    if (this.isActive || this.isOfCurrentThing) {
      this.updateStore();
    }

    notifyThingStateChange(
      this.isClientOfThing() ? this.thingId : "*",
      message.out ? TSCE_MSGO : TSCE_MSGI,
      {
        topic: message.topic,
      }
    );
  }
  clear() {
    this.messages = [];
    if (this.isActive || this.isOfCurrentThing) {
      this.updateStore();
    }
  }
  read() {
    this.unreadMessageCount = 0;
  }

  pub(topic, { paytype, payload }, opts, cb) {
    // const { topic, qos, payload, retain } = this.messageRecord;
    if (!this.isConnected) {
      notifyFail("Client not connected");
      return;
    }

    opts = JSON.parse(JSON.stringify(opts));
    if (opts.properties) {
      if (opts.properties.userProperties) {
        let keyCount = 0;
        const userProperties = {};
        Object.keys(opts.properties.userProperties).forEach((key) => {
          if (opts.properties.userProperties[key]) {
            userProperties[key] = opts.properties.userProperties[key];
            keyCount++;
          }
        });
        if (keyCount) {
          opts.properties.userProperties = userProperties;
        } else {
          delete opts.properties.userProperties;
        }
      }
      if (Object.keys(opts.properties) === 0) {
        delete opts.properties;
      }
    }
    if (!topic && !opts.properties?.topicAlias) {
      notifyFail("Topic Required!");
      return;
    }
    if (topic && (topic.includes("+") || topic.includes("#"))) {
      notifyFail("Topic Cannot Contain '+' or '#'!");
      return false;
    }

    const args = [topic];
    args.push(convertPayloadByType(payload, paytype || "JSON"));
    if (opts) args.push(opts);
    args.push((err) => {
      if (err) {
        console.error("err", err);
        if (typeof cb === "function") {
          return cb(err);
        } else {
          return notifyFail(err?.toString() || "Publishing failure");
        }
      }
      const message = {
        out: true,
        createAt: dayjs().format("YYYY-MM-DD HH:mm:ss:SSS"),
        topic,
        payload,
        ...(opts || {}),
      };
      this.push(message);
      cb();
    });
    this.client.publish(...args);
  }

  onsub(topic, cb, err, granted) {
    // console.log(topic, err, granted);
    if (err) {
      console.error("err", err);
      if (typeof cb === "function") {
        cb(err);
      } else {
        notifyFail(err?.toString() || "Subscription failure");
      }
    } else {
      if (
        granted &&
        granted[0] &&
        [0, 1, 2].includes(granted[0].qos) === false
      ) {
        if (typeof cb === "function") {
          cb(new Error("Subscription failure"));
        } else {
          notifyFail("Subscription failure");
        }
      } else {
        if (typeof topic === "string") {
          this.subs[topic] = true;
        } else {
          topic.forEach((t) => {
            this.subs[t] = true;
          });
        }
        this.updateStore();
        if (typeof cb === "function") {
          cb();
        } else {
          notifyDone("Topic(s) Subscribed");
        }
      }
    }
  }
  onsubmap(map, cb, err) {
    if (err) {
      console.error("err", err);
      if (typeof cb === "function") {
        cb(err);
      } else {
        notifyFail(err?.toString() || "Subscription failure");
      }
    } else {
      Object.keys(map).forEach((topic) => {
        this.subs[topic] = true;
      });
      this.updateStore();
      if (typeof cb === "function") {
        cb();
      } else {
        notifyDone("Topic(s) Subscribed");
      }
    }
  }
  sub(topic, opts, cb) {
    if (!this.isConnected) {
      notifyFail("Client not connected");
      return;
    }
    const args = [topic];
    if (
      typeof topic === "string" ||
      (typeof topic === "object" && topic instanceof Array)
    ) {
      if (opts) args.push(opts);
      args.push(this.onsub.bind(this, topic, cb));
    } else {
      args.push(this.onsubmap.bind(this, topic, cb));
    }
    // console.log(args);
    this.client.subscribe(...args);
  }

  unsub(topics, cb) {
    if (!this.isConnected) {
      notifyFail("Client not connected");
      return;
    }
    this.client.unsubscribe(topics, (err, packet) => {
      // console.log(err, packet);
      if (err) {
        console.error("err", err);
        if (typeof cb === "function") {
          return cb(err);
        } else {
          notifyFail(err?.toString() || "Unsubscriebe failure");
        }
      } else {
        topics.forEach((topic) => {
          this.subs[topic] = false;
        });
        this.updateStore();

        if (typeof cb === "function") {
          cb();
        } else {
          notifyDone("Topic(s) Unsubscribed");
        }
      }
    });
  }
}

export const createDelegate = (connConfig) => {
  deleteDelegateById(connConfig.id);
  delegateSharedStates[connConfig.id] = new MqttDelegate(connConfig);
  return delegateSharedStates[connConfig.id];
};

export const getDelegateById = (id, connConfig) => {
  if (delegateSharedStates[id]) {
    return delegateSharedStates[id];
  }
  if (connConfig) {
    return createDelegate(connConfig);
  }
  return null;
};

export const deleteDelegateById = (id) => {
  if (delegateSharedStates[id]) {
    delegateSharedStates[id].destroy();
    delete delegateSharedStates[id];
  }
};

store.state.mqtt.connectionConfigs.forEach(createDelegate);

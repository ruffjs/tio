import mqtt from "mqtt";
import store from "@/store";
import { ElNotification } from "element-plus";
import _ from "lodash";
import dayjs from "dayjs";

const getSSLFile = (sslPath) => {
  const { ca, cert, key } = sslPath;
  try {
    const res = {
      ca: ca !== "" ? ca : undefined,
      cert: cert !== "" ? cert : undefined,
      key: key !== "" ? key : undefined,
    };
    return res;
  } catch (err) {
    ElNotification({
      title: err.toString(),
      message: "",
      type: "error",
    });
  }
};

const setMQTT5Properties = (option) => {
  if (option === undefined) {
    return undefined;
  }
  const properties = _.cloneDeep(option);
  return Object.fromEntries(
    Object.entries(properties).filter(([_, v]) => v !== null && v !== undefined)
  );
};

const setWillMQTT5Properties = (option) => {
  if (option === undefined) {
    return undefined;
  }
  const properties = _.cloneDeep(option);
  return Object.fromEntries(
    Object.entries(properties).filter(([_, v]) => v !== null && v !== undefined)
  );
};

const convertSecondsToMs = (seconds) => seconds * 1000;

const getClientOptions = (record) => {
  const mqttVersionDict = {
    "3.1.1": 4,
    "5.0": 5,
  };
  const {
    clientId,
    username,
    password,
    keepalive,
    clean,
    connectTimeout,
    ssl,
    certType,
    mqttVersion,
    reconnect,
    reconnectPeriod, // reconnectPeriod = 0 disabled automatic reconnection in the client
    will,
    rejectUnauthorized,
    clientIdWithTime,
  } = record;
  const protocolVersion = mqttVersionDict[mqttVersion];
  const options = {
    clientId,
    keepalive,
    clean,
    reconnectPeriod: reconnect ? reconnectPeriod : 0,
    protocolVersion,
  };
  options.connectTimeout = convertSecondsToMs(connectTimeout);
  // Append timestamp to MQTT client id
  if (clientIdWithTime) {
    const clickIconTime = Date.parse(new Date().toString());
    options.clientId = `${options.clientId}_${clickIconTime}`;
  }
  // Auth
  if (username !== "") {
    options.username = username;
  }
  if (password !== "") {
    options.password = password;
  }
  // MQTT Version
  if (protocolVersion === 5 && record.properties) {
    const properties = setMQTT5Properties(record.properties);
    if (properties && Object.keys(properties).length > 0) {
      options.properties = properties;
    }
  }
  // else if (protocolVersion === 3) {
  //   options.protocolId = 'MQIsdp'
  // }
  // SSL
  if (ssl) {
    options.rejectUnauthorized =
      rejectUnauthorized === undefined ? true : rejectUnauthorized;
    if (certType === "self") {
      const sslRes = getSSLFile({
        ca: record.ca,
        cert: record.cert,
        key: record.key,
      });
      if (sslRes) {
        options.ca = sslRes.ca;
        options.cert = sslRes.cert;
        options.key = sslRes.key;
      }
    }
  }
  // Will Message
  if (will) {
    const {
      lastWillTopic: topic,
      lastWillPayload: payload,
      lastWillQos: qos,
      lastWillRetain: retain,
    } = will;
    if (topic) {
      options.will = { topic, payload, qos, retain };
      if (protocolVersion === 5) {
        const { properties } = will;
        if (properties) {
          const willProperties = setWillMQTT5Properties(properties);
          if (willProperties && Object.keys(willProperties).length > 0) {
            options.will.properties = willProperties;
          }
        }
      }
    }
  }
  // Auto Resubscribe, Valid only when reconnecting
  options.resubscribe = store.getters["mqtt/autoResub"];
  return options;
};

const getUrl = (record) => {
  const { host, port, path } = record;
  const protocol = getMQTTProtocol(record);

  let url = `${protocol}://${host}:${port}`;
  if (protocol === "ws" || protocol === "wss") {
    url = `${url}${path.startsWith("/") ? "" : "/"}${path}`;
  }
  return url;
};

export const createClient = (record) => {
  const options = getClientOptions(record);
  const url = getUrl(record);
  const curConnectClient = mqtt.connect(url, options);

  return { curConnectClient, connectUrl: url };
};

// Prevent old data from missing protocol field
export const getMQTTProtocol = (data) => {
  const { protocol, ssl } = data;
  if (!protocol) {
    return ssl ? "wss" : "ws";
  }
  return protocol;
};

export const convertPayloadByType = (value, type, way) => {
  const validJSONType = (jsonValue, warnMessage) => {
    try {
      JSON.parse(jsonValue);
    } catch (error) {
      this.$message.warning(`${warnMessage} ${error.toString()}`);
    }
  };
  const genPublishPayload = (publishType, publishValue) => {
    if (publishType === "Base64" || publishType === "Hex") {
      const $type = publishType.toLowerCase();
      return Buffer.from(publishValue, $type);
    }
    if (publishType === "JSON") {
      validJSONType(publishValue, "Publish message");
    }
    return publishValue;
  };
  const genReceivePayload = (receiveType, receiveValue) => {
    if (receiveType === "Base64" || receiveType === "Hex") {
      const $type = receiveType.toLowerCase();
      return receiveValue.toString($type);
    }
    if (receiveType === "JSON") {
      validJSONType(receiveValue.toString(), "Received message");
    }
    return receiveValue.toString();
  };
  if (way === "publish" && typeof value === "string") {
    return genPublishPayload(type, value);
  } else if (way === "receive" && typeof value !== "string") {
    return genReceivePayload(type, value);
  }
  return value;
};

export const getInitMeatModel = () => ({
  userProperties: {},
  contentType: undefined,
  payloadFormatIndicator: false,
  messageExpiryInterval: undefined,
  topicAlias: undefined,
  responseTopic: undefined,
  correlationData: undefined,
  subscriptionIdentifier: undefined,
});

export const getDefaultForm = () => {
  const { hostname, protocol } = window.location;
  return {
    userrole: "thing",
    clientId: "", // getClientId(),
    createAt: dayjs().format("YYYY-MM-DD HH:mm:ss:SSS"),
    updateAt: dayjs().format("YYYY-MM-DD HH:mm:ss:SSS"),
    name: "",
    clean: true,
    protocol: protocol === "https:" ? "wss" : "ws",
    host: hostname,
    keepalive: 60,
    connectTimeout: 10,
    reconnect: true,
    reconnectPeriod: 4000,
    username: "",
    password: "",
    path: "/",
    port: 8083,
    ssl: false,
    certType: "",
    rejectUnauthorized: true,
    ca: "",
    cert: "",
    key: "",
    mqttVersion: "5.0",
    subscriptions: [],
    pushProps: {},
    will: {
      lastWillTopic: "",
      lastWillPayload: "",
      lastWillQos: 0,
      lastWillRetain: false,
      properties: {
        payloadFormatIndicator: undefined,
        willDelayInterval: undefined,
        messageExpiryInterval: undefined,
        contentType: "",
        responseTopic: "",
        correlationData: undefined,
        userProperties: undefined,
      },
    },
    properties: {
      sessionExpiryInterval: undefined,
      receiveMaximum: undefined,
      maximumPacketSize: undefined,
      topicAliasMaximum: undefined,
      requestResponseInformation: undefined,
      requestProblemInformation: undefined,
      userProperties: undefined,
      authenticationMethod: undefined,
      authenticationData: undefined,
    },
    clientIdWithTime: false,
  };
};

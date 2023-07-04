import pubTopicsShadow from "./mqtt-suggestions/pub-topics-shadow";
import pubTopicsThing from "./mqtt-suggestions/pub-topics-thing";
import { getShadow, updateShadow } from "./mqtt-suggestions/sub-topics-shadow";
import {
  common,
  invokeDirectMethod,
  ntp,
} from "./mqtt-suggestions/sub-topics-thing";

export const tools = [
  {
    key: "mqtt",
    icon: "Connection",
    name: "MQTT Clients",
    height: 350,
  },
  {
    key: "logs",
    icon: "Memo",
    name: "HTTP Logs",
    height: 248,
  },
  {
    key: "apis",
    icon: "Compass",
    name: "Swagger (View API Definitions)",
    link: "/docs",
  },
];

export const mqttFields = [
  {
    key: "broker",
    label: "Host",
    type: "string",
  },
  {
    key: "clientId",
    label: "Client Id",
    type: "string",
  },
  {
    key: "username",
    label: "Username",
    type: "string",
  },
  {
    key: "password",
    label: "Password",
    type: "password",
  },
  // {
  //   key: "clean",
  //   label: "Clean Session",
  //   type: "boolean",
  // },
  {
    key: "mqttVersion",
    label: "MQTT Version",
    type: "tag",
  },
  {
    key: "keepalive",
    label: "Keep Alive",
    tips: "",
    type: "tag",
  },
];

export const suggestedPubTopics = [...pubTopicsShadow, ...pubTopicsThing];

export const suggestedSubTopics = [
  common,
  ntp,
  invokeDirectMethod,
  getShadow,
  updateShadow,
];

export const qosOptions = [
  {
    value: 0,
    label: "At most once",
  },
  {
    value: 1,
    label: "At least once",
  },
  {
    value: 2,
    label: "Exactly once",
  },
];

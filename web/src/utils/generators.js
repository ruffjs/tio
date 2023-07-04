import { v4 as uuidv4 } from "uuid";

export const genClientIdSuffix = () =>
  `${Math.random().toString(16).substring(2, 10)}`;
export const genConnectionId = () => `conn-${uuidv4()}`;
export const genSubscriptionId = () => `subcription_${uuidv4()}`;
export const genMessageId = () => `message_${uuidv4()}`;
export const genMqttClientToken = () => `token-${uuidv4()}`;
export const genConnectedCallbackToken = () => `ccbt-${uuidv4()}`;

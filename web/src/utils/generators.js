import { v7 as uuidv7 } from "uuid";

export const genClientIdSuffix = () =>
  `${Math.random().toString(16).substring(2, 10)}`;

export const genConnectionId = () => `conn-${uuidv7()}`;
export const genDelegateId = () => `dlgt-${uuidv7()}`;
export const genSubscriptionId = () => `subs_${uuidv7()}`;
export const genMessageId = () => `msg${uuidv7()}`;

export const genConnectedCallbackToken = () => `ccbt-${uuidv7()}`;
export const genClientToken = () => `ct-${uuidv7()}`;

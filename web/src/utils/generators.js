import { v4 as uuidv4 } from "uuid";

export const genClientIdSuffix = () =>
  `${Math.random().toString(16).substring(2, 10)}`;

export const genConnectionId = () => `conn-${uuidv4()}`;
export const genDelegateId = () => `dlgt-${uuidv4()}`;
export const genSubscriptionId = () => `subs_${uuidv4()}`;
export const genMessageId = () => `msg${uuidv4()}`;

export const genConnectedCallbackToken = () => `ccbt-${uuidv4().substring(24)}`;
export const genClientToken = () => `ct-${uuidv4().substring(24)}`;

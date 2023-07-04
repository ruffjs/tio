export const REQ_ROUTE_CHG_EVT = "request-route-change";
export const TH_STATUS_CHG_EVT = "thing-state-change";

export const TSCE_HTTP = "http-requested";
export const TSCE_MSGO = "message-out";
export const TSCE_MSGI = "message-in";
export const TSCE_MQTT = "mqtt-connected";
export const TSCE_MQTO = "mqtt-disconnected";

const RequestLogOutEvent = new CustomEvent(REQ_ROUTE_CHG_EVT, {
  detail: {
    name: "Login",
  },
});

export const requestLogOut = () => {
  window.dispatchEvent(RequestLogOutEvent);
};

export const notifyThingStateChange = (
  thingId,
  type = "not-specified",
  about = null
) => {
  const timestamp = Date.now();
  const ThingStateChangeEvent = new CustomEvent(TH_STATUS_CHG_EVT, {
    detail: {
      timestamp,
      thingId,
      about,
      type,
    },
  });
  window.dispatchEvent(ThingStateChangeEvent);
};

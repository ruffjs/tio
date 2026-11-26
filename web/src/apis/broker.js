import client from "./client";

export const getBrokerOverview = () =>
  client.http.get(`api/v1/mqttBroker/monitor/overview`);

export const getBrokerConnections = (params) =>
  client.http.get(`api/v1/mqttBroker/monitor/connections`, { params });

export const getBrokerConnection = (cid) =>
  client.http.get(`api/v1/mqttBroker/monitor/connections/${cid}`);

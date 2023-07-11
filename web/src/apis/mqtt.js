import client from "./client";
import querystring from "querystring";

export const kickOutClient = (clientId) =>
  client.http.delete(`api/v1/mqttBroker/clients/${clientId}`);

export const getBrokerStats = () =>
  client.http.get(`api/v1/mqttBroker/embed/stats`);

import client from "./client";

export const kickOutClient = (clientId) =>
  client.http.delete(`api/v1/mqttBroker/clients/${clientId}`);

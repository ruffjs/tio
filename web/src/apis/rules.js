import client from "./client";
import querystring from "querystring";

export const getRulesConfig = () => client.http.get("/api/v1/rules/config");
export const saveRulesConfig = (cfg) => client.http.put("/api/v1/rules/config", cfg);
export const testRuleProcess = (thingId, topic, payload, processConfigs) =>
  client.http.post("/api/v1/rules/test", { thingId, topic, payload, processConfigs });


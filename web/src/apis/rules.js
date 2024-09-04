import client from "./client";
import querystring from "querystring";

export const getRulesConfig = (params) => client.http.get("/api/v1/rules/config?" + querystring.stringify(params));
export const saveRulesConfig = (cfg) => client.http.put("/api/v1/rules/config", cfg);
export const toggleRuleComponet = (type, name, enable) => client.http.put(`/api/v1/rules/${type}/${name}`, { enable });
export const testRuleProcess = (thingId, topic, payload, processConfigs) =>
  client.http.post("/api/v1/rules/test", { thingId, topic, payload, processConfigs });


import client from "./client";

export const getConfig = () => client.http.get(`private/api/config`);

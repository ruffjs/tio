import client from "./client";
import querystring from "querystring";

export const getThings = (params) =>
  client.http.get(`api/v1/things/?${querystring.stringify(params)}`);

export const postThing = (thing) => client.http.post("api/v1/things", thing);

export const getThing = (thingId) =>
  client.http.get(`api/v1/things/${thingId}`);

export const deleteThing = (thingId) =>
  client.http.delete(`api/v1/things/${thingId}`);

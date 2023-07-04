import client from "./client";
import querystring from "querystring";

export const getThings = (params) =>
  client.get(`api/v1/things/?${querystring.stringify(params)}`);

export const postThing = (thing) => client.post("api/v1/things", thing);

export const getThing = (thingId) => client.get(`api/v1/things/${thingId}`);

export const deleteThing = (thingId) =>
  client.delete(`api/v1/things/${thingId}`);

import client from "./client";
import querystring from "querystring";

export const queryShadows = ({ pageIndex, pageSize, query }) =>
  client.http.post(
    `api/v1/things/shadows/query?${querystring.stringify({
      pageIndex,
      pageSize,
    })}`,
    {
      query,
    }
  );

export const getDefaultShadow = (thingId) =>
  client.http.get(`/api/v1/things/${thingId}/shadows/default`);

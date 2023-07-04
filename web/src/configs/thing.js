import { genMqttClientToken } from "@/utils/generators";

export const metaFields = [
  {
    key: "thingId",
    label: "Thing Id",
    type: "string",
  },
  {
    key: "enabled",
    label: "Enabled",
    type: "boolean",
  },
  {
    key: "authType",
    label: "Auth Type",
    tips: "",
    type: "tag",
  },
  {
    key: "authValue",
    label: "Auth Value",
    type: "password",
  },
  {
    key: "createAt",
    label: "Created At",
    type: "time",
  },
];

export const shadowApis = {
  invoke: {
    name: "Request Direct Method",
    method: "post",
    url: "/api/v1/things/{id}/methods/{name}",
    link: "/docs/#/shadows/func8",
    params: [
      {
        key: "name",
        label: "Method Name",
        type: "path",
        required: true,
      },
    ],
    urlResolver: (params) => {
      return `api/v1/things/${params.id || "{id}"}/methods/${
        params.name || "{name}"
      }`;
    },
    payloadResolver: () => {
      return {
        connTimeout: 0,
        data: {},
        respTimeout: 0,
      };
    },
  },
  desire: {
    name: "Set Desired",
    method: "put",
    url: "/api/v1/things/{id}/shadows/default/state/desired",
    link: "/docs/#/shadows/func6",
    params: [],
    urlResolver: (params) => {
      return `api/v1/things/${
        params.id || "{id}"
      }/shadows/default/state/desired`;
    },
    payloadResolver: () => {
      return {
        clientToken: genMqttClientToken(),
        state: {
          desired: {},
        },
        version: 0,
      };
    },
  },
  tags: {
    name: "Set Tags",
    method: "put",
    url: "/api/v1/things/{id}/shadows/tags",
    link: "/docs/#/shadows/func9",
    params: [],
    urlResolver: (params) => {
      return `/api/v1/things/${params.id || "{id}"}/shadows/tags`;
    },
    payloadResolver: () => {
      return {
        tags: {},
        version: 0,
      };
    },
  },
};

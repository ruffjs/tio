import { genClientToken } from "@/utils/generators";

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
    link: "/docs/#/shadows/invoke-direct-method",
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
        connTimeout: 30,
        respTimeout: 20,
        data: {},
      };
    },
  },
  desire: {
    name: "Set Desired",
    method: "put",
    url: "/api/v1/things/{id}/shadows/default/state/desired",
    link: "/docs/#/shadows/set-state-desired",
    params: [],
    urlResolver: (params) => {
      return `api/v1/things/${
        params.id || "{id}"
      }/shadows/default/state/desired`;
    },
    payloadResolver: () => {
      return {
        clientToken: genClientToken(),
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
    link: "/docs/#/shadows/set-tags",
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

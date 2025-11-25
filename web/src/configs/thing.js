import { genClientToken } from "@/utils/generators";

// 创建可复用的配置生成函数
export const createMetaFields = (t) => [
  {
    key: "thingId",
    label: t('things.thingId'),
    type: "string",
  },
  {
    key: "enabled",
    label: t('things.enabled'),
    type: "boolean",
  },
  {
    key: "authType",
    label: t('things.authType'),
    tips: "",
    type: "tag",
  },
  {
    key: "authValue",
    label: t('things.authValue'),
    type: "password",
  },
  {
    key: "createdAt",
    label: t('things.createdAt'),
    type: "time",
  },
];

export const createShadowApis = (t) => ({
  invoke: {
    name: t('things.requestDirectMethod'),
    method: "post",
    url: "/api/v1/things/{id}/methods/{name}",
    link: "/docs/#/shadows/invoke-direct-method",
    params: [
      {
        key: "name",
        label: t('things.requestPath'),
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
        respTimeout: 10,
        data: {},
      };
    },
  },
  desire: {
    name: t('things.setDesired'),
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
    name: t('things.setTags'),
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
});

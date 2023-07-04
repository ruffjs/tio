import { genMqttClientToken } from "@/utils/generators";

export default [
  {
    name: "Report Property",
    code: "$iothub/things/{thingId}/messages/property",
    forThing: true,
    topicResolver: (params) => {
      return `$iothub/things/${params.thingId}/messages/property`;
    },
    payloadType: "JSON",
    payloadResolver: (params) => {
      return JSON.stringify({}, null, 2);
    },
  },
  {
    name: "Set Will",
    code: "$iothub/things/{thingId}/messages/will/#",
    forThing: true,
    topicResolver: (params) => {
      return `$iothub/things/${params.thingId}/messages/will/#`;
    },
    payloadType: "JSON",
    payloadResolver: (params) => {
      return JSON.stringify({}, null, 2);
    },
  },
  {
    name: "Custom Message",
    code: "$iothub/user/things/{thingId}/presence",
    forServer: true,
    topicResolver: (params) => {
      return `$iothub/things/${params.thingId}/presence`;
    },
    payloadType: "JSON",
    payloadResolver: (params) => {
      return JSON.stringify(
        {
          thingId: "",
          timestamp: Date.now(),
          eventType: "connected",
          disconnectReason: "",
          remoteAddr: "<ip:port>", // 客户端来源地址
        },
        null,
        2
      );
    },
  },
  {
    name: "Request NTP",
    code: "$iothub/things/{thingId}/req",
    forThing: true,
    topicResolver: (params) => {
      return `$iothub/things/${params.thingId}/req`;
    },
    payloadType: "JSON",
    payloadResolver: (params) => {
      return JSON.stringify(
        {
          clientSendTime: Date.now(),
        },
        null,
        2
      );
    },
  },
  {
    name: "Response NTP",
    code: "$iothub/things/{thingId}/resp",
    forServer: true,
    topicResolver: (params) => {
      return `$iothub/things/+/resp`;
    },
    payloadType: "JSON",
    payloadResolver: (params) => {
      return JSON.stringify(
        {
          clientSendTime: Date.now() - 2,
          serverRecvTime: Date.now() - 1,
          serverSendTime: Date.now(),
        },
        null,
        2
      );
    },
  },
  {
    name: "OTA Task",
    code: "$iothub/things/{thingId}/ota/task",
    forServer: true,
    topicResolver: (params) => {
      return `$iothub/things/+/ota/task`;
    },
    payloadType: "JSON",
    payloadResolver: (params) => {
      return JSON.stringify(
        {
          clientToken: Date.now(), // 任务生成方指定
          taskId: "", // ota 任务 id
          type: "app", // ota 类型，app/appconfig/os 等
          meta: {
            // meta 内字段信息可选、可扩展
            version: "",
            md5: "", // ota 文件 md5
            "content-type": "application/octet-stream", // 文件格式
            "content-length": 0, // 文件大小
          },
          fileUrl: "http://", // 更新包地址，一般是 http/https 文件地址
        },
        null,
        2
      );
    },
  },
  {
    name: "OTA Task Result",
    code: "$iothub/things/{thingId}/ota/task/result",
    forThing: true,
    topicResolver: (params) => {
      return `$iothub/things/${params.thingId}/ota/task`;
    },
    payloadType: "JSON",
    payloadResolver: (params) => {
      return JSON.stringify(
        {
          clientToken: "", // 原请求 clientToken
          taskId: "", // ota 任务 id
          status: "ongoing", // 状态
          progress: 0, // 0-100 百分比 ， 在 status 为 ongoing 时有
          errCode: undefined, // 错误码，在 status 为 failed 时有
          errMsg: undefined, // 错误 message，在 status 为 failed 时有
          timestamp: Date.now(), // ms
        },
        null,
        2
      );
    },
  },
  {
    name: "Custom Message",
    code: "$iothub/user/things/{thingId}/#",
    forThing: true,
    forServer: true,
    topicResolver: (params) => {
      return `$iothub/user/things/${params.thingId}/#`;
    },
    payloadType: "JSON",
    payloadResolver: (params) => {
      return JSON.stringify({}, null, 2);
    },
  },
  {
    name: "Request Direct Method",
    code: "$iothub/things/{thingId}/methods/{name}/req",
    forServer: true,
    topicResolver: (params) => {
      return `$iothub/things/${params.thingId}/methods/+/req`;
    },
    payloadType: "JSON",
    payloadResolver: (params) => {
      return JSON.stringify(
        {
          clientToken: genMqttClientToken(),
          data: {},
        },
        null,
        2
      );
    },
  },
  {
    name: "Response Direct Method",
    code: "$iothub/things/{thingId}/methods/{name}/resp",
    forThing: true,
    topicResolver: (params) => {
      return `$iothub/things/${params.thingId}/shadows/methods/+/resp`;
    },
    payloadType: "JSON",
    payloadResolver: (params) => {
      return JSON.stringify(
        {
          clientToken: "", // 和 req 收到的对应
          code: 200, // 类似 http，400 请求参数错误， 404 设备不存在， 504 设备超时未响应
          messge: "OK",
          data: {},
        },
        null,
        2
      );
    },
  },
];

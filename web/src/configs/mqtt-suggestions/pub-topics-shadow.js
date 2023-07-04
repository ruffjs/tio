import { genMqttClientToken } from "@/utils/generators";

export default [
  {
    name: "Get Shadow",
    code: "$iothub/things/{thingId}/shadows/name/default/get",
    forThing: true,
    topicResolver: (params) => {
      return `$iothub/things/${params.thingId}/shadows/name/default/get`;
    },
    payloadType: "JSON",
    payloadResolver: (params) => {
      return JSON.stringify({}, null, 2);
    },
  },
  {
    name: "Update Shadow",
    code: "$iothub/things/{thingId}/shadows/name/default/update",
    forThing: true,
    topicResolver: (params) => {
      return `$iothub/things/${params.thingId}/shadows/name/default/update`;
    },
    payloadType: "JSON",
    payloadResolver: (params) => {
      return JSON.stringify(
        {
          state: {
            reported: {},
          },
          clientToken: genMqttClientToken(),
          version: 0,
        },
        null,
        2
      );
    },
  },
];

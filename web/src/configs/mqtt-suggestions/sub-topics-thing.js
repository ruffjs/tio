export const common = {
  group: "Common",
  topics: [
    {
      name: "Property Reported",
      code: "$iothub/things/{thingId}/property",
      forServer: true,
      topicResolver: (params) => {
        return `$iothub/things/${params.thingId}/property`;
      },
    },
    {
      name: "Thing's Will",
      code: "$iothub/things/{thingId}/will/#",
      forServer: true,
      topicResolver: (params) => {
        return `$iothub/things/${params.thingId}/will/#`;
      },
    },
    {
      name: "Presence",
      code: "$iothub/things/{thingId}/presence",
      forServer: true,
      topicResolver: (params) => {
        return `$iothub/things/${params.thingId}/presence`;
      },
    },
    {
      name: "Custom Topic",
      code: "$iothub/user/things/{thingId}/#",
      forServer: true,
      topicResolver: (params) => {
        return `$iothub/user/things/${params.thingId}/#`;
      },
    },
  ],
};

export const ntp = {
  group: "NTP",
  topics: [
    {
      name: "Request NTP",
      code: "$iothub/things/{thingId}/req",
      forServer: true,
      suggested: false,
      topicResolver: (params) => {
        return `$iothub/things/${params.thingId}/req`;
      },
    },
    {
      name: "Response NTP",
      code: "$iothub/things/{thingId}/ntp/resp",
      forThing: true,
      suggested: false,
      topicResolver: (params) => {
        return `$iothub/things/${params.thingId}/ntp/resp`;
      },
    },
  ],
};

export const invokeDirectMethod = {
  group: "Direct Method",
  topics: [
    {
      name: "Direct Method Request",
      code: "$iothub/things/{thingId}/methods/{name}/req",
      forThing: true,
      suggested: true,
      topicResolver: (params) => {
        return `$iothub/things/${params.thingId}/methods/+/req`;
      },
    },
    {
      name: "Direct Method Response",
      code: "$iothub/things/{thingId}/methods/{name}/resp",
      forServer: true,
      suggested: false,
      topicResolver: (params) => {
        return `$iothub/things/${params.thingId}/methods/{name}/resp`;
      },
    },
  ],
};

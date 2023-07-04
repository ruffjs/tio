export const getShadow = {
  group: "Get Shadow",
  topics: [
    {
      name: "Get Accepted",
      code: "$iothub/things/{thingId}/shadows/name/default/get/accepted",
      forThing: true,
      suggested: true,
      topicResolver: (params) => {
        return `$iothub/things/${params.thingId}/shadows/name/default/get/accepted`;
      },
    },
    {
      name: "Get Rejected",
      code: "$iothub/things/{thingId}/shadows/name/default/get/rejected",
      forThing: true,
      suggested: true,
      topicResolver: (params) => {
        return `$iothub/things/${params.thingId}/shadows/name/default/get/rejected`;
      },
    },
  ],
};

export const updateShadow = {
  group: "Update Shadow",
  topics: [
    {
      name: "Update Accepted",
      code: "$iothub/things/{thingId}/shadows/name/default/update/accepted",
      forThing: true,
      suggested: true,
      topicResolver: (params) => {
        return `$iothub/things/${params.thingId}/shadows/name/default/update/accepted`;
      },
    },
    {
      name: "Update Rejected",
      code: "$iothub/things/{thingId}/shadows/name/default/update/rejected",
      forThing: true,
      suggested: true,
      topicResolver: (params) => {
        return `$iothub/things/${params.thingId}/shadows/name/default/update/rejected`;
      },
    },
    {
      name: "Update Documents Notify",
      code: "$iothub/things/{thingId}/shadows/name/default/update/documents",
      forThing: true,
      forServer: true,
      suggested: true,
      topicResolver: (params) => {
        return `$iothub/things/${params.thingId}/shadows/name/default/update/documents`;
      },
    },
    {
      name: "Update Delta Notify",
      code: "$iothub/things/{thingId}/shadows/name/default/update/delta",
      forThing: true,
      suggested: true,
      topicResolver: (params) => {
        return `$iothub/things/${params.thingId}/shadows/name/default/update/delta`;
      },
    },
  ],
};

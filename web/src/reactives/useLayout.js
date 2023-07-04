import { computed } from "vue";
import { useStore } from "vuex";
import { tools } from "@/configs/tool";
import { genConnectedCallbackToken } from "@/utils/generators";

export const teleports = {
  G: "#global-message-panel",
  D: "#mqtt-client-message-panel",
};
export default () => {
  const store = useStore();
  const activeToolKey = computed(() => store.state.layout.activeToolKey);
  const activeToolConf = computed(() => {
    const conf = tools.find(
      (tool) => tool.key === store.state.layout.activeToolKey
    );
    return conf || null;
  });
  const activeToolHeight = computed(() => `${store.state.layout.bottomGap}px`);

  const switchActiveTool = (tool) => {
    const globalTeleport = document.querySelector(teleports.G);
    globalTeleport.style.visibility = "hidden";
    globalTeleport.style.height = "90px";
    if (tool.key === activeToolKey.value) {
      store.dispatch("layout/switchActiveTool", "");
    } else {
      store.dispatch("layout/switchActiveTool", tool.key);
    }
  };

  const isConnFormVisible = computed(() => store.state.layout.mqttForm.visible);
  const editConnId = computed(() => store.state.layout.mqttForm.connIdToEdit);
  const isForCreate = computed(() => !store.state.layout.mqttForm.connIdToEdit);
  const createThingId = computed(
    () => store.state.layout.mqttForm.thingIdForCreate
  );
  const connectedCbT = computed(
    () => store.state.layout.mqttForm.connectedCallbackToken
  );

  const showMqttForm = (connIdToEdit = null, thingIdForCreate = null) => {
    const connectedCallbackToken = thingIdForCreate
      ? genConnectedCallbackToken()
      : "";
    store.commit("layout/setState", {
      mqttForm: {
        visible: true,
        connIdToEdit,
        thingIdForCreate,
        connectedCallbackToken,
      },
    });
    return connectedCallbackToken;
  };

  const hideMqttForm = () => {
    store.commit("layout/setState", {
      mqttForm: {
        visible: false,
        connIdToEdit: null,
        thingIdForCreate: "",
        connectedCallbackToken: "",
      },
    });
  };

  const isSubscriptionsVisible = computed(
    () => store.state.layout.mqttSubs.visible
  );
  const subscriptionsConnConfig = computed(
    () => store.state.layout.mqttSubs.connConfig
  );
  const showMqttSubs = (connConfig) => {
    store.commit("layout/setState", {
      mqttSubs: {
        visible: true,
        connConfig,
      },
    });
  };
  const hideMqttSubs = () => {
    store.commit("layout/setState", {
      mqttSubs: {
        visible: false,
        connConfig: null,
      },
    });
  };

  return {
    activeToolKey,
    activeToolConf,
    activeToolHeight,
    switchActiveTool,

    isConnFormVisible,
    editConnId,
    isForCreate,
    createThingId,
    connectedCbT,
    showMqttForm,
    hideMqttForm,

    isSubscriptionsVisible,
    subscriptionsConnConfig,
    showMqttSubs,
    hideMqttSubs,
  };
};

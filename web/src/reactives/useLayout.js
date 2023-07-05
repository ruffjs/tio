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
    globalTeleport.style.height = "100px";
    if (tool.key === activeToolKey.value) {
      store.dispatch("layout/switchActiveTool", "");
    } else {
      store.dispatch("layout/switchActiveTool", tool.key);
    }
  };

  const isConnFormVisible = computed(
    () => store.state.layout.mqttConnForm.visible
  );
  const editConnId = computed(
    () => store.state.layout.mqttConnForm.connIdToEdit
  );
  const isForCreate = computed(
    () => !store.state.layout.mqttConnForm.connIdToEdit
  );
  const createThingId = computed(
    () => store.state.layout.mqttConnForm.thingIdForCreate
  );
  const connectedCbT = computed(
    () => store.state.layout.mqttConnForm.connectedCallbackToken
  );

  const showMqttConnForm = (connIdToEdit = null, thingIdForCreate = null) => {
    const connectedCallbackToken = thingIdForCreate
      ? genConnectedCallbackToken()
      : "";
    store.commit("layout/setState", {
      mqttConnForm: {
        visible: true,
        connIdToEdit,
        thingIdForCreate,
        connectedCallbackToken,
      },
    });
    return connectedCallbackToken;
  };

  const hideMqttConnForm = () => {
    store.commit("layout/setState", {
      mqttConnForm: {
        visible: false,
        connIdToEdit: null,
        thingIdForCreate: "",
        connectedCallbackToken: "",
      },
    });
  };

  const isSubsFormVisible = computed(
    () => store.state.layout.mqttSubsForm.visible
  );
  const subsConnConfig = computed(
    () => store.state.layout.mqttSubsForm.connConfig
  );
  const subsFormData = computed(
    () => store.state.layout.mqttSubsForm.subscription
  );
  const showMqttSubsForm = (connConfig, subscription) => {
    store.commit("layout/setState", {
      mqttSubsForm: {
        visible: true,
        connConfig,
        subscription,
      },
    });
  };
  const hideMqttSubsForm = () => {
    store.commit("layout/setState", {
      mqttSubsForm: {
        visible: false,
        connConfig: null,
        subscription: null,
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
    showMqttConnForm,
    hideMqttConnForm,

    isSubsFormVisible,
    subsConnConfig,
    subsFormData,
    showMqttSubsForm,
    hideMqttSubsForm,
  };
};

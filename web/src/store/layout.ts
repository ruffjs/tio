import { tools } from "@/configs/tool";
import { StoreOptions } from "vuex/types/index.js";

export default {
  namespaced: true,
  state() {
    let bottomGap = 30;
    const root = document.documentElement;
    const activeToolKey =
      localStorage.getItem("$tiopg/layout/activeToolKey") || "";
    const activeToolConf = tools.find((tool) => tool.key === activeToolKey);
    if (activeToolConf) {
      bottomGap = activeToolConf.height!;
    }
    root.style.setProperty("--layout-top-gap", "50px");
    root.style.setProperty("--layout-bottom-gap", `${bottomGap}px`);
    return {
      activeToolKey,
      topGap: 50,
      bottomGap,
      mqttForm: {
        visible: false,
        connIdToEdit: null,
        thingIdForCreate: "",
        connectedCallbackToken: "",
      },
      mqttSubs: {
        visible: false,
        connConfig: null,
      },
    };
  },
  getters: {},
  mutations: {
    setState(state, payload) {
      Object.assign(state, payload);
    },
  },
  actions: {
    switchActiveTool({ commit }, activeToolKey) {
      const activeToolConf = tools.find((tool) => tool.key === activeToolKey);
      if (typeof activeToolConf?.link === "string") {
        window.open(activeToolConf.link, "_blank");
        return;
      }

      let bottomGap = 30;
      if (typeof activeToolConf?.height === "number") {
        bottomGap = activeToolConf.height;
      }
      document.documentElement.style.setProperty(
        "--layout-bottom-gap",
        `${bottomGap}px`
      );
      localStorage.setItem("$tiopg/layout/activeToolKey", activeToolKey);
      commit("setState", {
        activeToolKey,
        bottomGap,
      });
    },
  },
} as StoreOptions<{
  activeToolKey: string;
  topGap: number;
  bottomGap: number;
}>;

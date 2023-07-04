import { StoreOptions } from "vuex/types/index.js";

const storeConnections = (connections: any[]) => {
  if (typeof connections === "object" && connections instanceof Array) {
    localStorage.setItem(
      "$tiopg/mqtt/connections",
      JSON.stringify(connections)
    );
  } else {
    localStorage.setItem("$tiopg/mqtt/connections", "[]");
  }
};

const restoreConnections = () => {
  try {
    const connJSON = localStorage.getItem("$tiopg/mqtt/connections") || "[]";
    return JSON.parse(connJSON);
  } catch (error) {
    return [];
  }
};

export default {
  namespaced: true,
  state() {
    const delegateSharedStates = {};
    const connectionConfigs = restoreConnections();
    let currentConnId = null;
    if (connectionConfigs.length > 0) {
      currentConnId = connectionConfigs[0].id;
      connectionConfigs.forEach((config: any) => {
        delegateSharedStates[config.id] = {
          id: config.id,
          client: { connected: false },
        };
      });
    }

    return {
      connectionConfigs,
      currentConnId,
      delegateSharedStates,
      connecting: false,
      retryTimes: 0,
      autoResub: true,
    };
  },
  getters: {
    autoResub: (state: any) => state.autoResub,
  },
  mutations: {
    setState(state: any, payload: any) {
      Object.assign(state, payload);
    },
  },
  actions: {
    updateConnConfigs({ commit }, payload) {
      const connectionConfigs = payload || [];
      storeConnections(connectionConfigs);
      commit("setState", {
        connectionConfigs,
      });
    },
    updateDelegateStates({ state, commit }, payload) {
      const delegateSharedStates = state.delegateSharedStates;
      if (payload.forDelete) {
        delete delegateSharedStates[payload.id];
      } else {
        delegateSharedStates[payload.id] = payload;
      }
      commit("setState", {
        delegateSharedStates,
        connecting: false,
        retryTimes: 0,
      });
    },
  },
} as StoreOptions<{
  connectionConfigs: any[];
  currentConnId: string | null;
  delegateSharedStates: Record<string, any>;
  connecting: boolean;
  retryTimes: number;
  autoResub: boolean;
}>;

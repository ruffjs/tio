import { StoreOptions } from "vuex/types/index.js";
type HTTPRequestLog = {
  error?: any;
  time: string;
  req: {
    method: string;
    url: string;
    data: any;
  };
  res: {
    code: number;
    data: any;
    message: string;
  };
};

export default {
  namespaced: true,
  state() {
    return {
      tioConfig: {},
      things: [],
      shadowListUpdateTag: 0,
      currentShadow: {},
      httpRequestLogs: [],
    };
  },
  getters: {},
  mutations: {
    setState(state: any, payload: any) {
      Object.assign(state, payload);
    },
  },
  actions: {
    addReqLog({ state, commit }, payload: HTTPRequestLog) {
      const { httpRequestLogs } = state;
      httpRequestLogs.unshift(payload);
      commit("setState", {
        httpRequestLogs,
      });
    },
    tioConfig({ commit }, tioConfig) {
      console.log('set tioConfig', tioConfig);
      commit("setState", {
        tioConfig,
      });
    },
  },
} as StoreOptions<{
  tioConfig: {};
  things: any[];
  shadowListUpdateTag: number;
  currentShadow: Record<string, any>;
  httpRequestLogs: Array<HTTPRequestLog>;
}>;

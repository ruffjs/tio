import { StoreOptions } from "vuex/types/index.js";

export default {
  namespaced: true,
  state() {
    const auth = localStorage.getItem("$tiopg/user/auth") || "";
    return {
      auth,
    };
  },
  getters: {},
  mutations: {
    setState(state, payload) {
      Object.assign(state, payload);
    },
  },
  actions: {},
} as StoreOptions<{
  auth: string;
}>;

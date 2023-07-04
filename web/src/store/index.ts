import { createStore } from "vuex";

import app from "./app";
import layout from "./layout";
import mqtt from "./mqtt";
import user from "./user";

const store = createStore({
  modules: { app, layout, mqtt, user },
});

export default store;

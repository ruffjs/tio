import { createRouter, createWebHashHistory } from "vue-router";
import store from "@/store";
import { REQ_ROUTE_CHG_EVT } from "@/utils/event";

import List from "@/components/views/List.vue";
import Login from "@/components/views/Login.vue";
import NotFound from "@/components/views/NotFound.vue";
import Thing from "@/components/views/Thing.vue";

const routes = [
  {
    path: "/login",
    name: Login.name,
    meta: Login.customOptions,
    component: Login,
  },
  {
    path: "/things/:thingId",
    name: Thing.name,
    meta: Thing.customOptions,
    component: Thing,
  },
  {
    path: "/",
    name: List.name,
    meta: List.customOptions,
    component: List,
    children: [
      {
        path: "/things/:thingId",
        name: Thing.name,
        meta: Thing.customOptions,
        component: Thing,
      },
    ],
  },
  {
    path: "/:pathMatch(.*)*",
    name: NotFound.name,
    meta: NotFound.customOptions,
    component: NotFound,
  },
];

const router = createRouter({
  history: createWebHashHistory(),
  routes,
  //   linkActiveClass: 'active'
});

router.beforeEach(async (to, _from, next) => {
  // console.log(to.name);
  if (store.state.user.auth) {
    if (to.name === Login.name) {
      next({ name: List.name });
    } else {
      next();
    }
  } else {
    if (to.name !== Login.name) {
      next({ name: Login.name });
    } else {
      next();
    }
  }
});

window.addEventListener(REQ_ROUTE_CHG_EVT, (message: Event) => {
  const { detail } = message as CustomEvent;
  router.push(detail);
});

export default router;

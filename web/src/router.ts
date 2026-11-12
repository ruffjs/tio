import { createRouter, createWebHashHistory } from "vue-router";
import store from "@/store";
import { REQ_ROUTE_CHG_EVT } from "@/utils/event";

const List = () => import("@/components/views/List.vue");
const Login = () => import("@/components/views/Login.vue");
const NotFound = () => import("@/components/views/NotFound.vue");
const Thing = () => import("@/components/views/Thing.vue");
const Rule = () => import("@/components/views/Rule.vue");

const routes = [
  {
    path: "/login",
    name: "Login",
    meta: { title: "Login TIO", zIndex: 1999, actived: false, standalone: true },
    component: Login,
  },
  {
    path: "/rules",
    name: "Rule",
    component: Rule,
  },
  {
    path: "/things/:thingId",
    name: "Thing",
    meta: { title: (route) => `${route.params.thingId || "(thingId)"} | Thing`, zIndex: 0, actived: true },
    component: Thing,
  },
  {
    path: "/",
    name: "List",
    meta: { title: "TIO Playground", zIndex: 0, actived: true },
    component: List,
    children: [
      {
        path: "/things/:thingId",
        name: "Thing",
        meta: { title: (route) => `${route.params.thingId || "(thingId)"} | Thing`, zIndex: 0, actived: true },
        component: Thing,
      },
    ],
  },
  {
    path: "/:pathMatch(.*)*",
    name: "NotFound",
    meta: { title: "View Not Found", zIndex: 0, actived: false },
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
    if (to.name === "Login") {
      next({ name: "List" });
    } else {
      next();
    }
  } else {
    if (to.name !== "Login") {
      next({ name: "Login" });
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

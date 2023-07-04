import axios from "axios";
import store from "@/store";
import dayjs from "dayjs";
import { ElNotification } from "element-plus";
import { requestLogOut } from "@/utils/event";

const log = (res, error) => {
  // console.log(res);
  const time = (res.headers.date ? dayjs(res.headers.date) : dayjs()).format(
    "YYYY-MM-DD HH:mm:ss"
  );
  const { method, url, data } = res.config;
  store.dispatch("app/addReqLog", {
    error,
    time,
    req: {
      method,
      url,
      data: data ? JSON.parse(data) : null,
    },
    res: {
      data: res.data,
    },
  });
};
const client = axios.create({ baseURL: "/" });
client.interceptors.request.use(
  (req) => {
    // console.log(req.headers);
    Object.assign(req.headers || {}, {
      Authorization: store.state.user.auth ? store.state.user.auth : undefined,
    });
    return req;
  },
  (err) => {
    throw err;
  }
);
client.interceptors.response.use(
  (res) => {
    if (res.data) {
      log(res, null);
    } else {
    }
    return res.data;
  },
  (err) => {
    const { response: resp, code, message, stack } = err;
    // console.log(err);
    if (resp) {
      log(resp, { code, message });
      if (resp?.status === 401) {
        ElNotification({
          title: "Authorization Failed",
          message: "You must provide a pair of correct username and password",
          type: "error",
          zIndex: 10000,
        });
        localStorage.setItem("$tiopg/user/auth", "");
        store.commit("user/setState", { auth: "" });
        // router.push({ name: "Login" });
        requestLogOut();
        throw resp;
      } else if (resp?.status === 404) {
        throw err;
      }
      throw resp.data;
    } else {
    }
    throw err;
  }
);

export default client;

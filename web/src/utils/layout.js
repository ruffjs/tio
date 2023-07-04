import { ElNotification } from "element-plus";

export const notifyFail = (message) =>
  ElNotification({
    title: "Failure",
    message,
    type: "error",
    // duration: 0,
  });
export const notifyWarn = (message) =>
  ElNotification({
    title: "Warning",
    message,
    type: "warning",
    // duration: 0,
  });
export const notifyDone = (message) =>
  ElNotification({
    title: "Success",
    message,
    type: "success",
    // duration: 0,
  });

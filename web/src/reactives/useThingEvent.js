import { onMounted, onUnmounted } from "vue";
import { TH_STATUS_CHG_EVT } from "@/utils/event";

export default () => {
  const cbs = [];
  const handleSomethingStatusChange = (message) => {
    cbs.forEach((cb) => cb(message.detail));
  };

  const onSomethingStatusChange = (cb) => {
    if (typeof cb === "function") {
      cbs.push(cb);
    }
  };

  onMounted(() => {
    window.addEventListener(TH_STATUS_CHG_EVT, handleSomethingStatusChange);
  });

  onUnmounted(() => {
    cbs.length = 0;
    window.removeEventListener(TH_STATUS_CHG_EVT, handleSomethingStatusChange);
  });

  return {
    onSomethingStatusChange,
  };
};

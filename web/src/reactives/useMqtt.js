import { ref, computed } from "vue";
import { useStore } from "vuex";
import { genConnectionId, genSubscriptionId } from "@/utils/generators";
import dayjs from "dayjs";
import { createDelegate, getDelegateById } from "@/utils/MqttDelegate";

export default () => {
  const store = useStore();
  const connectionConfigs = computed(() => store.state.mqtt.connectionConfigs);
  const connections = computed(() =>
    connectionConfigs.value.map((config) => {
      return {
        config,
        ...config,
        ...store.state.mqtt.delegateSharedStates[config.id],
      };
    })
  );
  const connecting = computed(() => store.state.mqtt.connecting);
  const retryTimes = computed(() => store.state.mqtt.retryTimes);
  const delegateSharedStates = computed(
    () => store.state.mqtt.delegateSharedStates
  );

  const currentConnId = computed(() => store.state.mqtt.currentConnId);
  const selectedConn = computed(() => {
    const currentConnId = store.state.mqtt.currentConnId;
    if (currentConnId) {
      const config = getConnConfig(currentConnId);
      return {
        config,
        ...config,
        ...store.state.mqtt.delegateSharedStates[currentConnId],
      };
    }
    return null;
  });

  const setConnConfig = (id, config) => {
    const configs = [...store.state.mqtt.connectionConfigs];
    if (!id) {
      config.id = genConnectionId();
      disconn(config);
      configs.push(config);
    } else {
      const index = configs.findIndex((c) => c.id === id);
      configs[index] = config;
    }
    store.dispatch("mqtt/updateConnConfigs", configs);
    return config;
  };
  const getConnConfig = (id) =>
    connectionConfigs.value.find((c) => c.id === id);

  const getConnConfigsByClientId = (clientId) => {
    return connectionConfigs.value.filter(
      (connection) => connection.clientId === clientId
    );
  };

  const selectConnection = (config) => {
    const delegate = getDelegateById(config.id, config);
    delegate.read();
    delegate.updateStore();
    store.commit("mqtt/setState", {
      currentConnId: config.id,
    });
  };

  const connect = (config, connectedToken = "") => {
    if (connecting.value) {
      return false;
    }
    const delegate = getDelegateById(config.id, config);
    delegate.connect(config, connectedToken);
  };
  const disconn = (config) => {
    if (connecting.value) {
      return false;
    }
    const delegate = getDelegateById(config.id, config);
    delegate.cancel();
  };

  const removeConnection = (id) => {
    const configs = [...connectionConfigs.value];
    const index = configs.findIndex((config) => config.id === id);
    configs.splice(index, 1);
    if (configs.length) {
      selectConnection(configs[0]);
    } else {
      store.commit("mqtt/setState", {
        currentConnId: null,
      });
    }
    store.dispatch("mqtt/updateConnConfigs", configs);
    store.dispatch("mqtt/updateDelegateStates", {
      id,
      forDelete: true,
    });
    const delegate = getDelegateById(id);
    if (delegate) delegate.destroy();
  };

  const publish = (config, options, cb) => {
    const { topic, payload, paytype, qos, retain, properties } = options;
    const delegate = getDelegateById(config.id, config);
    if (delegate) {
      delegate.pub(
        topic,
        { payload, paytype },
        { qos, retain, properties },
        (err) => {
          if (err) {
            cb && cb(err);
          } else {
            setConnConfig(config.id, {
              ...config,
              pushProps: { qos, retain },
            });
            cb && cb();
          }
        }
      );
    }
  };

  const subscribe = (config, options, cb) => {
    const { topic, opts, multiple, configs } = options;
    const delegate = getDelegateById(config.id, config);
    if (delegate) {
      const onSub = (err) => {
        if (err) {
          cb && cb(err);
        } else {
          // console.log("subscribed", topic, config);
          config.subscriptions = config.subscriptions || [];
          let topics = [];
          if (multiple) {
            topics = Object.keys(topic);
          } else if (typeof topic === "string") {
            topics.push(topic);
          } else {
            topics = topic;
          }
          topics.forEach((t) => {
            const sub = config.subscriptions.find((s) => s.topic === t);
            if (sub) {
              sub.opts = multiple ? topic[t] : opts;
            } else {
              const conf = configs?.find((c) => c.topic === t);
              if (conf) {
                conf.id = genSubscriptionId();
                config.subscriptions.push(conf);
              }
            }
          });
          setConnConfig(config.id, config);
          cb && cb();
        }
      };
      if (multiple) {
        delegate.sub(topic, null, onSub);
      } else {
        delegate.sub(topic, opts, onSub);
      }
    }
  };

  const unsubscribe = (config, options, cb) => {
    let topics = [];
    if (typeof options === "string") {
      topics.push(options);
    } else if (typeof options === "object" && options instanceof Array) {
      topics = options;
    }
    if (topics.length) {
      const delegate = getDelegateById(config.id, config);
      if (delegate)
        delegate.unsub(topics, (err) => {
          // console.log("unsubscribed", topics, config);
          cb && cb(err);
        });
    }
  };

  const clearMessages = (config) => {
    const delegate = getDelegateById(config.id, config);
    if (delegate) delegate.clear();
  };

  const removeTopic = (config, topic) => {
    const delegate = getDelegateById(config.id);
    if (delegate) delegate.removeTopic(topic);
  };

  return {
    connecting,
    retryTimes,
    connectionConfigs,
    connections,
    delegateSharedStates,
    currentConnId,
    selectedConn,
    setConnConfig,
    getConnConfig,
    removeConnection,
    getConnConfigsByClientId,
    selectConnection,
    connect,
    disconn,
    publish,
    subscribe,
    unsubscribe,
    clearMessages,
    removeTopic,
  };
};

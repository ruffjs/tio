import { suggestedSubTopics, suggestedPubTopics } from "@/configs/tool";

import { genSubscriptionId } from "./generators";
import { notifyDone, notifyFail, notifyWarn } from "./layout";

export const serverPubTopics = [];
export const thingPubTopics = [];
export const serverSubTopics = [];
export const thingSubTopics = [];
const suggestedTopicsForThing = [];

export const getSuggestedTopicsForThing = (thingId) => {
  return suggestedTopicsForThing.map(({ name, code, topicResolver }) => {
    return {
      id: genSubscriptionId(),
      name,
      keep: true,
      opts: { qos: 0 },
      topic: topicResolver({ thingId }),
    };
  });
};

export const subscribeAll = ({ config, subMap, subscribe }) => {
  const subs = (config.subscriptions || []).filter((sub) => !subMap[sub.topic]);
  // console.log(config, subs, subMap);
  if (subs.length) {
    subscribe(
      config,
      { topic: convertSubsConfigSubMap(subs), multiple: true, configs: subs },
      (err) => {
        if (err) {
          console.log("subscribe all error", err);
          notifyFail("subscribe topics failed.");
        } else {
          notifyDone("All configured subscriptions have been subscribed.");
        }
      }
    );
  } else {
    notifyWarn("All configured subscriptions have already been subscribed.");
  }
};

export const getSubscribedSubs = (subMap) =>
  Object.keys(subMap || {}).filter((topic) => subMap[topic]);

export const unsubscribeAll = ({ config, subMap, unsubscribe }) => {
  const subs = getSubscribedSubs(subMap);
  if (subs.length) {
    unsubscribe(config, subs, (err) => {
      if (err) {
        console.log("subscribe all error", err);
        notifyFail("unsubscribe topics failed.");
      } else {
        notifyDone("All subscriptions have been unsubscribed.");
      }
    });
  } else {
    notifyWarn("None of subscriptions need to be unsubscribed.");
  }
};

export const convertSubsConfigSubMap = (subs) => {
  const map = {};
  subs.forEach((sub) => {
    map[sub.topic] = sub.opts;
  });
  return map;
};

export const matchTopicMethod = (filter, topic) => {
  let _filter = filter;
  let _topic = topic;
  if (filter.includes("$share")) {
    // shared subscription format: $share/{ShareName}/{filter}
    _filter = filter.split("/").slice(2).join("/");
  }
  const filterArray = _filter.split("/");
  const length = filterArray.length;
  const topicArray = _topic.split("/");
  for (let i = 0; i < length; i += 1) {
    const left = filterArray[i];
    const right = topicArray[i];
    if (left === "#") {
      return topicArray.length >= length - 1;
    }
    if (left !== right && left !== "+") {
      return false;
    }
  }
  return length === topicArray.length;
};

const init = () => {
  suggestedPubTopics.forEach((suggestion) => {
    if (suggestion.forServer) {
      serverPubTopics.push(suggestion);
    }
    if (suggestion.forThing) {
      thingPubTopics.push(suggestion);
    }
  });
  suggestedSubTopics.forEach(({ group, topics }) => {
    topics.forEach((topic) => {
      if (topic.forServer) {
        serverSubTopics.push({
          ...topic,
          group,
        });
      }
      if (topic.forThing) {
        thingSubTopics.push({
          ...topic,
          group,
        });
        if (topic.suggested) {
          suggestedTopicsForThing.push({
            ...topic,
            group,
          });
        }
      }
    });
  });
};
init();

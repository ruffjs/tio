export const diffState = (state) => {
  const { desired, reported } = state || { desired: {}, reported: {} };
  const delta = mergeState(reported, desired);
  return [delta !== void 0, delta];
};

function isEmpty(obj) {
  return typeof obj === "undefined"
    ? true
    : typeof obj === "object"
    ? obj === null || Object.keys(obj).length === 0
    : false;
}

function mergeState(preState, curState) {
  let keys = Object.keys(preState);
  const delta = {};
  // merge existed kv
  for (let i = 0; i < keys.length; i++) {
    if (typeof preState[keys[i]] === "object") {
      if (curState[keys[i]] !== undefined && curState[keys[i]] !== null) {
        const subDelta = mergeState(preState[keys[i]], curState[keys[i]]);
        if (subDelta !== undefined) {
          delta[keys[i]] = subDelta;
        }
      } else {
        delta[keys[i]] = undefined;
      }
    } else if (preState[keys[i]] !== curState[keys[i]]) {
      if (curState[keys[i]] === undefined) {
        delta[keys[i]] = undefined;
      } else {
        delta[keys[i]] = curState[keys[i]];
      }
    }
  }
  keys = Object.keys(curState);
  // merge unexisted kv
  for (let i = 0; i < keys.length; i++) {
    if (preState[keys[i]] === undefined && curState[keys[i]] !== null) {
      delta[keys[i]] = curState[keys[i]];
    }
  }
  if (Object.keys(delta).length === 0) {
    return undefined;
  }

  const trimmed = JSON.parse(JSON.stringify(delta));
  return isEmpty(trimmed) ? undefined : trimmed;
}

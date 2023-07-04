export const diffState = (state) => {
  let hasDelta = false;
  const delta = {};
  const { desired, reported } = state || { desired: {}, reported: {} };
  Object.keys(desired).forEach((key) => {
    if (JSON.stringify(reported[key]) === JSON.stringify(desired[key])) return;
    hasDelta = true;
    delta[key] = desired[key];
  });
  return [hasDelta, delta];
};

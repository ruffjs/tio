import { RuleConfig, RuleEditData, RuleValidateResult } from "./type";

/**
 * Merges a rule with the config by rule name, source name and sink name
 * @param config The whole config
 * @param ruleToMerge The rule to merge
 * @returns
 */
export function tryMerge(config: RuleConfig, ruleToMerge: RuleEditData, isNew: boolean): RuleValidateResult {
  const result: RuleValidateResult = { isValid: false, errors: [], sinksShared: [], sourcesShared: [], config };
  try {
    validateRule(config, ruleToMerge, isNew);
    result.config = doMerge(config, ruleToMerge, isNew);
  } catch (e: any) {
    result.errors = [e.message || e + ""];
    result.isValid = false;
    return result;
  }

  const ruleSrcNames = ruleToMerge.sources.map((s) => s.name);
  const ruleSinkNames = ruleToMerge.sinks.map((s) => s.name);

  const otherRules = config.rules.filter((r) => r.name != ruleToMerge.name);
  result.sourcesShared = ruleSrcNames.filter((s) => !!otherRules.find((r) => r.sources.includes(s)));
  result.sinksShared = ruleSinkNames.filter((s) => !!otherRules.find((r) => r.sinks.includes(s)));

  result.isValid = true;

  return result;
}

function doMerge(oldConfig: RuleConfig, ruleForMerge: RuleEditData, isNew: boolean): RuleConfig {
  const config: RuleConfig = JSON.parse(JSON.stringify(oldConfig));
  const rule: RuleEditData = JSON.parse(JSON.stringify(ruleForMerge));

  // merge sources and sinks
  rule.sources.forEach((s) => mergeToArr(config.sources, s));
  rule.sinks.forEach((s) => mergeToArr(config.sinks, s));

  // merge rules
  const newRule = {
    name: rule.name,
    note: rule.note,
    process: rule.process,
    sources: rule.sources.map((s) => s.name),
    sinks: rule.sinks.map((s) => s.name),
  };
  if (isNew) {
    config.rules.push(newRule);
  } else {
    const mr = mergeToArr(config.rules, newRule);
    if (!mr.replaced) {
      throw new Error(`Rule ${newRule.name} not found for update`);
    }
  }

  // remove unused sources and sinks
  // config.sources = config.sources.filter(s=>config.rules.find(r=>r.sources.includes(s.name)));
  // config.sinks = config.sinks.filter(s=>config.rules.find(r=>r.sinks.includes(s.name)));

  return config;
}

function mergeToArr(arr: { name: string }[], item: { name: string }) {
  let replaced = false;
  arr.forEach((a, i, arr) => {
    if (a.name === item.name) {
      arr[i] = item;
      replaced = true;
    }
  });
  if (!replaced) {
    arr.push(item);
  }
  return { replaced };
}

export function validateRule(config: RuleConfig, rule: RuleEditData, isNew: boolean) {
  if (isNew) {
    if (config.rules.find((r) => rule.name == r.name)) {
      throw new Error(`Rule ${rule.name} already exists`);
    }
  }

  if (!rule.name) {
    throw new Error("Rule name is required");
  }
  if (rule.sources.length == 0 || rule.sinks.length == 0 || rule.process.length == 0) {
    throw new Error("Rule sources, sinks and process are required");
  }

  for (const p of rule.process) {
    if (!p.type || !p.name || !p.runner) {
      throw new Error("Process type, name and runner are required");
    }
    if (p.runner == "js" && !p.js) {
      throw new Error("Process runner is js and js is required");
    }
    if (p.runner == "jq" && !p.jq) {
      throw new Error("Process runner is jq and jq is required");
    }
  }

  for (const s of rule.sources) {
    if (!s.name || !s.type) {
      throw new Error("Source name and type are required");
    }
  }

  for (const s of rule.sinks) {
    if (!s.name || !s.type) {
      throw new Error("Sink name and type are required");
    }
  }
}

// get components depend on the component which is specified by type and nam
export function getDependent(
  type: string,
  name: string,
  config: RuleConfig
): Array<{ type: string; names: Array<string> }> {
  const res = new Array<{ type: string; names: Array<string> }>();
  switch (type) {
    case "rule":
      break;
    case "connector":
      const srcNames = config.sources.filter((s) => s.connector == name).map((s) => s.name);
      const sinkNames = config.sinks.filter((s) => s.connector == name).map((s) => s.name);
      if (srcNames.length > 0) {
        res.push({ type: "source", names: srcNames });
      }
      if (sinkNames.length > 0) {
        res.push({ type: "sink", names: sinkNames });
      }
      break;
    case "source":
      const ruleNames = config.rules.filter((r) => r.sources.includes(name)).map((r) => r.name);
      if (ruleNames.length > 0) {
        res.push({ type: "rule", names: ruleNames });
      }
      break;
    case "sink":
      const names = config.rules.filter((r) => r.sinks.includes(name)).map((r) => r.name);
      if (names.length > 0) {
        res.push({ type: "rule", names });
      }
      break;
  }
  return res;
}

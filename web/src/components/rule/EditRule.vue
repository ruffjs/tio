<template>
  <div class="rule-edit-con">
    <header class="edit-toolbar">
      <div class="toolbar-main">
        <el-tooltip :content="$t('nav.backList')" placement="bottom">
          <el-button icon="ArrowLeft" link circle class="back-button" @click="emit('cancel')" />
        </el-tooltip>
        <div class="toolbar-title">
          <span>{{ isNew ? $t('rules.addRule') : $t('rules.editRule') }}</span>
          <h1>{{ form.name || $t('rules.name') }}</h1>
        </div>
      </div>
      <div class="toolbar-actions">
        <el-button @click="emit('cancel')">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="save">{{ $t('common.save') }}</el-button>
      </div>
    </header>

    <div class="edit-layout">
      <main class="edit-main">
        <section class="editor-section basic-section">
          <div class="section-header">
            <div>
              <span>Rule</span>
              <h2>{{ $t('rules.name') }}</h2>
            </div>
          </div>
          <el-form :model="form" label-width="auto" label-position="top" class="rule-form">
            <div class="basic-grid">
              <el-form-item :label="$t('rules.name')">
                <el-input v-model="form.name" placeholder="Name" :disabled="!isNew" />
              </el-form-item>
              <el-form-item label="Note">
                <el-input v-model="form.note" placeholder="Describe this rule" />
              </el-form-item>
            </div>
          </el-form>
        </section>

        <section class="editor-section process-section">
          <div class="section-header">
            <div>
              <span>Pipeline</span>
              <h2>{{ $t('rules.process') }}</h2>
            </div>
            <el-popover placement="bottom-end" title="Help" :width="430" trigger="hover">
              <template #default>
                Input data for script is a json object like:
                <br />
                {"thingId": "string", "topic": "string", "payload": {}, "shadow": {} }
                <br /><br />
                JavaScript must have function: function run(data)
                <br /><br />
                Transform should return the result for sinks. Filter should return true for sinks or false for ignored.
                <br /><br />
                JQ Script reference: https://jqlang.github.io/jq/
              </template>
              <template #reference>
                <el-button link class="help-button">
                  <el-icon><QuestionFilled /></el-icon>
                  {{ $t('rules.help') }}
                </el-button>
              </template>
            </el-popover>
          </div>

          <el-form :model="form" label-width="auto" label-position="top" class="rule-form">
            <div v-for="(p, index) in form.process" :key="index" class="process-item">
              <div class="process-index">{{ index + 1 }}</div>
              <div class="process-body">
                <div class="process-grid">
                  <el-form-item label="Type">
                    <el-select placeholder="Type" v-model="p.type">
                      <el-option label="Transform" value="transform"></el-option>
                      <el-option label="Filter" value="filter"></el-option>
                    </el-select>
                  </el-form-item>
                  <el-form-item label="Script Type">
                    <el-select placeholder="Script Type" v-model="p.runner">
                      <el-option label="JavaScript" value="js"></el-option>
                      <el-option label="JQ Script" value="jq"></el-option>
                    </el-select>
                  </el-form-item>
                  <el-form-item :label="$t('rules.name')">
                    <el-input v-model="p.name" placeholder="Name"></el-input>
                  </el-form-item>
                </div>
                <el-input
                  v-if="p.runner == 'jq'"
                  v-model="p.jq"
                  type="textarea"
                  placeholder="JQ Script eg: .payload"
                  :autosize="{ minRows: 4, maxRows: 20 }"
                />
                <JsEditor v-if="p.runner == 'js'" v-model="p.js" class="js-editor"></JsEditor>
              </div>
            </div>
          </el-form>
        </section>

        <section class="editor-section debug-section">
          <div class="section-header">
            <div>
              <span>Verify</span>
              <h2>{{ $t('rules.debug') }}</h2>
            </div>
            <el-button type="primary" plain @click="test">{{ $t('rules.test') }}</el-button>
          </div>
          <el-form :model="form" label-width="auto" label-position="top" class="rule-form">
            <div class="debug-grid">
              <el-form-item label="ThingId">
                <el-input v-model="form.testData.thingId" placeholder="ThingId" />
              </el-form-item>
              <el-form-item label="Topic">
                <el-input v-model="form.testData.topic" placeholder="Topic" />
              </el-form-item>
            </div>
            <el-form-item :label="$t('rules.payload')">
              <JSONEditor v-model="form.testData.payload" class="json-editor" />
            </el-form-item>
            <div v-if="testResult.success != undefined" class="test-output">
              <label>{{ $t('rules.result') }}</label>
              <el-input
                v-if="testResult.success"
                v-model="testResult.output"
                type="textarea"
                placeholder="Result"
                :autosize="{ minRow: 2, maxRow: 8 }"
              />
              <div v-if="testResult.success == false" class="test-result">
                <el-icon color="red" size="16"><Warning /></el-icon>
                <span>{{ testResult.message }}</span>
              </div>
            </div>
          </el-form>
        </section>
      </main>

      <aside class="endpoint-rail">
        <section class="endpoint-group">
          <div class="endpoint-header">
            <div>
              <span>Input</span>
              <h2>{{ $t('rules.source') }}</h2>
            </div>
            <el-button v-if="!ioAdd.source.show" icon="Plus" circle type="primary" size="small" @click="toAddIo('source')" />
          </div>

          <div class="endpoint-list">
            <article v-for="s in form.sources" :key="s.name" class="endpoint-item">
              <el-popover placement="left-start" title="Detail" :width="400" trigger="hover">
                <template #default>
                  name: {{ s.name }}
                  <br />
                  type: {{ s.type }}
                  <template v-if="s.connector">
                    <br />
                    connector: {{ s.connector }}
                  </template>
                  <span v-if="s.options && Object.keys(s.options).length > 0">
                    <br /><br />Options:
                  </span>
                  <template v-if="s.options" v-for="(v, k) in s.options" :key="k">
                    <br />
                    &nbsp; {{ k }} : {{ v }}
                  </template>
                </template>
                <template #reference>
                  <div class="endpoint-copy">
                    <el-tag effect="plain" round>{{ s.type }}</el-tag>
                    <strong>{{ s.name }}</strong>
                    <span v-if="s.connector">{{ s.connector }}</span>
                  </div>
                </template>
              </el-popover>
              <div class="endpoint-actions">
                <el-button icon="Edit" circle link type="primary" @click="editSource(s)"></el-button>
                <el-button icon="Delete" circle link type="danger" @click="delIo('source', s.name)"></el-button>
              </div>
            </article>
          </div>

          <div v-if="ioAdd.source.show" class="io-sel">
            <el-select v-model="ioAdd.source.selected" placeholder="Source">
              <el-option v-for="s in ioAdd.source.availabe" :key="s.name" :label="s.name" :value="s.name" />
            </el-select>
            <div class="io-sel-actions">
              <el-button type="primary" size="small" @click="addIo('source')">{{ $t('common.confirm') }}</el-button>
              <el-button type="success" plain size="small" @click="createNewSource">{{ $t('common.new') }}</el-button>
              <el-button size="small" @click="ioAdd.source.show = false">{{ $t('common.cancel') }}</el-button>
            </div>
          </div>
        </section>

        <section class="endpoint-group">
          <div class="endpoint-header">
            <div>
              <span>Output</span>
              <h2>{{ $t('rules.sink') }}</h2>
            </div>
            <el-button v-if="!ioAdd.sink.show" icon="Plus" circle type="primary" size="small" @click="toAddIo('sink')" />
          </div>

          <div class="endpoint-list">
            <article v-for="s in form.sinks" :key="s.name" class="endpoint-item">
              <el-popover placement="left-start" title="Detail" :width="500" trigger="hover">
                <template #default>
                  name: {{ s.name }}
                  <br />
                  type: {{ s.type }}
                  <template v-if="s.connector">
                    <br />
                    connector: {{ s.connector }}
                  </template>
                  <span v-if="s.options && Object.keys(s.options).length > 0">
                    <br /><br />Options:
                  </span>
                  <template v-if="s.options" v-for="(v, k) in s.options" :key="k">
                    <br />
                    &nbsp; {{ k }} : {{ v }}
                  </template>

                  <template v-if="ruleSchema.sinkTips[s.type]">
                    <h4>Tip</h4>
                    <div v-html="ruleSchema.sinkTips[s.type].note"></div>
                    <label>Example: </label>
                    <br />
                    <template v-for="c in ruleSchema.sinkTips[s.type].formatExamples" :key="c">
                      <code>{{ c }}</code>
                      <br />
                    </template>
                  </template>
                </template>
                <template #reference>
                  <div class="endpoint-copy">
                    <el-tag effect="plain" round>{{ s.type }}</el-tag>
                    <strong>{{ s.name }}</strong>
                    <span v-if="s.connector">{{ s.connector }}</span>
                  </div>
                </template>
              </el-popover>
              <div class="endpoint-actions">
                <el-button icon="Edit" circle link type="primary" @click="editSink(s)"></el-button>
                <el-button icon="Delete" circle link type="danger" @click="delIo('sink', s.name)"></el-button>
              </div>
            </article>
          </div>

          <div v-if="ioAdd.sink.show" class="io-sel">
            <el-select v-model="ioAdd.sink.selected" placeholder="Sink">
              <el-option v-for="s in ioAdd.sink.availabe" :key="s.name" :label="s.name" :value="s.name" />
            </el-select>
            <div class="io-sel-actions">
              <el-button type="primary" size="small" @click="addIo('sink')">{{ $t('common.confirm') }}</el-button>
              <el-button type="success" plain size="small" @click="createNewSink">{{ $t('common.new') }}</el-button>
              <el-button size="small" @click="ioAdd.sink.show = false">{{ $t('common.cancel') }}</el-button>
            </div>
          </div>
        </section>
      </aside>
    </div>
  </div>

  <el-drawer size="700" destroy-on-close v-model="componentEditor.drawerEdit.show" 
    v-if="componentEditor.drawerEdit.show" :title="componentEditor.drawerEdit.title"
    append-to-body>
    <EditOpt ref="editOptComponent" v-model="componentEditor.drawerEdit.data" 
      :schema="componentEditor.drawerEdit.schema"
      :optionsSchema="componentEditor.drawerEdit.optionsSchema" 
      :config="localConfig" 
      :type="componentEditor.drawerEdit.type" 
      :isNew="componentEditor.drawerEdit.new"
      @create-connector="createNewConnector" />
    <template #footer>
      <div style="flex: auto">
        <el-button type="primary" @click="confirmComponentEdit">{{ $t('common.confirm') }}</el-button>
        <el-button @click="cancelComponentEdit">{{ $t('common.cancel') }}</el-button>
      </div>
    </template>
  </el-drawer>
</template>

<script setup>
import { nextTick, onMounted, reactive, ref, watch } from 'vue';
import { ElNotification } from 'element-plus'
import * as api from '@/apis';

import JsEditor from '@/components/rule/JsEditor.vue';
import JSONEditor from '@/components/common/JSONEditor.vue';
import EditOpt from '@/components/rule/EditOpt.vue';
import { tryMerge } from './rule.ts';
import * as ruleSchema from '@/components/rule/rule-schema';
import { deepCopy } from '@/utils/common';
import { createComponentEditor } from './ComponentEditor';

const props = defineProps({
  config: Object,
  rule: Object,
  isNew: Boolean,
})

const emit = defineEmits(["cancel"]);


const form = reactive({
  // rule
  name: '',
  note: '',
  enabled: false,
  sources: [],
  sinks: [],
  process: [{
    type: "transform",
    name: "",
    runner: "js",
    js: `const run = data=> {
  // data format:
  // {"thingId": "string", "topic": "string", "payload": {}, "shadow": {} }
  return data.payload.msg;
}`,
    jq: "",
  }],

  // test
  testData: {
    thingId: "",
    topic: "",
    payload: '{"msg": "hello"}'
  }
})
const testResult = ref({})

const ioAdd = reactive({
  source: {
    show: false,
    selected: null,
    availabe: [],
  },
  sink: {
    show: false,
    selected: null,
    availabe: [],
  },
})

// Local configuration for temporarily storing edited components
const localConfig = reactive({
  sources: [],
  sinks: [],
  connectors: []
});

// Create component editor
const componentEditor = createComponentEditor(localConfig, (newConfig) => {
  // Update local configuration
  Object.assign(localConfig, newConfig);
  
  // Update available component list
  ioAdd.source.availabe = localConfig.sources.filter(s => !form.sources.find(ns => ns.name === s.name));
  ioAdd.sink.availabe = localConfig.sinks.filter(s => !form.sinks.find(ns => ns.name === s.name));

  // Replace rule's sources and sinks updated by component editor
  form.sources.forEach(s => {
    const c = localConfig.sources.find(cs => cs.name === s.name);
    if (c) {
      Object.assign(s, c);
    }
  });
  form.sinks.forEach(s => {
    const c = localConfig.sinks.find(cs => cs.name === s.name);
    if (c) {
      Object.assign(s, c);
    }
  });

});

const editOptComponent = ref();

onMounted(async () => {
  initRule()
})

const initRule = () => {
  const config = JSON.parse(JSON.stringify(props.config))

  let rule = props.rule
  const theRuleName = props.rule?.name

  if (theRuleName) {
    rule = config.rules.find(r => r.name == props.rule?.name)
  }

  if (rule) {
    rule = JSON.parse(JSON.stringify(rule))
    Object.assign(form, rule)

    form.process.forEach(p => {
      if (!p.runner) {
        p.runner = !!p.jq ? "jq" : "js"
      }
    })
    form.sources = rule.sources.map(s => config.sources.find(cs => cs.name == s))
    form.sinks = rule.sinks.map(s => config.sinks.find(cs => cs.name == s))
  }

  // Initialize local configuration
  localConfig.sources = deepCopy(props.config.sources || []);
  localConfig.sinks = deepCopy(props.config.sinks || []);
  localConfig.connectors = deepCopy(props.config.connectors || []);

  // Update available component list
  ioAdd.source.availabe = localConfig.sources.filter(s => !form.sources.find(ns => ns.name == s.name));
  ioAdd.sink.availabe = localConfig.sinks.filter(s => !form.sinks.find(ns => ns.name == s.name));
}

watch(() => form.sources, () => {
  ioAdd.source.availabe = localConfig.sources.filter(s => !form.sources.find(ns => ns.name === s.name));
}, { deep: true });

watch(() => form.sinks, () => {
  ioAdd.sink.availabe = localConfig.sinks.filter(s => !form.sinks.find(ns => ns.name === s.name));
}, { deep: true });

// Watch local configuration changes
watch(() => localConfig.sources, () => {
  ioAdd.source.availabe = localConfig.sources.filter(s => !form.sources.find(ns => ns.name === s.name));
}, { deep: true });

watch(() => localConfig.sinks, () => {
  ioAdd.sink.availabe = localConfig.sinks.filter(s => !form.sinks.find(ns => ns.name === s.name));
}, { deep: true });

const toAddIo = (type) => {
  const d = ioAdd[type]
  const availabel = d.availabe
  if (!availabel || availabel.length == 0) {
    ElNotification({ type: 'warning', message: 'No more ' + type })
    return
  }
  d.show = true
}

const addIo = (type) => {
  const d = ioAdd[type]
  if (!d.selected) {
    ElNotification({ message: 'Select ' + type, type: 'error' })
    return
  }
  const s = d.availabe.find(a => a.name == d.selected)
  if (!s) {
    ElNotification({ message: 'Select another ' + type, type: 'error' })
    return
  }
  form[type + 's'].push(s)
  d.show = false
  d.selected = null
}
const delIo = (type, name) => {
  const sl = form[type + 's']
  sl.splice(sl.findIndex(s => s.name == name), 1)
}

const test = async () => {
  let r = {}
  try {
    r = await api.testRuleProcess(
      form.testData.thingId,
      form.testData.topic,
      form.testData.payload,
      form.process
    )
  } catch (e) {
    console.log(e)
    ElNotification({ message: e.message || e, type: 'error' })
    return
  }
  testResult.value = r.data
  if (typeof r.data.output == 'object') {
    testResult.value.output = JSON.stringify(r.data.output, null, 2)
  }
  ElNotification({ message: 'Test returned', type: 'info' })
}

// Edit Source
const editSource = (source) => {
  componentEditor.editSource(source, null, (updatedSource) => {
    // Update rule's source
    const index = form.sources.findIndex(s => s.name === source.name);
    if (index !== -1) {
      form.sources[index] = updatedSource;
    }
  });
};

// Create new Source
const createNewSource = () => {
  componentEditor.showAddSource(null, (newSource) => {
    // Add to rule
    form.sources.push(newSource);
    ioAdd.source.show = false;
  });
};

// Edit Sink
const editSink = (sink) => {
  componentEditor.editSink(sink, null, (updatedSink) => {
    // Update rule's sink
    const index = form.sinks.findIndex(s => s.name === sink.name);
    if (index !== -1) {
      form.sinks[index] = updatedSink;
    }
  });
};

// Create new Sink
const createNewSink = () => {
  componentEditor.showAddSink(null, (newSink) => {
    // Add to rule
    form.sinks.push(newSink);
    ioAdd.sink.show = false;
  });
};

// 创建新 Connector（在编辑 Source/Sink 时）
const createNewConnector = async () => {
  await closeDrawer();
  // Save current edit state
  const currentEdit = deepCopy(componentEditor.drawerEdit);
  
  componentEditor.showAddConnector(currentEdit, (newConnector) => {
    // Update current edit's Source/Sink's connector field
    componentEditor.drawerEdit.data.connector = newConnector.name;
  });
};
const closeDrawer = async () => {
  componentEditor.drawerEdit.show = false;
  await nextTick();
}

// Confirm component edit
const confirmComponentEdit = async () => {
  const success = await componentEditor.confirmEdit(editOptComponent);
  if (success) {
    // Update available component list
    ioAdd.source.availabe = localConfig.sources.filter(s => !form.sources.find(ns => ns.name === s.name));
    ioAdd.sink.availabe = localConfig.sinks.filter(s => !form.sinks.find(ns => ns.name === s.name));
  }
};

// Cancel component edit
const cancelComponentEdit = () => {
  componentEditor.cancelEdit();
};

// Modify save method, merge local configuration into global configuration
const save = async () => {
  console.debug('======> save rule', form);
  console.debug('old config', props.config);
  try {
    // Create merged configuration
    const mergedConfig = deepCopy(props.config);
    
    // Merge local modified components
    mergedConfig.sources = localConfig.sources;
    mergedConfig.sinks = localConfig.sinks;
    mergedConfig.connectors = localConfig.connectors;
    
    const mergeResult = tryMerge(mergedConfig, form, props.isNew);
    if (!mergeResult.isValid) {
      ElNotification({ message: mergeResult.errors.join('\n'), type: 'error' });
      return;
    }
    console.debug('merged config:', mergeResult);
    await api.saveRulesConfig(mergeResult.config);
    ElNotification({ message: 'Save successfully', type: 'success' });
    emit('cancel');
  } catch (e) {
    const msg = e.message || e + '';
    ElNotification({ message: msg, type: 'error' });
  }
};

</script>

<style lang="scss" scoped>
.rule-edit-con {
  min-width: 0;
  color: var(--tio-text);
}

.edit-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  min-width: 0;
  margin-bottom: 16px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--tio-line);
}

.toolbar-main {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 14px;
}

.toolbar-title {
  min-width: 0;

  span {
    color: var(--tio-muted);
    font-size: 11px;
    font-weight: 760;
    letter-spacing: 0.11em;
    text-transform: uppercase;
  }

  h1 {
    max-width: 560px;
    margin: 2px 0 0;
    overflow: hidden;
    color: var(--tio-text-strong);
    font-size: 22px;
    font-weight: 760;
    letter-spacing: -0.02em;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.back-button,
.help-button {
  color: var(--tio-muted);
}

.back-button.el-button {
  width: 30px;
  height: 30px;
  border: 1px solid var(--tio-line);
  background: var(--tio-surface-soft);
}

.toolbar-actions {
  display: inline-flex;
  flex-shrink: 0;
  gap: 8px;
}

.edit-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 22px;
  align-items: start;
}

.edit-main {
  min-width: 0;
}

.editor-section {
  margin-bottom: 14px;
  padding: 16px;
  border-left: 3px solid var(--tio-line);
  border-radius: var(--tio-radius);
  background: var(--tio-surface-soft);

  &:first-child {
    border-left-color: var(--tio-accent);
  }

  &:last-child {
    margin-bottom: 0;
  }
}

.process-section {
  border-left-color: var(--tio-accent-strong);
}

.debug-section {
  border-left-color: var(--tio-muted);
}

.section-header,
.endpoint-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;

  span {
    color: var(--tio-muted);
    font-size: 11px;
    font-weight: 760;
    letter-spacing: 0.11em;
    text-transform: uppercase;
  }

  h2 {
    margin: 2px 0 0;
    color: var(--tio-text-strong);
    font-size: 17px;
    font-weight: 720;
  }
}

.rule-form {
  width: 100%;
}

.basic-grid,
.debug-grid {
  display: grid;
  grid-template-columns: minmax(0, 220px) minmax(0, 1fr);
  gap: 12px;
}

.process-section {
  overflow: hidden;
}

.process-item {
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr);
  gap: 12px;
  padding: 14px 0;
  border-top: 1px solid var(--tio-line);

  &:first-child {
    border-top: 0;
    padding-top: 0;
  }
}

.process-index {
  display: grid;
  width: 28px;
  height: 28px;
  place-items: center;
  border-radius: 999px;
  background: var(--tio-surface-soft);
  color: var(--tio-muted);
  font-size: 12px;
  font-weight: 760;
}

.process-body {
  min-width: 0;
}

.process-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.js-editor {
  margin-top: 4px;
}

.json-editor {
  .jse-main {
    .jse-text-mode {
      .jse-contents {
        .cm-gutters {
          display: none !important;
        }
      }
    }
  }
}

.test-output {
  display: grid;
  gap: 8px;

  label {
    color: var(--tio-text-strong);
    font-size: 13px;
    font-weight: 680;
  }
}

.test-result {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 12px;
  border-radius: var(--tio-radius);
  background: var(--tio-surface-soft);
  color: var(--tio-text);
}

.endpoint-rail {
  position: sticky;
  top: 16px;
  display: grid;
  gap: 22px;
  min-width: 0;
  padding-left: 20px;
  border-left: 1px solid var(--tio-line);
}

.endpoint-group {
  display: grid;
  gap: 12px;
}

.endpoint-list {
  display: grid;
  gap: 6px;
}

.endpoint-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-height: 52px;
  padding: 8px 0;
  border-bottom: 1px solid var(--tio-line);

  &:last-child {
    border-bottom: 0;
  }

  &:hover,
  &:focus-within {
    .endpoint-actions {
      opacity: 1;
      pointer-events: auto;
    }
  }
}

.endpoint-copy {
  display: grid;
  min-width: 0;
  gap: 4px;
  cursor: default;

  strong,
  span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  strong {
    color: var(--tio-text-strong);
    font-size: 13px;
  }

  span {
    color: var(--tio-muted);
    font-size: 12px;
  }

  :deep(.el-tag) {
    justify-self: start;
    border-color: var(--tio-line);
    background: var(--tio-surface-soft);
    color: var(--tio-muted);
  }
}

.endpoint-actions {
  display: inline-flex;
  flex-shrink: 0;
  gap: 2px;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.16s ease;
}

.io-sel {
  display: grid;
  gap: 8px;
  padding: 10px;
  border-radius: var(--tio-radius);
  background: var(--tio-surface-soft);
}

.io-sel-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.tip {
  color: var(--tio-muted);
  font-size: 14px;
}

@media (max-width: 1100px) {
  .edit-layout {
    grid-template-columns: 1fr;
  }

  .endpoint-rail {
    position: static;
    padding-left: 0;
    border-left: 0;
  }
}

@media (max-width: 760px) {
  .edit-toolbar,
  .section-header,
  .endpoint-header {
    flex-direction: column;
    align-items: stretch;
  }

  .toolbar-main {
    align-items: flex-start;
  }

  .basic-grid,
  .debug-grid,
  .process-grid {
    grid-template-columns: 1fr;
  }

  .endpoint-actions {
    opacity: 1;
    pointer-events: auto;
  }
}
</style>

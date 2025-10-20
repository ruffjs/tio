<template>

  <el-row :gutter="10" class="con">
    <el-col :span="20" style="margin-bottom: 10px;">
      <el-button icon="ArrowLeft" link @click="emit('cancel')">{{ $t('nav.backList') }}</el-button>
    </el-col>

    <el-col :span="14" class="part left">
      <el-form :model="form" label-width="auto" label-position="top" style="width: 100%;">
        <el-form-item>
          <el-col :span="6">
            <el-input v-model="form.name" placeholder="Name" :disabled="!isNew" />
          </el-col>
          <el-col :span="18">
            <el-input v-model="form.note" placeholder="note" />
          </el-col>
        </el-form-item>

        <div>{{ $t('rules.process') }}
          <el-popover placement="top-start" title="Help" :width="400" trigger="hover">
            <template #default>
              Input data for script is a json object like:
              <br />
              {"thingId": "string", "topic": "string", "payload": {}, "shadow": {} }
              <br />
              <br />
              "payload" is the data reported by thing, shadow is the thing shadow.

              <br />
              <br />
              JavaScript must have function: function run(data)
              <br />

              When type is transform, function should return the result for sinks.
              <br />
              When type is filter, function should return true for sinks or false for ignored.
              <br />
              <br />
              JQ Script reference: https://jqlang.github.io/jq/
              <br />
              When type is transform, jq script should return the result for sinks.
              <br />
              When type is filter, jq script should return a boolean result.
            </template>
            <template #reference>

              <el-icon>
                <QuestionFilled />
              </el-icon>
            </template>
          </el-popover>
        </div>
        <br />

        <el-form-item v-for="p in form.process">
          <el-col :span="8">
            <el-select placeholder="Type" v-model="p.type">
              <el-option label="Transform" value="transform"></el-option>
              <el-option label="Filter" value="filter"></el-option>
            </el-select>
          </el-col>
          <el-col :span="8">
            <el-select placeholder="Script Type" v-model="p.runner">
              <el-option label="JavaScript" value="js"></el-option>
              <el-option label="JQ Script" value="jq"></el-option>
            </el-select>
          </el-col>

          <el-col :span="8">
            <el-input v-model="p.name" placeholder="Name"></el-input>
          </el-col>
          <el-col :span="24">
            <el-input v-if="p.runner == 'jq'" v-model="p.jq" type="textarea" placeholder="JQ Script eg: .payload"
              :autosize="{ minRows: 4, maxRows: 20 }" style="margin-top: 10px" />
            <JsEditor v-if="p.runner == 'js'" v-model="p.js" class="js-editor"></JsEditor>
          </el-col>
        </el-form-item>

        <div>{{ $t('rules.debug') }}</div>
        <br />
        <el-form-item>
          <el-col :span="12">
            <el-input v-model="form.testData.thingId" placeholder="ThingId" />
          </el-col>
          <el-col :span="12">
            <el-input v-model="form.testData.topic" placeholder="Topic" />
          </el-col>
          <el-col :span="24">
            <label>{{ $t('rules.payload') }}</label>
            <JSONEditor v-model="form.testData.payload" class="json-editor" />
          </el-col>
          <el-col :span="24">
            <el-button type="primary" @click="test">{{ $t('rules.test') }}</el-button>
          </el-col>

          <el-col :span="24" v-if="testResult.success != undefined">
            <label>{{ $t('rules.result') }}</label>
            <el-input v-if="testResult.success" v-model="testResult.output" type="textarea" placeholder="Result"
              :autosize="{ minRow: 2, maxRow: 8 }" />
            <div v-if="testResult.success == false" class="test-result">
              <el-icon color="red" size="16">
                <Warning />
              </el-icon> &nbsp;
              <span>{{ testResult.message }}</span>
            </div>
          </el-col>
        </el-form-item>

        <br />
        <el-form-item>
          <el-col :span="24">
            <el-button type="primary" @click="save">{{ $t('common.save') }}</el-button>
            <el-button @click="emit('cancel')">{{ $t('common.cancel') }}</el-button>
          </el-col>
        </el-form-item>

      </el-form>
    </el-col>

    <el-col :span="10" class="part right">
      <el-form :model="form" label-width="auto" label-position="top" style="width: 100%; ">
        <div>{{ $t('rules.source') }}</div>
        <br />
        <div class="io">
          <div v-for="s in form.sources" class="io-item">
            <el-popover placement="top-start" title="Detail" :width="400" trigger="hover">
              <template #default>
                name: {{ s.name }}
                <br />
                type: {{ s.type }}

                <span v-if="s.options && Object.keys(s.options).length > 0">
                  <br />
                  <br />
                  Options:
                </span>
                <template v-if="s.options" v-for="(v, k) in s.options">
                  <br />
                  &nbsp; {{ k }} : {{ v }}
                </template>
              </template>
              <template #reference>
                <div>

                  <el-tag>{{ s.type }}</el-tag>
                  {{ s.name }}
                </div>

              </template>
            </el-popover>
            <div class="io-btn">
              <el-button icon="Edit" circle link type="primary" @click="editSource(s)"></el-button>
              <el-button icon="Delete" circle link type="danger" @click="delIo('source', s.name)"></el-button>
            </div>
          </div>
          <el-row>
            <!-- for add source to rule -->
            <div v-if="ioAdd.source.show" class="io-sel">
              <el-select v-model="ioAdd.source.selected">
                <el-option v-for="s in ioAdd.source.availabe" :label="s.name" :value="s.name" />
              </el-select>
              <el-button type="primary" @click="addIo('source')">{{ $t('common.confirm') }}</el-button>
              <el-button type="success" @click="createNewSource">{{ $t('common.new') }}</el-button>
              <el-button @click="ioAdd.source.show = false">{{ $t('common.cancel') }}</el-button>
            </div>
            <el-button v-else icon="Plus" circle style="float: right; margin-top: 10px" type="primary" size="small"
              @click="toAddIo('source')"></el-button>
          </el-row>
        </div>


        <el-divider></el-divider>
        <div>{{ $t('rules.sink') }}</div>
        <br />

        <div class="io">
          <div v-for="s in form.sinks" class="io-item">
            <el-popover placement="top-start" title="Detail" :width="500" trigger="hover">
              <template #default>
                name: {{ s.name }}
                <br />
                type: {{ s.type }}

                <span v-if="s.options && Object.keys(s.options).length > 0">
                  <br />
                  <br />
                  Options:
                </span>
                <template v-if="s.options" v-for="(v, k) in s.options">
                  <br />
                  &nbsp; {{ k }} : {{ v }}
                </template>

                <!-- Sink tip -->
                <template v-if="ruleSchema.sinkTips[s.type]">
                  <h4>Tip</h4>
                  <div v-html="ruleSchema.sinkTips[s.type].note"></div>
                  <label>Example: </label>
                  <br />
                  <template v-for="c in ruleSchema.sinkTips[s.type].formatExamples">
                    <code>
                    {{ c }}
                  </code>
                  <br/>
                  </template>
                </template>

              </template>
              <template #reference>
                <div>
                  <el-tag>{{ s.type }}</el-tag>
                  {{ s.name }}
                </div>

              </template>
            </el-popover>
            <div class="io-btn">
              <el-button icon="Edit" circle link type="primary" @click="editSink(s)"></el-button>
              <el-button icon="Delete" circle link type="danger" size="large"
                @click="delIo('sink', s.name)"></el-button>
            </div>
          </div>
          <el-row>
            <!-- for add sink to rule -->
            <div v-if="ioAdd.sink.show" class="io-sel">
              <el-select v-model="ioAdd.sink.selected">
                <el-option v-for="s in ioAdd.sink.availabe" :label="s.name" :value="s.name" />
              </el-select>
              <el-button type="primary" @click="addIo('sink')">{{ $t('common.confirm') }}</el-button>
              <el-button type="success" @click="createNewSink">{{ $t('common.new') }}</el-button>
              <el-button @click="ioAdd.sink.show = false">{{ $t('common.cancel') }}</el-button>
            </div>
            <el-button v-else icon="Plus" circle style="float: right; margin-top: 10px" type="primary" size="small"
              @click="toAddIo('sink')"></el-button>
          </el-row>
        </div>
      </el-form>
    </el-col>

  </el-row>

  <!-- Add Component Edit Drawer -->
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
import { computed, nextTick, onMounted, reactive, ref, shallowRef, watch } from 'vue';
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

const emit = defineEmits(["submit"]);


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

const showDebug = ref(false)
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
.con {
  padding: 10px 0 30px 20;

  .part {
    border-top: 1px #eaeaea solid;
    padding: 20px !important;
  }

  .right {
    border-left: 1px #eaeaea solid;
  }
}

.test-result {
  padding: 20px;
  border: 1px #eaeaea solid;
  border-radius: 3px;
  color: #666
}

.el-collapse-item__header {
  font-weight: bold;
}

.editor {
  width: 100%;
  height: 100%;
}

.js-editor {
  margin-top: 10px;
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

.tip {
  color: #999;
  font-size: 14px;
}

.io {
  margin-bottom: 20px;

  .io-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 10px;
    height: 60px;
    padding: 10px;
    border: 1px #DCDFE6 solid;
    border-radius: 3px;

    .io-btn {
      display: none;
    }
  }

  .io-item:hover {
    .io-btn {
      display: inline-block;
    }
  }

  .io-sel {
    display: flex;
    flex: 2;
    justify-content: space-between;
    margin-top: 10px;

    .el-button {
      margin-left: 12px;
    }
  }
}
</style>
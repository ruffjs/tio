<template>

  <el-row :gutter="10" class="con">
    <el-col :span="20" style="margin-bottom: 10px;">
      <el-button icon="ArrowLeft" link @click="emit('cancel')">Back</el-button>
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

        <div>Process
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

        <div>Debug</div>
        <br />
        <el-form-item>
          <el-col :span="12">
            <el-input v-model="form.testData.thingId" placeholder="ThingId" />
          </el-col>
          <el-col :span="12">
            <el-input v-model="form.testData.topic" placeholder="Topic" />
          </el-col>
          <el-col :span="24">
            <label>Payload</label>
            <JSONEditor v-model="form.testData.payload" class="json-editor" />
          </el-col>
          <el-col :span="24">
            <el-button type="primary" @click="test">Test</el-button>
          </el-col>

          <el-col :span="24" v-if="testResult.success != undefined">
            <label>Result</label>
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
            <el-button type="primary" @click="save">Save</el-button>
            <el-button @click="emit('cancel')">Cancel</el-button>
          </el-col>
        </el-form-item>

      </el-form>
    </el-col>

    <el-col :span="10" class="part right">
      <el-form :model="form" label-width="auto" label-position="top" style="width: 100%; ">
        <div>Sources</div>
        <br />
        <div class="io">
          <div v-for="s in form.sources" class="io-item">
            <div><el-tag>{{ s.type }}</el-tag> {{ s.name }} </div>
            <div class="io-btn">
              <el-button icon="Delete" circle link type="danger" @click="delIo('source', s.name)"></el-button>
            </div>
          </div>
          <el-row>
            <div v-if="ioAdd.source.show" class="io-sel">
              <el-select v-model="ioAdd.source.selected">
                <el-option v-for="s in ioAdd.source.availabe" :label="s.name" :value="s.name" />
              </el-select>
              <el-button type="primary" @click="addIo('source')">Confirm</el-button>
              <el-button @click="ioAdd.source.show = false">Cancel</el-button>
            </div>
            <el-button v-else icon="Plus" circle style="float: right; margin-top: 10px" type="primary" size="small"
              @click="toAddIo('source')"></el-button>
          </el-row>
        </div>


        <el-divider></el-divider>
        <div>Sinks</div>
        <br />

        <div class="io">
          <div v-for="s in form.sinks" class="io-item">
            <div><el-tag>{{ s.type }}</el-tag> {{ s.name }} </div>
            <div class="io-btn">
              <el-button icon="Edit" circle link type="primary" size="large"></el-button>
              <el-button icon="Delete" circle link type="danger" size="large"></el-button>
            </div>
          </div>
          <el-row>
            <div v-if="ioAdd.sink.show" class="io-sel">
              <el-select v-model="ioAdd.sink.selected" v-for="s in ioAdd.sink.availabe">
                <el-option :label="s.name" :value="s.name" />
              </el-select>
              <el-button type="primary" @click="addIo('sink')">Confirm</el-button>
              <el-button @click="ioAdd.sink.show = false">Cancel</el-button>
            </div>
            <el-button v-else icon="Plus" circle style="float: right; margin-top: 10px" type="primary" size="small"
              @click="toAddIo('sink')"></el-button>
          </el-row>
        </div>
      </el-form>
    </el-col>

  </el-row>

</template>

<script setup>
import { computed, nextTick, onMounted, reactive, ref, shallowRef, watch } from 'vue';
import { ElNotification } from 'element-plus'
import * as api from '@/apis';

import JsEditor from '@/components/rule/JsEditor.vue';
import JSONEditor from '@/components/common/JSONEditor.vue';
import { tryMerge } from './rule.ts';

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
  sources: [],
  sinks: [],
  process: [{
    type: "transform",
    name: "",
    runner: "js",
    js: "const run = data=> { \n\n}",
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
    form.name = rule.name
    form.note = rule.note
    form.process = rule.process

    form.process.forEach(p => {
      if (p.runner == undefined) {
        p.runner = !!p.jq ? "jq" : "js"
      }
    })
    form.sources = rule.sources.map(s => config.sources.find(cs => cs.name == s))
    form.sinks = rule.sinks.map(s => config.sinks.find(cs => cs.name == s))
  }

  ioAdd.source.availabe = props.config.sources.filter(s => !form.sources.find(ns => ns.name == s.name))
  ioAdd.sink.availabe = props.config.sinks.filter(s => !form.sinks.find(ns => ns.name == s.name))
}

watch(() => form.sources, () => {
  ioAdd.source.availabe = props.config.sources.filter(s => !form.sources.find(ns => ns.name == s.name))
}, { deep: true })
watch(() => form.sinks, () => {
  ioAdd.sink.availabe = props.config.sinks.filter(s => !form.sinks.find(ns => ns.name == s.name))
}, { deep: true })

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
  const s = d.availabe.find(a=>a.name == d.selected)
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
  if (r.data.output && typeof r.data.output == 'object') {
    testResult.value.output = JSON.stringify(r.data.output, null, 2)
  }
}

const save = async () => {
  console.debug('======> save rule', form)
  console.debug('old config', props.config)
  try {
    const mergeResult = tryMerge(props.config, form, props.isNew)
    if (!mergeResult.isValid) {
      ElNotification({ message: mergeResult.errors.join('\n'), type: 'error' })
      return
    }
    console.debug('merged config:', mergeResult)
    await api.saveRulesConfig(mergeResult.config)
    ElNotification({ message: 'Save success', type: 'success' })
    emit('cancel')
  } catch (e) {
    const msg = e.message || e + ''
    ElNotification({ message: msg, type: 'error' })
  }
}

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
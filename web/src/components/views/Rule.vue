<template>

  <div class="con">

    <EditRule v-if="showEditRule" :rule="currentRule" :config="data.config" :isNew="isNewRule"
      @cancel="afterRuleEdit" />
    <div v-show="!showEditRule">
      <el-card>
        <div>
          <label class="segment-title">{{ $t('rules.title') }}
            <el-popover placement="top-start" :title="$t('rules.help')" :width="550" trigger="hover">
              <template #default>
                <p>
                  {{ $t('rules.helpContent') }}
                </p>
              </template>
              <template #reference>

                <el-icon>
                  <QuestionFilled />
                </el-icon>
              </template>
            </el-popover>
          </label>
          <el-button type="primary" icon="Plus" size="small" @click="showAddRule">{{ $t('common.add') }}</el-button>
        </div>
        <el-table :data="data.config.rules" height="100%">
          <el-table-column prop="name" :label="$t('rules.name')">
            <template #default="scope">
              <el-button link type="primary" @click.prevent="editRule(scope.row)">
                {{ scope.row.name }}
              </el-button>
            </template>
          </el-table-column>
          <el-table-column prop="sources" :label="$t('rules.source')">
            <template #default="scope">
              <template v-for="(d, i) in scope.row.sources.map(n => getSrcDesp(n))">
                <br v-if="i > 0" />
                <el-tag type="info">{{ d }}</el-tag>
              </template>
            </template>
          </el-table-column>
          <el-table-column prop="sinks" :label="$t('rules.sink')">
            <template #default="scope">
              <template v-for="(d, i) in scope.row.sinks.map(n => getSinkDesp(n))">
                <br v-if="i > 0" />
                <el-tag type="info">{{ d }}</el-tag>
              </template>
            </template>
          </el-table-column>
          <el-table-column :label="$t('rules.status')">
            <template #header>
              {{ $t('rules.status') }}
              <el-tooltip :content="$t('common.info')">
                <el-icon>
                  <InfoFilled />
                </el-icon>
              </el-tooltip>
            </template>
            <template #default="scope">
              <component :is="getStatus('rule', scope.row.name)" />
            </template>
          </el-table-column>
          <el-table-column prop="note" label="Note">
          </el-table-column>
          <el-table-column prop="eanbled" :label="$t('rules.enabled')" width="70">
            <template #default="scope">
              <el-switch v-model="scope.row.enabled" size="small"
                @change="v => toggleEnable('rule', scope.row, v)">{{ $t('rules.enabled') }}</el-switch>
            </template>
          </el-table-column>
          <el-table-column :label="$t('rules.actions')" width="190">
            <template #default="scope">
              <el-button link type="primary" @click.prevent="editRule(scope.row)">
                {{ $t('common.edit') }}
              </el-button>
              <el-button link type="primary" @click.prevent="duplicateRule(scope.row)">
                {{ $t('common.duplicate') }}
              </el-button>
              <DeleteButton :title="$t('common.confirmDelete')" @confirm="delRule(scope.row)" />
            </template>

          </el-table-column>
        </el-table>
      </el-card>

      <br />
      <el-card>
        <div>
          <label class="segment-title">{{ $t('rules.connector') }}</label>
          <el-button type="primary" icon="Plus" size="small" @click="showAddConnector">{{ $t('common.add') }}</el-button>
        </div>

        <el-table :data="data.config.connectors" height="100%">
          <el-table-column prop="name" :label="$t('rules.name')">
            <template #default="scope">
              <el-button link type="primary" @click.prevent="editConn(scope.row)">
                {{ scope.row.name }}
              </el-button>
            </template>
          </el-table-column>
          <el-table-column prop="type" :label="$t('rules.type')">
          </el-table-column>
          <el-table-column :label="$t('rules.status')">
            <template #default="scope">
              <component :is="getStatus('connector', scope.row.name)" />
            </template>
          </el-table-column>
          <el-table-column prop="eanbled" :label="$t('rules.enabled')" width="70">
            <template #default="scope">
              <el-switch v-model="scope.row.enabled" size="small"
                @change="v => toggleEnable('connector', scope.row, v)">{{ $t('rules.enabled') }}</el-switch>
            </template>
          </el-table-column>
          <el-table-column :label="$t('rules.actions')" width="120">
            <template #default="scope">
              <el-button link type="primary" @click.prevent="editConn(scope.row)">
                {{ $t('common.edit') }}
              </el-button>
              <DeleteButton :title="$t('common.confirmDelete')" @confirm="delConn(scope.row)" />
            </template>
          </el-table-column>
        </el-table>
        <br />

        <div>
          <label class="segment-title">{{ $t('rules.source') }}</label>
          <el-button type="primary" icon="Plus" size="small" @click="showAddSrc">{{ $t('common.add') }}</el-button>
        </div>
        <el-table :data="data.config.sources" height="100%">
          <el-table-column prop="name" :label="$t('rules.name')">
            <template #default="scope">
              <el-button link type="primary" @click.prevent="editSrc(scope.row)">
                {{ scope.row.name }}
              </el-button>
            </template>
          </el-table-column>
          <el-table-column prop="type" :label="$t('rules.type')" />
          <el-table-column :label="$t('rules.status')">
            <template #default="scope">
              <component :is="getStatus('source', scope.row.name)" />
            </template>
          </el-table-column>
          <el-table-column prop="eanbled" :label="$t('rules.enabled')" width="70">
            <template #default="scope">
              <el-switch v-model="scope.row.enabled" size="small"
                @change="v => toggleEnable('source', scope.row, v)">{{ $t('rules.enabled') }}</el-switch>
            </template>
          </el-table-column>
          <el-table-column :label="$t('rules.actions')" width="120">
            <template #default="scope">
              <el-button link type="primary" @click.prevent="editSrc(scope.row)">
                {{ $t('common.edit') }}
              </el-button>
              <DeleteButton title="Confimr delete?" @confirm="delSrc(scope.row)" />
            </template>
          </el-table-column>
        </el-table>
        <br />

        <div>
          <label class="segment-title">{{ $t('rules.sink') }}</label>
          <el-button type="primary" icon="Plus" size="small" @click="showAddSink">{{ $t('common.add') }}</el-button>
        </div>
        <el-table :data="data.config.sinks" height="100%">
          <el-table-column prop="name" :label="$t('rules.name')">
            <template #default="scope">
              <el-button link type="primary" @click.prevent="editSink(scope.row)">
                {{ scope.row.name }}
              </el-button>
            </template>
          </el-table-column>
          <el-table-column prop="type" :label="$t('rules.type')" />
          <el-table-column :label="$t('rules.status')">
            <template #default="scope">
              <component :is="getStatus('sink', scope.row.name)" />
            </template>
          </el-table-column>
          <el-table-column prop="eanbled" :label="$t('rules.enabled')" width="70">
            <template #default="scope">
              <el-switch v-model="scope.row.enabled" size="small"
                @change="v => toggleEnable('sink', scope.row, v)">{{ $t('rules.enabled') }}</el-switch>
            </template>
          </el-table-column>
          <el-table-column :label="$t('rules.actions')" width="120">
            <template #default="scope">
              <el-button link type="primary" @click.prevent="editSink(scope.row)">
                {{ $t('common.edit') }}
              </el-button>
              <DeleteButton :title="$t('common.confirmDelete')" @confirm="delSink(scope.row)" />
            </template>
          </el-table-column>
        </el-table>
      </el-card>
    </div>
  </div>

  <el-drawer size="700" destroy-on-close v-model="drawerEdit.show" v-if="drawerEdit.show" :title="drawerEdit.title"
    append-to-body>
    <EditOpt ref="editOpt" v-model="drawerEdit.data" :schema="drawerEdit.schema"
      :optionsSchema="drawerEdit.optionsSchema" :config="data.config" :type="drawerEdit.type" :isNew="drawerEdit.new" />
    <template #footer>
      <div style="flex: auto">
        <el-button type="primary" @click="confirmEdit">{{ $t('common.confirm') }}</el-button>
        <el-button @click="cancelEdit">{{ $t('common.cancel') }}</el-button>
      </div>
    </template>
  </el-drawer>
</template>

<script setup>
import { h, onMounted, onUnmounted, reactive, ref } from 'vue';
import { ElMessageBox, ElTag, ElPopover, ElTable, ElTableColumn } from 'element-plus';
import * as api from '@/apis';
import { getDependent } from '@/components/rule/rule'
import EditRule from '@/components/rule/EditRule.vue';
import EditOpt from '@/components/rule/EditOpt.vue';
import * as ruleSchema from '@/components/rule/rule-schema'
import { deepCopy } from '@/utils/common'
import { ElNotification } from 'element-plus';
import DeleteButton from '@/components/list/DeleteButton.vue'

const emptyRuleConfig = {
  connectors: [],
  sources: [],
  sinks: [],
  rules: [],
}
const data = reactive({ config: emptyRuleConfig, status: {} })
const showEditRule = ref(false)
const currentRule = ref(null)
const isNewRule = ref(false)

const editOpt = ref()
const defaultEditData = {
  show: false,
  new: true,
  type: '',
  title: '',
  schema: [],
  optionsSchema: {},
  data: {},
}
const drawerEdit = reactive(Object.assign({}, defaultEditData))


let refreshInterval = null
onMounted(async () => {
  await loadRuleConfig()
  refreshInterval = setInterval(loadRuleConfig, 10 * 1000)
})
onUnmounted(() => {
  clearInterval(refreshInterval)
})

const loadRuleConfig = async () => {
  const r = await api.getRulesConfig({ withStatus: true })
  if (r.code != 200) {
    ElNotification({ message: 'Get rule config failed', type: 'error' })
    return
  }
  if (r.data?.config) {
    Object.assign(data, r.data)
  } else {
    ElNotification({ message: 'Rule is not configured', type: 'info' })
  }

  // avoid null value for array
  const rule = data.config
  if (!rule.connectors) rule.connectors = []
  if (!rule.sources) rule.sources = []
  if (!rule.sinks) rule.sinks = []
  if (!rule.rules) rule.rules = []
}

const toggleEnable = async (type, row, enable) => {
  const name = row.name
  const dp = getDependent(type, name, data.config)
  if (dp.length > 0) {
    const msg = (enable ? 'Enable' : 'Disable') + ` ${type} ${name} will affect `
      + dp.map(d => `${d.type}( ${d.names.join(", ")} )`).join(' and ')
    try {
      await ElMessageBox.confirm(msg, 'Confirm',
        {
          confirmButtonText: 'OK',
          cancelButtonText: 'Cancel',
          type: 'info',
        })
    } catch (e) {
      row.enabled = !enable
      return
    }
  }

  const r = await api.toggleRuleComponet(type, row.name, enable);
  if (r.code == 200) {
    ElNotification({ message: 'Success', type: 'success' })
  } else {
    ElNotification({ message: r.message || 'Failed', type: 'error' })
  }
  await loadRuleConfig()
}

const getStatus = (type, name) => {
  const status = data.status[type].find(c => c.name == name)?.status || {}

  let tagType = "info"
  if (['connected', 'running'].includes(status.status)) tagType = 'success'
  if (status.status == 'stopped') tagType = 'info'
  else 'danger'

  const tag = h(ElTag, { type: tagType }, { default: () => status.status })
  // connector has no metrics
  if (type == 'connector' && tagType == 'success') return tag;

  const reason = h('div', status.reason)
  let metric = h('div', '')
  if (status.metric && Object.keys(status.metric).length > 0) {
    const metricData = Object.keys(status.metric).map(k => ({ key: k, value: status.metric[k] }))
    metric = h(
      ElTable,
      { data: metricData },
      {
        default: () => [
          h(ElTableColumn, { prop: 'key', label: 'Metric' }),
          h(ElTableColumn, { prop: 'value', label: 'Value' }),
        ]
      },
    )
  }
  const popover = h(ElPopover,
    { width: 400 },
    {
      reference: () => tag,
      default: () => [reason, metric]
    },
  )

  return popover
}

// ----------- rule -----------
const showAddRule = () => {
  currentRule.value = null
  isNewRule.value = true
  showEditRule.value = true
}

const editRule = (rule) => {
  currentRule.value = rule
  isNewRule.value = false
  showEditRule.value = true
}
const duplicateRule = (rule) => {
  const dup = deepCopy(rule)
  dup.name = ''
  currentRule.value = dup
  isNewRule.value = true
  showEditRule.value = true
}
const delRule = (rule) => {
  const { rules } = data.config
  rules.splice(rules.findIndex(r => r.name == rule.name))
}

const getSrcDesp = (name) => {
  const s = data.config.sources.find(s => s.name == name)
  if (!s) {
    return `error(${name} not found)`
  }
  return `${s.type}:${s.name}`
}
const getSinkDesp = (name) => {
  const s = data.config.sinks.find(s => s.name == name)
  if (!s) {
    return `error(${name} not found)`
  }
  return `${s.type}:${s.name}`
}
const afterRuleEdit = async () => {
  showEditRule.value = false
  await loadRuleConfig()
}

const saveRule = async () => {
  try {
    await api.saveRulesConfig(data.config)
    ElNotification({ type: 'success', message: 'Saved successfully' })
  } catch (e) {
    ElNotification({ type: 'error', message: 'Failed to save: ' + (e.messge || e + '') })
    console.error('failed to save rule', deepCopy(data.config))
  }
}

// ----------- connector, source, sinks -----------

// common

const confirmEdit = async () => {
  const d = deepCopy(drawerEdit.data)
  const v = await editOpt.value.validate()
  if (!v.valid) return
  if (!data.config.connectors) {
    data.config.connectors = []
  }

  const list = data.config[drawerEdit.type + 's']
  const exist = list.find(c => c.name == d.name)
  if (exist && drawerEdit.new) {
    ElNotification({ type: 'error', message: 'Name already exists!' })
    return
  }
  if (drawerEdit.new) {
    d.enabled = true
    list.push(d)
  } else {
    for (let i = 0; i < list.length; i++) {
      if (list[i].name == d.name) {
        list[i] = d
        console.debug('updated list for ', d.name)
        break
      }
    }
  }

  Object.assign(drawerEdit, defaultEditData)
  await saveRule()
  await loadRuleConfig()
}
const cancelEdit = () => {
  Object.assign(drawerEdit, defaultEditData)
}

// connector 

const showAddConnector = () => {
  const d = drawerEdit
  d.type = 'connector'
  d.new = true
  d.title = "Add Connector"
  d.schema = deepCopy(ruleSchema.connector)
  d.data = deepCopy(ruleSchema.defaultConnector)
  d.optionsSchema = deepCopy(ruleSchema.connectorOptions)
  d.show = true
}
const editConn = row => {
  const d = drawerEdit
  d.type = 'connector'
  d.new = false
  d.title = "Edit Connector"
  d.schema = deepCopy(ruleSchema.connector)
  d.data = deepCopy(row)
  d.optionsSchema = deepCopy(ruleSchema.connectorOptions)
  d.show = true
}
const delConn = async row => {
  const { sources, sinks, connectors } = data.config
  const usedBySrc = sources.filter(s => s.connector == row.name).map(s => s.name)
  const usedBySink = sinks.filter(s => s.connector == row.name).map(s => s.name)
  let msg = ''
  if (usedBySrc.length > 0) {
    msg += 'sources: ' + usedBySrc.join(',')
  }
  if (usedBySink.length > 0) {
    msg += 'sink: ' + usedBySink.join(',')
  }
  if (msg) {
    ElNotification({ type: 'error', message: 'Connector is used by ' + msg })
    return
  }
  connectors.splice(connectors.findIndex(c => c.name == row.name), 1)
  await saveRule()
}

// source

const showAddSrc = () => {
  const d = drawerEdit
  d.type = 'source'
  d.new = true
  d.title = "Add Source"
  d.schema = deepCopy(ruleSchema.source)
  d.data = deepCopy(ruleSchema.defaultSrc)
  d.optionsSchema = deepCopy(ruleSchema.sourceOptions)
  d.show = true
}
const editSrc = row => {
  const d = drawerEdit
  d.type = 'source'
  d.new = false
  d.title = "Edit Source"
  d.schema = deepCopy(ruleSchema.source)
  d.data = deepCopy(row)
  d.optionsSchema = deepCopy(ruleSchema.sourceOptions)
  d.show = true
}
const delSrc = async row => {
  const { sources, rules } = data.config
  const usedByRule = rules.filter(r => r.sources.includes(row.name)).map(r => r.name)
  let msg = usedByRule.join(',')
  if (msg) {
    ElNotification({ type: 'error', message: 'Source is used by rule: ' + msg })
    return
  }
  sources.splice(sources.findIndex(c => c.name == row.name), 1)
  await saveRule()
}

// sink

const showAddSink = () => {
  const d = drawerEdit
  d.type = 'sink'
  d.new = true
  d.title = "Add Sink"
  d.schema = deepCopy(ruleSchema.sink)
  d.data = deepCopy(ruleSchema.defaultSink)
  d.optionsSchema = deepCopy(ruleSchema.sinkOptions)
  d.show = true
}
const editSink = row => {
  const d = drawerEdit
  d.type = 'sink'
  d.new = false
  d.title = "Edit Sink"
  d.schema = deepCopy(ruleSchema.sink)
  d.data = deepCopy(row)
  d.optionsSchema = deepCopy(ruleSchema.sinkOptions)
  d.show = true
}
const delSink = async row => {
  const { sinks, rules } = data.config
  const usedByRule = rules.filter(r => r.sinks.includes(row.name)).map(r => r.name)
  let msg = usedByRule.join(',')
  if (msg) {
    ElNotification({ type: 'error', message: 'Sink is used by rule: ' + msg })
    return
  }
  sinks.splice(sinks.findIndex(c => c.name == row.name), 1)
  await saveRule()
}


</script>

<style lang="scss" scoped>
.con {
  margin: 0 10px 30px 10px;

  .segment-title {
    display: inline-block;
    margin: 0px 30px 10px 0px;
    width: 100px;
    font-size: 14px;
  }
}
</style>
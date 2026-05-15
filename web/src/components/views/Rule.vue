<template>
  <div class="rule-con">
    <EditRule
      v-if="showEditRule"
      :rule="currentRule"
      :config="data.config"
      :isNew="isNewRule"
      @cancel="afterRuleEdit"
    />

    <div v-show="!showEditRule" class="rules-workspace">
      <header class="workspace-toolbar">
        <div class="toolbar-copy">
          <div class="title-row">
            <h1>{{ $t('rules.title') }}</h1>
            <el-popover placement="bottom-start" :title="$t('rules.help')" :width="550" trigger="hover">
              <template #default>
                <p class="help-text">{{ $t('rules.helpContent') }}</p>
              </template>
              <template #reference>
                <el-button class="help-button" link :aria-label="$t('rules.help')">
                  <el-icon><QuestionFilled /></el-icon>
                </el-button>
              </template>
            </el-popover>
          </div>
          <p>Sources, process steps, and sinks are managed as one integration flow.</p>
        </div>

        <div class="toolbar-actions">
          <div class="summary-pills" aria-label="Rule resources summary">
            <span>{{ $t('rules.title') }} <b>{{ data.config.rules.length }}</b></span>
            <span>{{ $t('rules.connector') }} <b>{{ data.config.connectors.length }}</b></span>
            <span>{{ $t('rules.source') }} <b>{{ data.config.sources.length }}</b></span>
            <span>{{ $t('rules.sink') }} <b>{{ data.config.sinks.length }}</b></span>
          </div>
          <el-button v-if="activeResourceTab === 'rule'" type="primary" icon="Plus" @click="showAddRule">
            {{ $t('rules.addRule') }}
          </el-button>
          <el-button v-if="activeResourceTab === 'connector'" type="primary" icon="Plus" @click="showAddConnector">
            {{ $t('common.add') }} {{ $t('rules.connector') }}
          </el-button>
          <el-button v-if="activeResourceTab === 'source'" type="primary" icon="Plus" @click="showAddSrc">
            {{ $t('common.add') }} {{ $t('rules.source') }}
          </el-button>
          <el-button v-if="activeResourceTab === 'sink'" type="primary" icon="Plus" @click="showAddSink">
            {{ $t('common.add') }} {{ $t('rules.sink') }}
          </el-button>
        </div>
      </header>

      <el-tabs v-model="activeResourceTab" class="workspace-tabs">
        <el-tab-pane name="rule">
          <template #label>
            <span class="tab-label">{{ $t('rules.title') }} <b>{{ data.config.rules.length }}</b></span>
          </template>
          <div class="table-shell">
            <el-table :data="data.config.rules" class="clean-table" row-class-name="action-row">
              <el-table-column prop="name" :label="$t('rules.name')" min-width="150">
                <template #default="scope">
                  <div class="name-cell">
                    <el-button link type="primary" class="name-link" @click.prevent="editRule(scope.row)">
                      {{ scope.row.name || '-' }}
                    </el-button>
                    <span v-if="scope.row.note" class="note-text">{{ scope.row.note }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column prop="sources" :label="$t('rules.source')" min-width="150">
                <template #default="scope">
                  <div class="chip-stack">
                    <el-tag v-for="name in scope.row.sources" :key="name" type="info" effect="plain" round>
                      {{ getSrcDesp(name) }}
                    </el-tag>
                  </div>
                </template>
              </el-table-column>
              <el-table-column prop="sinks" :label="$t('rules.sink')" min-width="150">
                <template #default="scope">
                  <div class="chip-stack">
                    <el-tag v-for="name in scope.row.sinks" :key="name" type="info" effect="plain" round>
                      {{ getSinkDesp(name) }}
                    </el-tag>
                  </div>
                </template>
              </el-table-column>
              <el-table-column :label="$t('rules.status')" width="110">
                <template #header>
                  <span class="status-header">
                    {{ $t('rules.status') }}
                    <el-tooltip :content="$t('common.info')">
                      <el-icon><InfoFilled /></el-icon>
                    </el-tooltip>
                  </span>
                </template>
                <template #default="scope">
                  <component :is="getStatus('rule', scope.row.name)" />
                </template>
              </el-table-column>
              <el-table-column prop="enabled" :label="$t('rules.enabled')" width="82" align="center">
                <template #default="scope">
                  <el-switch
                    v-model="scope.row.enabled"
                    size="small"
                    @change="v => toggleEnable('rule', scope.row, v)"
                  />
                </template>
              </el-table-column>
              <el-table-column :label="$t('rules.actions')" width="166" align="right">
                <template #default="scope">
                  <el-dropdown
                    trigger="click"
                    popper-class="rule-actions-popper"
                    @command="(command) => handleRuleCommand(command, scope.row)"
                  >
                    <el-button class="row-more-button" link icon="MoreFilled" />
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item command="edit">{{ $t('common.edit') }}</el-dropdown-item>
                        <el-dropdown-item command="duplicate">{{ $t('common.duplicate') }}</el-dropdown-item>
                        <el-dropdown-item command="delete" divided>{{ $t('common.delete') }}</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </el-tab-pane>

        <el-tab-pane name="connector">
          <template #label>
            <span class="tab-label">{{ $t('rules.connector') }} <b>{{ data.config.connectors.length }}</b></span>
          </template>
          <div class="resource-note">Connection profiles shared by sources and sinks.</div>
          <div class="table-shell">
            <el-table :data="data.config.connectors" class="clean-table" row-class-name="action-row">
              <el-table-column prop="name" :label="$t('rules.name')" min-width="150">
                <template #default="scope">
                  <el-button link type="primary" class="name-link" @click.prevent="editConn(scope.row)">
                    {{ scope.row.name }}
                  </el-button>
                </template>
              </el-table-column>
              <el-table-column prop="type" :label="$t('rules.type')" min-width="110" />
              <el-table-column :label="$t('rules.status')" width="110">
                <template #default="scope">
                  <component :is="getStatus('connector', scope.row.name)" />
                </template>
              </el-table-column>
              <el-table-column prop="enabled" :label="$t('rules.enabled')" width="82" align="center">
                <template #default="scope">
                  <el-switch
                    v-model="scope.row.enabled"
                    size="small"
                    @change="v => toggleEnable('connector', scope.row, v)"
                  />
                </template>
              </el-table-column>
              <el-table-column :label="$t('rules.actions')" width="108" align="right">
                <template #default="scope">
                  <el-dropdown
                    trigger="click"
                    popper-class="rule-actions-popper"
                    @command="(command) => handleResourceCommand('connector', command, scope.row)"
                  >
                    <el-button class="row-more-button" link icon="MoreFilled" />
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item command="edit">{{ $t('common.edit') }}</el-dropdown-item>
                        <el-dropdown-item command="delete" divided>{{ $t('common.delete') }}</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </el-tab-pane>

        <el-tab-pane name="source">
          <template #label>
            <span class="tab-label">{{ $t('rules.source') }} <b>{{ data.config.sources.length }}</b></span>
          </template>
          <div class="resource-note">Inputs that receive data before rule processing.</div>
          <div class="table-shell">
            <el-table :data="data.config.sources" class="clean-table" row-class-name="action-row">
              <el-table-column prop="name" :label="$t('rules.name')" min-width="150">
                <template #default="scope">
                  <el-button link type="primary" class="name-link" @click.prevent="editSrc(scope.row)">
                    {{ scope.row.name }}
                  </el-button>
                </template>
              </el-table-column>
              <el-table-column prop="type" :label="$t('rules.type')" min-width="100" />
              <el-table-column prop="connector" :label="$t('rules.connector')" min-width="120" />
              <el-table-column :label="$t('rules.status')" width="110">
                <template #default="scope">
                  <component :is="getStatus('source', scope.row.name)" />
                </template>
              </el-table-column>
              <el-table-column prop="enabled" :label="$t('rules.enabled')" width="82" align="center">
                <template #default="scope">
                  <el-switch
                    v-model="scope.row.enabled"
                    size="small"
                    @change="v => toggleEnable('source', scope.row, v)"
                  />
                </template>
              </el-table-column>
              <el-table-column :label="$t('rules.actions')" width="108" align="right">
                <template #default="scope">
                  <el-dropdown
                    trigger="click"
                    popper-class="rule-actions-popper"
                    @command="(command) => handleResourceCommand('source', command, scope.row)"
                  >
                    <el-button class="row-more-button" link icon="MoreFilled" />
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item command="edit">{{ $t('common.edit') }}</el-dropdown-item>
                        <el-dropdown-item command="delete" divided>{{ $t('common.delete') }}</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </el-tab-pane>

        <el-tab-pane name="sink">
          <template #label>
            <span class="tab-label">{{ $t('rules.sink') }} <b>{{ data.config.sinks.length }}</b></span>
          </template>
          <div class="resource-note">Outputs that receive transformed rule results.</div>
          <div class="table-shell">
            <el-table :data="data.config.sinks" class="clean-table" row-class-name="action-row">
              <el-table-column prop="name" :label="$t('rules.name')" min-width="150">
                <template #default="scope">
                  <el-button link type="primary" class="name-link" @click.prevent="editSink(scope.row)">
                    {{ scope.row.name }}
                  </el-button>
                </template>
              </el-table-column>
              <el-table-column prop="type" :label="$t('rules.type')" min-width="100" />
              <el-table-column prop="connector" :label="$t('rules.connector')" min-width="120" />
              <el-table-column :label="$t('rules.status')" width="110">
                <template #default="scope">
                  <component :is="getStatus('sink', scope.row.name)" />
                </template>
              </el-table-column>
              <el-table-column prop="enabled" :label="$t('rules.enabled')" width="82" align="center">
                <template #default="scope">
                  <el-switch
                    v-model="scope.row.enabled"
                    size="small"
                    @change="v => toggleEnable('sink', scope.row, v)"
                  />
                </template>
              </el-table-column>
              <el-table-column :label="$t('rules.actions')" width="108" align="right">
                <template #default="scope">
                  <el-dropdown
                    trigger="click"
                    popper-class="rule-actions-popper"
                    @command="(command) => handleResourceCommand('sink', command, scope.row)"
                  >
                    <el-button class="row-more-button" link icon="MoreFilled" />
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item command="edit">{{ $t('common.edit') }}</el-dropdown-item>
                        <el-dropdown-item command="delete" divided>{{ $t('common.delete') }}</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </el-tab-pane>
      </el-tabs>
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
const activeResourceTab = ref('rule')

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
  if (!data.status) data.status = {}
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
  const status = data.status[type]?.find(c => c.name == name)?.status || {}
  const statusText = status.status || 'unknown'

  let tagType = 'info'
  if (['connected', 'running'].includes(statusText)) tagType = 'success'
  if (statusText && !['connected', 'running', 'stopped', 'unknown'].includes(statusText)) tagType = 'danger'

  const tag = h(ElTag, { type: tagType, effect: 'plain', round: true }, { default: () => statusText })
  // connector has no metrics
  if (type == 'connector' && tagType == 'success') return tag;

  const reason = h('div', { class: 'status-reason' }, status.reason || '')
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
const handleRuleCommand = async (command, rule) => {
  if (command == 'edit') {
    editRule(rule)
    return
  }
  if (command == 'duplicate') {
    duplicateRule(rule)
    return
  }
  if (command == 'delete') {
    try {
      await ElMessageBox.confirm(rule.name, 'Confirm delete?', {
        confirmButtonText: 'OK',
        cancelButtonText: 'Cancel',
        type: 'warning',
      })
      await delRule(rule)
    } catch (e) {
      // user cancelled
    }
  }
}
const delRule = async (rule) => {
  const { rules } = data.config
  rules.splice(rules.findIndex(r => r.name == rule.name), 1)
  await saveRule()
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

const handleResourceCommand = async (type, command, row) => {
  if (command == 'edit') {
    if (type == 'connector') editConn(row)
    if (type == 'source') editSrc(row)
    if (type == 'sink') editSink(row)
    return
  }

  if (command == 'delete') {
    try {
      await ElMessageBox.confirm(row.name, 'Confirm delete?', {
        confirmButtonText: 'OK',
        cancelButtonText: 'Cancel',
        type: 'warning',
      })
      if (type == 'connector') await delConn(row)
      if (type == 'source') await delSrc(row)
      if (type == 'sink') await delSink(row)
    } catch (e) {
      // user cancelled
    }
  }
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
.rule-con {
  box-sizing: border-box;
  min-width: 0;
  max-width: 100%;
  margin: 0 0 30px;
  color: var(--tio-text);
  overflow: hidden;
}

.rules-workspace {
  min-width: 0;
  padding: 2px 2px 14px;
}

.workspace-toolbar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  min-width: 0;
  margin-bottom: 14px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--tio-line);
}

.toolbar-copy {
  min-width: 0;

  p {
    margin: 6px 0 0;
    color: var(--tio-muted);
    line-height: 1.6;
  }
}

.title-row {
  display: flex;
  align-items: center;
  gap: 8px;

  h1 {
    margin: 0;
    color: var(--tio-text-strong);
    font-size: 24px;
    font-weight: 760;
    letter-spacing: -0.02em;
  }
}

.help-button {
  color: var(--tio-muted);
}

.help-text {
  margin: 0;
  white-space: pre-line;
  line-height: 1.6;
}

.toolbar-actions {
  display: flex;
  flex-shrink: 0;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
}

.summary-pills {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 6px;

  span {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 5px 9px;
    border-radius: 999px;
    background: var(--tio-surface-soft);
    color: var(--tio-muted);
    font-size: 12px;
  }

  b {
    color: var(--tio-text-strong);
    font-weight: 760;
  }
}

.workspace-tabs {
  min-width: 0;

  :deep(.el-tabs__header) {
    margin-bottom: 12px;
  }

  :deep(.el-tabs__nav-wrap::after) {
    height: 1px;
    background: var(--tio-line);
  }

  :deep(.el-tabs__content),
  :deep(.el-tab-pane) {
    min-width: 0;
  }
}

.tab-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;

  b {
    min-width: 20px;
    padding: 1px 6px;
    border-radius: 999px;
    background: var(--tio-surface-soft);
    color: var(--tio-muted);
    font-size: 11px;
    font-weight: 700;
    text-align: center;
  }
}

.resource-note {
  margin: 0 0 10px;
  color: var(--tio-muted);
  font-size: 13px;
}

.table-shell {
  max-width: 100%;
  min-width: 0;
  overflow-x: auto;
  border: 1px solid var(--tio-line);
  border-radius: var(--tio-radius);
}

.clean-table {
  width: 100%;
  min-width: 680px;

  :deep(.el-table__inner-wrapper::before) {
    display: none;
  }

  :deep(th.el-table__cell) {
    background: var(--tio-surface-soft);
    color: var(--tio-muted);
    font-size: 12px;
    font-weight: 700;
  }

  :deep(td.el-table__cell) {
    border-color: var(--tio-line);
  }

  :deep(.el-table__row:hover > td.el-table__cell) {
    background: var(--tio-surface-soft);
  }

  :deep(.el-table__row:hover .row-more-button),
  :deep(.row-more-button:focus) {
    opacity: 1;
  }
}

.name-cell {
  display: flex;
  min-width: 0;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
}

.name-link {
  min-height: auto;
  padding: 0;
  color: var(--tio-link);
  font-weight: 650;
}

.note-text {
  max-width: 360px;
  overflow: hidden;
  color: var(--tio-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-header {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.chip-stack {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;

  :deep(.el-tag) {
    max-width: 220px;
    border-color: var(--tio-line);
    background: var(--tio-surface-soft);
    color: var(--tio-muted);
  }
}

.row-more-button.el-button {
  opacity: 0.34;
  transition: opacity 0.16s ease;
}

:global(.status-reason) {
  color: var(--tio-text);
  line-height: 1.5;
}

@media (max-width: 900px) {
  .workspace-toolbar {
    flex-direction: column;
  }

  .toolbar-actions,
  .summary-pills {
    justify-content: flex-start;
  }

  .row-more-button.el-button {
    opacity: 1;
  }
}
</style>

<style lang="scss">
.rule-actions-popper {
  min-width: 120px;
}
</style>

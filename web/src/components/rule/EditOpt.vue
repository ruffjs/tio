<template>
  <div>
    <el-form :model="model" label-width="auto" style="width: 100%;" ref="formEl" status-icon>
      <template v-for="(s, name) in schema">
        <template v-if="s != null">
          <el-form-item v-if="['text', 'number', 'checkbox', 'select'].includes(s.type)" :label="s.label"
            :rules="s.rules" :prop="name">
            <el-input v-if="s.type == 'text'" v-model="model[name]" :placeholder="s.placeholder" :disabled="!isNew" />
            <el-input type="password" show-password v-if="s.type == 'password'" v-model="model[name]"
              :placeholder="s.placeholder" />
            <el-input-number v-if="s.type == 'number'" v-model="model[name]" :placeholder="s.placeholder" />
            <el-select v-if="s.type == 'select'" v-model="model[name]" :placeholder="s.placeholder">
              <el-option v-for="o in s.options" :label="o.label" :value="o.value" />
            </el-select>
          </el-form-item>

          <template v-if="s.type == 'object'">
            <el-divider />
            <label>{{ s.label }}</label>
            <el-form-item v-for="(t, tname ) in s.items" :label="t.label" :rules="t.rules" :prop="name + '.' + tname">
              <el-input v-if="t.type == 'text'" v-model="model[name][tname]" :placeholder="t.placeholder" />
              <el-input type="password" show-password v-if="t.type == 'password'" v-model="model[name][tname]"
                :placeholder="t.placeholder" />
              <el-input-number v-if="t.type == 'number'" v-model="model[name][tname]" :placeholder="t.placeholder" />
              <el-checkbox v-if="t.type == 'checkbox'" v-model="model[name][tname]" :title="t.placeholder" />
              <el-select v-if="t.type == 'select'" v-model="model[name][tname]" :placeholder="t.placeholder">
                <el-option v-for="o in t.options" :label="o.label" :value="o.value" />
              </el-select>
              <div v-if="t.type == 'kv'">
                <el-row>
                  <el-form-item>
                    <el-input />
                  </el-form-item>
                  &nbsp; : &nbsp;
                  <el-form-item>
                    <el-input />
                  </el-form-item>
                </el-row>
              </div>
            </el-form-item>
          </template>
        </template>
      </template>
    </el-form>
    <div v-if="type == 'sink' && sinkTip">
      <el-divider />
      <h4>Tip</h4>
      <div v-html="sinkTip.note"></div>
      <label>Example: </label>
      <br />
      <template v-for="c in sinkTip.formatExamples">
        <code>{{ c }} </code> <br />
      </template>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref, watch } from 'vue';
import * as ruleSchema from '@/components/rule/rule-schema'

const model = defineModel()
const props = defineProps({
  type: 'source' | 'sink' | 'connector',
  isNew: Boolean,
  schema: Object,
  optionsSchema: Object,
  config: Object,
})

const sinkTip = ref("")

const formEl = ref()

let tmpConnSchema = props.schema['connector']
let tmpOptSchema = props.schema['options']
let typeChanged = false
const fillOptions = () => {
  const opt = props.optionsSchema[model.value.type]
  if (props.schema['options'].items.length == 0) {
    delete props.schema['options']
  } else {
    tmpOptSchema.items = opt
    props.schema['options'] = tmpOptSchema
  }

  if (model.value.type == 'embed-mqtt') {
    props.schema['connector'] = undefined
  } else if (tmpConnSchema) {
    if (props.config.connectors) {
      const connOpt = props.config.connectors.filter(c => c.type == model.value.type)
        .map(c => ({ value: c.name, label: c.name }))
      tmpConnSchema['options'] = connOpt
    }
    props.schema['connector'] = tmpConnSchema
  }

  if (props.type == 'sink') {
    sinkTip.value = ruleSchema.sinkTips[model.value.type]
  }

  // clear options value when type changed
  if (typeChanged) {
    model.value.options = {}
  }
}

onMounted(() => {
  fillOptions()
})
watch(() => model.value.type, (n, o) => {
  typeChanged = true
  fillOptions()
})

defineExpose({
  validate: async () => {
    let result = { valid: false, fields: [] }
    if (!formEl.value) return result

    await formEl.value.validate((valid, fields) => {
      if (valid) {
        console.log('submit!')
        result.valid = valid
      } else {
        console.log('error submit!', fields)
        result.valid = valid
        result.fields = fields
      }
    })
    return result
  }
})

</script>

<style lang="scss" scoped></style>
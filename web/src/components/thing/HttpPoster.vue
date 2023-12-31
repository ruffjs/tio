<template>
  <el-drawer
    :model-value="!!code"
    :title="`${api ? api.name : 'HTTP Poster'}`"
    :modal="false"
    size="max(32vw, 570px)"
    class="http-poster"
    modal-class="http-poster-mask"
    append-to-body
    @close="emit('close')"
  >
    <template #footer>
      <div style="flex: auto">
        <el-button
          :disabled="hasJSONError"
          :loading="submitting"
          type="primary"
          @click="handleSubmit"
          >Submit</el-button
        >
      </div>
    </template>
    <el-form
      v-loading="
        submitting
          ? {
              text: 'Submitting...',
            }
          : false
      "
      :model="form"
      :rules="rules"
      ref="formRef"
      class="http-poster-form"
    >
      <el-row :gutter="10">
        <el-col :span="23">
          <el-form-item :label-width="formLabelWidth" label="API" prop="api">
            <el-input v-model="form.api" disabled size="small" /></el-form-item
        ></el-col>
        <el-col :span="1">
          <el-icon @click="handleOpenDoc"><Link /></el-icon>
        </el-col>
        <el-col :span="23">
          <el-form-item :label-width="formLabelWidth" label="METHOD" prop="method">
            <el-select v-model="form.method" disabled size="small">
              <el-option label="POST" value="post" />
              <el-option label="GET" value="get" />
              <el-option label="PUT" value="put" />
              <el-option label="DELETE" value="delete" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="1"></el-col>
        <el-col :span="23">
          <el-form-item :label-width="formLabelWidth" label="Url" prop="url">
            <el-input v-model="form.url" disabled size="small" />
          </el-form-item>
        </el-col>
        <el-col :span="1"></el-col>
        <el-col :span="23">
          <el-form-item :label-width="formLabelWidth" label="Thing Id" prop="id">
            <el-input v-model="form.id" :disabled="Boolean(payload)" size="small" />
          </el-form-item>
        </el-col>
        <el-col :span="1"></el-col>
        <el-col :span="23">
          <el-form-item
            v-for="param in params"
            :label-width="formLabelWidth"
            :label="param.label"
            :prop="param.key"
            :key="param.key"
          >
            <el-input v-model="form[param.key]" size="small" />
          </el-form-item>
        </el-col>
        <el-col :span="1"></el-col>
        <el-col :span="23">
          <el-form-item :label-width="formLabelWidth" label="Headers" prop="headers">
            <KeyValueEditor
              title=""
              v-model="form.headers"
              :disabled="Boolean(payload)"
              style="margin-bottom: 10px"
            />
          </el-form-item>
        </el-col>
        <el-col :span="1"></el-col>
        <el-col :span="23">
          <el-form-item :label-width="formLabelWidth" label="Body" prop="body">
            <JSONEditor
              v-model="form.body"
              v-model:has-error="hasJSONError"
              :read-only="Boolean(payload)"
              class="http-poster-body-json"
              :style="{ opacity: Boolean(payload) ? 0.6 : 1 }"
            />
          </el-form-item> </el-col
        ><el-col :span="1"></el-col>
      </el-row>
    </el-form>
    <el-card
      header="Response"
      shadow="never"
      :class="['http-poster-res', isError ? 'is-error' : '']"
    >
      <JSONEditor
        :model-value="result"
        read-only
        mode="tree"
        class="http-poster-resp-json"
      />
    </el-card>
  </el-drawer>
</template>

<script setup>
import { onMounted, reactive, ref, shallowRef, watch } from "vue";
import { shadowApis } from "@/configs/thing";
import KeyValueEditor from "@/components/common/KeyValueEditor.vue";
import { request } from "@/apis";
import { notifyThingStateChange, TSCE_HTTP } from "@/utils/event";
import JSONEditor from "../common/JSONEditor.vue";

const formLabelWidth = "110px";
const defaultRes = JSON.stringify({
  code: 0,
  message: "",
  data: "",
});

const emit = defineEmits(["close", "done"]);
const props = defineProps({
  code: {
    type: String,
    requried: true,
  },
  thingId: {
    type: String,
    requried: true,
  },
  payload: {
    type: Object,
  },
});
const submitting = ref(false);
const hasJSONError = ref(false);
const api = shallowRef(null);
const params = ref([]);
const formRef = ref();
const form = reactive({
  method: "",
  api: "",
  url: "",
  id: "",
  headers: { "Content-Type": "application/json" },
  body: "",
});
const rules = reactive({
  method: [{ required: true, message: "Please select or input" }],
  // api: [{ required: true, message: "Please input" }],
  url: [{ required: true, message: "Please input" }],
  id: [{ required: true, message: "Please input" }],
});
const isError = ref(false);
const result = ref(defaultRes);

const handleOpenDoc = () => window.open(api.value.link, "_blank");

const handleSubmit = async () => {
  if (!formRef.value) return;
  try {
    const valid = await formRef.value.validate();
    if (valid) {
      submitting.value = true;
      isError.value = false;
      result.value = defaultRes;
      const { url, method, body, headers } = form;
      const res = await request({
        url,
        method,
        headers,
        data: body,
      });
      result.value = JSON.stringify(res, null, 4);
      notifyThingStateChange(props.thingId, TSCE_HTTP, {
        url,
        method,
      });
      emit("done");
      // emit("close");
    }
  } catch (error) {
    console.error("error", error);
    isError.value = true;
    result.value = JSON.stringify({ error });
  } finally {
    submitting.value = false;
  }
};

watch(
  props,
  () => {
    api.value = shadowApis[props.code] || null;
    const _params = [];
    if (api.value) {
      form.method = api.value.method;
      form.api = api.value.url;
      form.id = props.thingId;
      form.url = api.value.urlResolver(form);
      form.body = JSON.stringify(
        props.payload || api.value.payloadResolver(form),
        null,
        4
      );
      api.value.params.forEach((param) => {
        form[param.key] = "";
        _params.push(param);
        if (param.required) {
          rules[param.key] = [{ required: true, message: "Please input" }];
        } else {
          delete rules[param.key];
        }
      });
    }
    params.value = _params;
    result.value = defaultRes;
  },
  { deep: true, immediate: true }
);

watch(
  form,
  () => {
    form.url = api.value?.urlResolver(form);
  },
  { deep: true }
);
</script>

<style scoped lang="scss">
.http-poster {
  .http-poster-form.el-form {
    width: 100%;
    margin-bottom: 20px;
    padding-top: 20px;
    border-radius: 4px;
    border: 1px solid var(--el-border-color);

    .el-form-item {
      margin-bottom: 13px;
    }

    .el-col-1 {
      padding-top: 5px;
      margin-left: -5px;
      text-align: left;
    }
    .el-col-23 {
      .el-select,
      .el-input-number {
        width: 100%;
      }
    }
  }
  .http-poster-res {
    width: 100%;

    &.is-error {
      color: red;
    }
  }
}
</style>

<style lang="scss">
.http-poster-mask {
  width: max(32vw, 570px);
  height: 100vh;
  inset: unset !important;
  right: 0 !important;
  .http-poster {
    .el-drawer__header {
      margin-bottom: 20px;
      .el-drawer__title {
        font-weight: 700;
      }
    }
    .el-drawer__body {
      padding: 0 var(--el-drawer-padding-primary) 10px;
    }
    .el-form {
      .el-form-item {
        .el-form-item__label {
          font-size: 12px;
        }
        .el-form-item__error {
          margin-top: -2px;
          padding-top: 0;
        }
      }
      .el-col-1 {
        .el-icon {
          cursor: pointer;
        }
      }
      .el-col-23 {
        .el-input-number {
          .el-input__inner {
            text-align: left;
          }
        }
        .http-poster-body-json {
          .jse-main {
            position: relative;
            height: auto;
            min-height: 172px;
            max-height: 244px;
          }
        }
      }
    }
    .http-poster-res {
      .el-card__header {
        padding: 10px var(--el-card-padding);
      }
      .el-card__body {
        padding: 5px 0;
        .http-poster-resp-json {
          .jse-main {
            .jse-tree-mode {
              border: none;
              .jse-contents {
                border: none;
              }
            }
          }
        }
      }
    }
  }
}
</style>

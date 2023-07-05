<template>
  <JsonEditorVue
    v-model="object"
    :mode="mode"
    :main-menu-bar="false"
    :navigation-bar="readOnly"
    :status-bar="false"
    :read-only="readOnly"
    class="json-editor-n-viewer"
    @change="onChange"
  />
</template>

<script setup>
import { ref, watch } from "vue";
import "vanilla-jsoneditor/themes/jse-theme-dark.css";
import JsonEditorVue from "json-editor-vue";

const error = ref(null);
const object = ref({});
const content = ref("{}");
const emit = defineEmits(["update:modelValue", "update:hasError"]);
const props = defineProps({
  mode: {
    type: String,
    default: "text",
  },
  modelValue: {
    type: String,
    default: "{}",
  },
  hasError: {
    type: Boolean,
    default: false,
  },
  readOnly: Boolean,
});

watch(
  () => props.modelValue,
  (model) => {
    // if (typeof model === "undefined") {
    //   const newContent = JSON.stringify("");
    //   if (content.value !== newContent) {
    //     object.value = "";
    //     content.value = newContent;
    //   }
    // } else if (typeof model === "object" || typeof model === "number") {
    //   const newContent = JSON.stringify(model);
    //   if (content.value !== newContent) {
    //     object.value = model;
    //     content.value = newContent;
    //   }
    // } else
    if (typeof model === "string") {
      try {
        // console.log(model, content.value);
        const newObject = JSON.parse(model || '""');
        const newContent = JSON.stringify(newObject);
        if (content.value !== newContent) {
          object.value = newObject;
          content.value = newContent;
        }
        emit("update:hasError", false);
      } catch (error) {
        object.value = model;
        content.value = model;
        emit("update:hasError", true);
      }
    } else {
      object.value = {};
      content.value = JSON.stringify({});
      emit("update:hasError", false);
    }
  },
  {
    immediate: true,
  }
);

const checkData = () => {
  if (error.value) {
    return [false, null];
  }
  if (typeof content.value === "string") {
    return [true, content.value];
  }
  return [true, JSON.stringify(content.value, null, 2)];
};

const onChange = (updatedContent, previousContent, { contentErrors }) => {
  if ((error.value = contentErrors)) return emit("update:hasError", true);
  // console.log(updatedContent, previousContent);
  const obj =
    updatedContent.json !== undefined
      ? updatedContent.json
      : JSON.parse(updatedContent.text);

  content.value = JSON.stringify(obj);
  emit("update:hasError", false);
  emit("update:modelValue", JSON.stringify(obj || {}, null, 2));
};

// const onBlur = () => {};

defineExpose({
  validate: () => {
    if (error.value) {
      throw error.value;
    }
  },
});
</script>

<style lang="scss">
.json-editor-n-viewer {
  position: relative;
  width: 100%;
  height: 100%;

  .jse-main {
    border-radius: 4px;
    .jse-text-mode,
    .jse-tree-mode {
      border-radius: 4px;
      .jse-contents {
        border-radius: 4px;
      }
    }
  }
  &.jse-theme-dark {
    .jse-main {
      border-radius: 4px;
      overflow: hidden;
    }
  }
}
</style>

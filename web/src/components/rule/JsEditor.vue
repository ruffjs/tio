<template>
  <div class="edit-container">
    <textarea ref="editRef" v-model="model"></textarea>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, ref, shallowRef, watch } from "vue";
import codemirror from "codemirror/lib/codemirror";
import "codemirror/addon/edit/matchbrackets";
import "codemirror/mode/javascript/javascript";
import "codemirror/addon/hint/javascript-hint";

const model = defineModel()

const editRef = ref();
const editor = shallowRef();
const emit = defineEmits(["update:modelValue"]);

const clear = () => {
  // 清空编辑器内容
  editor.value?.setValue("");
};

const resize = () => {
  editor.value?.refresh();
};

const createEditor = async () => {
  // MIME types defined: text/javascript, application/javascript, application/x-javascript, text/ecmascript, application/ecmascript, application/json, application/x-json, application/manifest+json, application/ld+json, text/typescript, application/typescript.
  const mime = "application/x-javascript";
  editor.value = codemirror.fromTextArea(editRef.value, {
    value: model,
    mode: mime,
    indentWithTabs: true,
    smartIndent: true,
    lineNumbers: true,
    hintOptions: {
      completeSingle: false,
    },
    matchBrackets: true,
    cursorHeight: 1,
    lineWrapping: true,
    // readOnly: false,
    extraKeys: { Ctrl: "autocomplete" },
  });
  editor.value.on("inputRead", () => {
    // editor.value.showHint();
    emit("update:modelValue", editor.value.getValue() || "");
  });
  editor.value.on("blur", () => { });
};

defineExpose({
  syncValue: () => {
    emit("update:modelValue", editor.value.getValue() || "");
  },
  syncValueTrim: () => {
    emit("update:modelValue", editor.value.getValue().trim() || "");
  },
});

watch(
  () => model,
  (value) => {
    editor.value?.setValue(value.trim());
  }
);

onMounted(async () => {
  await nextTick();
  if (!editor.value) createEditor();
});

</script>

<style scoped lang="scss">
.edit-container {
  width: 100%;
  height: 100%;
  line-height: 20px;
  font-size: medium;

  textarea {
    width: 100%;
    height: 100%;
  }
}
</style>

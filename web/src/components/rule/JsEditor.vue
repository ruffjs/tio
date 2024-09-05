<template>
  <div>
    <div class="edit-container">
      <textarea ref="editRef" v-model="model"></textarea>
    </div>

    <el-row>
      <el-tooltip content="Auto height for code editor for see all code" placement="top">
        <el-checkbox link type="primary" @change="onAutoHeightCheck">auto height</el-checkbox>
      </el-tooltip>
    </el-row>
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
  if (!editRef.value) {
    console.debug("editRef is null", editRef.value)
    return
  }
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
  console.debug("editor inited")
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

const onAutoHeightCheck = (b) => {
  if (b) {
    editor.value.setSize('100%', 'auto')
  } else {
    editor.value.setSize('100%', '300px')
  }
}

</script>

<style lang="scss" scoped>
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
<style lang="scss">
.CodeMirror {
  border: 1px #eaeaea solid;
  border-radius: 3px;
}
</style>

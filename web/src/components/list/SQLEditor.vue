<template>
  <div class="sql-edit-container">
    <textarea ref="editRef" :model-value="modelValue"></textarea>
  </div>
</template>

<script setup>
import { nextTick, onMounted, ref, shallowRef, watch } from "vue";
import codemirror from "codemirror/lib/codemirror";
import "codemirror/theme/ambiance.css";
import "codemirror/lib/codemirror.css";
import "codemirror/addon/hint/show-hint.css";
import "codemirror/addon/edit/matchbrackets";
import "codemirror/addon/selection/active-line";
import "codemirror/mode/sql/sql";
import "codemirror/addon/hint/show-hint";
import "codemirror/addon/hint/sql-hint";
import useTheme from "@/reactives/useTheme";

const editRef = ref();
const editor = shallowRef();
const { effectiveTheme } = useTheme();
const emit = defineEmits(["update:modelValue", "update:focused", "update:blured", "submit"]);
const props = defineProps({
  focused: Boolean,
  modelValue: {
    type: String,
    default: "",
  },
  readOnly: {
    type: [Boolean, String],
  },
});

const clear = () => {
  // 清空SQL编辑器内容
  editor.value?.setValue("");
};

const resize = () => {
  editor.value?.refresh();
};

const createEditor = async () => {
  const mime = "text/x-sql"; // 'text/'为编辑器语言前缀; 支持：javascript、XML/HTML、java、SQL、Python等（详细请查询官网）
  editor.value = codemirror.fromTextArea(editRef.value, {
    value: props.modelValue,
    mode: mime,
    indentWithTabs: true,
    smartIndent: true,
    lineNumbers: false,
    hintOptions: {
      completeSingle: false,
    },
    matchBrackets: true,
    cursorHeight: 1,
    lineWrapping: true,
    readOnly: props.readOnly,
    theme: effectiveTheme.value === "dark" ? "ambiance" : "default",
    // extraKeys: { Ctrl: "autocomplete" },
  });
  editor.value.on("inputRead", () => {
    editor.value.showHint();
  });
  editor.value.on("focus", () => {
    emit("update:blured", false);
    emit("update:focused", true);
  });
  editor.value.on("blur", () => {});
  editor.value.setValue(props.modelValue.trim());
  editor.value.on('keydown', (cm, event) => {
    if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
      console.log('Shift + Enter pressed');
      emit('submit');
    }
  })
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
  () => props.modelValue,
  (value) => {
    editor.value?.setValue(value.trim());
  }
);

watch(effectiveTheme, (value) => {
  editor.value?.setOption("theme", value === "dark" ? "ambiance" : "default");
  editor.value?.refresh();
});

onMounted(async () => {
  await nextTick();
  if (!editor.value) createEditor();
});

</script>

<style scoped lang="scss">
.sql-edit-container {
  width: 100%;
  height: 100%;
  textarea {
    width: 100%;
    height: 100%;
  }
}
</style>

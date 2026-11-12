<template>
  <div ref="editRef" class="tio-code-editor sql-edit-container"></div>
</template>

<script setup>
import { computed, watch } from "vue";
import { Compartment, EditorState } from "@codemirror/state";
import { EditorView, keymap } from "@codemirror/view";
import { sql } from "@codemirror/lang-sql";
import useCodeEditor from "@/reactives/useCodeEditor";

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

const readOnly = new Compartment();
const readOnlyExtensions = () => [
  EditorState.readOnly.of(!!props.readOnly),
  EditorView.editable.of(props.readOnly !== "nocursor"),
];
const { editRef, editor } = useCodeEditor(computed(() => props.modelValue.trim()), [
  sql(),
  readOnly.of(readOnlyExtensions()),
  EditorView.domEventHandlers({
    focus: () => {
      emit("update:blured", false);
      emit("update:focused", true);
    },
  }),
  keymap.of(["Ctrl-Enter", "Meta-Enter"].map((key) => ({
    key,
    run: () => {
      emit("submit");
      return true;
    },
    stopPropagation: true,
  }))),
]);

watch(() => props.readOnly, () => {
  editor.value?.dispatch({ effects: readOnly.reconfigure(readOnlyExtensions()) });
});

defineExpose({
  syncValue: () => emit("update:modelValue", editor.value.state.doc.toString()),
  syncValueTrim: () => emit("update:modelValue", editor.value.state.doc.toString().trim()),
});
</script>

<style scoped lang="scss">
.sql-edit-container {
  width: 100%;
  height: 100%;
}
</style>

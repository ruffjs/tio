<template>
  <div>
    <div ref="editRef" class="tio-code-editor edit-container" :class="{ 'auto-height': autoHeight }"></div>

    <el-row>
      <el-tooltip content="Auto height for code editor for see all code" placement="top">
        <el-checkbox v-model="autoHeight" link type="primary">auto height</el-checkbox>
      </el-tooltip>
    </el-row>
  </div>
</template>

<script setup>
import { ref } from "vue";
import { EditorView, lineNumbers } from "@codemirror/view";
import { javascript } from "@codemirror/lang-javascript";
import useCodeEditor from "@/reactives/useCodeEditor";

const model = defineModel();
const autoHeight = ref(false);
const { editRef, editor } = useCodeEditor(model, [
  javascript(),
  lineNumbers(),
  EditorView.updateListener.of((update) => {
    if (update.docChanged) model.value = update.state.doc.toString();
  }),
]);

const emit = defineEmits(["update:modelValue"]);
defineExpose({
  syncValue: () => emit("update:modelValue", editor.value.state.doc.toString()),
  syncValueTrim: () => emit("update:modelValue", editor.value.state.doc.toString().trim()),
});
</script>

<style lang="scss" scoped>
.edit-container {
  width: 100%;
  line-height: 20px;
  font-size: medium;

  :deep(.cm-editor) {
    height: 300px;
  }

  &.auto-height :deep(.cm-editor) {
    height: auto;
  }
}
</style>

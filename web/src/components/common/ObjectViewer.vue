<template>
  <el-dialog
    :model-value="visible"
    :title="type"
    @close="emit('close')"
    width="70vw"
    class="object-view-dialog"
    append-to-body
  >
    <div class="object-view-contents">
      <JSONEditor
        :mode="asTree ? 'tree' : 'text'"
        :model-value="content"
        read-only
        class="object-view-main jse-theme-dark"
      />
    </div>
  </el-dialog>
</template>

<script setup>
import { watch, ref } from "vue";
import JSONEditor from "./JSONEditor.vue";

const emit = defineEmits(["close"]);
const props = defineProps({
  visible: Boolean,
  asTree: Boolean,
  data: Object,
  type: String,
});

const content = ref("");
watch(
  props,
  () => {
    if (props.visible && props.type && props.data) {
      content.value = JSON.stringify(props.data, null, 2);
    }
  },
  { deep: true, immediate: true }
);
</script>
<style scoped lang="scss"></style>

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
      <textarea readonly class="object-view-main">{{ content }}</textarea>
    </div>
  </el-dialog>
</template>

<script setup>
import { watch, ref } from "vue";

const emit = defineEmits(["close"]);
const props = defineProps({
  visible: Boolean,
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

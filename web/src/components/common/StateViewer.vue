<template>
  <el-dialog
    :model-value="visible"
    :title="title"
    @close="emit('close')"
    width="max(70vw, 840px)"
    class="object-view-dialog"
    append-to-body
  >
    <template #footer>
      <span class="object-view-dialog-footer">
        <template v-if="type !== 'both'">
          <el-button v-if="comparing" @click="comparing = false">
            {{ type === "reported" ? "Reported" : "Desired" }} Only
          </el-button>
          <el-button v-else="comparing" @click="comparing = true"> Compare </el-button>
        </template>
        <el-button type="primary" @click="emit('close')"> Close </el-button>
      </span>
    </template>
    <div class="object-view-contents">
      <textarea readonly class="object-view-left">{{ content1 }}</textarea>
      <textarea v-if="comparing" readonly class="object-view-right">{{
        content2
      }}</textarea>
    </div>
  </el-dialog>
</template>

<script setup>
import { computed, watch, ref } from "vue";

const emit = defineEmits(["close"]);
const props = defineProps({
  visible: Boolean,
  data: Object,
  type: String,
});

const comparing = ref(false);
const content1 = ref("");
const content2 = ref("");
const title = computed(
  () =>
    `View State (${
      props.type === "both"
        ? "Both Desired & Reported"
        : props.type === "reported"
        ? "Reported"
        : "Desired"
    })`
);

watch(
  props,
  () => {
    if (props.visible && props.type && props.data) {
      if (props.type === "reported") {
        content1.value = JSON.stringify(props.data.reported, null, 2);
        content2.value = JSON.stringify(props.data.desired, null, 2);
      } else {
        if (props.type === "both") {
          comparing.value = true;
        }
        content1.value = JSON.stringify(props.data.desired, null, 2);
        content2.value = JSON.stringify(props.data.reported, null, 2);
      }
    }
  },
  { deep: true, immediate: true }
);
</script>
<style lang="scss"></style>

<style scoped lang="scss"></style>

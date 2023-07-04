<template>
  <div class="key-value-pass">
    <code>{{ computedValue }}</code>
    <el-icon v-if="visible" @click="visible = false"><View /></el-icon>
    <el-icon v-else @click="visible = true"><Hide /></el-icon>
  </div>
</template>

<script setup>
import { computed, ref } from "vue";

const props = defineProps({
  value: {
    type: String,
    default: "",
  },
});
const visible = ref(false);

const computedValue = computed(() => {
  if (visible.value) {
    return props.value;
  }
  return new Array(props.value.length).fill("*").join("");
});
</script>

<style scoped lang="scss">
.key-value-pass {
  display: flex;
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
  gap: 6px;

  width: 100%;
  .el-icon {
    color: gray;
    cursor: pointer;
  }
}
</style>

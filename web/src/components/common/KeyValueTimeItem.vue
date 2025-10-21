<template>
  <div class="key-value-time">
    <div class="key-value-time-block">{{ computedData }}</div>
  </div>
</template>

<script setup>
import dayjs from "dayjs";
import { computed } from "vue";

const props = defineProps({
  time: [String, Number],
});

const computedData = computed(() => {
  const time = dayjs(props.time);
  return time.format("YYYY-MM-DD HH:mm:ss");
});
</script>

<style scoped lang="scss">
.key-value-time {
  font-size: 12px;
  text-align: right;
  margin: 5px 0;
  .key-value-time-block {
    height: 15px;
    line-height: 15px;
  }
}
</style>

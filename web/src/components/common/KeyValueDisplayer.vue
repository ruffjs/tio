<template>
  <div v-for="field in fields" class="key-value-item">
    <div class="key-value-label">{{ field.label }}</div>
    <div class="key-value-value">
      <div v-if="field.type === 'boolean'" class="key-value-boolean">
        <el-icon v-if="data[field.key]" color="var(--el-color-success)" size="24"
          ><Open
        /></el-icon>
        <el-icon v-else color="var(--el-color-info)"><TurnOff /></el-icon>
      </div>
      <div v-else-if="field.type === 'tag'" class="key-value-tag">
        <el-tag size="small">{{ data[field.key] }}</el-tag>
      </div>
      <div v-else-if="field.type === 'password'" class="key-value-password">
        <KeyValuePassItem :value="data[field.key]" />
      </div>
      <div v-else-if="field.type === 'time'" class="key-value-time">
        <KeyValueTimeItem :time="data[field.key]" />
      </div>
      <div v-else class="key-value-string">{{ data[field.key] }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import KeyValuePassItem from "./KeyValuePassItem.vue";
import KeyValueTimeItem from "./KeyValueTimeItem.vue";

defineProps({
  fields: {
    type: Array as () => Array<{
      key: string;
      label: string;
      type: "string" | "boolean" | "tag" | "password" | "time";
    }>,
    required: true,
  },
  data: {
    type: Object as () => Record<string, any>,
    required: true,
  },
});
</script>

<style scoped lang="scss">
.key-value-item {
  display: flex;
  flex-direction: row;
  justify-content: space-between;
  align-items: center;

  width: 100%;
  min-height: 28px;
  margin-top: 2px;
  padding: 1px 5px;
  border: solid 1px rgba($color: #000000, $alpha: 0.1);
  border-radius: 2px;

  &:first-child {
    margin-top: 0;
    border-radius: 5px 5px 2px 2px;
  }
  &:last-child {
    border-radius: 2px 2px 5px 5px;
  }

  .key-value-label {
    margin-right: 10px;
    font-size: 12px;
    font-weight: 700;
    color: #555;
  }
  .key-value-value {
    max-width: 140px;
    font-size: 12px;
    text-align: right;
    .key-value-string {
      line-height: 24px;
    }
    .key-value-boolean {
      height: 24px;
      font-size: 0;
    }
  }
}
</style>

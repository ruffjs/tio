<template>
  <el-button link type="danger" size="small" v-if="disabled" disabled>
    Delete Thing
  </el-button>
  <el-popconfirm
    :title="title"
    :confirm-button-text="confirmButtonText"
    :cancel-button-text="cancelButtonText"
    cancel-button-type="default"
    v-else
    @cancel="emit('cancel')"
    @confirm="emit('confirm')"
  >
    <template #reference>
      <el-button link type="danger" size="small"> Delete </el-button>
    </template>
  </el-popconfirm>
</template>

<script setup>
const emit = defineEmits(["confirm", "cancel"]);
defineProps({
  title: String,
  confirmButtonText: {
    type: String,
    default: "Yes",
  },
  cancelButtonText: {
    type: String,
    default: "No",
  },
  disabled: Boolean,
});
</script>

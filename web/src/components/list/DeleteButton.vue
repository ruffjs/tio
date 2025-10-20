<template>
  <el-button link type="danger" size="small" v-if="disabled" disabled>
    {{ $t('common.delete') }}
  </el-button>
  <el-popconfirm v-else :title="title" cancel-button-type="default" @cancel="emit('cancel')" @confirm="emit('confirm')">
    <template #reference>
      <el-button link type="danger" size="small"> {{ $t('common.delete') }} </el-button>
    </template>
  </el-popconfirm>
</template>

<script setup>
const emit = defineEmits(["confirm", "cancel"]);
defineProps({
  title: String,
  disabled: Boolean,
});
</script>

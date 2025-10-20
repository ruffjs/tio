<template>
  <el-tooltip
    :content="isToolAreaMaxSize ? $t('common.resizeToDefaultHeight') : $t('common.resizeToMaximumHeight')"
    placement="top"
  >
    <el-button
      :icon="isToolAreaMaxSize ? 'ArrowDownBold' : 'ArrowUpBold'"
      type="info"
      size="small"
      plain
      @click="toggleToolAreaSize"
    />
  </el-tooltip>
</template>

<script setup>
import useLayout from "@/reactives/useLayout";

const { isToolAreaMaxSize, toggleToolAreaSize } = useLayout();
</script>

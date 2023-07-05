<template>
  <el-tooltip
    :content="isToolAreaMaxSize ? 'resize to default height' : 'resize to maximum height'"
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

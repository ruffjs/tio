<template>
  <div class="tool-box" :style="{ height: activeToolHeight }">
    <div class="tool-tabs">
      <div
        v-for="tool in tools"
        :class="{
          'tool-tab-item': true,
          active: tool.key === activeToolKey,
        }"
        :key="tool.key"
        @click="switchActiveTool(tool)"
      >
        <el-icon><Component :is="tool.icon" /></el-icon> <span>{{ tool.name }}</span>
      </div>
    </div>
    <div class="tools-container">
      <div
        class="tool-container"
        :style="{ top: activeToolKey === 'mqtt' ? '0%' : '200%' }"
      >
        <MqttClients />
      </div>
      <div
        class="tool-container"
        :style="{ top: activeToolKey === 'logs' ? '0%' : '200%' }"
      >
        <HttpLogs />
      </div>
      <div
        class="tool-container"
        :style="{ top: activeToolKey === 'code' ? '0%' : '200%' }"
      >
        <CodeSnippet />
      </div>
    </div>
  </div>
</template>

<script setup>
import useLayout from "@/reactives/useLayout";
import MqttClients from "@/components/tools/MqttClients.vue";
import HttpLogs from "@/components/tools/HttpLogs.vue";
import CodeSnippet from "@/components/tools/CodeSnippet.vue";
import { tools } from "@/configs/tool";

const { activeToolKey, activeToolHeight, switchActiveTool } = useLayout();
</script>

<style scoped lang="scss">
.tool-box {
  position: relative;
  border-top: 1px solid var(--tio-line);
  background: var(--tio-surface-solid);
  box-shadow: none;
  transition: height ease-in-out 0.1s;

  .tool-tabs {
    display: flex;
    justify-content: start;
    align-items: center;
    z-index: 10;
  }
  .tool-tabs {
    position: absolute;
    bottom: 0;
    left: 0;
    width: 100%;
    height: 30px;
    min-width: 0;
    border-top: 1px solid var(--tio-line);
    background: var(--tio-surface-solid);
    color: var(--tio-text);
    backdrop-filter: none;

    user-select: none;

    .tool-tab-item {
      display: flex;
      justify-content: start;
      align-items: center;
      padding: 0 10px 1px;
      border-top: 2px solid transparent;
      line-height: 28px;
      font-size: 12px;
      cursor: pointer;
      color: var(--tio-muted);
      transition: background-color 0.18s ease, color 0.18s ease;

      .el-icon {
        font-size: 14px;
        margin-right: 4px;
      }

      &:hover {
        background: var(--tio-surface-soft);
        color: var(--tio-text-strong);
      }

      &.active {
        background: var(--tio-accent-soft);
        border-top-color: var(--tio-accent);
        color: var(--tio-text-strong);
      }

      &.active:hover {
        background: var(--tio-accent-soft);
      }
    }
  }

  .tools-container {
    position: relative;
    width: 100%;
    height: calc(100% - 30px);
    overflow: hidden;
    .tool-container {
      position: absolute;
      width: 100%;
      height: 100%;
      top: 200%;
    }
  }
}

:global(:root[data-theme="light"]) .tool-box {
  box-shadow: none;

  .tool-tabs {
    background: var(--tio-surface-solid);
  }
}
</style>

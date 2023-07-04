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
        :style="{ top: activeToolKey === 'mqtt' ? '0%' : '100%' }"
      >
        <MqttClients />
      </div>
      <div
        class="tool-container"
        :style="{ top: activeToolKey === 'logs' ? '0%' : '100%' }"
      >
        <HttpLogs />
      </div>
      <div
        class="tool-container"
        :style="{ top: activeToolKey === 'code' ? '0%' : '100%' }"
      >
        <CodeSnippet />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from "vue";
import useLayout from "@/reactives/useLayout";
import MqttClients from "@/components/tools/MqttClients.vue";
import HttpLogs from "@/components/tools/HttpLogs.vue";
import CodeSnippet from "@/components/tools/CodeSnippet.vue";
import { tools } from "@/configs/tool";

const { activeToolKey, activeToolConf, activeToolHeight, switchActiveTool } = useLayout();
</script>

<style scoped lang="scss">
.tool-box {
  background-color: #fafafa;
  box-shadow: 0 -1px 2px rgba($color: #000000, $alpha: 0.05);
  transition: height ease-in-out 0.1s;

  .tool-tabs {
    display: flex;
    justify-content: start;
    align-items: center;
    z-index: 10;
  }
  .tool-tabs {
    position: fixed;
    bottom: 0;
    left: 0;
    width: 100vw;
    height: 30px;
    min-width: 1080px;
    border-top: solid 1px rgba($color: #000000, $alpha: 0.1);
    background-color: #f2f2f2;

    user-select: none;

    .tool-tab-item {
      display: flex;
      justify-content: start;
      align-items: center;
      padding: 0 10px 1px;
      line-height: 28px;
      font-size: 12px;
      cursor: pointer;
      .el-icon {
        font-size: 14px;
        margin-right: 4px;
      }

      &:hover {
        background-color: rgba(#000000, 0.1);
      }
      &.active {
        background-color: rgba(#000000, 0.2);
      }
      &.active:hover {
        background-color: rgba(#000000, 0.25);
      }
    }
  }

  .tools-container {
    width: 100%;
    height: calc(100% - 30px);
    .tool-container {
      position: absolute;
      width: 100%;
      height: 100%;
      top: 100%;
      left: 0;
      // transition: top ease-in-out 0.1s;
    }
  }
}
</style>

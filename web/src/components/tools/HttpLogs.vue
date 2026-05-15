<template>
  <div class="http-logs-panel">
    <div class="http-logs-header">
      <div class="http-logs-title">HTTP Request Logs</div>
      <div class="http-logs-opts">
        <SwitchSizeButton />
        <el-pagination
          small
          layout="total,prev, pager, next"
          :hide-on-single-page="false"
          v-model:current-page="currentPage"
          :background-color="false"
          :page-size="pageSize"
          :total="all.length"
        />
      </div>
    </div>
    <div v-if="all.length" class="http-logs-list">
      <div v-for="log in logs" :class="['http-logs-item', log.error ? 'has-error' : '']">
        <div class="http-logs-desc">
          <code class="http-logs-method">{{ log.req.method }}</code>
          <code class="http-logs-url">{{ log.req.url }}</code>
          <code class="http-logs-time">{{ log.time }}</code>
        </div>
        <div class="http-logs-payload">
          <el-button
            v-if="['post', 'put'].includes(log.req.method)"
            type="primary"
            size="small"
            icon="Upload"
            plain
            @click="viewObject(log.req.data, 'Request Body')"
            >Req Body</el-button
          >
          <el-button
            :type="log.error ? 'danger' : 'success'"
            size="small"
            icon="Download"
            plain
            @click="viewObject(log.res.data, 'Response Data')"
            >Res Data</el-button
          >
          <el-button
            v-if="log.error"
            type="danger"
            size="small"
            icon="CircleClose"
            plain
            @click="viewObject(log.error, 'Error Detail')"
            >Err Detail</el-button
          >
        </div>
      </div>
    </div>
    <el-empty v-else description="No HTTP request logs" :image-size="60" />
  </div>
  <ObjectViewer
    :visible="!!objectToBeView"
    :data="objectToBeView"
    :type="titleOfViewer"
    @close="handleCloseViewer"
  />
</template>

<script setup>
import { computed, ref } from "vue";
import { useStore } from "vuex";
import SwitchSizeButton from "@/components/common/SwitchSizeButton.vue";
import ObjectViewer from "@/components/common/ObjectViewer.vue";
import useObjectViewer from "@/reactives/useObjectViewer";

const pageSize = 50;
const store = useStore();
const {
  objectToBeView,
  titleOfViewer,
  viewObject,
  handleCloseViewer,
} = useObjectViewer();
const currentPage = ref(1);
const all = computed(() => store.state.app.httpRequestLogs);
const logs = computed(() => {
  const start = pageSize * (currentPage.value - 1);
  const end = pageSize * currentPage.value;
  return all.value.slice(start, end);
});
</script>

<style scoped lang="scss">
.http-logs-panel {
  width: 100%;
  height: 100%;
  color: var(--tio-text);

  .http-logs-header {
    display: flex;
    flex-direction: row;
    justify-content: space-between;
    align-items: center;

    width: 100%;
    height: 32px;
    padding: 0 5px;
    background: var(--tio-surface-soft);
    border-top: 1px solid var(--tio-border);
    border-bottom: 1px solid var(--tio-border);

    .http-logs-title {
      line-height: 24px;
      font-size: 16px;
      font-weight: 700;
      color: var(--tio-muted);
      .active {
        color: var(--tio-text-strong);
      }
    }
    .http-logs-opts {
      display: flex;
      flex-direction: row-reverse;
      justify-content: start;
      align-items: center;
      height: 28px;
      padding: 2px;
      .el-pagination {
        --el-pagination-bg-color: transparent;
        --el-pagination-button-disabled-bg-color: transparent;
      }
    }
  }
  .http-logs-list {
    width: 100%;
    height: calc(100% - 32px);
    padding: 5px;
    overflow-x: hidden;
    overflow-y: auto;

    .http-logs-item {
      display: flex;
      flex-direction: row;
      justify-content: space-between;
      align-items: center;

      min-height: 25px;
      border-bottom: 1px dashed var(--tio-border);
      line-height: 24px;
      font-size: 12px;
      font-family: var(--tio-mono);

      .http-logs-time {
        color: var(--tio-muted);
        font-weight: 600;
      }
      .http-logs-method {
        color: var(--tio-text-strong);
        font-weight: 900;
        text-transform: uppercase;
      }
      .http-logs-url {
        margin: 0 10px;
        color: var(--tio-text);
        font-weight: 500;
      }
      .http-logs-payload {
        .el-button--small {
          width: 90px;
          --el-button-size: 18px;
          padding: 2px 10px;
        }
      }

      &.has-error {
        .http-logs-time,
        .http-logs-method,
        .http-logs-url,
        .http-logs-payload {
          font-weight: 900;
          color: var(--el-color-danger) !important;
        }
      }
    }
  }
}
</style>

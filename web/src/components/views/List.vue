<template>
  <div class="main-view">
    <div class="thing-detail" :style="{ height: route.params.thingId ? '100%' : '0%' }">
      <router-view></router-view>
    </div>
    <div v-show="showList" class="thing-list" :class="{ active }" v-loading="querying">
      <!-- 搜索区域 -->
      <div class="list-view-search">
        <div class="list-view-search-left">
          <div class="list-view-query-editor">
            <SQLEditor v-model="query" v-model:focused="focused" ref="sqlEditor" @submit="handleSearch" />
            <div class="list-view-query-link">
              <el-tooltip :content="$t('things.queryTips')" placement="top">
                <el-button icon="Link" size="small" circle @click="handleOpenDoc" />
              </el-tooltip>
            </div>
          </div>
        </div>
        <div class="list-view-search-right">
          <el-button type="primary" @click="handleSearch" :title="$t('things.queryShortcut')" :loading="querying" icon="Search">
            {{ $t('things.query') }}
          </el-button>
          <!-- <el-button v-if="active" @click="handleClear" size="small" icon="Refresh" text /> -->
          <el-button v-if="queryHistory.length > 0" @click="showHistory = !showHistory" text size="small" icon="Clock" :title="$t('common.history')" />
        </div>
      </div>

      <!-- 查询历史 -->
      <div v-if="showHistory" class="list-view-active-body">
        <div class="query-history" @click="showHistory = false">
          <div class="query-history-header">
            <h4>{{ $t('common.history') }}</h4>
            <div class="query-history-actions">
              <el-button text @click.stop="clearQueryHistory" size="small">
                <el-icon>
                  <Delete />
                </el-icon>
                {{ $t('common.clear') }}
              </el-button>
              <el-button text @click="showHistory = false">
                <el-icon>
                  <Close />
                </el-icon>
              </el-button>
            </div>
          </div>
          <div class="query-history-content">
            <div v-for="(historyQuery, index) in queryHistory" :key="index" class="query-history-item"
              @click.stop="selectFromHistory(historyQuery)">
              <div class="query-history-item-content">
                {{ historyQuery }}
              </div>
            </div>
            <div v-if="queryHistory.length === 0" class="query-history-empty">
              <el-text type="info">{{ $t('common.noHistory') }}</el-text>
            </div>
          </div>
        </div>
      </div>

      <!-- 主要内容区域 -->
      <div v-if="active && !showHistory" class="list-view-active-body">
        <!-- SQL 模板建议 -->
        <div v-if="focused" class="sql-editor-tpls" @click="focused = false">
          <div class="sql-editor-tpls-header">
            <h4>{{ $t('common.example') }}</h4>
            <el-button text @click="focused = false">
              <el-icon>
                <Close />
              </el-icon>
            </el-button>
          </div>
          <div class="sql-editor-tpls-content">
            <div v-for="(suggestion, index) in suggestions" :key="index" class="sql-editor-tpl-item"
              @click.stop="handleSelect(suggestion)">
              <div class="sql-editor-tpl-item-header">
                <el-tag size="small" type="primary">
                  {{ $t(suggestion.label) }}
                </el-tag>
                <el-tag v-if="suggestion.autoTrigger" size="small" type="success">
                  {{ $t('things.auto') }}
                </el-tag>
              </div>
              <div class="sql-editor-tpl-item-content">
                {{ suggestion.value }}
              </div>
            </div>
          </div>
        </div>

        <!-- 数据列表 -->
        <div v-else-if="total > 0" class="list-view-things">
          <ThingsList :items="list" :page-index="params.pageIndex" :page-size="params.pageSize" :total="total"
            :isStandard="isSelectAll" @page-index-change="handlePageIndexChange"
            @page-size-change="handlePageSizeChange" />
        </div>

        <!-- 空状态 -->
        <div v-else-if="empty" class="list-view-empty">
          <el-empty :image-size="120" :description="$t('things.queryTips')">
            <el-button type="primary" @click="handleClear">
              {{ $t('common.reset') }}
            </el-button>
          </el-empty>
        </div>
      </div>

      <!-- 非激活状态 -->
      <div v-else-if="!showHistory" class="list-view-inactive-body">
        <div v-if="error" class="list-view-error">
          <div class="list-view-error-header">
            <el-text type="danger">
              <el-icon>
                <Warning />
              </el-icon>
              {{ $t('common.error') }}
            </el-text>
            <el-button text @click="error = ''">
              <el-icon>
                <Close />
              </el-icon>
            </el-button>
          </div>
          <div class="list-view-error-content">
            <JSONEditor mode="tree" :model-value="error" read-only />
          </div>
        </div>
        <div v-else class="list-view-tips">
          <div class="list-view-tips-content">
            <el-icon size="48" color="#909399">
              <Search />
            </el-icon>
            <h3>{{ $t('things.queryTips') }}</h3>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: "List",
  inheritAttrs: false,
  customOptions: { title: "TIO Playground", zIndex: 0, actived: true },
};
</script>
<script setup>
import { ref, reactive, watch, computed, onMounted, onUnmounted, nextTick } from "vue";
import { useRoute } from "vue-router";
import { suggestions } from "@/configs/query";
import { queryShadows } from "@/apis";
import { TSCE_MQTO, TSCE_MQTT } from "@/utils/event";
import ThingsList from "@/components/list/ThingsList.vue";
import SQLEditor from "@/components/list/SQLEditor.vue";
import JSONEditor from "@/components/common/JSONEditor.vue";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import useThingEvent from "@/reactives/useThingEvent";

const defaultPageSize = 20;
const placeholder = suggestions[0].value;

const sqlEditor = ref();
const route = useRoute();
const { shadowListUpdateTag } = useThingsAndShadows();
const { onSomethingStatusChange } = useThingEvent();

const query = ref("SELECT * FROM shadow");
const querying = ref(false);
const isSelectAll = ref(true);
const list = ref([]);
const total = ref(0);
const focused = ref(false);
const blured = ref(false);
const empty = ref(false);
const error = ref("");
const showHistory = ref(false);
const queryHistory = ref([]);
const active = computed(() => focused.value || total.value > 0);

const router = useRoute();
const showList = computed(() => !router.path.includes('/things/'));

const params = reactive({
  pageIndex: 1,
  pageSize: defaultPageSize,
  query: placeholder,
});

const reset = () => {
  focused.value = false;
  list.value = [];
  total.value = 0;
  empty.value = false;
  error.value = "";
};

const handleOpenDoc = () => window.open("/docs/#/shadows/query", "_blank");

const handleClear = () => {
  query.value = placeholder;
  reset();
};

const handleSelect = async (suggestion) => {
  query.value = suggestion.value;
  if (suggestion.autoTrigger) {
    await nextTick();
    handleSearch();
  } else if (total.value > 0) {
    focused.value = false;
  }
};

const handleSearch = (syncSql = true) => {
  showHistory.value = false
  if (syncSql) {
    sqlEditor.value?.syncValueTrim();
  }
  const value = query.value?.trim() || placeholder;
  query.value = value;
  params.query = value;
  params.pageIndex = 1;
  fetchList();
};

// 键盘快捷键处理
const handleKeydown = (event) => {
  if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
    event.preventDefault();
    handleSearch();
  } else if (event.key === 'Escape') {
    if (focused.value) {
      focused.value = false;
    } else if (error.value) {
      error.value = '';
    }
  }
};

// 查询历史相关方法
const loadQueryHistory = () => {
  try {
    const history = JSON.parse(localStorage.getItem('queryHistory') || '[]');
    queryHistory.value = history;
  } catch (err) {
    console.warn('Failed to load query history:', err);
    queryHistory.value = [];
  }
};

const clearQueryHistory = () => {
  localStorage.removeItem('queryHistory');
  queryHistory.value = [];
};

const selectFromHistory = (historyQuery) => {
  debugger
  query.value = historyQuery;
  showHistory.value = false;
  handleSearch(false);
};

// 添加键盘事件监听
onMounted(() => {
  document.addEventListener('keydown', handleKeydown);
  loadQueryHistory();
});

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown);
});

const handlePageIndexChange = (value) => {
  params.pageIndex = value;
  fetchList();
};

const handlePageSizeChange = (value) => {
  params.pageSize = value;
  fetchList();
};

const fetchList = async () => {
  try {
    querying.value = true;
    isSelectAll.value = params.query.toLowerCase().startsWith("select *");
    const { data } = await queryShadows(params);
    reset();
    list.value = data.content;
    total.value = data.total;
    empty.value = data.total === 0;

    // 保存成功的查询到本地存储
    if (params.query && params.query !== placeholder) {
      const q = JSON.parse(localStorage.getItem('queryHistory') || '[]');
      const newHistory = [params.query, ...q.filter(q => q !== params.query)].slice(0, 10);
      queryHistory.value = newHistory;
      localStorage.setItem('queryHistory', JSON.stringify(newHistory));
    }
  } catch (err) {
    reset();
    console.error('Query failed:', err);

    // 更友好的错误处理
    if (err?.code === 400) {
      error.value = JSON.stringify({
        message: "查询语法错误",
        details: err.message || err,
        suggestion: "请检查SQL语法是否正确"
      }, null, 2);
    } else if (err?.code === 500) {
      error.value = JSON.stringify({
        message: "服务器内部错误",
        details: err.message || err,
        suggestion: "请稍后重试或联系管理员"
      }, null, 2);
    } else if (err?.code === 404) {
      error.value = JSON.stringify({
        message: "资源未找到",
        details: err.message || err,
        suggestion: "请检查查询条件是否正确"
      }, null, 2);
    } else {
      error.value = JSON.stringify({
        message: "查询失败",
        details: err?.message || err || "未知错误",
        suggestion: "请检查网络连接或稍后重试"
      }, null, 2);
    }
  } finally {
    querying.value = false;
  }
};

const refresh = () => {
  if (total.value || empty.value) fetchList();
};

watch(shadowListUpdateTag, refresh);
onSomethingStatusChange(({ thingId: eventThingId, type, about }) => {
  const shadow = list.value.find(({ thingId }) => thingId === eventThingId);
  if (shadow) {
    switch (type) {
      case TSCE_MQTT:
      case TSCE_MQTO:
        refresh();
        break;

      default:
        break;
    }
  }
});
</script>

<style scoped lang="scss">
.main-view {
  width: 100%;
  padding: 16px;
  overflow: hidden;

  .thing-detail {
    width: 100%;
    height: auto;
    border-radius: 8px;
    background-color: white;
  }

  .thing-list {
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    height: 100%;
    min-height: 400px;
    margin-bottom: 20px;

    &.active {
      justify-content: start;

      .list-view-search {
        width: 100%;
        height: 100px;
        border-radius: 12px;
        background-color: white;
        border: 1px solid #e4e7ed;

        .list-view-search-left {
          height: 100px;

          .list-view-query-editor {
            .list-view-query-link {
              top: 8px;
            }
          }
        }

        .list-view-search-right {
          width: auto;
          min-width: 200px;
          height: 100px;
        }
      }
    }

    .list-view-search {
      display: flex;
      width: 100%;
      max-width: 800px;
      height: 70px;
      padding: 0;
      border-radius: 8px;
      background-color: white;
      border: 1px solid #e4e7ed;
      overflow: hidden;


      .list-view-search-left {
        flex: 1;
        width: 0;
        padding: 8px;
        padding-right: 0;

        .list-view-query-editor {
          position: relative;
          width: 100%;
          height: 100%;
          border: 1px solid #dcdfe6;
          border-radius: 6px;
          overflow: hidden;

          &:focus-within {
            border-color: #409eff;
          }

          .list-view-query-link {
            position: absolute;
            top: 12px;
            right: 8px;
            width: 28px;
            height: 28px;
            line-height: 28px;
            text-align: center;
            opacity: 0;
            z-index: 10;
          }

          &:hover {
            .list-view-query-link {
              opacity: 1;
            }
          }
        }
      }

      .list-view-search-right {
        display: flex;
        flex-direction: row;
        justify-content: center;
        align-items: center;
        gap: 12px;
        width: auto;
        min-width: 200px;
        height: 70px;
        padding: 8px 16px;

        .el-button {
          flex: 1;
          height: 36px;
          margin: 0;
          font-size: 13px;
          font-weight: 500;
          white-space: nowrap;

          &.el-button--text {
            flex: 0;
            width: 32px;
            height: 32px;
            padding: 0;
            color: #909399;

            &:hover {
              color: #409eff;
              background-color: rgba(64, 158, 255, 0.1);
            }
          }
        }
      }
    }

    .list-view-active-body {
      flex: 1;
      width: 100%;
      height: 0;
      // max-height: calc(100% - 100px);
      margin-top: 16px;

      .list-view-things {
        width: 100%;
        height: 100%;
        background-color: white;
        border-radius: 6px;
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
        overflow: hidden;
      }

      .sql-editor-tpls {
        width: 100%;
        height: 100%;
        background-color: white;
        border-radius: 8px;
        overflow: hidden;

        .sql-editor-tpls-header {
          display: flex;
          justify-content: space-between;
          align-items: center;
          padding: 16px;
          border-bottom: 1px solid #e4e7ed;
          background-color: #fafafa;

          h4 {
            margin: 0;
            font-size: 16px;
            font-weight: 600;
            color: #303133;
          }
        }

        .sql-editor-tpls-content {
          padding: 12px;
          overflow-y: auto;
          height: calc(100% - 60px);

          .sql-editor-tpl-item {
            margin-bottom: 8px;
            padding: 10px 12px;
            background-color: #f8f9fa;
            border: 1px solid #e9ecef;
            border-radius: 6px;
            cursor: pointer;

            &:hover {
              background-color: #e3f2fd;
              border-color: #409eff;
            }

            .sql-editor-tpl-item-header {
              display: flex;
              gap: 8px;
              margin-bottom: 4px;
            }

            .sql-editor-tpl-item-content {
              font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
              font-size: 12px;
              line-height: 1.4;
              color: #606266;
              word-break: break-all;
            }
          }
        }
      }

      .query-history {
        width: 100%;
        max-width: 800px;
        height: 400px;
        background-color: white;
        border-radius: 8px;
        overflow: hidden;
        margin: 16px auto;

        .query-history-header {
          display: flex;
          justify-content: space-between;
          align-items: center;
          padding: 16px;
          border-bottom: 1px solid #e4e7ed;
          background-color: #fafafa;

          h4 {
            margin: 0;
            font-size: 16px;
            font-weight: 600;
            color: #303133;
          }

          .query-history-actions {
            display: flex;
            gap: 8px;
          }
        }

        .query-history-content {
          padding: 16px;
          overflow-y: auto;
          height: calc(100% - 60px);

          .query-history-item {
            margin-bottom: 8px;
            padding: 12px;
            background-color: #f8f9fa;
            border: 1px solid #e9ecef;
            border-radius: 6px;
            cursor: pointer;

            &:hover {
              background-color: #e3f2fd;
              border-color: #409eff;
            }

            .query-history-item-content {
              font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
              font-size: 13px;
              line-height: 1.4;
              color: #606266;
              word-break: break-all;
            }
          }

          .query-history-empty {
            display: flex;
            align-items: center;
            justify-content: center;
            height: 100px;
            color: #909399;
          }
        }
      }

      .list-view-empty {
        width: 100%;
        height: 100%;
        display: flex;
        align-items: center;
        justify-content: center;
        background-color: white;
        border-radius: 8px;

        .el-empty {
          .el-empty__description {
            color: #909399;
            font-size: 14px;
          }
        }
      }
    }

    .list-view-inactive-body {
      width: 100%;
      max-width: 600px;
      height: 300px;

      .list-view-error {
        width: 100%;
        height: 100%;
        background-color: white;
        border-radius: 8px;
        overflow: hidden;

        .list-view-error-header {
          display: flex;
          justify-content: space-between;
          align-items: center;
          padding: 12px 16px;
          background-color: #fef0f0;
          border-bottom: 1px solid #fbc4c4;

          .el-text {
            display: flex;
            align-items: center;
            gap: 8px;
            font-weight: 500;
          }
        }

        .list-view-error-content {
          height: calc(100% - 50px);
          padding: 16px;
        }
      }

      .list-view-tips {
        margin-top: 20px;
        width: 100%;
        height: 100%;
        display: flex;
        align-items: center;
        justify-content: center;
        background-color: white;
        border-radius: 8px;
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);

        .list-view-tips-content {
          text-align: center;
          padding: 32px;

          .el-icon {
            margin-bottom: 16px;
          }

          h3 {
            margin: 0 0 8px 0;
            font-size: 18px;
            font-weight: 600;
            color: #303133;
          }

          p {
            margin: 0 0 24px 0;
            font-size: 14px;
            color: #909399;
            line-height: 1.5;
          }

          .el-button {
            border-radius: 6px;
            padding: 12px 24px;
            font-weight: 500;
          }
        }
      }
    }
  }
}

// 响应式设计
@media (max-width: 768px) {
  .main-view {
    padding: 8px;

    .thing-list {
      .list-view-search {
        flex-direction: column;
        height: auto;
        padding: 12px;

        .list-view-search-left {
          width: 100%;
          height: auto;
          padding: 0 0 12px 0;

          .list-view-query-editor {
            height: 50px;
          }
        }

        .list-view-search-right {
          width: 100%;
          height: auto;
          flex-direction: row;
          gap: 8px;
          padding: 0;
          min-width: auto;

          .el-button {
            flex: 1;
            height: 40px;
            font-size: 12px;
          }
        }
      }

      &.active {
        .list-view-search {
          height: auto;
        }
      }
    }
  }
}
</style>

<style lang="scss">
.main-view {
  .thing-list {
    .list-view-search {
      .CodeMirror {
        height: 100%;
        line-height: 1.5;
        color: #303133;
        direction: ltr;
        background-color: white;
        font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
        font-size: 14px;

        .CodeMirror-scroll {
          width: 100%;
          height: 100%;
          padding: 8px;
        }

        .CodeMirror-lines {
          padding: 0;
        }

        .CodeMirror-cursor {
          border-left: 2px solid #409eff;
        }

        .CodeMirror-selected {
          background-color: rgba(64, 158, 255, 0.2);
        }

        .CodeMirror-focused .CodeMirror-selected {
          background-color: rgba(64, 158, 255, 0.3);
        }
      }
    }

    .list-view-error {
      .jse-main {
        position: relative;
        height: 100%;
        border-radius: 4px;

        .jse-tree-mode {
          border: none;
          background-color: transparent;

          .jse-contents {
            border: none;
            background-color: #fef0f0;
          }

          .jse-key {
            color: #e6a23c;
            font-weight: 600;
          }

          .jse-string {
            color: #67c23a;
          }

          .jse-number {
            color: #409eff;
          }

          .jse-boolean {
            color: #f56c6c;
          }
        }
      }
    }
  }
}

/* CodeMirror 提示框样式优化 */
.CodeMirror-hints {
  z-index: 9999 !important;
  border-radius: 6px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  border: 1px solid #e4e7ed;
  background-color: white;

  .CodeMirror-hint {
    padding: 8px 12px;
    font-size: 13px;
    line-height: 1.4;
    border-radius: 4px;
    transition: background-color 0.2s;

    &:hover {
      background-color: #f5f7fa;
    }

    &.CodeMirror-hint-active {
      background-color: #e3f2fd;
      color: #409eff;
    }
  }
}

/* 加载状态优化 */
.el-loading-mask {
  background-color: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(2px);
}

.el-loading-spinner {
  .circular {
    width: 40px;
    height: 40px;
  }
}

/* 按钮悬停效果优化 */
.el-button {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);

  &:hover {
    transform: translateY(-1px);
  }

  &:active {
    transform: translateY(0);
  }

  &.is-loading {
    pointer-events: none;
  }
}

/* 表格样式优化 */
.el-table {
  .el-table__header {
    th {
      background-color: #fafafa;
      font-weight: 600;
      color: #303133;
    }
  }

  .el-table__row {
    transition: background-color 0.3s;

    &:hover {
      background-color: #f5f7fa;
    }
  }
}

/* 分页组件样式优化 */
.el-pagination {
  .el-pager li {
    transition: all 0.3s;
    border-radius: 4px;

    &:hover {
      background-color: #f5f7fa;
    }

    &.is-active {
      background-color: #409eff;
      color: white;
    }
  }

  .btn-prev,
  .btn-next {
    transition: all 0.3s;
    border-radius: 4px;

    &:hover {
      background-color: #f5f7fa;
    }
  }
}
</style>

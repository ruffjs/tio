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
          <el-button v-if="queryHistory.length > 0" @click="showHistory = !showHistory" text size="small" type="primary" icon="Clock" :title="$t('common.history')" />
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
  color: var(--tio-text);

  .thing-detail {
    width: 100%;
    height: auto;
    border-radius: var(--tio-radius);
    background: transparent;
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
        border-radius: var(--tio-radius-lg);

        .list-view-search-left {
          height: 100px;

          .list-view-query-editor .list-view-query-link {
            top: 8px;
          }
        }

        .list-view-search-right {
          width: auto;
          min-width: 200px;
          height: 100px;
        }
      }
    }

    .list-view-search,
    .list-view-things,
    .sql-editor-tpls,
    .query-history,
    .list-view-empty,
    .list-view-error,
    .list-view-tips {
      border: 1px solid var(--tio-border);
      background: var(--tio-surface);
      color: var(--tio-text);
      box-shadow: none;
    }

    .list-view-search {
      position: relative;
      display: flex;
      width: 100%;
      max-width: 800px;
      height: 70px;
      padding: 0;
      border-radius: var(--tio-radius);
      overflow: hidden;

      .list-view-search-left {
        flex: 1;
        width: 0;
        padding: 8px 0 8px 8px;

        .list-view-query-editor {
          position: relative;
          width: 100%;
          height: 100%;
          overflow: hidden;
          border: 1px solid var(--tio-border);
          border-radius: var(--tio-radius);
          background: var(--tio-surface);
          transition: border-color 0.18s ease, background-color 0.18s ease;

          &:focus-within {
            border-color: var(--tio-accent-strong);
            background: var(--tio-surface);
          }

          .list-view-query-link {
            position: absolute;
            top: 12px;
            right: 8px;
            z-index: 10;
            width: 28px;
            height: 28px;
            line-height: 28px;
            text-align: center;
            opacity: 0;
          }

          &:hover .list-view-query-link {
            opacity: 1;
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
          font-weight: 650;
          white-space: nowrap;

          &.el-button--text {
            flex: 0;
            width: 32px;
            height: 32px;
            padding: 0;
            color: var(--tio-muted);

            &:hover {
              background: var(--tio-accent-soft);
              color: var(--tio-text-strong);
            }
          }
        }
      }
    }

    .list-view-active-body {
      flex: 1;
      width: 100%;
      height: 0;
      margin-top: 16px;

      .list-view-things {
        width: 100%;
        height: 100%;
        overflow: hidden;
        border-radius: var(--tio-radius);
      }

      .sql-editor-tpls {
        width: 100%;
        height: 100%;
        overflow: hidden;
        border-radius: var(--tio-radius);

        .sql-editor-tpls-header {
          display: flex;
          justify-content: space-between;
          align-items: center;
          padding: 16px;
          border-bottom: 1px solid var(--tio-border);
          background: var(--tio-surface-soft);

          h4 {
            margin: 0;
            color: var(--tio-text-strong);
            font-size: 16px;
            font-weight: 650;
          }
        }

        .sql-editor-tpls-content {
          height: calc(100% - 60px);
          padding: 12px;
          overflow-y: auto;

          .sql-editor-tpl-item {
            margin-bottom: 8px;
            padding: 10px 12px;
            border: 1px solid var(--tio-border);
            border-radius: var(--tio-radius);
            background: var(--tio-surface-soft);
            cursor: pointer;
            transition: background-color 0.18s ease, border-color 0.18s ease;

            &:hover {
              border-color: var(--tio-accent-strong);
              background: var(--tio-accent-soft);
            }

            .sql-editor-tpl-item-header {
              display: flex;
              gap: 8px;
              margin-bottom: 4px;
            }

            .sql-editor-tpl-item-content {
              color: var(--tio-text);
              font-family: var(--tio-mono);
              font-size: 12px;
              line-height: 1.4;
              word-break: break-all;
            }
          }
        }
      }

      .query-history {
        width: 100%;
        max-width: 800px;
        height: 400px;
        overflow: hidden;
        margin: 16px auto;
        border-radius: var(--tio-radius);

        .query-history-header {
          display: flex;
          justify-content: space-between;
          align-items: center;
          padding: 16px;
          border-bottom: 1px solid var(--tio-border);
          background: var(--tio-surface-soft);

          h4 {
            margin: 0;
            color: var(--tio-text-strong);
            font-size: 16px;
            font-weight: 650;
          }

          .query-history-actions {
            display: flex;
            gap: 8px;
          }
        }

        .query-history-content {
          height: calc(100% - 60px);
          padding: 16px;
          overflow-y: auto;

          .query-history-item {
            margin-bottom: 8px;
            padding: 12px;
            border: 1px solid var(--tio-border);
            border-radius: var(--tio-radius);
            background: var(--tio-surface-soft);
            cursor: pointer;
            transition: background-color 0.18s ease, border-color 0.18s ease;

            &:hover {
              border-color: var(--tio-accent-strong);
              background: var(--tio-accent-soft);
            }

            .query-history-item-content {
              color: var(--tio-text);
              font-family: var(--tio-mono);
              font-size: 13px;
              line-height: 1.4;
              word-break: break-all;
            }
          }

          .query-history-empty {
            display: flex;
            align-items: center;
            justify-content: center;
            height: 100px;
            color: var(--tio-muted);
          }
        }
      }

      .list-view-empty {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 100%;
        height: 100%;
        border-radius: var(--tio-radius);
      }
    }

    .list-view-inactive-body {
      width: 100%;
      max-width: 600px;
      height: 300px;

      .list-view-error {
        width: 100%;
        height: 100%;
        overflow: hidden;
        border-color: rgba(239, 68, 68, 0.24);
        border-radius: var(--tio-radius);

        .list-view-error-header {
          display: flex;
          justify-content: space-between;
          align-items: center;
          padding: 12px 16px;
          border-bottom: 1px solid rgba(239, 68, 68, 0.24);
          background: var(--tio-danger-soft);

          .el-text {
            display: flex;
            align-items: center;
            gap: 8px;
            font-weight: 650;
          }
        }

        .list-view-error-content {
          height: calc(100% - 50px);
          padding: 16px;
        }
      }

      .list-view-tips {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 100%;
        height: 100%;
        margin-top: 20px;
        border-radius: var(--tio-radius);

        .list-view-tips-content {
          padding: 32px;
          text-align: center;

          .el-icon {
            margin-bottom: 16px;
            color: var(--tio-muted);
          }

          h3 {
            margin: 0 0 8px;
            color: var(--tio-text-strong);
            font-size: 18px;
            font-weight: 650;
          }

          p {
            margin: 0 0 24px;
            color: var(--tio-muted);
            font-size: 14px;
            line-height: 1.5;
          }
        }
      }
    }
  }

  :deep(.CodeMirror) {
    height: 100%;
    line-height: 1.5;
    direction: ltr;
    font-size: 14px;

    .CodeMirror-scroll {
      width: 100%;
      height: 100%;
      padding: 8px;
    }

    .CodeMirror-lines {
      padding: 0;
    }
  }

  :deep(.jse-main) {
    position: relative;
    height: 100%;
    border-radius: var(--tio-radius);
  }
}

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
          padding: 0 0 12px;

          .list-view-query-editor {
            height: 50px;
          }
        }

        .list-view-search-right {
          width: 100%;
          height: auto;
          min-width: auto;
          flex-direction: row;
          gap: 8px;
          padding: 0;

          .el-button {
            flex: 1;
            height: 40px;
            font-size: 12px;
          }
        }
      }

      &.active .list-view-search {
        height: auto;
      }
    }
  }
}
</style>

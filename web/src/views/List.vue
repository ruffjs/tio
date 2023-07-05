<template>
  <div
    class="list-view"
    :style="{ backgroundColor: !focused && total > 0 ? 'white' : 'transparent' }"
  >
    <div class="thing-detail" :style="{ height: route.params.thingId ? '100%' : '0%' }">
      <router-view></router-view>
    </div>
    <div class="things-list" :class="{ active }">
      <div class="list-view-search">
        <div class="list-view-search-left">
          <SQLEditor v-model="query" v-model:focused="focused" ref="sqlEditor" />
        </div>
        <div class="list-view-search-right">
          <el-button icon="Close" v-if="active" @click="handleClear" />
          <el-button icon="Search" @click="handleSearch" />
        </div>
      </div>
      <div v-if="active" class="list-view-active-body">
        <div v-if="focused" class="sql-editor-tpls" @click="focused = false">
          <div
            v-for="suggestion in suggestions"
            class="sql-editor-tpl-item"
            @click.stop="handleSelect(suggestion)"
          >
            <el-tag size="small" style="float: left; margin-right: 10px">
              {{ suggestion.label }}
            </el-tag>
            <el-tag
              v-if="suggestion.autoTrigger"
              size="small"
              style="float: left; margin-right: 10px"
            >
              AUTO
            </el-tag>
            <span>{{ suggestion.value }}</span>
          </div>
        </div>
        <div v-else="total > 0" class="list-view-things">
          <ThingsList
            :items="list"
            :page-size="params.pageSize"
            :total="total"
            :isStandard="isSelectAllFields"
            @page-index-change="handlePageIndexChange"
            @page-size-change="handlePageSizeChange"
          />
        </div>
      </div>
      <div v-else class="list-view-inactive-body">
        <div v-if="error" class="list-view-error">
          <JSONEditor mode="tree" :model-value="error" disabled class="" />
        </div>
        <div v-else-if="empty" class="list-view-emtpy">
          The current query statement returns no Things
        </div>
        <div v-else class="list-view-tips">
          Type in or select the SQL statement to query Things
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
import { suggestions } from "@/configs/list";
import { queryShadows } from "@/apis";
import ThingsList from "@/components/list/ThingsList.vue";
import SQLEditor from "@/components/list/SQLEditor.vue";
import JSONEditor from "@/components/common/JSONEditor.vue";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import { TH_STATUS_CHG_EVT, TSCE_MQTO, TSCE_MQTT } from "@/utils/event";

const defaultPageSize = 20;
const placeholder = suggestions[0].value;
const route = useRoute();
const { shadowListUpdateTag } = useThingsAndShadows();
const query = ref("SELECT * FROM shadow");
const params = reactive({
  pageIndex: 1,
  pageSize: defaultPageSize,
  query: placeholder,
});
const isSelectAllFields = ref(true);

const sqlEditor = ref();
const list = ref([]);
const total = ref(0);
const focused = ref(false);
const blured = ref(false);
const empty = ref(false);
const error = ref("");
const active = computed(() => focused.value || total.value > 0);

const reset = () => {
  focused.value = false;
  list.value = [];
  total.value = 0;
  empty.value = false;
  error.value = "";
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
const handleSearch = () => {
  sqlEditor.value?.syncValueTrim();
  const value = query.value?.trim() || placeholder;
  query.value = value;
  params.query = value;
  params.pageIndex = 1;
  fetchList();
};
const handleClear = () => {
  query.value = placeholder;
  reset();
};

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
    // console.log(params);
    isSelectAllFields.value = params.query.toLowerCase().startsWith("select *");
    const { data } = await queryShadows(params);
    reset();
    list.value = data.content;
    total.value = data.total;
    empty.value = data.total === 0;
  } catch (err) {
    // console.error("fetchList error:", err);
    if (err?.code === 400) {
      reset();
      error.value = JSON.stringify(err, null, 2);
    }
  }
};

const refresh = () => {
  if (total.value || empty.value) {
    fetchList();
  }
};
watch(shadowListUpdateTag, refresh);
const onSomethingStatusChange = (message) => {
  const { thingId: eventThingId, type, about } = message.detail;
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
};

onMounted(() => {
  window.addEventListener(TH_STATUS_CHG_EVT, onSomethingStatusChange);
});
onUnmounted(() => {
  window.removeEventListener(TH_STATUS_CHG_EVT, onSomethingStatusChange);
});
</script>

<style scoped lang="scss">
.list-view {
  width: 100%;
  height: 100%;
  padding-top: var(--layout-top-gap);
  padding-bottom: var(--layout-bottom-gap);

  overflow: hidden;

  .things-list {
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;

    width: 100%;
    height: 100%;
    min-height: 268px;

    &.active {
      justify-content: start;
      .list-view-search {
        width: 100%;
        height: 93px;
        border-radius: 0;
        border-bottom: solid 1px rgba($color: #000000, $alpha: 0.2);

        .list-view-search-left {
          height: 92px;
        }
        .list-view-search-right {
          width: 92px;
          height: 92px;
        }
      }
    }
    .list-view-search {
      display: flex;
      width: 624px;
      height: 52px;
      padding: 0px;
      border-radius: 4px;
      background-color: white;
      overflow: hidden;
      transition: all ease-in-out 0.05s;

      .list-view-search-left {
        flex: 1;
        width: 0;
      }

      .list-view-search-right {
        display: flex;
        flex-direction: column;
        justify-content: space-around;
        align-items: center;
        gap: 4px;

        position: relative;
        width: 52px;
        height: 52px;
        padding: 4px;

        .el-button {
          flex: 1;
          width: 100%;
          height: 0;
          margin-left: 0;
        }
      }
    }

    .list-view-active-body {
      flex: 1;
      width: 100%;
      height: 0;
      max-height: calc(100% - 92px);

      .list-view-things {
        width: 100%;
        height: 100%;
      }

      .sql-editor-tpls {
        width: 100%;
        height: 100%;
        padding: 0px 6px 6px;
        overflow-x: hidden;
        overflow-y: auto;

        .sql-editor-tpl-item {
          margin-top: 6px;
          padding: 10px 5px;
          background-color: white;
          line-height: 20px;
          font-size: 13px;
          word-wrap: normal;
          word-break: keep-all;
          cursor: pointer;
        }
      }
    }

    .list-view-inactive-body {
      width: 624px;
      height: 168px;
      max-width: 624px;
      max-width: 624px;

      .list-view-error {
        width: 624px;
        height: 168px;
        max-width: 624px;
        max-width: 624px;
        margin-top: 10px;
        padding: 5px;
        border-radius: 4px;
        background-color: rgba($color: #ffffff, $alpha: 0.6);
      }

      .list-view-emtpy,
      .list-view-tips {
        width: 100%;
        height: 168px;
        line-height: 108px;
        text-align: center;
        font-size: 14px;
      }
      .list-view-emtpy {
        font-weight: 500;
        color: #555;
      }
      .list-view-tips {
        font-weight: 400;
        color: #999;
      }
    }
  }
}

.thing-detail {
  width: 100%;
  background-color: white;
  transition: height ease-in-out 0.2s;
}
</style>

<style lang="scss">
.list-view {
  .things-list {
    .list-view-search {
      .CodeMirror {
        width: 100%;
        height: 92px;
        line-height: 22px;
        color: black;
        direction: ltr;
        background-color: white;

        .CodeMirror-scroll {
          width: 100%;
          max-height: 92px;
          padding-bottom: 0;
        }
      }
    }

    .list-view-error {
      .jse-main {
        position: relative;
        height: 148px;

        .jse-tree-mode {
          border: none;
          background-color: transparent;
          .jse-contents {
            border: none;
          }
        }
      }
    }
  }
}
/* // 这句为了解决匹配框显示有问题而加 */
.CodeMirror-hints {
  z-index: 9999 !important;
}
</style>

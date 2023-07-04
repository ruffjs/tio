<template>
  <div
    class="list-view"
    :style="{ backgroundColor: total > 0 ? 'white' : 'transparent' }"
  >
    <div class="thing-detail" :style="{ height: route.params.thingId ? '100%' : '0%' }">
      <router-view></router-view>
    </div>
    <div class="things-list">
      <div class="list-view-search">
        <el-autocomplete
          v-model="query"
          :placeholder="placeholder"
          :fetch-suggestions="querySearch"
          clearable
          class="list-view-search-input"
          :handle-key-enter="handleSelect"
          @select="handleSelect"
          @clear="handleClear"
        >
          <template #default="{ item }">
            <el-tag v-if="item.autoTrigger" size="small" style="margin-right: 5px">
              AUTO
            </el-tag>
            <span style="font-size: 12px">{{ item.value }}</span>
          </template>
          <template #append>
            <el-button :icon="Search" @click="handleSearch" />
          </template>
        </el-autocomplete>
      </div>
      <div v-if="total > 0" class="list-view-things">
        <ThingsList
          :items="list"
          :page-size="params.pageSize"
          :total="total"
          :isStandard="isSelectAllFields"
          @page-index-change="handlePageIndexChange"
          @page-size-change="handlePageSizeChange"
        />
      </div>
      <div v-else-if="error" class="list-view-error">
        <textarea readonly>{{ error }}</textarea>
      </div>
      <div v-else-if="empty" class="list-view-emtpy">
        The current query statement returns no Things
      </div>
      <div v-else class="list-view-emtpy">
        Enter or select the SQL statement to query Things
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: "List",
  inheritAttrs: false,
  customOptions: { title: "Tio Playground", zIndex: 0, actived: true },
};
</script>
<script setup>
import { ref, reactive, watch, computed, onMounted, onUnmounted } from "vue";
import { Search } from "@element-plus/icons-vue";
import { suggestions } from "@/configs/list";
import { queryShadows } from "@/apis";
import ThingsList from "@/components/list/ThingsList.vue";
import { useRoute } from "vue-router";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import { TH_STATUS_CHG_EVT, TSCE_MQTO, TSCE_MQTT } from "@/utils/event";

const defaultPageSize = 20;
const placeholder = suggestions[0].value;
const route = useRoute();
const { shadowListUpdateTag } = useThingsAndShadows();
const query = ref("");
const params = reactive({
  pageIndex: 1,
  pageSize: defaultPageSize,
  query: placeholder,
});
const isSelectAllFields = ref(true);

const list = ref([]);
const total = ref(0);
const empty = ref(false);
const error = ref("");

const querySearch = (query, cb) => {
  const results = query
    ? suggestions.filter(
        (suggestion) => suggestion.value.toLowerCase().indexOf(query.toLowerCase()) === 0
      )
    : suggestions;
  // call callback function to return suggestions
  // console.log(results);
  cb(results);
};
const handleSelect = (suggestion) => {
  params.query = suggestion.value;
  if (suggestion.autoTrigger) {
    fetchList();
  }
};
const handleSearch = () => {
  if (query.value) {
    params.query = query.value;
  } else {
    query.value = placeholder;
    params.query = placeholder;
  }
  params.pageIndex = 1;
  fetchList();
};
const handleClear = () => {
  Object.assign(params, {
    pageIndex: 1,
    pageSize: defaultPageSize,
    query: placeholder,
  });
  list.value = [];
  total.value = 0;
  error.value = "";
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
    // console.log("fetchList data:", data);
    list.value = data.content;
    total.value = data.total;
    empty.value = data.total === 0;
    error.value = "";
  } catch (err) {
    console.error("fetchList error:", err);
    if (err?.code === 400) {
      list.value = [];
      total.value = 0;
      empty.value = false;
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
    .list-view-search {
      width: 40vw;
      height: 100px;
      padding: 36px 0 22px;
    }

    .list-view-error {
      width: 40vw;
      height: 168px;
      // margin: 0 auto;
      > textarea {
        width: 100%;
        height: 100%;
        padding: 10px;
        border: none;
        background-color: white;
        color: red;
        resize: none;
        outline: none;
        overflow-x: hidden;
        overflow-y: auto;
      }
    }

    .list-view-things {
      width: 100%;
      height: calc(100% - 100px);
    }

    .list-view-emtpy {
      width: 100%;
      height: 168px;
      line-height: 108px;
      text-align: center;
      font-size: 14px;
      color: #999;
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
  .list-view-search {
    .list-view-search-input {
      width: 40vw;
    }
  }
}
</style>

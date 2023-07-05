<template>
  <div class="custom-thing-table">
    <el-table :data="data" scrollbar-always-on height="100%" style="width: 100%">
      <el-table-column
        v-if="withId"
        fixed
        prop="thingId"
        label="Thing Id"
        min-width="180"
      />
      <template v-for="column in columns">
        <el-table-column
          v-if="column.type === 'boolean'"
          :prop="column.prop"
          :label="column.label"
          min-width="100"
        >
          <template #default="scope">
            <el-tag v-if="scope.row[column.prop]">True</el-tag>
            <el-tag v-else>False</el-tag>
          </template>
        </el-table-column>
        <el-table-column
          v-else-if="column.type === 'time'"
          :prop="column.prop"
          :label="column.label"
          min-width="140"
        >
          <template #default="scope">
            {{ formatTime(scope.row[column.prop]) }}
          </template>
        </el-table-column>
        <el-table-column
          v-else-if="column.type === 'object'"
          :prop="column.prop"
          :label="column.label"
          min-width="140"
        >
          <template #default="scope">
            <el-button
              link
              type="primary"
              size="small"
              @click.prevent="viewObject(scope.row[column.prop], column.label)"
            >
              View {{ column.label }}
            </el-button>
          </template>
        </el-table-column>
        <el-table-column
          v-else
          :prop="column.prop"
          :label="column.label"
          :min-width="column.minWidth"
        />
      </template>
      <el-table-column
        fixed="right"
        label="Operations"
        align="center"
        :min-width="withId ? 260 : 80"
      >
        <template #default="scope">
          <el-button
            v-if="withId"
            link
            type="primary"
            size="small"
            @click.prevent="viewThing(scope.row)"
          >
            Goto Thing
          </el-button>
          <el-button
            link
            type="primary"
            size="small"
            @click.prevent="viewObject(scope.row, 'Raw Data', true)"
          >
            View Raw
          </el-button>
          <DeleteButton
            title="Are you sure to delete this Thing?"
            v-if="withId"
            @confirm="deleteThing(scope.row)"
          />
        </template>
      </el-table-column>
    </el-table>
  </div>
  <ObjectViewer
    :visible="!!objectToBeView"
    :data="objectToBeView"
    :type="titleOfViewer"
    :as-tree="viewObjectAsTree"
    @close="handleCloseViewer"
  />
</template>

<script setup>
import { ref, watch } from "vue";
import { useRouter } from "vue-router";
import dayjs from "dayjs";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import ObjectViewer from "@/components/common/ObjectViewer.vue";
import DeleteButton from "./DeleteButton.vue";
import useObjectViewer from "@/reactives/useObjectViewer";

const formatTime = (time) => {
  return dayjs(time).format("YYYY-MM-DD HH:mm:ss");
};

const props = defineProps({
  data: Array,
});
const router = useRouter();
const withId = ref(true);
const columns = ref([]);
const { delThing, setCurrentShadow } = useThingsAndShadows();
const {
  objectToBeView,
  titleOfViewer,
  viewObjectAsTree,
  viewObject,
  handleCloseViewer,
} = useObjectViewer();

const viewThing = (thing) => {
  if (thing.thingId) {
    setCurrentShadow({ fromList: true });
    router.push(`/things/${thing.thingId}`);
  }
};

const deleteThing = (thing) => {
  if (thing.thingId) {
    delThing(thing.thingId).then((ok) => {
      if (ok) {
        console.log("deleted");
      }
    });
  }
};

watch(
  () => props.data,
  (data) => {
    if (data && data[0]) {
      const keys = Object.keys(data[0]);
      withId.value = false;
      columns.value = keys
        .filter((key) => {
          if (key === "thingId") {
            withId.value = true;
            return false;
          }
          return true;
        })
        .map((prop) => {
          let type = typeof data[0][prop];
          if (prop.endsWith("_time") || prop.endsWith("Time")) {
            type = "time";
          }
          return {
            prop,
            type,
            label: prop.replace(/(^\w|\_\w)/g, (w) => w.replace("_", " ").toUpperCase()),
          };
        });
    } else {
      columns.value = [];
    }
  },
  { immediate: true }
);
</script>

<style scoped lang="scss">
.custom-thing-table {
  width: 100%;
  height: 100%;
}
</style>

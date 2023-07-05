<template>
  <div class="standard-thing-table">
    <el-table :data="computedData" scrollbar-always-on height="100%" style="width: 100%">
      <el-table-column fixed prop="thingId" label="Thing Id" min-width="180" />
      <el-table-column prop="connected" label="Connected" align="center" width="160">
        <template #default="scope">
          <el-alert
            v-if="scope.row.connected"
            title="Connected"
            type="success"
            show-icon
            :closable="false"
          />
          <el-alert v-else title="Disconnected" type="info" :closable="false" />
        </template>
      </el-table-column>
      <el-table-column
        prop="remoteAddr"
        label="Remote Address"
        align="center"
        min-width="180"
      >
        <template #default="scope">
          {{ scope.row.remoteAddr || "-" }}
        </template>
      </el-table-column>
      <el-table-column prop="createdAt" label="Created At" align="center" min-width="180">
        <template #default="scope">{{
          formatTime(scope.row.createdAt)
        }}</template></el-table-column
      >
      <el-table-column prop="updatedAt" label="Updated At" align="center" min-width="180">
        <template #default="scope">{{
          formatTime(scope.row.updatedAt)
        }}</template></el-table-column
      >
      <el-table-column label="State" align="center">
        <el-table-column prop="state" label="Desired" align="center" min-width="120">
          <template #default="scope">
            <el-button
              link
              type="primary"
              size="small"
              @click.prevent="viewState(scope.row.state, 'desired')"
            >
              View Desired
            </el-button>
          </template>
        </el-table-column>
        <el-table-column prop="state" label="Reported" align="center" min-width="120">
          <template #default="scope">
            <el-button
              link
              type="primary"
              size="small"
              @click.prevent="viewState(scope.row.state, 'reported')"
            >
              View Reported
            </el-button>
          </template>
        </el-table-column>
        <el-table-column prop="__ui_delta" label="Delta" align="center" width="150">
          <template #default="scope">
            <el-alert
              v-if="scope.row.__ui_delta === null"
              title="No Delta"
              type="success"
              show-icon
              :closable="false"
            />
            <el-button
              v-else
              link
              type="primary"
              size="small"
              @click.prevent="viewObject(scope.row.__ui_delta, 'Delta', true)"
            >
              View Delta
            </el-button>
          </template>
        </el-table-column>
      </el-table-column>
      <el-table-column prop="version" label="Version" align="center" width="90" />
      <el-table-column fixed="right" label="Operations" align="center" min-width="260">
        <template #default="scope">
          <el-button
            link
            type="primary"
            size="small"
            :disabled="scope.row.thingId === void 0"
            @click.prevent="viewThing(scope.row.__ui_origin)"
          >
            Goto Thing
          </el-button>
          <el-button
            link
            type="primary"
            size="small"
            :disabled="scope.row.thingId === void 0"
            @click.prevent="viewObject(scope.row.__ui_origin, 'Raw Data', true)"
          >
            View Raw
          </el-button>
          <DeleteButton
            title="Are you sure to delete this Thing?"
            :disabled="scope.row.thingId === void 0"
            @confirm="deleteThing(scope.row.__ui_origin)"
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
    @close="handleCloseViewers"
  />
  <StateViewer
    :visible="!!selectedState"
    :data="selectedState"
    :type="titleOfViewer"
    @close="handleCloseViewers"
  />
</template>

<script setup>
import { computed, ref } from "vue";
import { useRouter } from "vue-router";
import dayjs from "dayjs";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import ObjectViewer from "@/components/common/ObjectViewer.vue";
import StateViewer from "@/components/common/StateViewer.vue";
import DeleteButton from "./DeleteButton.vue";
import useObjectViewer from "@/reactives/useObjectViewer";
import { diffState } from "@/utils/shadow";

const formatTime = (time) => {
  return dayjs(time).format("YYYY-MM-DD HH:mm:ss");
};

const isSolved = (thing) => {
  const { desired, reported } = thing.state || {};
  return JSON.stringify(desired) === JSON.stringify(reported);
};

const props = defineProps({
  data: Array,
});
const router = useRouter();
const { setCurrentShadow, delThing } = useThingsAndShadows();
const {
  objectToBeView,
  titleOfViewer,
  viewObjectAsTree,
  viewObject: _viewObject,
  handleCloseViewer,
} = useObjectViewer();
const selectedState = ref(null);
const computedData = computed(() => {
  return props.data.map((shadow) => {
    const [hasDelta, __ui_delta] = diffState(shadow.state);
    if (hasDelta) {
      return { ...shadow, __ui_origin: shadow, __ui_delta };
    } else {
      return { ...shadow, __ui_origin: shadow, __ui_delta: null };
    }
  });
});

const viewState = (state, type) => {
  selectedState.value = state;
  objectToBeView.value = null;
  titleOfViewer.value = type;
};
const viewObject = (obj, type, asTree = false) => {
  selectedState.value = null;
  _viewObject(obj, type, asTree);
};
const handleCloseViewers = () => {
  selectedState.value = null;
  handleCloseViewer();
};

const viewThing = (thing) => {
  if (thing.thingId) {
    setCurrentShadow({ ...thing, fromList: true });
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
</script>

<style scoped lang="scss">
.standard-thing-table {
  width: 100%;
  height: 100%;

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    height: 40px;
    padding: 5px 10px;
    border-bottom: solid 1px #e6e6e6;
    .card-thing-id {
      font-weight: 600;
    }
    .card-button {
      font-size: 13px;
    }
  }

  .card-body {
    height: 160px;
    padding: 5px 10px;
    overflow-x: hidden;
    overflow-y: auto;

    .card-body-prop {
      width: 100%;
      height: 30px;
      display: flex;
      justify-content: space-between;
      align-items: center;
      border-bottom: solid 1px #f2f2f2;
      .card-body-prop-label {
        font-size: 13px;
        font-weight: 600;
      }
    }
  }
}
</style>

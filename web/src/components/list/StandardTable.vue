<template>
  <div class="standard-thing-table">
    <el-table
      :data="data"
      scrollbar-always-on
      height="100%"
      size="small"
      style="width: 100%"
    >
      <el-table-column fixed prop="thingId" label="Thing Id" min-width="180">
        <template #default="scope">
          <el-button
            link
            type="primary"
            size="small"
            @click.prevent="viewThing(scope.row)"
          >
            {{ scope.row.thingId }}
          </el-button></template
        >
      </el-table-column>
      <el-table-column prop="connected" label="Connected" align="center" width="120">
        <template #default="scope">
          <el-button
            v-if="scope.row.connected"
            type="success"
            size="small"
            icon="CircleCheckFilled"
            plain
            round
            >True</el-button
          >
          <el-button v-else type="info" size="small" icon="Warning" plain round
            >False</el-button
          >
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
      <el-table-column prop="version" label="Version" align="center" width="90" />
      <el-table-column fixed="right" label="Operations" align="center" min-width="180">
        <template #default="scope">
          <el-button
            link
            type="primary"
            size="small"
            :disabled="scope.row.thingId === void 0"
            @click.prevent="viewObject(scope.row, 'Raw Data', true)"
          >
            View Shadow
          </el-button>
          <DeleteButton
            title="Are you sure to delete this Thing?"
            :disabled="scope.row.thingId === void 0"
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
import { computed, ref } from "vue";
import { useRouter } from "vue-router";
import dayjs from "dayjs";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import ObjectViewer from "@/components/common/ObjectViewer.vue";
import DeleteButton from "./DeleteButton.vue";
import useObjectViewer from "@/reactives/useObjectViewer";
import { diffState } from "@/utils/shadow";
import useMqtt from "@/reactives/useMqtt";
import { notifyDone } from "@/utils/layout";

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
const { removeConnectionsByClientId } = useMqtt();
const {
  objectToBeView,
  titleOfViewer,
  viewObjectAsTree,
  viewObject,
  handleCloseViewer,
} = useObjectViewer();

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
        // console.log("deleted", thing.thingId);
        removeConnectionsByClientId(thing.thingId);
        notifyDone(`Delete ${thing.thingId}`);
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

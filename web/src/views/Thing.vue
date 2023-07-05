<template>
  <div class="thing-view">
    <div class="thing-view-left">
      <div class="thing-view-back">
        <el-button type="info" icon="Back" @click="handleBack2List"
          >Back to List</el-button
        >
      </div>
      <div class="thing-view-left-main">
        <div class="thing-view-meta">
          <KeyValueDisplayer :data="thing" :fields="metaFields" />
        </div>
        <div class="thing-update-btn">
          <el-button icon="Aim" @click="(posterCode = 'invoke'), (posterData = null)"
            >Request Direct Method</el-button
          >
        </div>
        <div class="thing-update-btn">
          <el-button icon="RefreshRight" @click="getBasicInfo"
            >Reload Thing's Data</el-button
          >
        </div>
        <MqttClients />
      </div>
    </div>
    <div class="thing-view-right">
      <ShadowProps :shadow="shadow" />
      <ShadowTags
        :data="shadow?.tags"
        @update="(payload) => ((posterCode = 'tags'), (posterData = payload || null))"
      />
      <div class="thing-view-state">
        <ShadowData @call="(code) => ((posterCode = code), (posterData = null))" />
      </div>
    </div>
  </div>
  <HttpPoster
    :code="posterCode"
    :thing-id="thingId"
    :payload="posterData"
    @done="updateCurrentShadow"
    @close="posterCode = ''"
  />
</template>

<script>
export default {
  name: "Thing",
  inheritAttrs: false,
  customOptions: {
    title: (route) => `${route.params.thingId || "(thingId)"} | Thing`,
    zIndex: 0,
    actived: true,
  },
  components: { KeyValueDisplayer, MqttClients },
};
</script>

<script setup>
import { computed, nextTick, onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { getThing } from "@/apis";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import { metaFields } from "@/configs/thing";
import KeyValueDisplayer from "@/components/common/KeyValueDisplayer.vue";
import ShadowProps from "@/components/thing/ShadowProps.vue";
import ShadowTags from "@/components/thing/ShadowTags.vue";
import ShadowData from "@/components/thing/ShadowData.vue";
import HttpPoster from "@/components/thing/HttpPoster.vue";
import MqttClients from "@/components/thing/MqttClientsOfThing.vue";
import dayjs from "dayjs";
import { TH_STATUS_CHG_EVT, TSCE_MQTO, TSCE_MQTT, TSCE_MSGO } from "@/utils/event";
import useThingEvent from "@/reactives/useThingEvent";

const router = useRouter();
const {
  route,
  selectedThingId: thingId,
  currentShadow: shadow,
  setCurrentShadow,
  updateCurrentShadow,
} = useThingsAndShadows();
const { onSomethingStatusChange } = useThingEvent();
const isFromList = ref(false);
const thing = reactive({});
const posterCode = ref("");
const posterData = ref(null);

const handleBack2List = () => {
  if (isFromList.value) {
    router.back();
  } else {
    router.replace("/");
  }
  setCurrentShadow(null);
};

const getBasicInfo = async () => {
  try {
    const res = await getThing(thingId.value);
    // console.log("getBasicInfo", res);
    Object.assign(thing, res.data);
  } catch (error) {
    console.error("error", error);
  }
};

onSomethingStatusChange(async ({ thingId: eventThingId, type, about }) => {
  if (eventThingId === thingId.value) {
    console.log(type, about);
    switch (type) {
      case TSCE_MQTT:
      case TSCE_MQTO:
        updateCurrentShadow();
        break;

      default:
        break;
    }
  } else if (eventThingId === "*" && type === TSCE_MSGO) {
    if (about.topic.startsWith(`$iothub/things/${thingId.value}/`)) {
      await nextTick();
      updateCurrentShadow();
    }
  }
});

onMounted(() => {
  if (shadow.value.fromList) isFromList.value = true;
  getBasicInfo();
  updateCurrentShadow();
});
</script>

<style scoped lang="scss">
.thing-view {
  display: flex;
  flex-direction: row;
  justify-content: space-between;
  align-items: center;

  width: 100%;
  height: 100%;
  min-height: 268px;

  .thing-view-left {
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    align-items: center;

    width: 240px;
    height: 100%;

    .thing-view-back {
      width: 100%;
      padding: 10px;
      .el-button {
        display: flex;
        flex-direction: row;
        justify-content: space-between;
        align-items: center;
        width: 100%;
        font-weight: 600;
      }
    }

    .thing-view-left-main {
      flex: 1;
      width: 100%;
      height: 0;
      padding: 0 10px 10px;
      overflow-x: hidden;
      overflow-y: auto;
      .thing-view-meta {
        width: 100%;
        overflow: hidden;
      }
      .thing-update-btn {
        width: 100%;
        height: 32px;
        margin-top: 2px;
        .el-button {
          width: 100%;
          font-weight: 300;
        }
      }
    }
  }

  .thing-view-right {
    flex: 1;
    width: 0;
    height: 100%;
    padding: 10px;
    overflow-x: hidden;
    overflow-y: auto;
  }
}
</style>

import { computed } from "vue";
import { useStore } from "vuex";
import { getThings, postThing, deleteThing, getDefaultShadow } from "@/apis";
import { useRoute } from "vue-router";

export default () => {
  const store = useStore();
  const route = useRoute();
  const things = computed(() => store.state.app.things);
  const shadowListUpdateTag = computed(
    () => store.state.app.shadowListUpdateTag
  );
  const selectedThingId = computed(() => route.params.thingId);
  const currentShadow = computed(() =>
    selectedThingId.value ? store.state.app.currentShadow : {}
  );

  const updateThings = async () => {
    try {
      let hasNext = true,
        pageIndex = 1;
      const things = [];
      while (hasNext) {
        const { data } = await getThings({
          pageIndex,
          pageSize: 9999,
          withAuthValue: true,
        });
        things.push(...data.content);
        if (things.length < data.total) {
          pageIndex++;
        } else {
          hasNext = false;
        }
      }
      // const raw = things[0];
      // for (let index = 0; index < 2000; index++) {
      //   things.push({
      //     ...raw,
      //     thingId: raw.thingId + "__" + index,
      //   });
      // }
      store.commit("app/setState", {
        things,
      });
      return true;
    } catch (error) {
      console.log("updateThings error:", error);
      return false;
    }
  };

  const addThing = async ({ thingId, password }) => {
    try {
      const res = await postThing({
        thingId,
        password,
      });
      console.log("addThing res:", res);
      updateThings();
      store.commit("app/setState", {
        shadowListUpdateTag: store.state.app.shadowListUpdateTag + 1,
      });
      return true;
    } catch (error) {
      console.log("addThing error:", error);
      return false;
    }
  };

  const requestUpdateShadowList = () => {
    store.commit("app/setState", {
      shadowListUpdateTag: store.state.app.shadowListUpdateTag + 1,
    });
  };

  const delThing = async (thingId) => {
    try {
      const res = await deleteThing(thingId);
      console.log("delThing res:", res);
      updateThings();
      requestUpdateShadowList();
      return true;
    } catch (error) {
      console.log("delThing error:", error);
      return false;
    }
  };

  const setCurrentShadow = (shadow) => {
    store.commit("app/setState", {
      currentShadow: shadow || {},
    });
  };

  const updateCurrentShadow = async () => {
    if (selectedThingId.value) {
      try {
        const res = await getDefaultShadow(selectedThingId.value);
        // console.log("getShadow", res);
        setCurrentShadow(res.data);
      } catch (error) {
        console.error("error", error);
      }
    } else {
      setCurrentShadow(null);
    }
  };

  return {
    route,
    things,
    shadowListUpdateTag,
    selectedThingId,
    currentShadow,
    requestUpdateShadowList,
    addThing,
    delThing,
    updateThings,
    setCurrentShadow,
    updateCurrentShadow,
  };
};

<script setup lang="ts">
import { useStore } from "vuex";
import Layout from "@/components/layout/Layout.vue";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import { getConfig } from "@/apis";
import { onBeforeMount } from "vue";

const store = useStore();
const { updateThings } = useThingsAndShadows();

onBeforeMount(async () => {
  if (store.state.user.auth) {
    updateThings();
    const c = await getConfig();
    store.dispatch("app/tioConfig", c.data);
  }
})
</script>

<template>
  <Layout />
</template>
<script setup lang="ts">
import { useStore } from "vuex";
import Layout from "@/components/layout/Layout.vue";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";

const store = useStore();
const { updateThings } = useThingsAndShadows();
if (store.state.user.auth) {
  updateThings();
}
</script>

<template>
  <Layout />
</template>
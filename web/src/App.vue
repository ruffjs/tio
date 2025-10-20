<script setup lang="ts">
import { useStore } from "vuex";
import { useI18n } from "vue-i18n";
import Layout from "@/components/layout/Layout.vue";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import { getConfig } from "@/apis";
import { onBeforeMount, computed } from "vue";
import zhCN from "element-plus/es/locale/lang/zh-cn";
import en from "element-plus/es/locale/lang/en";

const store = useStore();
const { locale } = useI18n();
const { updateThings } = useThingsAndShadows();

// 根据当前语言动态切换 Element Plus 语言包
const elementLocale = computed(() => {
  return locale.value === 'zh' ? zhCN : en;
});

onBeforeMount(async () => {
  if (store.state.user.auth) {
    updateThings();
    const c = await getConfig();
    store.dispatch("app/tioConfig", c.data);
  }
})
</script>

<template>
  <el-config-provider :locale="elementLocale">
    <Layout />
  </el-config-provider>
</template>
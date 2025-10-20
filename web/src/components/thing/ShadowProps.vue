<template>
  <div class="shadow-props">
    <div class="shadow-prop">
      <el-alert
        :title="$t('things.updatedAt')"
        :description="dayjs(shadow.updatedAt).format('MMM DD, YYYY HH:mm:ss')"
        :type="shadow.connected ? 'success' : 'info'"
        :closable="false"
      />
    </div>
    <div class="shadow-prop">
      <el-alert
        :title="$t('things.version')"
        :description="String(shadow.version)"
        :type="shadow.connected ? 'success' : 'info'"
        :closable="false"
      />
    </div>
    <div v-if="shadow.remoteAddr" class="shadow-prop">
      <el-alert
        :title="$t('things.remoteAddress')"
        :description="shadow.remoteAddr"
        :type="shadow.connected ? 'success' : 'info'"
        :closable="false"
      />
    </div>
    <div class="shadow-prop">
      <el-alert
        v-if="shadow.connected"
        :title="$t('things.connected')"
        :description="$t('mqtt.connected')"
        type="success"
        show-icon
        :closable="false"
      />
      <el-alert
        v-else
        :title="$t('things.connected')"
        :description="$t('mqtt.disconnected')"
        type="info"
        :closable="false"
      />
    </div>
  </div>
</template>

<script setup>
import dayjs from "dayjs";

defineProps({
  shadow: {
    type: Object,
    required: true,
  },
});
</script>

<style scoped lang="scss">
.shadow-props {
  display: flex;
  flex-direction: row-reverse;
  justify-content: end;
  align-items: center;

  width: 100%;
  height: 63px;

  .shadow-prop {
    min-width: 130px;
    font-weight: 600;
  }
  .shadow-prop + .shadow-prop {
    margin-right: 10px;
  }
}
</style>

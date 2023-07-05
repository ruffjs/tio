<template>
  <div class="things-list">
    <div class="things-list-container">
      <div class="things-list-items">
        <StandardTable v-if="isStandard" :data="items" />
        <CustomTable v-else :data="items" />
      </div>
    </div>
    <div class="things-list-paging">
      <el-pagination
        background
        small
        layout="total, prev, pager, next, sizes"
        :total="total"
        :current-page="pageIndex"
        :page-size="pageSize"
        :page-sizes="[20, 50, 100, 200]"
        @current-change="handlePageIndexChange"
        @size-change="handlePageSizeChange"
      />
    </div>
  </div>
</template>

<script setup>
import StandardTable from "./StandardTable.vue";
import CustomTable from "./CustomTable.vue";

const emit = defineEmits(["page-index-change", "page-size-change"]);
const props = defineProps({
  isStandard: Boolean,
  items: Array,
  pageIndex: Number,
  pageSize: Number,
  total: Number,
});

const handlePageIndexChange = (value) => {
  emit("page-index-change", value);
};
const handlePageSizeChange = (value) => {
  emit("page-size-change", value);
};
</script>

<style scoped lang="scss">
.things-list {
  display: flex;
  flex-direction: column;
  justify-content: start;
  align-items: center;
  width: 100%;
  height: 100%;
  .things-list-container {
    flex: 1;
    width: 100%;
    height: 0;
    overflow: hidden;
    .things-list-items {
      width: 100%;
      height: 100%;
    }
  }
  .things-list-paging {
    display: flex;
    justify-content: center;
    align-items: center;
    width: 100%;
    height: 40px;
    padding: 2px 15px 3px;
  }
}
</style>

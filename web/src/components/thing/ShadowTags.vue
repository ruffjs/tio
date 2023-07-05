<template>
  <el-card class="shadow-tags-card" shadow="never">
    <template #header>
      <div class="shadow-tags-card-header">
        <span
          >Shadow Tags
          <el-tag type="info" size="small" class="shadow-tags-card-header-tag"
            >Tag color show as blue while the value is not a string, or as gray.</el-tag
          ></span
        >
        <div class="shadow-tags-card-buttons">
          <el-button
            v-if="data"
            icon="View"
            size="small"
            @click="viewObject(data, 'Tags Raw')"
            >View Raw</el-button
          >
          <el-button icon="Plus" size="small" @click="emit('update')">Set Tags</el-button>
        </div>
      </div>
    </template>
    <div v-if="tags.length" class="shadow-tags-list">
      <el-tag
        v-for="tag in tags"
        :key="tag.key"
        :type="tag.type === 'warning' ? '' : tag.type"
        class="shadow-tags-item"
        closable
        @close="
          emit('update', {
            tags: { [tag.key]: null },
            version: 0,
          })
        "
      >
        <div class="shadow-tags-item-label">{{ tag.key }}</div>
        <div class="shadow-tags-item-value">
          <el-button
            v-if="tag.type === 'warning'"
            type="primary"
            size="small"
            link
            @click="viewObject(tag.value, tag.key + ' Tag')"
          >
            [Object]
          </el-button>
          <span v-else>{{ tag.value }}</span>
        </div>
      </el-tag>
    </div>
    <div v-else class="shadow-tags-empty">
      <p>No tags right now.</p>
      <el-button type="default" icon="Plus" size="small" @click="emit('update')"
        >Add Some Now</el-button
      >
    </div>
  </el-card>
  <ObjectViewer
    :visible="!!objectToBeView"
    :data="objectToBeView"
    :type="titleOfViewer"
    as-tree
    @close="handleCloseViewer"
  />
</template>

<script setup>
import { watch, ref } from "vue";
import ObjectViewer from "@/components/common/ObjectViewer.vue";
import useObjectViewer from "@/reactives/useObjectViewer";

const emit = defineEmits(["update"]);
const props = defineProps({
  data: {
    type: Object,
    default: null,
  },
});
const tags = ref([]);

const convertDataTypeToTagType = (value) => {
  const type = typeof value;
  switch (type) {
    case "number":
    case "bigint":
      return ["", value.toString()];

    case "boolean":
      return ["", value ? "true" : "false"];

    case "object":
      if (value === null) return ["primary", "null"];
      return ["warning", value];

    case "string":
      return ["info", value || "-"];

    case "undefined":
      return ["info", "undefined"];

    case "function":
      return ["info", "(Function)"];

    case "symbol":
      return ["info", "(Symbol)"];
  }
};

const {
  objectToBeView,
  titleOfViewer,
  viewObject,
  handleCloseViewer,
} = useObjectViewer();

watch(
  () => props.data,
  (data) => {
    if (data) {
      const keys = Object.keys(props.data);
      const _tags = [];
      keys.forEach((key) => {
        const [type, value] = convertDataTypeToTagType(data[key]);
        _tags.push({
          key,
          type,
          value,
        });
      });
      tags.value = _tags;
    }
  },
  { immediate: true }
);
</script>

<style scoped lang="scss">
.shadow-tags-card {
  width: 100%;
  margin-top: 10px;

  .shadow-tags-card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    .shadow-tags-card-header-tag {
      height: 18px;
      padding: 0px 4px;
      line-height: 16px;
      font-size: 12px;
    }
  }

  .shadow-tags-list {
    display: flex;
    flex-direction: row;
    justify-content: start;
    align-items: start;
    flex-wrap: wrap;
    gap: 10px;

    .shadow-tags-item {
      padding: 5px 5px 2px 10px;
      user-select: none;
      &.el-tag.is-closable {
        align-items: start;
      }
      .shadow-tags-item-label {
        line-height: 14px;
        font-size: 14px;
        font-weight: 700;
        cursor: default;
      }
      .shadow-tags-item-value {
        line-height: 30px;
        font-size: 12px;
        &:has(button) {
          line-height: 24px;
        }
        button {
          padding: 0;
        }
      }
    }
  }

  .shadow-tags-empty {
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;

    line-height: 30px;
    font-size: 12px;
    color: var(--el-color-info);
  }
}
</style>

<style lang="scss">
.shadow-tags-card {
  .el-card__header {
    padding: 10px var(--el-card-padding);
  }
  .shadow-tags-list {
    .shadow-tags-item {
      height: 46px;
      .el-tag__content {
        height: 34px;
      }
    }
  }
}
</style>

<template>
  <section class="shadow-tags-card">
    <div class="shadow-tags-card-header">
      <span>{{ $t('things.shadowTags') }}</span>
      <div class="shadow-tags-card-buttons">
        <el-button icon="Plus" size="small" @click="emit('update')">{{ $t('things.setTags') }}</el-button>
      </div>
    </div>
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
      <p>{{ $t('things.noTagsRightNow') }}</p>
    </div>
  </section>
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
  padding: 10px 16px;
  background: transparent;

  .shadow-tags-card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;

    > span {
      color: var(--tio-text-strong);
      font-size: 13px;
      font-weight: 750;
      letter-spacing: 0.02em;
    }
  }

  .shadow-tags-list {
    display: flex;
    flex-direction: row;
    justify-content: start;
    align-items: start;
    flex-wrap: wrap;
    gap: 8px;

    .shadow-tags-item {
      height: auto;
      min-height: 28px;
      max-width: 100%;
      padding: 2px 6px 2px 8px;
      border-radius: var(--tio-radius);
      user-select: none;

      &.el-tag.is-closable {
        align-items: center;
      }

      .shadow-tags-item-label {
        display: inline;
        overflow-wrap: anywhere;
        line-height: 20px;
        font-size: 12px;
        font-weight: 700;
        cursor: default;

        &::after {
          content: ":";
          margin-right: 4px;
        }
      }
      .shadow-tags-item-value {
        display: inline;
        min-width: 0;
        overflow-wrap: anywhere;
        line-height: 20px;
        font-size: 12px;
        &:has(button) {
          display: inline-flex;
          line-height: 20px;
        }
        button {
          padding: 0;
        }
      }
    }
  }

  .shadow-tags-empty {
    display: flex;
    justify-content: flex-start;
    align-items: center;
    min-height: 28px;
    color: var(--tio-muted);
    font-size: 12px;

    p {
      margin: 0;
    }
  }
}
</style>

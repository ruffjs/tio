<template>
  <div class="key-value-editor">
    <div class="editor-header">
      <span class="editor-title">{{ title }}</span>
      <el-button
        v-if="!disabled"
        icon="Plus"
        class="btn-props-plus"
        link
        @click="addItem"
      />
    </div>
    <div class="key-value-editor-rows" :style="{ 'max-height': maxHeight }">
      <div v-for="(item, index) in dataList" class="key-value-editor-row" :key="index">
        <a v-if="!disabled" class="btn-check" @click="checkItem(index)">
          <el-icon v-if="item.checked" class="el-icon-check"><Select /></el-icon>
          <el-icon v-else class="el-icon-check disable-icon"><Check /></el-icon>
        </a>
        <el-input
          placeholder="Key"
          size="small"
          :disabled="disabled"
          v-model="item.key"
          class="input-prop user-prop-key"
          @input="handleInputChange"
        />
        <el-input
          placeholder="Value"
          size="small"
          :disabled="disabled"
          v-model="item.value"
          class="input-prop user-prop-value"
          @input="handleInputChange"
        />
        <el-button
          v-if="!disabled"
          icon="Delete"
          class="btn-delete"
          link
          @click="deleteItem(index)"
        />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, onMounted } from "vue";

const emit = defineEmits(["update:modelValue"]);
const props = defineProps({
  title: {
    type: String,
    required: false,
    default: "",
  },
  maxHeight: {
    type: String,
    required: false,
    default: "100%",
  },
  disabled: {
    type: Boolean,
    required: false,
    default: false,
  },
  modelValue: Object,
});
const dataList = ref([]);

const addItem = () => {
  const list = dataList.value;
  list.push({ key: "", value: "", checked: true });
  dataList.value = list;
};
const deleteItem = (index) => {
  const list = dataList.value;
  if (list.length > 1) {
    list.splice(index, 1);
    dataList.value = list;
    handleInputChange();
  } else {
    dataList.value = [{ key: "", value: "", checked: true }];
    emit("update:modelValue", null);
  }
};
const checkItem = (index) => {
  const list = dataList.value;
  list[index].checked = !list[index].checked;
  dataList.value = list;
  handleInputChange();
};

const handleInputChange = () => {
  const checkedList = dataList.value.filter((pair) => pair.checked);
  const objData = {};
  checkedList.forEach(({ key, value }) => {
    if (key === "") return;
    const objValue = objData[key];
    if (objValue) {
      const _value = value;
      if (Array.isArray(objValue)) {
        objData[key] = [...objValue, _value];
      } else {
        objData[key] = [objValue, _value];
      }
    } else {
      objData[key] = value;
    }
  });
  emit("update:modelValue", objData);
};

const processObjToArry = () => {
  if (props.modelValue === undefined || props.modelValue === null) {
    dataList.value = [{ key: "", value: "", checked: true }];
    return;
  }
  const list = [];
  Object.entries(props.modelValue).forEach(([key, value]) => {
    if (typeof value === "string") {
      list.push({ key, value, checked: true });
    } else if (typeof value === "object" && value instanceof Array) {
      value.forEach((item) => {
        list.push({ key, value: item, checked: true });
      });
    }
  });
  if (list.length) {
    dataList.value = list;
  } else {
    dataList.value = [{ key: "", value: "", checked: false }];
  }
};
watch(
  () => props.modelValue,
  (val, oldVal) => {
    if (oldVal === undefined) {
      processObjToArry();
    }
  },
  {
    immediate: true,
  }
);
</script>

<style scoped lang="scss">
.key-value-editor {
  width: 100%;
  padding: 5px 10px 10px;
  border-radius: 4px;
  border: 1px solid var(--el-border-color);
  .editor-header {
    margin-bottom: 5px;
    .editor-title {
      color: var(--color-text-default);
    }
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .key-value-editor-rows {
    overflow-y: scroll;
    white-space: nowrap;
    .key-value-editor-row {
      display: flex;
      justify-content: space-between;
      align-items: center;
      &:not(:last-child) {
        margin-bottom: 10px;
      }
      .input-prop {
        padding: 0px;
        margin-right: 10px;
      }
      .btn-check {
        height: 24px;
        padding: 5px 0;
        line-height: 14px;
        cursor: pointer;
        .el-icon-check {
          font-size: 14px;
          margin-right: 10px;
        }
        .disable-icon {
          color: dimgray;
        }
      }
    }
  }
}
</style>

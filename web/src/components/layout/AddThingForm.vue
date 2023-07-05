<template>
  <el-dialog
    :model-value="true"
    width="480"
    title="Add Thing"
    append-to-body
    @close="emit('close')"
  >
    <el-form
      :model="form"
      :rules="rules"
      ref="formRef"
      @keyup.enter.native="handleConfirm"
    >
      <el-form-item label="Thing Id" prop="thingId" :label-width="formLabelWidth">
        <el-input
          v-model="form.thingId"
          autocomplete="off"
          placeholder="Please input thing id"
        />
      </el-form-item>
      <el-form-item label="Password" prop="password" :label-width="formLabelWidth">
        <el-input
          v-model="form.password"
          type="password"
          placeholder="Please input password"
          show-password
          autocomplete="off"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="emit('close')">Cancel</el-button>
        <el-button type="primary" @click="handleConfirm"> Confirm </el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { reactive, ref } from "vue";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";

const formLabelWidth = "100px";
const rules = {
  thingId: [
    { required: true, message: "Please input Thing Id", trigger: "change" },
    // { min: 1, max: 30, message: "Length should be 1 to 30", trigger: "change", },
    {
      validator: (_rule: any, value: any, callback: any) => {
        const reg = /^[A-Za-z0-9-_]+$/;
        if (value !== "" && value !== undefined && value !== null && !reg.test(value)) {
          callback(new Error("只能包含字母、数字、减号和下划线"));
        } else {
          callback();
        }
      },
      trigger: "change",
    },
  ],
  password: [
    {
      required: true,
      message: "Please input password",
      trigger: "change",
    },
    {
      validator: (_rule: any, value: any, callback: any) => {
        const reg = /^[A-Za-z0-9_~!@#$%^&*()-+./]+$/;
        if (value !== "" && value !== undefined && value !== null && !reg.test(value)) {
          callback(new Error("只能包含字母、数字、减号、点和下划线"));
        } else {
          callback();
        }
      },
      trigger: "change",
    },
  ],
};

const emit = defineEmits(["close"]);
const { addThing } = useThingsAndShadows();
const formRef = ref();
const form = reactive({
  thingId: "",
  password: "",
});

const handleConfirm = async () => {
  if (!formRef.value) return;
  try {
    const valid = await formRef.value.validate();
    if (valid) {
      if (await addThing(form)) {
        emit("close");
      }
    } else {
      console.log("error submit!");
      return false;
    }
  } catch (error) {
    console.error("validate error!", error);
  }
};
</script>
<style scoped>
.el-input {
  width: 300px;
}
.dialog-footer button:first-child {
  margin-right: 10px;
}
</style>

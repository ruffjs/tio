<template>
  <el-dialog
    :model-value="true"
    width="480"
    :title="t('things.addThing')"
    append-to-body
    @close="emit('close')"
  >
    <el-form
      :model="form"
      :rules="rules"
      ref="formRef"
      @keyup.enter.native="handleConfirm"
    >
      <el-form-item :label="t('things.thingId')" prop="thingId" :label-width="formLabelWidth">
        <el-input
          v-model="form.thingId"
          autocomplete="off"
          :placeholder="t('things.thingIdPlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="t('things.authType')" prop="authType" :label-width="formLabelWidth">
        <el-radio-group v-model="form.authType">
          <el-radio label="password">{{ t('things.authTypePassword') }}</el-radio>
          <el-radio label="certificate">{{ t('things.authTypeCertificate') }}</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item
        v-if="form.authType === 'password'"
        :label="t('login.password')"
        prop="password"
        :label-width="formLabelWidth"
      >
        <el-input
          v-model="form.password"
          type="password"
          :placeholder="t('things.passwordPlaceholder')"
          show-password
          autocomplete="off"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="emit('close')">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="handleConfirm">{{ t('common.confirm') }}</el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script lang="ts" setup>
import { reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";

const { t } = useI18n();

const formLabelWidth = "100px";
const rules = {
  thingId: [
    { required: true, message: "Please input Thing Id, which can contain number,letter, -, _ ", trigger: "change" },
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
      required: false,
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
  authType: "password",
});

watch(
  () => form.authType,
  (authType) => {
    if (authType === "certificate") {
      form.password = "";
    }
  }
);

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

<template>
  <div class="login-view">
    <div class="login-mask"></div>
    <dialog class="login-dialog" open>
      <el-form
        ref="formRef"
        v-loading="loading"
        :model="ruleForm"
        :rules="rules"
        status-icon
        label-width="90px"
        class="demo-ruleForm"
        @keyup.enter.native="submitForm"
      >
        <el-form-item label="Username" prop="user">
          <el-input v-model.number="ruleForm.user" autocomplete="off" />
        </el-form-item>
        <el-form-item label="Password" prop="pass">
          <el-input
            v-model="ruleForm.pass"
            type="password"
            show-password
            autocomplete="off"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="submitForm">Authorize</el-button>
        </el-form-item>
      </el-form>
    </dialog>
  </div>
</template>

<script>
export default {
  name: "Login",
  inheritAttrs: false,
  customOptions: { title: "登录TIO", zIndex: 2040, actived: false },
};
</script>

<script setup>
import { reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { useStore } from "vuex";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import { ElNotification } from "element-plus";

const loading = ref(false);
const checkName = (_rule, value, callback) => {
  if (!value) {
    return callback(new Error("Please input the username"));
  } else {
    callback();
  }
};
const validatePass = (_rule, value, callback) => {
  if (value === "") {
    callback(new Error("Please input the password"));
  } else {
    callback();
  }
};
const rules = reactive({
  user: [{ validator: checkName, trigger: "blur" }],
  pass: [{ validator: validatePass, trigger: "blur" }],
});

const store = useStore();
const router = useRouter();
const { updateThings } = useThingsAndShadows();
const formRef = ref();
const ruleForm = reactive({
  user: "admin",
  pass: "",
});

const submitForm = async () => {
  if (!formRef.value) return;
  try {
    const valid = await formRef.value.validate();
    if (valid) {
      console.log("submit!");
      loading.value = true;
      const base = `${ruleForm.user}:${ruleForm.pass}`;
      const auth = `Basic ${btoa(base)}`;
      localStorage.setItem("$tiopg/user/auth", auth);
      store.commit("user/setState", { auth });
      if (await updateThings()) {
        router.push("/");
      } else {
        ElNotification({
          title: "Login Failed",
          description: "Unkown error while login, please check your tio server.",
          type: "error",
        });
      }
    }
  } catch (error) {
    console.error("validate error!", error);
  } finally {
    loading.value = false;
  }
};
</script>

<style scoped lang="scss">
.login-view {
  width: 100%;
  height: 100%;
  .login-mask {
    width: 100%;
    height: 100%;
    background-color: rgba($color: #000000, $alpha: 0.3);
  }
  .login-dialog {
    position: absolute;
    top: 50vh;
    width: 400px;
    height: 200px;
    padding: 34px 34px;
    transform: translateY(-200px);
    border: none;
    border-radius: 2px;
    box-shadow: 0 0 5px 1px rgba($color: #000000, $alpha: 0.3);
  }
}
</style>

<template>
  <div class="login-view">
    <dialog class="login-dialog" open>
      <button class="login-theme-button" type="button" @click="toggleTheme" :title="`Switch to ${nextThemeMode}`">
        <el-icon>
          <Monitor v-if="themeMode === 'auto'" />
          <Sunny v-else-if="themeMode === 'dark'" />
          <Moon v-else />
        </el-icon>
      </button>
      <div class="login-brand">
        <img class="login-logo" :src="tioLogoUrl" alt="" aria-hidden="true" />
        <div>
          <h1>TIO Playground</h1>
          <p>{{ $t('login.login') }}</p>
        </div>
      </div>
        <el-form
          ref="formRef"
          v-loading="loading"
          :model="form"
          :rules="rules"
          status-icon
          label-position="top"
          class="login-form"
          @keyup.enter.native="submitForm"
        >
        <el-form-item :label="$t('login.host')" prop="host">
          <el-input v-model.number="form.host" autocomplete="off" />
        </el-form-item>
        <el-form-item :label="$t('login.username')" prop="user">
          <el-input v-model.number="form.user" autocomplete="off" />
        </el-form-item>
        <el-form-item :label="$t('login.password')" prop="pass">
          <el-input
            v-model="form.pass"
            type="password"
            show-password
            autocomplete="off"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="submitForm">{{ $t('login.login') }}</el-button>
        </el-form-item>
      </el-form>
    </dialog>
  </div>
</template>

<script>
export default {
  name: "Login",
  inheritAttrs: false,
  customOptions: { title: "Login TIO", zIndex: 1999, actived: false, standalone: true },
};
</script>

<script setup>
import { reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { useStore } from "vuex";
import { useI18n } from "vue-i18n";
import useThingsAndShadows from "@/reactives/useThingsAndShadows";
import { ElNotification } from "element-plus";
import { getUri, recreateClient } from "@/apis";
import useTheme from "@/reactives/useTheme";

const { t } = useI18n();
const { themeMode, nextThemeMode, toggleTheme } = useTheme();
const publicPath = import.meta.env.BASE_URL.endsWith("/")
  ? import.meta.env.BASE_URL
  : `${import.meta.env.BASE_URL}/`;
const tioLogoUrl = `${publicPath}tio-logo.svg`;

const loading = ref(false);
const checkName = (_rule, value, callback) => {
  if (!value) {
    return callback(new Error(t('login.usernameRequired')));
  } else {
    callback();
  }
};
const validatePass = (_rule, value, callback) => {
  if (value === "") {
    callback(new Error(t('login.passwordRequired')));
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
const form = reactive({
  host: localStorage.getItem("$tiopg/client/url") || "/",
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

      if (form.host !== getUri()) {
        recreateClient(form.host || "/");
      }

      const base = `${form.user}:${form.pass}`;
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
  position: relative;
  display: grid;
  place-items: center;
  width: 100%;
  min-height: 100vh;
  padding: 24px;
  overflow: hidden;
  background:
    radial-gradient(circle at 24% 18%, var(--tio-accent-soft), transparent 30%),
    var(--tio-bg);

  .login-dialog {
    position: relative;
    width: min(440px, calc(100vw - 32px));
    margin: 0;
    padding: 28px;
    border: 1px solid var(--tio-line);
    border-radius: var(--tio-radius-lg);
    background: var(--tio-surface);
    color: var(--tio-text);
    box-shadow: none;
  }

  .login-theme-button {
    position: absolute;
    top: 14px;
    right: 14px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 34px;
    height: 34px;
    padding: 0;
    border: 1px solid var(--tio-line);
    border-radius: var(--tio-radius);
    background: transparent;
    color: var(--tio-text);
    cursor: pointer;

    &:hover {
      border-color: var(--tio-accent-strong);
      background: var(--tio-accent-soft);
      color: var(--tio-text-strong);
    }
  }

  .login-brand {
    display: flex;
    align-items: center;
    gap: 14px;
    margin-bottom: 24px;
    padding-right: 44px;

    h1 {
      margin: 0;
      font-size: 24px;
      line-height: 1.1;
      color: var(--tio-text-strong);
    }

    p {
      margin: 5px 0 0;
      color: var(--tio-muted);
      font-size: 13px;
    }
  }

  .login-logo {
    width: 52px;
    height: 52px;
    flex: 0 0 auto;
  }

  .login-form {
    :deep(.el-form-item) {
      margin-bottom: 18px;
    }

    :deep(.el-form-item__label) {
      margin-bottom: 6px;
      color: var(--tio-muted);
      font-size: 12px;
      font-weight: 700;
      line-height: 1.2;
    }

    :deep(.el-input__wrapper) {
      min-height: 38px;
    }
  }

  :deep(.el-form-item:last-child) {
    margin-top: 6px;
    margin-bottom: 0;
  }

  :deep(.el-button) {
    width: 100%;
    min-height: 38px;
  }
}

:global(:root[data-theme="light"]) {
  .login-view {
    background:
      radial-gradient(circle at 24% 18%, rgba(82, 101, 125, 0.08), transparent 30%),
      var(--tio-bg);
  }
}

@media (max-width: 520px) {
  .login-view {
    padding: 16px;

    .login-dialog {
      width: 100%;
      padding: 22px;
    }

    .login-brand {
      align-items: flex-start;
      gap: 12px;
    }
  }
}
</style>

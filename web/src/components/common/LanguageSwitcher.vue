<template>
  <button
    type="button"
    class="nav-button language-switcher"
    :title="nextLanguage"
    @click="toggleLanguage"
  >
    <svg class="language-icon" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      <path
        d="M12.87 15.07l-2.54-2.51.03-.03c1.74-1.94 2.98-4.17 3.71-6.53H17V4h-7V2H8v2H1v2h11.17c-.67 1.93-1.73 3.76-3.17 5.36-.93-1.03-1.7-2.16-2.31-3.35h-2c.73 1.63 1.73 3.17 2.98 4.56L3 17.54 4.46 19l5-5 3.11 3.11.3-2.04zM18.5 10h-2L12 22h2l1.12-3h4.75L21 22h2l-4.5-12zm-2.62 7l1.62-4.33L19.12 17h-3.24z"
      />
    </svg>
    {{ currentLanguage }}
  </button>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { locale } = useI18n()

// 按钮上显示当前语言，点击切到另一种
const currentLanguage = computed(() => (locale.value === 'zh' ? '中' : 'EN'))
const nextLanguage = computed(() => (locale.value === 'zh' ? 'English' : '中文'))

const toggleLanguage = () => {
  locale.value = locale.value === 'zh' ? 'en' : 'zh'
  localStorage.setItem('tio-language', locale.value)
}
</script>

<style scoped lang="scss">
.language-switcher {
  .language-icon {
    width: 15px;
    height: 15px;
    margin-right: 4px;
  }
}
</style>

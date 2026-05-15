<template>
  <el-dropdown @command="handleCommand">
    <span class="language-switcher">
      <el-icon><Setting /></el-icon>
      {{ currentLanguage }}
      <el-icon class="el-icon--right"><arrow-down /></el-icon>
    </span>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item command="zh" :class="{ active: locale === 'zh' }">
          中文
        </el-dropdown-item>
        <el-dropdown-item command="en" :class="{ active: locale === 'en' }">
          English
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Setting, ArrowDown } from '@element-plus/icons-vue'

const { locale } = useI18n()

const currentLanguage = computed(() => {
  return locale.value === 'zh' ? '中文' : 'English'
})

const handleCommand = (command) => {
  locale.value = command
  localStorage.setItem('tio-language', command)
}
</script>

<style scoped lang="scss">
.language-switcher {
  display: flex;
  align-items: center;
  min-height: 34px;
  padding: 0 10px;
  border: 1px solid var(--tio-line);
  border-radius: var(--tio-radius);
  background: transparent;
  cursor: pointer;
  color: var(--tio-text);
  font-size: 12px;
  font-weight: 650;
  transition: background-color 0.18s ease, border-color 0.18s ease, color 0.18s ease;
  
  .el-icon {
    margin-right: 4px;
  }
  
  &:hover {
    border-color: var(--tio-accent-strong);
    background: var(--tio-accent-soft);
    color: var(--tio-text-strong);
  }
}

:deep(.el-dropdown-menu__item.active) {
  color: var(--el-color-primary);
  font-weight: bold;
}
</style>

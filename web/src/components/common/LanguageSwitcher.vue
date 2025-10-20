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
  cursor: pointer;
  color: #fff;
  font-size: 12px;
  
  .el-icon {
    margin-right: 4px;
  }
  
  &:hover {
    color: var(--el-color-primary);
  }
}

:deep(.el-dropdown-menu__item.active) {
  color: var(--el-color-primary);
  font-weight: bold;
}
</style>

import { createI18n } from 'vue-i18n'
import en from '../locales/en.json'
import zh from '../locales/zh.json'

const messages = {
  en,
  zh
}

// 从 localStorage 读取语言设置，默认为中文
const savedLocale = localStorage.getItem('tio-language') || 'zh'

const i18n = createI18n({
  legacy: false,
  locale: savedLocale,
  fallbackLocale: 'en',
  messages
})

export default i18n

import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import en from './en.json'
import ko from './ko.json'

const savedLocale = localStorage.getItem('nullus-locale')
const lng = savedLocale === 'ko' || savedLocale === 'en' ? savedLocale : 'en'

void i18n.use(initReactI18next).init({
  resources: {
    en: { translation: en },
    ko: { translation: ko },
  },
  lng,
  fallbackLng: 'en',
  interpolation: {
    escapeValue: false,
  },
})

i18n.on('languageChanged', (lang) => {
  localStorage.setItem('nullus-locale', lang)
})

export default i18n

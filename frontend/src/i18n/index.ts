import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import en from './en.json';
import ru from './ru.json';
import kz from './kz.json';

export const SUPPORTED_LANGS = ['en', 'ru', 'kz'] as const;
export type SupportedLang = (typeof SUPPORTED_LANGS)[number];

i18n.use(LanguageDetector).use(initReactI18next).init({
  fallbackLng: 'en',
  supportedLngs: [...SUPPORTED_LANGS],
  resources: {
    en: { translation: en },
    ru: { translation: ru },
    kz: { translation: kz },
  },
  detection: {
    order: ['localStorage', 'navigator'],
    lookupLocalStorage: 'sc.lang',
    caches: ['localStorage'],
  },
  interpolation: { escapeValue: false },
});

export default i18n;

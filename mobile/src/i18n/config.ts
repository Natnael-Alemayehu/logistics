import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import * as SecureStore from 'expo-secure-store';

import en from './en.json';
import am from './am.json';

const LANGUAGE_KEY = 'app_language';

const resources = {
  en: { translation: en },
  am: { translation: am },
};

const getStoredLanguage = async (): Promise<string | null> => {
  try {
    return await SecureStore.getItemAsync(LANGUAGE_KEY);
  } catch {
    return null;
  }
};

const storeLanguage = async (language: string): Promise<void> => {
  try {
    await SecureStore.setItemAsync(LANGUAGE_KEY, language);
  } catch {}
};

const languageDetector = {
  type: 'languageDetector' as const,
  async: true,
  detect: async (callback: (lng: string) => void) => {
    const storedLanguage = await getStoredLanguage();
    if (storedLanguage) {
      callback(storedLanguage);
      return;
    }
    callback('en');
  },
  init: () => {},
  cacheUserLanguage: async (lng: string) => {
    await storeLanguage(lng);
  },
};

i18n
  .use(languageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: 'en',
    supportedLngs: ['en', 'am'],
    interpolation: {
      escapeValue: false,
    },
    react: {
      useSuspense: false,
    },
  });

export default i18n;
export { useTranslation } from 'react-i18next';
export type { TFunction } from 'i18next';

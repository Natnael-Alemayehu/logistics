import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';
import * as SecureStore from 'expo-secure-store';
import { i18n } from '@/i18n';

interface SettingsState {
  language: 'en' | 'am';
  trackingEnabled: boolean;
  wifiOnlySync: boolean;
  notificationsEnabled: boolean;

  setLanguage: (lang: 'en' | 'am') => void;
  toggleTracking: () => void;
  toggleWifiOnlySync: () => void;
  toggleNotifications: () => void;
}

const secureStorage = {
  getItem: async (name: string): Promise<string | null> => {
    return SecureStore.getItemAsync(name);
  },
  setItem: async (name: string, value: string): Promise<void> => {
    await SecureStore.setItemAsync(name, value);
  },
  removeItem: async (name: string): Promise<void> => {
    await SecureStore.deleteItemAsync(name);
  },
};

export const useSettingsStore = create<SettingsState>()(
  persist(
    (set) => ({
      language: 'en',
      trackingEnabled: true,
      wifiOnlySync: false,
      notificationsEnabled: true,

      setLanguage: (lang) => {
        set({ language: lang });
        i18n.changeLanguage(lang);
      },

      toggleTracking: () => {
        set((state) => ({ trackingEnabled: !state.trackingEnabled }));
      },

      toggleWifiOnlySync: () => {
        set((state) => ({ wifiOnlySync: !state.wifiOnlySync }));
      },

      toggleNotifications: () => {
        set((state) => ({ notificationsEnabled: !state.notificationsEnabled }));
      },
    }),
    {
      name: 'settings-storage',
      storage: createJSONStorage(() => secureStorage),
    }
  )
);

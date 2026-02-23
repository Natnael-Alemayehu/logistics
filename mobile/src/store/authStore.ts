import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';
import * as SecureStore from 'expo-secure-store';
import { driverLogin, refreshToken as refreshTokenApi } from '@/services/api/auth';
import { api } from '@/services/api';
import { performInitialSync, InitialSyncResult } from '@/services/initialSync';
import * as syncMetadataRepo from '@/db/repositories/syncMetadata';
import type { User } from '@/types/user';

interface AuthState {
  user: User | null;
  accessToken: string | null;
  refreshToken: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  isInitialSyncing: boolean;
  initialSyncProgress: number;
  initialSyncError: string | null;

  login: (phone: string, pin: string) => Promise<InitialSyncResult | null>;
  logout: () => Promise<void>;
  refreshTokens: () => Promise<void>;
  setUser: (user: User | null) => void;
  setTokens: (accessToken: string, refreshToken: string) => void;
  setLoading: (loading: boolean) => void;
  clearAuth: () => void;
}

const ACCESS_TOKEN_KEY = 'access_token';
const REFRESH_TOKEN_KEY = 'refresh_token';

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

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,
      accessToken: null,
      refreshToken: null,
      isAuthenticated: false,
      isLoading: false,
      isInitialSyncing: false,
      initialSyncProgress: 0,
      initialSyncError: null,

      login: async (phone: string, pin: string) => {
        set({ isLoading: true, isInitialSyncing: false, initialSyncError: null });
        try {
          const response = await driverLogin(phone, pin);

          const accessToken = response.access_token;
          const refreshToken = response.refresh_token;
          const user: User = {
            id: response.user.id,
            email: '',
            name: response.user.name,
            role: 'driver',
            phone: response.user.phone,
          };

          await SecureStore.setItemAsync(ACCESS_TOKEN_KEY, accessToken);
          await SecureStore.setItemAsync(REFRESH_TOKEN_KEY, refreshToken);

          set({
            user,
            accessToken,
            refreshToken,
            isAuthenticated: true,
            isLoading: false,
            isInitialSyncing: true,
            initialSyncProgress: 0,
          });

          let syncResult: InitialSyncResult | null = null;
          try {
            syncResult = await performInitialSync(user.id, (progress) => {
              set({ initialSyncProgress: progress.percentage });
            });

            if (syncResult.success) {
              await syncMetadataRepo.upsert({
                driver_id: user.id,
                last_sync_at: new Date().toISOString(),
              });
            }

            set({
              isInitialSyncing: false,
              initialSyncError: syncResult.error ?? null,
            });
          } catch (syncError) {
            set({
              isInitialSyncing: false,
              initialSyncError: syncError instanceof Error ? syncError.message : 'Initial sync failed',
            });
          }

          return syncResult;
        } catch (error) {
          set({ isLoading: false, isInitialSyncing: false });
          throw error;
        }
      },

      logout: async () => {
        try {
          await api.post('/auth/logout', {});
        } catch {
          // Ignore logout API errors
        } finally {
          await SecureStore.deleteItemAsync(ACCESS_TOKEN_KEY);
          await SecureStore.deleteItemAsync(REFRESH_TOKEN_KEY);
          set({
            user: null,
            accessToken: null,
            refreshToken: null,
            isAuthenticated: false,
          });
        }
      },

      refreshTokens: async () => {
        const { refreshToken: currentRefreshToken } = get();
        if (!currentRefreshToken) {
          throw new Error('No refresh token available');
        }

        try {
          const response = await refreshTokenApi(currentRefreshToken);

          const newAccessToken = response.access_token;
          const newRefreshToken = response.refresh_token;

          await SecureStore.setItemAsync(ACCESS_TOKEN_KEY, newAccessToken);
          await SecureStore.setItemAsync(REFRESH_TOKEN_KEY, newRefreshToken);

          set({
            accessToken: newAccessToken,
            refreshToken: newRefreshToken,
          });
        } catch (error) {
          get().clearAuth();
          throw error;
        }
      },

      setUser: (user) => {
        set({
          user,
          isAuthenticated: !!user,
        });
      },

      setTokens: (accessToken, refreshToken) => {
        set({
          accessToken,
          refreshToken,
        });
      },

      setLoading: (loading) => {
        set({ isLoading: loading });
      },

      clearAuth: () => {
        SecureStore.deleteItemAsync(ACCESS_TOKEN_KEY);
        SecureStore.deleteItemAsync(REFRESH_TOKEN_KEY);
        set({
          user: null,
          accessToken: null,
          refreshToken: null,
          isAuthenticated: false,
          isInitialSyncing: false,
          initialSyncProgress: 0,
          initialSyncError: null,
        });
      },
    }),
    {
      name: 'auth-storage',
      storage: createJSONStorage(() => secureStorage),
      partialize: (state) => ({
        user: state.user,
        isAuthenticated: state.isAuthenticated,
      }),
    }
  )
);

export const restoreAuthSession = async (): Promise<boolean> => {
  try {
    const [accessToken, refreshToken] = await Promise.all([
      SecureStore.getItemAsync(ACCESS_TOKEN_KEY),
      SecureStore.getItemAsync(REFRESH_TOKEN_KEY),
    ]);

    if (accessToken && refreshToken) {
      useAuthStore.setState({
        accessToken,
        refreshToken,
        isAuthenticated: true,
        isLoading: false,
      });
      return true;
    }

    useAuthStore.setState({ isLoading: false });
    return false;
  } catch {
    useAuthStore.setState({ isLoading: false });
    return false;
  }
};

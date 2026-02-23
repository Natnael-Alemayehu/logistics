import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';
import * as SecureStore from 'expo-secure-store';
import { api } from '@/services/api';
import type { User, AuthTokens } from '@/types/user';

interface AuthState {
  user: User | null;
  accessToken: string | null;
  refreshToken: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;

  login: (phone: string, pin: string) => Promise<void>;
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

      login: async (phone: string, pin: string) => {
        set({ isLoading: true });
        try {
          const response = await api.post<{ user: User; tokens: AuthTokens }>(
            '/auth/login',
            { phone, pin }
          );

          const { user, tokens } = response;

          await SecureStore.setItemAsync(ACCESS_TOKEN_KEY, tokens.accessToken);
          await SecureStore.setItemAsync(REFRESH_TOKEN_KEY, tokens.refreshToken);

          set({
            user,
            accessToken: tokens.accessToken,
            refreshToken: tokens.refreshToken,
            isAuthenticated: true,
            isLoading: false,
          });
        } catch (error) {
          set({ isLoading: false });
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
        const { refreshToken } = get();
        if (!refreshToken) {
          throw new Error('No refresh token available');
        }

        try {
          const response = await api.post<{ tokens: AuthTokens }>(
            '/auth/refresh',
            { refreshToken }
          );

          const { tokens } = response;

          await SecureStore.setItemAsync(ACCESS_TOKEN_KEY, tokens.accessToken);
          await SecureStore.setItemAsync(REFRESH_TOKEN_KEY, tokens.refreshToken);

          set({
            accessToken: tokens.accessToken,
            refreshToken: tokens.refreshToken,
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

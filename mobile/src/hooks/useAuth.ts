import { useCallback, useEffect, useState } from 'react';
import { useAuthStore, restoreAuthSession } from '@/store/authStore';
import type { User } from '@/types/user';

export function useAuth() {
  const {
    user,
    accessToken,
    refreshToken,
    isAuthenticated,
    isLoading,
    login,
    logout,
    refreshTokens,
    setUser,
    setTokens,
    clearAuth,
    setLoading,
  } = useAuthStore();

  const [isRestoring, setIsRestoring] = useState(true);

  useEffect(() => {
    restoreSession();
  }, []);

  const restoreSession = useCallback(async () => {
    setIsRestoring(true);
    try {
      await restoreAuthSession();
    } finally {
      setIsRestoring(false);
    }
  }, []);

  const isLoggedIn = isAuthenticated && !!accessToken && !!user;

  const isDriver = user?.role === 'driver';

  const isAdmin = user?.role === 'admin';

  const hasRole = useCallback(
    (role: User['role']) => user?.role === role,
    [user?.role]
  );

  return {
    user,
    accessToken,
    refreshToken,
    isAuthenticated,
    isLoading,
    isRestoring,
    isLoggedIn,
    isDriver,
    isAdmin,
    hasRole,
    login,
    logout,
    refreshTokens,
    setUser,
    setTokens,
    clearAuth,
    setLoading,
    restoreSession,
  };
}

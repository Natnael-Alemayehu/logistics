import { useCallback, useState } from 'react';
import { Alert, ToastAndroid, Platform } from 'react-native';
import { isNetworkError, isAuthError, AppError, ERROR_CODES } from '../utils/errors';
import { useAuthStore } from '../store/authStore';
import { router } from 'expo-router';
import { i18n } from '../i18n';

interface ErrorHandlerResult {
  error: Error | null;
  handleError: (error: unknown) => void;
  showErrorMessage: (message: string) => void;
  clearError: () => void;
}

export type { ErrorHandlerResult };

export function useErrorHandler(): ErrorHandlerResult {
  const [error, setError] = useState<Error | null>(null);
  const logout = useAuthStore((state) => state.logout);

  const showErrorMessage = useCallback((message: string) => {
    if (Platform.OS === 'android') {
      ToastAndroid.show(message, ToastAndroid.LONG);
    } else {
      Alert.alert(i18n.t('common.error'), message);
    }
  }, []);

  const handleError = useCallback((err: unknown) => {
    let error: Error;
    let message: string;

    if (err instanceof Error) {
      error = err;
    } else if (typeof err === 'string') {
      error = new Error(err);
    } else {
      error = new Error(i18n.t('errors.message'));
    }

    console.error('Error handled:', error);
    setError(error);

    if (error instanceof AppError) {
      message = error.message;
    } else if (isNetworkError(error)) {
      message = i18n.t('errors.network');
    } else if (isAuthError(error)) {
      message = i18n.t('errors.unauthorized');
      logout();
      router.replace('/(auth)');
      showErrorMessage(message);
      return;
    } else {
      message = error.message || i18n.t('errors.message');
    }

    showErrorMessage(message);
  }, [logout, showErrorMessage]);

  const clearError = useCallback(() => {
    setError(null);
  }, []);

  return {
    error,
    handleError,
    showErrorMessage,
    clearError,
  };
}

export function createAppError(code: keyof typeof ERROR_CODES, originalError?: Error): AppError {
  return new AppError(ERROR_CODES[code], undefined, originalError);
}

export default useErrorHandler;

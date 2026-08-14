export const ERROR_CODES = {
  UNKNOWN: 'UNKNOWN',
  NETWORK_ERROR: 'NETWORK_ERROR',
  TIMEOUT: 'TIMEOUT',
  UNAUTHORIZED: 'UNAUTHORIZED',
  FORBIDDEN: 'FORBIDDEN',
  NOT_FOUND: 'NOT_FOUND',
  VALIDATION_ERROR: 'VALIDATION_ERROR',
  SERVER_ERROR: 'SERVER_ERROR',
  GPS_PERMISSION_DENIED: 'GPS_PERMISSION_DENIED',
  GPS_UNAVAILABLE: 'GPS_UNAVAILABLE',
  GPS_TIMEOUT: 'GPS_TIMEOUT',
  CAMERA_PERMISSION_DENIED: 'CAMERA_PERMISSION_DENIED',
  STORAGE_PERMISSION_DENIED: 'STORAGE_PERMISSION_DENIED',
  SYNC_FAILED: 'SYNC_FAILED',
  OFFLINE: 'OFFLINE',
  SESSION_EXPIRED: 'SESSION_EXPIRED',
  INVALID_CREDENTIALS: 'INVALID_CREDENTIALS',
  ACCOUNT_LOCKED: 'ACCOUNT_LOCKED',
  TASK_UPDATE_FAILED: 'TASK_UPDATE_FAILED',
  PHOTO_UPLOAD_FAILED: 'PHOTO_UPLOAD_FAILED',
  LOCATION_VERIFICATION_FAILED: 'LOCATION_VERIFICATION_FAILED',
} as const;

export type ErrorCode = (typeof ERROR_CODES)[keyof typeof ERROR_CODES];

const ERROR_MESSAGES: Record<ErrorCode, string> = {
  UNKNOWN: 'An unexpected error occurred. Please try again.',
  NETWORK_ERROR: 'Network connection error. Please check your internet connection.',
  TIMEOUT: 'Request timed out. Please try again.',
  UNAUTHORIZED: 'You are not authorized to perform this action.',
  FORBIDDEN: 'Access denied. You do not have permission for this action.',
  NOT_FOUND: 'The requested resource was not found.',
  VALIDATION_ERROR: 'Please check your input and try again.',
  SERVER_ERROR: 'Server error. Please try again later.',
  GPS_PERMISSION_DENIED: 'Location permission is required for this feature.',
  GPS_UNAVAILABLE: 'GPS is unavailable. Please check your device settings.',
  GPS_TIMEOUT: 'Unable to get your location. Please try again.',
  CAMERA_PERMISSION_DENIED: 'Camera permission is required to take photos.',
  STORAGE_PERMISSION_DENIED: 'Storage permission is required for this feature.',
  SYNC_FAILED: 'Failed to sync data. Will retry automatically.',
  OFFLINE: 'You are offline. Some features may be unavailable.',
  SESSION_EXPIRED: 'Your session has expired. Please log in again.',
  INVALID_CREDENTIALS: 'Invalid phone number or PIN.',
  ACCOUNT_LOCKED: 'Your account has been locked. Please contact support.',
  TASK_UPDATE_FAILED: 'Failed to update task. Please try again.',
  PHOTO_UPLOAD_FAILED: 'Failed to upload photo. Please try again.',
  LOCATION_VERIFICATION_FAILED: 'Unable to verify your location. Please move closer to the destination.',
};

export function getErrorMessage(code: string): string {
  return ERROR_MESSAGES[code as ErrorCode] || ERROR_MESSAGES.UNKNOWN;
}

export function isNetworkError(error: Error): boolean {
  const networkErrorMessages = [
    'network',
    'Network request failed',
    'Failed to fetch',
    'ECONNREFUSED',
    'ENOTFOUND',
    'ETIMEDOUT',
    'ERR_NETWORK',
  ];

  const message = error.message.toLowerCase();
  return networkErrorMessages.some((msg) => message.includes(msg.toLowerCase()));
}

export function isAuthError(error: Error): boolean {
  const authErrorCodes = [401, 403];
  const authErrorMessages = ['unauthorized', 'forbidden', 'session expired', 'invalid token'];

  const message = error.message.toLowerCase();

  if (authErrorCodes.some((code) => message.includes(String(code)))) {
    return true;
  }

  return authErrorMessages.some((msg) => message.includes(msg));
}

export function isOfflineError(error: Error): boolean {
  const offlineMessages = ['offline', 'no internet', 'network unavailable', 'ERR_INTERNET_DISCONNECTED'];
  const message = error.message.toLowerCase();
  return offlineMessages.some((msg) => message.includes(msg));
}

export function isGPSError(error: Error): boolean {
  const gpsCodes = [
    ERROR_CODES.GPS_PERMISSION_DENIED,
    ERROR_CODES.GPS_UNAVAILABLE,
    ERROR_CODES.GPS_TIMEOUT,
  ];
  const message = error.message.toLowerCase();
  return gpsCodes.some((code) => message.includes(code.toLowerCase()));
}

export class AppError extends Error {
  code: ErrorCode;
  originalError?: Error;

  constructor(code: ErrorCode, message?: string, originalError?: Error) {
    super(message || getErrorMessage(code));
    this.name = 'AppError';
    this.code = code;
    this.originalError = originalError;
  }
}
import { Platform, Dimensions } from 'react-native';

export const { width: SCREEN_WIDTH, height: SCREEN_HEIGHT } = Dimensions.get('window');

export const isIOS = Platform.OS === 'ios';
export const isAndroid = Platform.OS === 'android';

export const Colors = {
  primary: '#2563eb',
  secondary: '#6b7280',
  success: '#22c55e',
  warning: '#f59e0b',
  error: '#ef4444',
  info: '#3b82f6',
  background: '#f9fafb',
  surface: '#ffffff',
  text: '#111827',
  textSecondary: '#6b7280',
  border: '#e5e7eb',
  gray: {
    50: '#f9fafb',
    100: '#f3f4f6',
    200: '#e5e7eb',
    300: '#d1d5db',
    400: '#9ca3af',
    500: '#6b7280',
    600: '#4b5563',
    700: '#374151',
    800: '#1f2937',
    900: '#111827',
  },
};

export const Spacing = {
  xs: 4,
  sm: 8,
  md: 12,
  lg: 16,
  xl: 20,
  xxl: 24,
  xxxl: 32,
};

export const FontSize = {
  xs: 10,
  sm: 12,
  md: 14,
  lg: 16,
  xl: 18,
  xxl: 20,
  xxxl: 24,
  display: 32,
};

export const BorderRadius = {
  none: 0,
  sm: 4,
  md: 8,
  lg: 12,
  xl: 16,
  xxl: 24,
  full: 9999,
};

export const Shadows = {
  none: {},
  sm: {
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 1 },
    shadowOpacity: 0.05,
    shadowRadius: 2,
    elevation: 1,
  },
  md: {
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 4,
    elevation: 3,
  },
  lg: {
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 4 },
    shadowOpacity: 0.15,
    shadowRadius: 8,
    elevation: 5,
  },
  xl: {
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 8 },
    shadowOpacity: 0.2,
    shadowRadius: 16,
    elevation: 8,
  },
};

export const StatusColors = {
  pending: Colors.warning,
  in_progress: Colors.info,
  in_transit: Colors.info,
  completed: Colors.success,
  delivered: Colors.success,
  cancelled: Colors.error,
  failed: Colors.error,
  on_hold: Colors.gray[500],
  returned: Colors.gray[600],
};

export const Typography = {
  heading: {
    fontSize: FontSize.xxxl,
    fontWeight: 'bold' as const,
  },
  title: {
    fontSize: FontSize.xl,
    fontWeight: '600' as const,
  },
  subtitle: {
    fontSize: FontSize.lg,
    fontWeight: '500' as const,
  },
  body: {
    fontSize: FontSize.md,
    fontWeight: 'normal' as const,
  },
  caption: {
    fontSize: FontSize.sm,
    fontWeight: 'normal' as const,
  },
  label: {
    fontSize: FontSize.sm,
    fontWeight: '500' as const,
  },
};

export type ColorKey = keyof typeof Colors;
export type StatusKey = keyof typeof StatusColors;

export const colors = Colors;
export const spacing = Spacing;
export const typography = Typography;
import { ConfigContext, ExpoConfig } from 'expo/config'

export default ({ config }: ConfigContext): ExpoConfig => {
  return {
    ...config,
    name: 'Logistics Mobile',
    slug: 'logistics-mobile',
    version: '1.0.0',
    orientation: 'portrait',
    icon: './assets/icon.png',
    userInterfaceStyle: 'automatic',
    newArchEnabled: true,
    scheme: 'logistics',
    splash: {
      image: './assets/splash-icon.png',
      resizeMode: 'contain',
      backgroundColor: '#2563eb',
    },
    ios: {
      supportsTablet: true,
      bundleIdentifier: 'com.logistics.mobile',
      infoPlist: {
        NSLocationWhenInUseUsageDescription:
          'This app needs access to your location to track deliveries and provide real-time updates.',
        NSLocationAlwaysAndWhenInUseUsageDescription:
          'This app needs access to your location to track deliveries in the background.',
        NSLocationAlwaysUsageDescription:
          'This app needs access to your location to track deliveries in the background.',
        NSCameraUsageDescription:
          'This app needs access to your camera to capture proof of delivery photos.',
        NSPhotoLibraryUsageDescription:
          'This app needs access to your photo library to save proof of delivery photos.',
        UIBackgroundModes: ['location', 'fetch', 'remote-notification'],
        MGLMapboxMetricsEnabledSettingShownInApp: true,
      },
    },
    android: {
      package: 'com.logistics.mobile',
      adaptiveIcon: {
        foregroundImage: './assets/adaptive-icon.png',
        backgroundColor: '#2563eb',
      },
      edgeToEdgeEnabled: true,
      permissions: [
        'android.permission.ACCESS_COARSE_LOCATION',
        'android.permission.ACCESS_FINE_LOCATION',
        'android.permission.ACCESS_BACKGROUND_LOCATION',
        'android.permission.CAMERA',
        'android.permission.READ_EXTERNAL_STORAGE',
        'android.permission.WRITE_EXTERNAL_STORAGE',
        'android.permission.RECEIVE_BOOT_COMPLETED',
        'android.permission.WAKE_LOCK',
        'android.permission.FOREGROUND_SERVICE',
        'android.permission.FOREGROUND_SERVICE_LOCATION',
      ],
    },
    web: {
      favicon: './assets/favicon.png',
      bundler: 'metro',
    },
    plugins: [
      '@maplibre/maplibre-react-native',
      [
        'expo-router',
        {
          root: './src/app',
        },
      ],
      [
        'expo-secure-store',
        {
          configureAndroidBackup: true,
          faceIDPermission: 'Allow $(PRODUCT_NAME) to use Face ID.',
        },
      ],
      'expo-sqlite',
      [
        'expo-location',
        {
          locationAlwaysAndWhenInUsePermission:
            'Allow $(PRODUCT_NAME) to use your location.',
          locationAlwaysPermission: 'Allow $(PRODUCT_NAME) to use your location.',
          locationWhenInUsePermission: 'Allow $(PRODUCT_NAME) to use your location.',
        },
      ],
      [
        'expo-camera',
        {
          cameraPermission:
            'Allow $(PRODUCT_NAME) to access your camera for proof of delivery photos.',
        },
      ],
      [
        'expo-notifications',
        {
          icon: './assets/notification-icon.png',
          color: '#2563eb',
          sounds: [],
        },
      ],
    ],
    extra: {
      eas: {
        projectId: 'logistics-mobile',
      },
      apiUrl: process.env.EXPO_PUBLIC_API_URL || 'http://localhost:8080',
    },
  }
}

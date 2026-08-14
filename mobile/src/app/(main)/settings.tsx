import { 
  View, 
  Text, 
  StyleSheet, 
  ScrollView, 
  Pressable, 
  Alert,
  Switch,
  ActivityIndicator,
} from 'react-native';
import { Stack } from 'expo-router';
import { useSettingsStore } from '@store/settingsStore';
import { useNetworkStore } from '@store/networkStore';
import { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { cacheService, CacheInfo } from '@/services/cache/CacheService';
import { stopLocationTracking, isLocationTrackingActive } from '@/services/location';
import { useNotifications } from '@/hooks/useNotifications';
import { syncService } from '@/services/sync/SyncService';
import { getCurrentConnectionInfo } from '@/services/connectivity';

export default function SettingsScreen() {
  const { t } = useTranslation();
  const language = useSettingsStore((state) => state.language);
  const setLanguage = useSettingsStore((state) => state.setLanguage);
  const trackingEnabled = useSettingsStore((state) => state.trackingEnabled);
  const toggleTracking = useSettingsStore((state) => state.toggleTracking);
  const wifiOnlySync = useSettingsStore((state) => state.wifiOnlySync);
  const toggleWifiOnlySync = useSettingsStore((state) => state.toggleWifiOnlySync);
  const notificationsEnabled = useSettingsStore((state) => state.notificationsEnabled);
  const toggleNotifications = useSettingsStore((state) => state.toggleNotifications);
  const isOnline = useNetworkStore((state) => state.isOnline);
  const connectionType = useNetworkStore((state) => state.connectionType);
  
  const { unregisterFromBackend, registerForPushNotifications } = useNotifications();

  const [cacheInfo, setCacheInfo] = useState<CacheInfo | null>(null);
  const [isLoadingCache, setIsLoadingCache] = useState(true);
  const [isClearingCache, setIsClearingCache] = useState(false);

  const loadCacheInfo = useCallback(async () => {
    setIsLoadingCache(true);
    try {
      const info = await cacheService.getCacheInfo();
      setCacheInfo(info);
    } catch (error) {
      console.error('Failed to load cache info:', error);
    } finally {
      setIsLoadingCache(false);
    }
  }, []);

  useEffect(() => {
    loadCacheInfo();
  }, [loadCacheInfo]);

  const handleLanguageChange = () => {
    Alert.alert(t('settings.language'), t('settings.selectLanguage'), [
      {
        text: t('settings.english'),
        onPress: () => setLanguage('en'),
        style: language === 'en' ? 'default' : undefined,
      },
      {
        text: t('settings.amharic'),
        onPress: () => setLanguage('am'),
        style: language === 'am' ? 'default' : undefined,
      },
      { text: t('common.cancel'), style: 'cancel' },
    ]);
  };

  const handleTrackingToggle = useCallback(async (newValue: boolean) => {
    if (!newValue) {
      Alert.alert(
        t('settings.disableTracking'),
        t('settings.disableTrackingConfirm'),
        [
          { text: t('common.cancel'), style: 'cancel' },
          {
            text: t('settings.disable'),
            style: 'destructive',
            onPress: async () => {
              const isActive = await isLocationTrackingActive();
              if (isActive) {
                await stopLocationTracking();
              }
              toggleTracking();
              Alert.alert(t('common.success'), t('settings.trackingDisabled'));
            },
          },
        ]
      );
    } else {
      toggleTracking();
      Alert.alert(t('common.success'), t('settings.trackingEnabled'));
    }
  }, [toggleTracking, t]);

  const handleWifiOnlySyncToggle = useCallback(async (newValue: boolean) => {
    if (newValue) {
      const connectionInfo = await getCurrentConnectionInfo();
      if (connectionInfo.connectionType !== 'wifi') {
        Alert.alert(
          t('settings.wifiOnlySync'),
          t('settings.wifiOnlySyncWarning'),
          [
            { text: t('common.cancel'), style: 'cancel' },
            {
              text: t('settings.enable'),
              onPress: () => {
                toggleWifiOnlySync();
                Alert.alert(t('common.success'), t('settings.wifiOnlySyncEnabled'));
              },
            },
          ]
        );
      } else {
        toggleWifiOnlySync();
        Alert.alert(t('common.success'), t('settings.wifiOnlySyncEnabled'));
      }
    } else {
      toggleWifiOnlySync();
      if (isOnline) {
        Alert.alert(t('common.success'), t('settings.wifiOnlySyncDisabled'));
      }
    }
  }, [toggleWifiOnlySync, isOnline, t]);

  const handleNotificationsToggle = useCallback(async (newValue: boolean) => {
    if (!newValue) {
      Alert.alert(
        t('settings.disableNotifications'),
        t('settings.disableNotificationsConfirm'),
        [
          { text: t('common.cancel'), style: 'cancel' },
          {
            text: t('settings.disable'),
            style: 'destructive',
            onPress: async () => {
              await unregisterFromBackend();
              toggleNotifications();
              Alert.alert(t('common.success'), t('settings.notificationsDisabled'));
            },
          },
        ]
      );
    } else {
      toggleNotifications();
      const success = await registerForPushNotifications();
      if (success) {
        Alert.alert(t('common.success'), t('settings.notificationsEnabled'));
      }
    }
  }, [toggleNotifications, unregisterFromBackend, registerForPushNotifications, t]);

  const handleClearCache = () => {
    Alert.alert(
      t('settings.clearCache'),
      t('settings.clearCacheConfirm'),
      [
        { text: t('common.cancel'), style: 'cancel' },
        {
          text: t('settings.clearSyncedData'),
          onPress: () => performCacheClear({ clearSyncedEvents: true, clearOldEventsDays: 7 }),
        },
        {
          text: t('settings.clearAll'),
          style: 'destructive',
          onPress: () => performCacheClear({ clearSyncedEvents: true, clearOldEventsDays: 0, clearPhotos: true }),
        },
      ]
    );
  };

  const performCacheClear = async (options: { clearSyncedEvents?: boolean; clearOldEventsDays?: number; clearPhotos?: boolean }) => {
    setIsClearingCache(true);
    try {
      const result = await cacheService.clearCache(options);
      await loadCacheInfo();
      
      const message = t('settings.cacheClearedDetails', {
        events: result.clearedEvents,
        photos: result.clearedPhotos,
        freed: `${(result.freedBytes / 1024 / 1024).toFixed(1)} MB`,
      });
      
      Alert.alert(t('common.success'), message);
    } catch (error) {
      Alert.alert(t('common.error'), t('settings.cacheClearFailed'));
    } finally {
      setIsClearingCache(false);
    }
  };

  const handleAbout = () => {
    Alert.alert(
      t('settings.about'),
      `${t('settings.version')}: 1.0.0\nBuild: 2024.01\n\n${t('settings.support')}:\nsupport@logistics.et`,
      [{ text: 'OK' }]
    );
  };

  return (
    <>
      <Stack.Screen
        options={{
          title: t('settings.title'),
          headerStyle: { backgroundColor: '#2563eb' },
          headerTintColor: '#fff',
        }}
      />
      <ScrollView style={styles.container}>
        <View style={styles.section}>
          <Text style={styles.sectionTitle}>{t('settings.language')}</Text>
          <Pressable style={styles.settingItem} onPress={handleLanguageChange}>
            <Text style={styles.settingLabel}>{t('settings.appLanguage')}</Text>
            <View style={styles.settingValue}>
              <Text style={styles.settingValueText}>
                {language === 'en' ? t('settings.english') : t('settings.amharic')}
              </Text>
              <Text style={styles.chevron}>›</Text>
            </View>
          </Pressable>
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>{t('settings.notifications')}</Text>
          <View style={styles.settingItem}>
            <Text style={styles.settingLabel}>{t('settings.pushNotifications')}</Text>
            <Switch
              value={notificationsEnabled}
              onValueChange={handleNotificationsToggle}
              trackColor={{ false: '#d1d5db', true: '#93c5fd' }}
              thumbColor={notificationsEnabled ? '#2563eb' : '#f4f3f4'}
            />
          </View>
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>{t('settings.tracking')}</Text>
          <View style={styles.settingItem}>
            <View>
              <Text style={styles.settingLabel}>{t('settings.backgroundTracking')}</Text>
              <Text style={styles.settingHint}>
                {t('settings.backgroundTrackingHint')}
              </Text>
            </View>
            <Switch
              value={trackingEnabled}
              onValueChange={handleTrackingToggle}
              trackColor={{ false: '#d1d5db', true: '#93c5fd' }}
              thumbColor={trackingEnabled ? '#2563eb' : '#f4f3f4'}
            />
          </View>
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>{t('settings.dataSync')}</Text>
          <View style={styles.settingItem}>
            <View>
              <Text style={styles.settingLabel}>{t('settings.wifiOnlySync')}</Text>
              <Text style={styles.settingHint}>
                {t('settings.wifiOnlySyncHint')}
              </Text>
            </View>
            <Switch
              value={wifiOnlySync}
              onValueChange={handleWifiOnlySyncToggle}
              trackColor={{ false: '#d1d5db', true: '#93c5fd' }}
              thumbColor={wifiOnlySync ? '#2563eb' : '#f4f3f4'}
            />
          </View>
          <Pressable 
            style={styles.settingItem} 
            onPress={handleClearCache}
            disabled={isClearingCache}
          >
            <Text style={styles.settingLabel}>{t('settings.clearCache')}</Text>
            <View style={styles.settingValue}>
              {isClearingCache ? (
                <ActivityIndicator size="small" color="#2563eb" />
              ) : (
                <>
                  <Text style={styles.settingValueText}>
                    {isLoadingCache ? '...' : cacheInfo?.formattedSize ?? '0 MB'}
                  </Text>
                  <Text style={styles.chevron}>›</Text>
                </>
              )}
            </View>
          </Pressable>
          {cacheInfo && (
            <View style={styles.cacheDetails}>
              <Text style={styles.cacheDetailText}>
                {t('settings.trackingEvents')}: {cacheInfo.trackingEventsCount}
              </Text>
              <Text style={styles.cacheDetailText}>
                {t('settings.syncedEvents')}: {cacheInfo.syncedEventsCount}
              </Text>
              <Text style={styles.cacheDetailText}>
                {t('settings.photos')}: {cacheInfo.podsCount}
              </Text>
            </View>
          )}
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>{t('settings.about')}</Text>
          <Pressable style={styles.settingItem} onPress={handleAbout}>
            <Text style={styles.settingLabel}>{t('settings.appVersion')}</Text>
            <Text style={styles.settingValueText}>1.0.0</Text>
          </Pressable>
          <Pressable style={styles.settingItem} onPress={handleAbout}>
            <Text style={styles.settingLabel}>{t('settings.support')}</Text>
            <View style={styles.settingValue}>
              <Text style={styles.settingValueText}>{t('settings.contactUs')}</Text>
              <Text style={styles.chevron}>›</Text>
            </View>
          </Pressable>
        </View>

        <View style={styles.debugSection}>
          <Text style={styles.debugTitle}>{t('settings.debugInfo')}</Text>
          <View style={styles.debugCard}>
            <Text style={styles.debugText}>Build: 2024.01</Text>
            <Text style={styles.debugText}>
              {t('settings.network')}: {isOnline ? t('settings.online') : t('settings.offline')} ({connectionType})
            </Text>
            <Text style={styles.debugText}>{t('settings.deviceId')}: mobile-device-001</Text>
            <Text style={styles.debugText}>
              {t('settings.tracking')}: {trackingEnabled ? t('settings.enabled') : t('settings.disabled')}
            </Text>
            <Text style={styles.debugText}>
              {t('settings.wifiOnlySync')}: {wifiOnlySync ? t('settings.enabled') : t('settings.disabled')}
            </Text>
          </View>
        </View>
      </ScrollView>
    </>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f9fafb',
  },
  section: {
    marginTop: 24,
    paddingHorizontal: 16,
  },
  sectionTitle: {
    fontSize: 12,
    fontWeight: '600',
    color: '#6b7280',
    textTransform: 'uppercase',
    marginBottom: 8,
  },
  settingItem: {
    backgroundColor: '#fff',
    padding: 16,
    borderRadius: 12,
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 1,
  },
  settingLabel: {
    fontSize: 16,
    color: '#1f2937',
  },
  settingHint: {
    fontSize: 12,
    color: '#6b7280',
    marginTop: 2,
  },
  settingValue: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  settingValueText: {
    fontSize: 16,
    color: '#6b7280',
  },
  chevron: {
    fontSize: 20,
    color: '#d1d5db',
    marginLeft: 8,
  },
  cacheDetails: {
    backgroundColor: '#fff',
    paddingHorizontal: 16,
    paddingVertical: 12,
    borderBottomLeftRadius: 12,
    borderBottomRightRadius: 12,
  },
  cacheDetailText: {
    fontSize: 12,
    color: '#6b7280',
    marginBottom: 4,
  },
  debugSection: {
    marginTop: 32,
    marginHorizontal: 16,
    marginBottom: 32,
  },
  debugTitle: {
    fontSize: 12,
    fontWeight: '600',
    color: '#9ca3af',
    textTransform: 'uppercase',
    marginBottom: 8,
  },
  debugCard: {
    backgroundColor: '#f3f4f6',
    padding: 12,
    borderRadius: 8,
  },
  debugText: {
    fontSize: 12,
    color: '#6b7280',
    fontFamily: 'monospace',
    marginBottom: 4,
  },
});

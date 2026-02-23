import { 
  View, 
  Text, 
  StyleSheet, 
  ScrollView, 
  Pressable, 
  Alert,
  Switch,
} from 'react-native';
import { Stack } from 'expo-router';
import { useSettingsStore } from '@store/settingsStore';
import { useNetworkStore } from '@store/networkStore';
import { useState } from 'react';

export default function SettingsScreen() {
  const language = useSettingsStore((state) => state.language);
  const setLanguage = useSettingsStore((state) => state.setLanguage);
  const trackingEnabled = useSettingsStore((state) => state.trackingEnabled);
  const toggleTracking = useSettingsStore((state) => state.toggleTracking);
  const wifiOnlySync = useSettingsStore((state) => state.wifiOnlySync);
  const toggleWifiOnlySync = useSettingsStore((state) => state.toggleWifiOnlySync);
  const notificationsEnabled = useSettingsStore((state) => state.notificationsEnabled);
  const toggleNotifications = useSettingsStore((state) => state.toggleNotifications);
  const isOnline = useNetworkStore((state) => state.isOnline);

  const [cacheSize, setCacheSize] = useState('24.5 MB');

  const handleLanguageChange = () => {
    Alert.alert('Select Language', 'Choose your preferred language', [
      {
        text: 'English',
        onPress: () => setLanguage('en'),
        style: language === 'en' ? 'default' : undefined,
      },
      {
        text: 'አማርኛ (Amharic)',
        onPress: () => setLanguage('am'),
        style: language === 'am' ? 'default' : undefined,
      },
      { text: 'Cancel', style: 'cancel' },
    ]);
  };

  const handleClearCache = () => {
    Alert.alert(
      'Clear Cache',
      'This will delete all cached data including offline shipments. Are you sure?',
      [
        { text: 'Cancel', style: 'cancel' },
        {
          text: 'Clear',
          style: 'destructive',
          onPress: () => {
            // TODO: Implement cache clearing
            setCacheSize('0 MB');
            Alert.alert('Success', 'Cache cleared successfully');
          },
        },
      ]
    );
  };

  const handleAbout = () => {
    Alert.alert(
      'Logistics Driver App',
      'Version: 1.0.0\nBuild: 2024.01\n\nFor support, contact:\nsupport@logistics.et',
      [{ text: 'OK' }]
    );
  };

  return (
    <>
      <Stack.Screen
        options={{
          title: 'Settings',
          headerStyle: { backgroundColor: '#2563eb' },
          headerTintColor: '#fff',
        }}
      />
      <ScrollView style={styles.container}>
        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Language</Text>
          <Pressable style={styles.settingItem} onPress={handleLanguageChange}>
            <Text style={styles.settingLabel}>App Language</Text>
            <View style={styles.settingValue}>
              <Text style={styles.settingValueText}>
                {language === 'en' ? 'English' : 'አማርኛ'}
              </Text>
              <Text style={styles.chevron}>›</Text>
            </View>
          </Pressable>
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Notifications</Text>
          <View style={styles.settingItem}>
            <Text style={styles.settingLabel}>Push Notifications</Text>
            <Switch
              value={notificationsEnabled}
              onValueChange={toggleNotifications}
              trackColor={{ false: '#d1d5db', true: '#93c5fd' }}
              thumbColor={notificationsEnabled ? '#2563eb' : '#f4f3f4'}
            />
          </View>
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Tracking</Text>
          <View style={styles.settingItem}>
            <View>
              <Text style={styles.settingLabel}>Background Tracking</Text>
              <Text style={styles.settingHint}>
                Share location while app is in background
              </Text>
            </View>
            <Switch
              value={trackingEnabled}
              onValueChange={toggleTracking}
              trackColor={{ false: '#d1d5db', true: '#93c5fd' }}
              thumbColor={trackingEnabled ? '#2563eb' : '#f4f3f4'}
            />
          </View>
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Data & Sync</Text>
          <View style={styles.settingItem}>
            <View>
              <Text style={styles.settingLabel}>WiFi-Only Sync</Text>
              <Text style={styles.settingHint}>
                Only sync data when connected to WiFi
              </Text>
            </View>
            <Switch
              value={wifiOnlySync}
              onValueChange={toggleWifiOnlySync}
              trackColor={{ false: '#d1d5db', true: '#93c5fd' }}
              thumbColor={wifiOnlySync ? '#2563eb' : '#f4f3f4'}
            />
          </View>
          <Pressable style={styles.settingItem} onPress={handleClearCache}>
            <Text style={styles.settingLabel}>Clear Cache</Text>
            <View style={styles.settingValue}>
              <Text style={styles.settingValueText}>{cacheSize}</Text>
              <Text style={styles.chevron}>›</Text>
            </View>
          </Pressable>
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>About</Text>
          <Pressable style={styles.settingItem} onPress={handleAbout}>
            <Text style={styles.settingLabel}>App Version</Text>
            <Text style={styles.settingValueText}>1.0.0</Text>
          </Pressable>
          <Pressable style={styles.settingItem} onPress={handleAbout}>
            <Text style={styles.settingLabel}>Support</Text>
            <View style={styles.settingValue}>
              <Text style={styles.settingValueText}>Contact Us</Text>
              <Text style={styles.chevron}>›</Text>
            </View>
          </Pressable>
        </View>

        <View style={styles.debugSection}>
          <Text style={styles.debugTitle}>Debug Info</Text>
          <View style={styles.debugCard}>
            <Text style={styles.debugText}>Build: 2024.01</Text>
            <Text style={styles.debugText}>
              Network: {isOnline ? 'Online' : 'Offline'}
            </Text>
            <Text style={styles.debugText}>Device ID: mobile-device-001</Text>
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
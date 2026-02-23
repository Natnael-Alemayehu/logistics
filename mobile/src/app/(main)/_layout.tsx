import { Tabs } from 'expo-router';
import { View, Text, StyleSheet } from 'react-native';
import { useNetworkStore } from '@store/networkStore';
import { useSyncStore } from '@store/syncStore';
import { useEffect } from 'react';

const ICONS: Record<string, string> = {
  home: '📦',
  map: '📍',
  profile: '👤',
  settings: '⚙️',
};

function SyncIndicator() {
  const isOnline = useNetworkStore((state) => state.isOnline);
  const isSyncing = useSyncStore((state) => state.isSyncing);
  const pendingCount = useSyncStore((state) => state.pendingCount);

  if (!isOnline) {
    return (
      <View style={styles.offlineBanner}>
        <Text style={styles.offlineText}>⚠️ Offline - Data will sync later</Text>
      </View>
    );
  }

  if (isSyncing) {
    return (
      <View style={styles.syncBanner}>
        <Text style={styles.syncText}>🔄 Syncing...</Text>
      </View>
    );
  }

  if (pendingCount > 0) {
    return (
      <View style={styles.pendingBanner}>
        <Text style={styles.pendingText}>{pendingCount} pending</Text>
      </View>
    );
  }

  return null;
}

export default function MainLayout() {
  const getPendingCount = useSyncStore((state) => state.getPendingCount);

  useEffect(() => {
    getPendingCount();
  }, [getPendingCount]);

  return (
    <>
      <SyncIndicator />
      <Tabs
        screenOptions={{
          headerShown: true,
          headerStyle: styles.header,
          headerTitleStyle: styles.headerTitle,
          headerTintColor: '#fff',
          tabBarActiveTintColor: '#2563eb',
          tabBarInactiveTintColor: '#6b7280',
          tabBarStyle: styles.tabBar,
        }}
      >
        <Tabs.Screen
          name="index"
          options={{
            title: 'Shipments',
            headerTitle: 'My Shipments',
            tabBarIcon: ({ focused }: { focused: boolean }) => (
              <Text style={{ fontSize: 20, opacity: focused ? 1 : 0.6 }}>
                {ICONS.home}
              </Text>
            ),
          }}
        />
        <Tabs.Screen
          name="map"
          options={{
            title: 'Map',
            headerTitle: 'Delivery Map',
            tabBarIcon: ({ focused }: { focused: boolean }) => (
              <Text style={{ fontSize: 20, opacity: focused ? 1 : 0.6 }}>
                {ICONS.map}
              </Text>
            ),
          }}
        />
        <Tabs.Screen
          name="profile"
          options={{
            title: 'Profile',
            headerTitle: 'Driver Profile',
            tabBarIcon: ({ focused }: { focused: boolean }) => (
              <Text style={{ fontSize: 20, opacity: focused ? 1 : 0.6 }}>
                {ICONS.profile}
              </Text>
            ),
          }}
        />
        <Tabs.Screen
          name="settings"
          options={{
            title: 'Settings',
            headerTitle: 'Settings',
            tabBarIcon: ({ focused }: { focused: boolean }) => (
              <Text style={{ fontSize: 20, opacity: focused ? 1 : 0.6 }}>
                {ICONS.settings}
              </Text>
            ),
          }}
        />
      </Tabs>
    </>
  );
}

const styles = StyleSheet.create({
  header: {
    backgroundColor: '#2563eb',
  },
  headerTitle: {
    color: '#fff',
    fontWeight: '600',
  },
  tabBar: {
    backgroundColor: '#fff',
    borderTopWidth: 1,
    borderTopColor: '#e5e7eb',
    paddingTop: 8,
    paddingBottom: 8,
    height: 60,
  },
  offlineBanner: {
    backgroundColor: '#fef3c7',
    padding: 8,
    alignItems: 'center',
  },
  offlineText: {
    color: '#92400e',
    fontSize: 14,
    fontWeight: '500',
  },
  syncBanner: {
    backgroundColor: '#dbeafe',
    padding: 8,
    alignItems: 'center',
  },
  syncText: {
    color: '#1e40af',
    fontSize: 14,
    fontWeight: '500',
  },
  pendingBanner: {
    backgroundColor: '#f3f4f6',
    padding: 8,
    alignItems: 'center',
  },
  pendingText: {
    color: '#4b5563',
    fontSize: 12,
  },
});
import { Tabs } from 'expo-router';
import { View, Text, StyleSheet, TouchableOpacity } from 'react-native';
import { useNetworkStore } from '@store/networkStore';
import { useSyncStore } from '@store/syncStore';
import { useEffect } from 'react';
import { SyncStatus } from '@/types/components';
import SyncIndicator from '@components/ui/SyncIndicator';
import SyncProgress from '@components/ui/SyncProgress';
import PendingDataBadge from '@components/ui/PendingDataBadge';
import { colors, spacing } from '@/utils/theme';

const ICONS: Record<string, string> = {
  home: '📦',
  map: '📍',
  profile: '👤',
  settings: '⚙️',
};

function SyncStatusHeader() {
  const isOnline = useNetworkStore((state) => state.isOnline);
  const isSyncing = useSyncStore((state) => state.isSyncing);
  const pendingCount = useSyncStore((state) => state.pendingCount);
  const lastError = useSyncStore((state) => state.lastError);
  const syncProgress = useSyncStore((state) => state.syncProgress);

  const getSyncStatus = (): SyncStatus => {
    if (lastError) return 'error';
    if (isSyncing) return 'syncing';
    if (pendingCount > 0) return 'pending';
    return 'synced';
  };

  return (
    <View style={styles.syncHeader}>
      <SyncIndicator
        status={getSyncStatus()}
        pendingCount={pendingCount}
        style={styles.syncIndicator}
      />
      {!isOnline && (
        <View style={styles.offlineBadge}>
          <Text style={styles.offlineText}>Offline</Text>
        </View>
      )}
    </View>
  );
}

function TabBarIcon({ 
  icon, 
  focused, 
  pendingCount 
}: { 
  icon: string; 
  focused: boolean;
  pendingCount?: number;
}) {
  return (
    <View style={styles.tabIconContainer}>
      <Text style={{ fontSize: 20, opacity: focused ? 1 : 0.6 }}>
        {icon}
      </Text>
      {pendingCount !== undefined && pendingCount > 0 && (
        <PendingDataBadge count={pendingCount} style={styles.tabBadge} />
      )}
    </View>
  );
}

export default function MainLayout() {
  const getPendingCount = useSyncStore((state) => state.getPendingCount);
  const isSyncing = useSyncStore((state) => state.isSyncing);
  const syncProgress = useSyncStore((state) => state.syncProgress);
  const pendingCount = useSyncStore((state) => state.pendingCount);

  useEffect(() => {
    getPendingCount();
  }, [getPendingCount]);

  return (
    <View style={styles.container}>
      {isSyncing && syncProgress.phase !== 'idle' && (
        <SyncProgress
          visible={true}
          progress={syncProgress.percentage}
          current={syncProgress.itemsProcessed}
          total={syncProgress.totalItems}
          message={syncProgress.message}
        />
      )}
      
      <Tabs
        screenOptions={{
          headerShown: true,
          headerStyle: styles.header,
          headerTitleStyle: styles.headerTitle,
          headerTintColor: '#fff',
          tabBarActiveTintColor: '#2563eb',
          tabBarInactiveTintColor: '#6b7280',
          tabBarStyle: styles.tabBar,
          headerRight: () => <SyncStatusHeader />,
        }}
      >
        <Tabs.Screen
          name="index"
          options={{
            title: 'Shipments',
            headerTitle: 'My Shipments',
            tabBarIcon: ({ focused }: { focused: boolean }) => (
              <TabBarIcon icon={ICONS.home} focused={focused} />
            ),
          }}
        />
        <Tabs.Screen
          name="map"
          options={{
            title: 'Map',
            headerTitle: 'Delivery Map',
            tabBarIcon: ({ focused }: { focused: boolean }) => (
              <TabBarIcon icon={ICONS.map} focused={focused} />
            ),
          }}
        />
        <Tabs.Screen
          name="profile"
          options={{
            title: 'Profile',
            headerTitle: 'Driver Profile',
            tabBarIcon: ({ focused }: { focused: boolean }) => (
              <TabBarIcon icon={ICONS.profile} focused={focused} />
            ),
          }}
        />
        <Tabs.Screen
          name="settings"
          options={{
            title: 'Settings',
            headerTitle: 'Settings',
            tabBarIcon: ({ focused }: { focused: boolean }) => (
              <TabBarIcon icon={ICONS.settings} focused={focused} pendingCount={pendingCount} />
            ),
          }}
        />
      </Tabs>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
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
  syncHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    marginRight: spacing.sm,
  },
  syncIndicator: {
    marginRight: spacing.sm,
  },
  offlineBadge: {
    backgroundColor: colors.warning,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    borderRadius: 4,
  },
  offlineText: {
    color: '#92400e',
    fontSize: 11,
    fontWeight: '600',
  },
  tabIconContainer: {
    position: 'relative',
    alignItems: 'center',
    justifyContent: 'center',
  },
  tabBadge: {
    position: 'absolute',
    top: -4,
    right: -8,
  },
});

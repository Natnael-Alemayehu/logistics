import { 
  View, 
  Text, 
  StyleSheet, 
  ScrollView, 
  Pressable, 
  Alert,
  ActivityIndicator,
} from 'react-native';
import { router, Stack } from 'expo-router';
import { useAuthStore } from '@store/authStore';
import { useSyncStore } from '@store/syncStore';
import { useState, useEffect } from 'react';

interface DriverStats {
  deliveriesToday: number;
  deliveriesWeek: number;
  deliveriesMonth: number;
}

export default function ProfileScreen() {
  const user = useAuthStore((state) => state.user);
  const logout = useAuthStore((state) => state.logout);
  const lastSyncAt = useSyncStore((state) => state.lastSyncAt);
  const pendingCount = useSyncStore((state) => state.pendingCount);
  const sync = useSyncStore((state) => state.sync);
  const isSyncing = useSyncStore((state) => state.isSyncing);
  const [stats, setStats] = useState<DriverStats>({
    deliveriesToday: 0,
    deliveriesWeek: 0,
    deliveriesMonth: 0,
  });

  useEffect(() => {
    loadStats();
  }, []);

  const loadStats = async () => {
    // TODO: Load from API
    setStats({
      deliveriesToday: 8,
      deliveriesWeek: 42,
      deliveriesMonth: 156,
    });
  };

  const handleLogout = () => {
    Alert.alert('Logout', 'Are you sure you want to logout?', [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Logout',
        style: 'destructive',
        onPress: async () => {
          await logout();
          router.replace('/(auth)/login');
        },
      },
    ]);
  };

  const handleSync = async () => {
    try {
      await sync();
      Alert.alert('Success', 'Data synced successfully');
    } catch (error) {
      Alert.alert('Error', 'Failed to sync data');
    }
  };

  return (
    <>
      <Stack.Screen
        options={{
          title: 'Driver Profile',
          headerStyle: { backgroundColor: '#2563eb' },
          headerTintColor: '#fff',
        }}
      />
      <ScrollView style={styles.container}>
        <View style={styles.header}>
          <View style={styles.avatar}>
            <Text style={styles.avatarText}>
              {user?.name?.charAt(0)?.toUpperCase() ?? 'D'}
            </Text>
          </View>
          <Text style={styles.name}>{user?.name ?? 'Driver'}</Text>
          <Text style={styles.phone}>{user?.phone ?? '+251 9X XXX XXXX'}</Text>
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Vehicle Assignment</Text>
          <View style={styles.vehicleCard}>
            <Text style={styles.vehicleIcon}>🚚</Text>
            <View>
              <Text style={styles.vehiclePlate}>ET-1234-AA</Text>
              <Text style={styles.vehicleType}>Truck - Light Duty</Text>
            </View>
          </View>
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Statistics</Text>
          <View style={styles.statsGrid}>
            <View style={styles.statCard}>
              <Text style={styles.statValue}>{stats.deliveriesToday}</Text>
              <Text style={styles.statLabel}>Today</Text>
            </View>
            <View style={styles.statCard}>
              <Text style={styles.statValue}>{stats.deliveriesWeek}</Text>
              <Text style={styles.statLabel}>This Week</Text>
            </View>
            <View style={styles.statCard}>
              <Text style={styles.statValue}>{stats.deliveriesMonth}</Text>
              <Text style={styles.statLabel}>This Month</Text>
            </View>
          </View>
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Sync Status</Text>
          <View style={styles.syncCard}>
            <View style={styles.syncRow}>
              <Text style={styles.syncLabel}>Last Sync:</Text>
              <Text style={styles.syncValue}>
                {lastSyncAt
                  ? new Date(lastSyncAt).toLocaleString()
                  : 'Never'}
              </Text>
            </View>
            <View style={styles.syncRow}>
              <Text style={styles.syncLabel}>Pending Items:</Text>
              <Text style={[styles.syncValue, pendingCount > 0 && styles.pendingText]}>
                {pendingCount}
              </Text>
            </View>
            <Pressable
              style={[styles.syncButton, isSyncing && styles.syncButtonDisabled]}
              onPress={handleSync}
              disabled={isSyncing}
            >
              {isSyncing ? (
                <ActivityIndicator color="#fff" />
              ) : (
                <Text style={styles.syncButtonText}>Sync Now</Text>
              )}
            </Pressable>
          </View>
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Storage</Text>
          <View style={styles.storageCard}>
            <Text style={styles.storageUsed}>24.5 MB</Text>
            <Text style={styles.storageLabel}>App storage used</Text>
            <Text style={styles.storageNote}>
              Offline data, photos, and cached content
            </Text>
          </View>
        </View>

        <Pressable style={styles.logoutButton} onPress={handleLogout}>
          <Text style={styles.logoutText}>Logout</Text>
        </Pressable>
      </ScrollView>
    </>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f9fafb',
  },
  header: {
    backgroundColor: '#2563eb',
    padding: 24,
    alignItems: 'center',
  },
  avatar: {
    width: 80,
    height: 80,
    borderRadius: 40,
    backgroundColor: '#fff',
    justifyContent: 'center',
    alignItems: 'center',
    marginBottom: 12,
  },
  avatarText: {
    fontSize: 32,
    fontWeight: 'bold',
    color: '#2563eb',
  },
  name: {
    fontSize: 24,
    fontWeight: 'bold',
    color: '#fff',
    marginBottom: 4,
  },
  phone: {
    fontSize: 14,
    color: '#e0e7ff',
  },
  section: {
    padding: 16,
  },
  sectionTitle: {
    fontSize: 12,
    fontWeight: '600',
    color: '#6b7280',
    textTransform: 'uppercase',
    marginBottom: 12,
  },
  vehicleCard: {
    backgroundColor: '#fff',
    padding: 16,
    borderRadius: 12,
    flexDirection: 'row',
    alignItems: 'center',
  },
  vehicleIcon: {
    fontSize: 32,
    marginRight: 12,
  },
  vehiclePlate: {
    fontSize: 16,
    fontWeight: '600',
    color: '#1f2937',
  },
  vehicleType: {
    fontSize: 14,
    color: '#6b7280',
  },
  statsGrid: {
    flexDirection: 'row',
    gap: 12,
  },
  statCard: {
    flex: 1,
    backgroundColor: '#fff',
    padding: 16,
    borderRadius: 12,
    alignItems: 'center',
  },
  statValue: {
    fontSize: 28,
    fontWeight: 'bold',
    color: '#2563eb',
  },
  statLabel: {
    fontSize: 12,
    color: '#6b7280',
    marginTop: 4,
  },
  syncCard: {
    backgroundColor: '#fff',
    padding: 16,
    borderRadius: 12,
  },
  syncRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    marginBottom: 8,
  },
  syncLabel: {
    fontSize: 14,
    color: '#6b7280',
  },
  syncValue: {
    fontSize: 14,
    color: '#1f2937',
    fontWeight: '500',
  },
  pendingText: {
    color: '#d97706',
  },
  syncButton: {
    backgroundColor: '#2563eb',
    padding: 12,
    borderRadius: 8,
    alignItems: 'center',
    marginTop: 12,
  },
  syncButtonDisabled: {
    backgroundColor: '#93c5fd',
  },
  syncButtonText: {
    color: '#fff',
    fontWeight: '600',
  },
  storageCard: {
    backgroundColor: '#fff',
    padding: 16,
    borderRadius: 12,
    alignItems: 'center',
  },
  storageUsed: {
    fontSize: 24,
    fontWeight: 'bold',
    color: '#1f2937',
  },
  storageLabel: {
    fontSize: 14,
    color: '#6b7280',
    marginTop: 4,
  },
  storageNote: {
    fontSize: 12,
    color: '#9ca3af',
    marginTop: 4,
  },
  logoutButton: {
    margin: 16,
    padding: 16,
    borderRadius: 12,
    backgroundColor: '#fee2e2',
    alignItems: 'center',
  },
  logoutText: {
    color: '#dc2626',
    fontSize: 16,
    fontWeight: '600',
  },
});
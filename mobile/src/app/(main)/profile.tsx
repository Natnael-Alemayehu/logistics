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
import { useTranslation } from 'react-i18next';
import { useDriverStats } from '@/hooks/useDriverStats';
import { useDriverVehicle } from '@/hooks/useDriverVehicle';
import { useStorageUsage } from '@/hooks/useStorageUsage';

export default function ProfileScreen() {
  const { t } = useTranslation();
  const user = useAuthStore((state) => state.user);
  const logout = useAuthStore((state) => state.logout);
  const lastSyncAt = useSyncStore((state) => state.lastSyncAt);
  const pendingCount = useSyncStore((state) => state.pendingCount);
  const sync = useSyncStore((state) => state.sync);
  const isSyncing = useSyncStore((state) => state.isSyncing);
  
  const { stats, isLoading: isLoadingStats } = useDriverStats();
  const { vehicle, isLoading: isLoadingVehicle } = useDriverVehicle();
  const { storage, isLoading: isLoadingStorage } = useStorageUsage();

  const handleLogout = () => {
    Alert.alert(t('auth.logout'), t('profile.logoutConfirm'), [
      { text: t('common.cancel'), style: 'cancel' },
      {
        text: t('auth.logout'),
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
      Alert.alert(t('common.success'), t('profile.syncSuccess'));
    } catch (error) {
      Alert.alert(t('common.error'), t('profile.syncError'));
    }
  };

  return (
    <>
      <Stack.Screen
        options={{
          title: t('profile.driverProfile'),
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
          <Text style={styles.name}>{user?.name ?? t('profile.driver')}</Text>
          <Text style={styles.phone}>{user?.phone ?? '+251 9X XXX XXXX'}</Text>
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>{t('profile.vehicleAssignment')}</Text>
          <View style={styles.vehicleCard}>
            {isLoadingVehicle ? (
              <ActivityIndicator size="small" color="#2563eb" />
            ) : vehicle ? (
              <>
                <Text style={styles.vehicleIcon}>🚚</Text>
                <View>
                  <Text style={styles.vehiclePlate}>{vehicle.plateNumber}</Text>
                  <Text style={styles.vehicleType}>{vehicle.type || t('profile.truckLightDuty')}</Text>
                </View>
              </>
            ) : (
              <>
                <Text style={styles.vehicleIcon}>🚚</Text>
                <View>
                  <Text style={styles.vehiclePlate}>{t('profile.noVehicleAssigned')}</Text>
                  <Text style={styles.vehicleType}>{t('profile.contactDispatcher')}</Text>
                </View>
              </>
            )}
          </View>
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>{t('profile.stats')}</Text>
          {isLoadingStats ? (
            <View style={styles.loadingContainer}>
              <ActivityIndicator size="small" color="#2563eb" />
            </View>
          ) : (
            <View style={styles.statsGrid}>
              <View style={styles.statCard}>
                <Text style={styles.statValue}>{stats.deliveriesToday}</Text>
                <Text style={styles.statLabel}>{t('profile.deliveriesToday')}</Text>
              </View>
              <View style={styles.statCard}>
                <Text style={styles.statValue}>{stats.deliveriesWeek}</Text>
                <Text style={styles.statLabel}>{t('profile.deliveriesWeek')}</Text>
              </View>
              <View style={styles.statCard}>
                <Text style={styles.statValue}>{stats.deliveriesMonth}</Text>
                <Text style={styles.statLabel}>{t('profile.deliveriesMonth')}</Text>
              </View>
            </View>
          )}
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>{t('profile.syncStatus')}</Text>
          <View style={styles.syncCard}>
            <View style={styles.syncRow}>
              <Text style={styles.syncLabel}>{t('profile.lastSync')}:</Text>
              <Text style={styles.syncValue}>
                {lastSyncAt
                  ? new Date(lastSyncAt).toLocaleString()
                  : t('profile.never')}
              </Text>
            </View>
            <View style={styles.syncRow}>
              <Text style={styles.syncLabel}>{t('profile.pendingItems')}:</Text>
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
                <Text style={styles.syncButtonText}>{t('profile.syncNow')}</Text>
              )}
            </Pressable>
          </View>
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>{t('profile.storage')}</Text>
          <View style={styles.storageCard}>
            {isLoadingStorage ? (
              <ActivityIndicator size="small" color="#2563eb" />
            ) : (
              <>
                <Text style={styles.storageUsed}>{storage.formatted.total}</Text>
                <Text style={styles.storageLabel}>{t('profile.storageUsed')}</Text>
                <Text style={styles.storageNote}>
                  {t('profile.storageBreakdown', {
                    photos: storage.formatted.photos,
                    database: storage.formatted.database,
                  })}
                </Text>
              </>
            )}
          </View>
        </View>

        <Pressable style={styles.logoutButton} onPress={handleLogout}>
          <Text style={styles.logoutText}>{t('auth.logout')}</Text>
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
  loadingContainer: {
    backgroundColor: '#fff',
    padding: 32,
    borderRadius: 12,
    alignItems: 'center',
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
    textAlign: 'center',
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

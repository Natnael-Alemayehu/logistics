import { 
  View, 
  Text, 
  StyleSheet, 
  FlatList, 
  Pressable, 
  RefreshControl,
  ActivityIndicator,
} from 'react-native';
import { router } from 'expo-router';
import { useState, useCallback } from 'react';
import { useNetworkStore } from '@store/networkStore';
import { useTrackingStore } from '@store/trackingStore';
import { useShipments, type Shipment } from '@hooks/useShipments';

type TabFilter = 'active' | 'completed' | 'all';

const STATUS_COLORS: Record<string, string> = {
  pending: '#fef3c7',
  assigned: '#dbeafe',
  in_transit: '#dbeafe',
  delayed: '#fee2e2',
  arrived: '#d1fae5',
  delivered: '#d1fae5',
  issue: '#fee2e2',
  cancelled: '#f3f4f6',
};

const STATUS_TEXT_COLORS: Record<string, string> = {
  pending: '#92400e',
  assigned: '#1e40af',
  in_transit: '#1e40af',
  delayed: '#dc2626',
  arrived: '#059669',
  delivered: '#059669',
  issue: '#dc2626',
  cancelled: '#6b7280',
};

function formatStatus(status: string): string {
  return status.replace('_', ' ').replace(/\b\w/g, (l) => l.toUpperCase());
}

export default function ShipmentsListScreen() {
  const [activeTab, setActiveTab] = useState<TabFilter>('active');
  const [refreshing, setRefreshing] = useState(false);
  const isOnline = useNetworkStore((state) => state.isOnline);
  const { shipments, isLoading, refetch } = useShipments();
  const { isTracking, activeShipmentId } = useTrackingStore();

  const filteredShipments = shipments.filter((shipment) => {
    if (activeTab === 'active') {
      return !['delivered', 'cancelled'].includes(shipment.status);
    }
    if (activeTab === 'completed') {
      return ['delivered', 'cancelled'].includes(shipment.status);
    }
    return true;
  });

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    try {
      await refetch();
    } finally {
      setRefreshing(false);
    }
  }, [refetch]);

  const renderShipment = ({ item }: { item: Shipment }) => {
    const isBeingTracked = isTracking && activeShipmentId === item.id;
    
    return (
      <Pressable
        style={[
          styles.card,
          isBeingTracked && styles.cardTracking,
        ]}
        onPress={() => router.push(`/shipment/${item.id}`)}
      >
        <View style={styles.cardHeader}>
          <Text style={styles.trackingNumber}>{item.tracking_number}</Text>
          <View style={styles.badgeContainer}>
            {isBeingTracked && (
              <View style={styles.trackingBadge}>
                <View style={styles.trackingDot} />
                <Text style={styles.trackingBadgeText}>Tracking</Text>
              </View>
            )}
            <View
              style={[
                styles.statusBadge,
                { backgroundColor: STATUS_COLORS[item.status] ?? '#f3f4f6' },
              ]}
            >
              <Text
                style={[
                  styles.statusText,
                  { color: STATUS_TEXT_COLORS[item.status] ?? '#6b7280' },
                ]}
              >
                {formatStatus(item.status)}
              </Text>
            </View>
          </View>
        </View>
        <Text style={styles.destination}>{item.destination ?? item.origin ?? ''}</Text>
        <View style={styles.cardFooter}>
          <Text style={styles.customer}>{item.customer_name}</Text>
        </View>
      </Pressable>
    );
  };

  const renderEmpty = () => (
    <View style={styles.emptyContainer}>
      <Text style={styles.emptyIcon}>📦</Text>
      <Text style={styles.emptyTitle}>No shipments</Text>
      <Text style={styles.emptyText}>
        {activeTab === 'active'
          ? 'You have no active shipments'
          : activeTab === 'completed'
          ? 'No completed shipments yet'
          : 'No shipments found'}
      </Text>
    </View>
  );

  return (
    <View style={styles.container}>
      <View style={styles.tabs}>
        {(['active', 'completed', 'all'] as TabFilter[]).map((tab) => (
          <Pressable
            key={tab}
            style={[styles.tab, activeTab === tab && styles.tabActive]}
            onPress={() => setActiveTab(tab)}
          >
            <Text
              style={[styles.tabText, activeTab === tab && styles.tabTextActive]}
            >
              {tab.charAt(0).toUpperCase() + tab.slice(1)}
            </Text>
          </Pressable>
        ))}
      </View>

      <FlatList
        data={filteredShipments}
        renderItem={renderShipment}
        keyExtractor={(item) => item.id}
        contentContainerStyle={styles.list}
        refreshControl={
          <RefreshControl refreshing={refreshing} onRefresh={onRefresh} />
        }
        ListEmptyComponent={renderEmpty}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f9fafb',
  },
  tabs: {
    flexDirection: 'row',
    backgroundColor: '#fff',
    borderBottomWidth: 1,
    borderBottomColor: '#e5e7eb',
  },
  tab: {
    flex: 1,
    paddingVertical: 12,
    alignItems: 'center',
  },
  tabActive: {
    borderBottomWidth: 2,
    borderBottomColor: '#2563eb',
  },
  tabText: {
    fontSize: 14,
    color: '#6b7280',
    fontWeight: '500',
  },
  tabTextActive: {
    color: '#2563eb',
    fontWeight: '600',
  },
  list: {
    padding: 16,
    flexGrow: 1,
  },
  card: {
    backgroundColor: '#fff',
    padding: 16,
    borderRadius: 12,
    marginBottom: 12,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 1 },
    shadowOpacity: 0.05,
    shadowRadius: 2,
    elevation: 2,
  },
  cardTracking: {
    borderWidth: 2,
    borderColor: '#059669',
    backgroundColor: '#f0fdf4',
  },
  cardHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 8,
  },
  trackingNumber: {
    fontSize: 16,
    fontWeight: '600',
    color: '#1f2937',
    flex: 1,
  },
  badgeContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  trackingBadge: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#d1fae5',
    paddingHorizontal: 8,
    paddingVertical: 4,
    borderRadius: 8,
  },
  trackingDot: {
    width: 6,
    height: 6,
    borderRadius: 3,
    backgroundColor: '#059669',
    marginRight: 4,
  },
  trackingBadgeText: {
    fontSize: 10,
    fontWeight: '600',
    color: '#065f46',
  },
  statusBadge: {
    paddingHorizontal: 10,
    paddingVertical: 4,
    borderRadius: 12,
  },
  statusText: {
    fontSize: 12,
    fontWeight: '600',
  },
  destination: {
    fontSize: 14,
    color: '#4b5563',
    marginBottom: 8,
  },
  cardFooter: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  customer: {
    fontSize: 12,
    color: '#6b7280',
  },
  syncIndicator: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  syncPending: {
    fontSize: 11,
    color: '#d97706',
  },
  emptyContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    paddingVertical: 60,
  },
  emptyIcon: {
    fontSize: 48,
    marginBottom: 16,
  },
  emptyTitle: {
    fontSize: 18,
    fontWeight: '600',
    color: '#374151',
    marginBottom: 8,
  },
  emptyText: {
    fontSize: 14,
    color: '#6b7280',
    textAlign: 'center',
  },
});
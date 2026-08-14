import { View, Text, StyleSheet, Pressable } from 'react-native';
import { Shipment } from '../../types';
import { ShipmentStatusBadge } from './ShipmentStatusBadge';
import { colors, spacing } from '../../utils/theme';
import { formatDateTime } from '../../utils/format';
import { useTranslation } from 'react-i18next';

interface ShipmentCardProps {
  shipment: Shipment;
  onPress: () => void;
  isSynced?: boolean;
}

export function ShipmentCard({ shipment, onPress, isSynced = true }: ShipmentCardProps) {
  const { t } = useTranslation();
  
  const truncateAddress = (address: string, maxLength = 35) => {
    if (address.length <= maxLength) return address;
    return `${address.substring(0, maxLength)}...`;
  };

  return (
    <Pressable style={styles.card} onPress={onPress}>
      <View style={styles.header}>
        <Text style={styles.trackingNumber}>{shipment.tracking_number}</Text>
        <View style={styles.headerRight}>
          <View style={styles.syncIndicator}>
            {isSynced ? (
              <Text style={styles.syncedIcon}>✓</Text>
            ) : (
              <Text style={styles.pendingIcon}>◷</Text>
            )}
          </View>
          <ShipmentStatusBadge status={shipment.status} size="sm" />
        </View>
      </View>
      
      <View style={styles.content}>
        <Text style={styles.customerName}>{shipment.customer_name}</Text>
        <Text style={styles.address} numberOfLines={1}>
          {truncateAddress(shipment.destination_address)}
        </Text>
      </View>

      {shipment.estimated_delivery && (
        <View style={styles.footer}>
          <Text style={styles.footerLabel}>{t('shipments.estDelivery')}:</Text>
          <Text style={styles.footerValue}>
            {formatDateTime(shipment.estimated_delivery)}
          </Text>
        </View>
      )}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  card: {
    backgroundColor: colors.surface,
    padding: spacing.lg,
    borderRadius: 12,
    marginBottom: spacing.md,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 1 },
    shadowOpacity: 0.05,
    shadowRadius: 2,
    elevation: 2,
  },
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: spacing.sm,
  },
  trackingNumber: {
    fontSize: 16,
    fontWeight: '600',
    color: colors.text,
    flex: 1,
  },
  headerRight: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
  },
  syncIndicator: {
    width: 20,
    height: 20,
    borderRadius: 10,
    justifyContent: 'center',
    alignItems: 'center',
  },
  syncedIcon: {
    fontSize: 12,
    color: colors.success,
    fontWeight: 'bold',
  },
  pendingIcon: {
    fontSize: 12,
    color: colors.warning,
  },
  content: {
    marginBottom: spacing.sm,
  },
  customerName: {
    fontSize: 14,
    color: colors.text,
    marginBottom: 2,
  },
  address: {
    fontSize: 12,
    color: colors.textSecondary,
  },
  footer: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs,
  },
  footerLabel: {
    fontSize: 11,
    color: colors.textSecondary,
  },
  footerValue: {
    fontSize: 11,
    color: colors.primary,
    fontWeight: '500',
  },
});

import { View, Text, StyleSheet } from 'react-native';
import { ShipmentStatus } from '../../types';
import { SHIPMENT_STATUSES } from '../../services/constants';
import { colors, spacing } from '../../utils/theme';

interface ShipmentStatusBadgeProps {
  status: ShipmentStatus;
  size?: 'sm' | 'md';
}

const STATUS_COLORS: Record<string, string> = {
  pending: '#6b7280',
  assigned: '#3b82f6',
  in_transit: '#f59e0b',
  delayed: '#f97316',
  arrived: '#8b5cf6',
  delivered: '#22c55e',
  issue: '#ef4444',
  cancelled: '#6b7280',
};

const STATUS_BACKGROUNDS: Record<string, string> = {
  pending: '#f3f4f6',
  assigned: '#dbeafe',
  in_transit: '#fef3c7',
  delayed: '#ffedd5',
  arrived: '#ede9fe',
  delivered: '#dcfce7',
  issue: '#fee2e2',
  cancelled: '#f3f4f6',
};

export function ShipmentStatusBadge({ status, size = 'md' }: ShipmentStatusBadgeProps) {
  const statusInfo = SHIPMENT_STATUSES.find(s => s.value === status);
  const label = statusInfo?.label || status;
  const backgroundColor = STATUS_BACKGROUNDS[status] || '#f3f4f6';
  const textColor = STATUS_COLORS[status] || '#6b7280';

  return (
    <View style={[styles.badge, size === 'sm' && styles.badgeSm, { backgroundColor }]}>
      <Text style={[styles.text, size === 'sm' && styles.textSm, { color: textColor }]}>
        {label}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  badge: {
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.xs,
    borderRadius: 12,
  },
  badgeSm: {
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    borderRadius: 8,
  },
  text: {
    fontSize: 12,
    fontWeight: '500',
    textTransform: 'capitalize',
  },
  textSm: {
    fontSize: 10,
  },
});

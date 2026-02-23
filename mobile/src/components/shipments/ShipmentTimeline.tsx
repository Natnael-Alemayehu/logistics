import { View, Text, StyleSheet } from 'react-native';
import { TrackingEvent, ShipmentStatus } from '../../types';
import { SHIPMENT_STATUSES } from '../../services/constants';
import { colors, spacing } from '../../utils/theme';
import { formatDateTime } from '../../utils/format';

interface ShipmentTimelineProps {
  events: TrackingEvent[];
  currentStatus?: ShipmentStatus;
}

const STATUS_ICONS: Record<string, string> = {
  pending: '○',
  assigned: '◐',
  in_transit: '◑',
  delayed: '⚠',
  arrived: '◉',
  delivered: '✓',
  issue: '✕',
  cancelled: '✕',
};

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

export function ShipmentTimeline({ events, currentStatus }: ShipmentTimelineProps) {
  const sortedEvents = [...events].sort(
    (a, b) => new Date(b.recorded_at).getTime() - new Date(a.recorded_at).getTime()
  );

  const getStatusLabel = (status?: ShipmentStatus) => {
    if (!status) return 'Update';
    const statusInfo = SHIPMENT_STATUSES.find(s => s.value === status);
    return statusInfo?.label || status;
  };

  if (events.length === 0) {
    return (
      <View style={styles.emptyContainer}>
        <Text style={styles.emptyText}>No tracking events available</Text>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      {sortedEvents.map((event, index) => {
        const isCurrent = event.status === currentStatus;
        const statusColor = event.status ? STATUS_COLORS[event.status] : colors.textSecondary;
        const icon = event.status ? STATUS_ICONS[event.status] : '●';

        return (
          <View key={event.id} style={styles.eventRow}>
            <View style={styles.timelineColumn}>
              <View
                style={[
                  styles.iconContainer,
                  isCurrent && styles.iconContainerActive,
                  { borderColor: statusColor },
                ]}
              >
                <Text style={[styles.icon, { color: statusColor }]}>{icon}</Text>
              </View>
              {index < sortedEvents.length - 1 && <View style={styles.line} />}
            </View>

            <View style={styles.contentColumn}>
              <View style={styles.eventHeader}>
                <Text style={[styles.statusLabel, isCurrent && styles.statusLabelActive]}>
                  {getStatusLabel(event.status)}
                </Text>
                <Text style={styles.timestamp}>{formatDateTime(event.recorded_at)}</Text>
              </View>
              {event.note && <Text style={styles.note}>{event.note}</Text>}
            </View>
          </View>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    paddingVertical: spacing.sm,
  },
  emptyContainer: {
    padding: spacing.xl,
    alignItems: 'center',
  },
  emptyText: {
    fontSize: 14,
    color: colors.textSecondary,
  },
  eventRow: {
    flexDirection: 'row',
  },
  timelineColumn: {
    width: 40,
    alignItems: 'center',
  },
  iconContainer: {
    width: 28,
    height: 28,
    borderRadius: 14,
    borderWidth: 2,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: colors.surface,
  },
  iconContainerActive: {
    borderWidth: 3,
  },
  icon: {
    fontSize: 14,
    fontWeight: 'bold',
  },
  line: {
    width: 2,
    flex: 1,
    backgroundColor: colors.border,
    marginVertical: spacing.xs,
  },
  contentColumn: {
    flex: 1,
    paddingBottom: spacing.lg,
  },
  eventHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 2,
  },
  statusLabel: {
    fontSize: 14,
    fontWeight: '500',
    color: colors.text,
    textTransform: 'capitalize',
  },
  statusLabelActive: {
    fontWeight: '600',
    color: colors.primary,
  },
  timestamp: {
    fontSize: 11,
    color: colors.textSecondary,
  },
  note: {
    fontSize: 12,
    color: colors.textSecondary,
    marginTop: 2,
  },
});

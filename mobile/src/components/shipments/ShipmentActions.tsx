import { View, Text, StyleSheet, Pressable, ActivityIndicator } from 'react-native';
import { Shipment, ShipmentStatus } from '../../types';
import { colors, spacing } from '../../utils/theme';

interface ShipmentActionsProps {
  shipment: Shipment;
  onAction: (action: string) => void;
  isLoading?: boolean;
  isTracking?: boolean;
  activeTrackingShipmentId?: string | null;
}

type ValidAction = 
  | 'accept' 
  | 'start_transit' 
  | 'report_delay' 
  | 'report_issue' 
  | 'mark_arrived' 
  | 'complete_delivery'
  | 'report_delay_with_reason';

const STATUS_TRANSITIONS: Record<ShipmentStatus, ValidAction[]> = {
  pending: ['accept'],
  assigned: ['start_transit', 'report_issue'],
  in_transit: ['report_delay', 'mark_arrived', 'report_issue'],
  delayed: ['start_transit', 'mark_arrived', 'report_issue'],
  arrived: ['complete_delivery', 'report_issue'],
  delivered: [],
  issue: ['start_transit', 'mark_arrived', 'complete_delivery'],
  cancelled: [],
};

const ACTION_CONFIG: Record<ValidAction, { label: string; style: 'primary' | 'secondary' | 'danger' }> = {
  accept: { label: 'Accept', style: 'primary' },
  start_transit: { label: 'Start Transit', style: 'primary' },
  report_delay: { label: 'Report Delay', style: 'secondary' },
  report_delay_with_reason: { label: 'Report Delay', style: 'secondary' },
  mark_arrived: { label: 'Mark Arrived', style: 'primary' },
  complete_delivery: { label: 'Complete Delivery', style: 'primary' },
  report_issue: { label: 'Report Issue', style: 'danger' },
};

export function ShipmentActions({ 
  shipment, 
  onAction, 
  isLoading = false,
  isTracking = false,
  activeTrackingShipmentId = null,
}: ShipmentActionsProps) {
  const availableActions = STATUS_TRANSITIONS[shipment.status] || [];
  const isThisShipmentTracking = activeTrackingShipmentId === shipment.id;

  if (availableActions.length === 0) {
    return null;
  }

  const getButtonStyle = (style: 'primary' | 'secondary' | 'danger') => {
    switch (style) {
      case 'primary':
        return styles.primaryButton;
      case 'secondary':
        return styles.secondaryButton;
      case 'danger':
        return styles.dangerButton;
    }
  };

  const getTextStyle = (style: 'primary' | 'secondary' | 'danger') => {
    switch (style) {
      case 'primary':
        return styles.primaryText;
      case 'secondary':
        return styles.secondaryText;
      case 'danger':
        return styles.dangerText;
    }
  };

  const shouldDisableAction = (action: ValidAction): boolean => {
    if (isLoading) return true;
    if (isTracking && !isThisShipmentTracking) return true;
    if (action === 'start_transit' && isThisShipmentTracking) return false;
    return false;
  };

  return (
    <View style={styles.container}>
      {isThisShipmentTracking && (
        <View style={styles.trackingIndicator}>
          <View style={styles.trackingDot} />
          <Text style={styles.trackingText}>Tracking Active</Text>
        </View>
      )}
      {isTracking && !isThisShipmentTracking && (
        <View style={styles.trackingWarning}>
          <Text style={styles.trackingWarningText}>
            Stop tracking current shipment to perform actions
          </Text>
        </View>
      )}
      <Text style={styles.sectionTitle}>Actions</Text>
      <View style={styles.buttonsContainer}>
        {availableActions.map((action) => {
          const config = ACTION_CONFIG[action];
          const isDisabled = shouldDisableAction(action);
          return (
            <Pressable
              key={action}
              style={[
                styles.button, 
                getButtonStyle(config.style), 
                isDisabled && styles.buttonDisabled
              ]}
              onPress={() => onAction(action)}
              disabled={isDisabled}
            >
              {isLoading ? (
                <ActivityIndicator color="#fff" size="small" />
              ) : (
                <Text style={[styles.buttonText, getTextStyle(config.style)]}>
                  {config.label}
                </Text>
              )}
            </Pressable>
          );
        })}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    marginTop: spacing.lg,
  },
  trackingIndicator: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#d1fae5',
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
    borderRadius: 8,
    marginBottom: spacing.sm,
  },
  trackingDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
    backgroundColor: colors.success,
    marginRight: spacing.sm,
  },
  trackingText: {
    fontSize: 12,
    fontWeight: '600',
    color: '#065f46',
  },
  trackingWarning: {
    backgroundColor: '#fef3c7',
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
    borderRadius: 8,
    marginBottom: spacing.sm,
  },
  trackingWarningText: {
    fontSize: 12,
    color: '#92400e',
  },
  sectionTitle: {
    fontSize: 14,
    fontWeight: '600',
    color: colors.textSecondary,
    marginBottom: spacing.sm,
    textTransform: 'uppercase',
    letterSpacing: 0.5,
  },
  buttonsContainer: {
    gap: spacing.sm,
  },
  button: {
    paddingVertical: spacing.md,
    paddingHorizontal: spacing.lg,
    borderRadius: 8,
    alignItems: 'center',
    justifyContent: 'center',
  },
  buttonDisabled: {
    opacity: 0.6,
  },
  primaryButton: {
    backgroundColor: colors.primary,
  },
  secondaryButton: {
    backgroundColor: colors.background,
    borderWidth: 1,
    borderColor: colors.border,
  },
  dangerButton: {
    backgroundColor: '#fee2e2',
  },
  buttonText: {
    fontSize: 14,
    fontWeight: '600',
  },
  primaryText: {
    color: '#ffffff',
  },
  secondaryText: {
    color: colors.text,
  },
  dangerText: {
    color: colors.error,
  },
});

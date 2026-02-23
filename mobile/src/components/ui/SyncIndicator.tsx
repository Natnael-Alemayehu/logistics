import React from 'react';
import {
  View,
  Text,
  TouchableOpacity,
  StyleSheet,
  ActivityIndicator,
  ViewStyle,
} from 'react-native';
import { colors, spacing } from '../../utils/theme';
import { SyncStatus } from '../../types/components';

interface SyncIndicatorProps {
  status: SyncStatus;
  pendingCount: number;
  onPress?: () => void;
  style?: ViewStyle;
}

const SyncIndicator: React.FC<SyncIndicatorProps> = ({
  status,
  pendingCount,
  onPress,
  style,
}) => {
  const getStatusConfig = () => {
    switch (status) {
      case 'synced':
        return {
          icon: '✓',
          color: colors.success,
          text: 'Synced',
          showSpinner: false,
        };
      case 'pending':
        return {
          icon: '🕐',
          color: colors.warning,
          text: 'Pending',
          showSpinner: false,
        };
      case 'syncing':
        return {
          icon: '',
          color: colors.info,
          text: 'Syncing',
          showSpinner: true,
        };
      case 'error':
        return {
          icon: '!',
          color: colors.error,
          text: 'Sync failed',
          showSpinner: false,
        };
      default:
        return {
          icon: '?',
          color: colors.gray[500],
          text: 'Unknown',
          showSpinner: false,
        };
    }
  };

  const config = getStatusConfig();

  const content = (
    <View style={[styles.container, style]}>
      <View style={[styles.statusContainer, { borderColor: config.color }]}>
        {config.showSpinner ? (
          <ActivityIndicator size="small" color={config.color} />
        ) : (
          <Text style={[styles.icon, { color: config.color }]}>{config.icon}</Text>
        )}
        <Text style={[styles.statusText, { color: config.color }]}>{config.text}</Text>
      </View>
      {pendingCount > 0 && (
        <View style={styles.badge}>
          <Text style={styles.badgeText}>{pendingCount}</Text>
        </View>
      )}
    </View>
  );

  if (onPress) {
    return (
      <TouchableOpacity
        onPress={onPress}
        activeOpacity={0.7}
        accessibilityLabel={`Sync status: ${config.text}${pendingCount > 0 ? `, ${pendingCount} pending` : ''}`}
        accessibilityRole="button"
      >
        {content}
      </TouchableOpacity>
    );
  }

  return (
    <View
      accessibilityLabel={`Sync status: ${config.text}${pendingCount > 0 ? `, ${pendingCount} pending` : ''}`}
    >
      {content}
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  statusContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: spacing.sm,
    paddingVertical: spacing.xs,
    borderRadius: 16,
    borderWidth: 1,
    backgroundColor: colors.surface,
  },
  icon: {
    fontSize: 14,
    fontWeight: 'bold',
  },
  statusText: {
    fontSize: 12,
    fontWeight: '500',
    marginLeft: spacing.xs,
  },
  badge: {
    position: 'absolute',
    top: -4,
    right: -8,
    minWidth: 18,
    height: 18,
    borderRadius: 9,
    backgroundColor: colors.error,
    justifyContent: 'center',
    alignItems: 'center',
    paddingHorizontal: 4,
  },
  badgeText: {
    color: '#ffffff',
    fontSize: 10,
    fontWeight: 'bold',
  },
});

export default SyncIndicator;

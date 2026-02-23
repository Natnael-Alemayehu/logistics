import React, { useState } from 'react';
import { View, Text, TouchableOpacity, StyleSheet, ViewStyle } from 'react-native';
import { colors, spacing } from '../../utils/theme';

type BannerVariant = 'info' | 'warning' | 'error' | 'success';

interface BannerProps {
  message: string;
  variant?: BannerVariant;
  icon?: React.ReactNode;
  dismissible?: boolean;
  onDismiss?: () => void;
  style?: ViewStyle;
}

const Banner: React.FC<BannerProps> = ({
  message,
  variant = 'info',
  icon,
  dismissible = false,
  onDismiss,
  style,
}) => {
  const [isVisible, setIsVisible] = useState(true);

  if (!isVisible) return null;

  const getBackgroundColor = (): string => {
    switch (variant) {
      case 'info':
        return '#dbeafe';
      case 'warning':
        return '#fef3c7';
      case 'error':
        return '#fee2e2';
      case 'success':
        return '#dcfce7';
      default:
        return '#dbeafe';
    }
  };

  const getBorderColor = (): string => {
    switch (variant) {
      case 'info':
        return '#3b82f6';
      case 'warning':
        return '#f59e0b';
      case 'error':
        return '#ef4444';
      case 'success':
        return '#22c55e';
      default:
        return '#3b82f6';
    }
  };

  const getTextColor = (): string => {
    switch (variant) {
      case 'info':
        return '#1e40af';
      case 'warning':
        return '#92400e';
      case 'error':
        return '#991b1b';
      case 'success':
        return '#166534';
      default:
        return '#1e40af';
    }
  };

  const handleDismiss = () => {
    setIsVisible(false);
    onDismiss?.();
  };

  return (
    <View
      style={[
        styles.container,
        {
          backgroundColor: getBackgroundColor(),
          borderLeftColor: getBorderColor(),
        },
        style,
      ]}
    >
      {icon && <View style={styles.iconContainer}>{icon}</View>}
      <Text style={[styles.message, { color: getTextColor() }]}>{message}</Text>
      {dismissible && (
        <TouchableOpacity onPress={handleDismiss} style={styles.closeButton}>
          <Text style={[styles.closeText, { color: getTextColor() }]}>×</Text>
        </TouchableOpacity>
      )}
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: spacing.md,
    paddingHorizontal: spacing.lg,
    borderRadius: 8,
    borderLeftWidth: 4,
  },
  iconContainer: {
    marginRight: spacing.sm,
  },
  message: {
    flex: 1,
    fontSize: 14,
    fontWeight: '500',
  },
  closeButton: {
    marginLeft: spacing.sm,
    padding: spacing.xs,
  },
  closeText: {
    fontSize: 20,
    fontWeight: 'bold',
  },
});

export default Banner;

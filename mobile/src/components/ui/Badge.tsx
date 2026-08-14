import React from 'react';
import { View, Text, StyleSheet, ViewStyle } from 'react-native';
import { colors, spacing } from '../../utils/theme';

type BadgeVariant = 'gray' | 'blue' | 'yellow' | 'orange' | 'purple' | 'green' | 'red';
type BadgeSize = 'sm' | 'md';

interface BadgeProps {
  text: string;
  variant?: BadgeVariant;
  size?: BadgeSize;
  style?: ViewStyle;
}

const Badge: React.FC<BadgeProps> = ({
  text,
  variant = 'gray',
  size = 'md',
  style,
}) => {
  const getBackgroundColor = (): string => {
    switch (variant) {
      case 'gray':
        return '#f3f4f6';
      case 'blue':
        return '#dbeafe';
      case 'yellow':
        return '#fef3c7';
      case 'orange':
        return '#ffedd5';
      case 'purple':
        return '#f3e8ff';
      case 'green':
        return '#dcfce7';
      case 'red':
        return '#fee2e2';
      default:
        return '#f3f4f6';
    }
  };

  const getTextColor = (): string => {
    switch (variant) {
      case 'gray':
        return '#4b5563';
      case 'blue':
        return '#1d4ed8';
      case 'yellow':
        return '#b45309';
      case 'orange':
        return '#c2410c';
      case 'purple':
        return '#7c3aed';
      case 'green':
        return '#15803d';
      case 'red':
        return '#dc2626';
      default:
        return '#4b5563';
    }
  };

  return (
    <View
      style={[
        styles.badge,
        styles[`badge_${size}`],
        { backgroundColor: getBackgroundColor() },
        style,
      ]}
    >
      <Text style={[styles.text, styles[`text_${size}`], { color: getTextColor() }]}>
        {text}
      </Text>
    </View>
  );
};

const styles = StyleSheet.create({
  badge: {
    borderRadius: 9999,
    alignItems: 'center',
    justifyContent: 'center',
  },
  badge_sm: {
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
  },
  badge_md: {
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.xs,
  },
  text: {
    fontWeight: '500',
  },
  text_sm: {
    fontSize: 10,
  },
  text_md: {
    fontSize: 12,
  },
});

export default Badge;

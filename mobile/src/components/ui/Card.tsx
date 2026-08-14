import React from 'react';
import { View, Text, StyleSheet, ViewStyle } from 'react-native';
import { colors, spacing } from '../../utils/theme';

type PaddingVariant = 'none' | 'sm' | 'md' | 'lg';

interface CardProps {
  children: React.ReactNode;
  padding?: PaddingVariant;
  header?: string;
  style?: ViewStyle;
}

const Card: React.FC<CardProps> = ({
  children,
  padding = 'md',
  header,
  style,
}) => {
  const getPaddingStyle = (): ViewStyle => {
    switch (padding) {
      case 'none':
        return {};
      case 'sm':
        return { padding: spacing.sm };
      case 'lg':
        return { padding: spacing.xxl };
      case 'md':
      default:
        return { padding: spacing.lg };
    }
  };

  return (
    <View style={[styles.card, getPaddingStyle(), style]}>
      {header && <Text style={styles.header}>{header}</Text>}
      {children}
    </View>
  );
};

const styles = StyleSheet.create({
  card: {
    backgroundColor: colors.surface,
    borderRadius: 12,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 4,
    elevation: 3,
  },
  header: {
    fontSize: 18,
    fontWeight: '600',
    color: colors.text,
    marginBottom: spacing.md,
  },
});

export default Card;

import React from 'react';
import { View, Text, StyleSheet, ViewStyle } from 'react-native';
import { colors } from '../../utils/theme';

interface PendingDataBadgeProps {
  count: number;
  hasError?: boolean;
  style?: ViewStyle;
}

const PendingDataBadge: React.FC<PendingDataBadgeProps> = ({
  count,
  hasError = false,
  style,
}) => {
  if (count === 0 && !hasError) return null;

  const displayText = count > 0 ? (count > 99 ? '99+' : String(count)) : '!';

  return (
    <View
      style={[
        styles.badge,
        style,
      ]}
      accessibilityLabel={count > 0 ? `${count} pending items` : 'Sync error'}
    >
      <Text style={styles.text}>{displayText}</Text>
    </View>
  );
};

const styles = StyleSheet.create({
  badge: {
    minWidth: 20,
    height: 20,
    borderRadius: 10,
    backgroundColor: colors.error,
    justifyContent: 'center',
    alignItems: 'center',
    paddingHorizontal: 5,
  },
  text: {
    color: '#ffffff',
    fontSize: 11,
    fontWeight: 'bold',
    lineHeight: 14,
  },
});

export default PendingDataBadge;

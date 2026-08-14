import React, { useEffect, useRef } from 'react';
import {
  View,
  Text,
  StyleSheet,
  Animated,
  Dimensions,
  ViewStyle,
} from 'react-native';
import { colors, spacing } from '../../utils/theme';

interface SyncProgressProps {
  visible: boolean;
  progress: number;
  current?: number;
  total?: number;
  message?: string;
  style?: ViewStyle;
}

const SyncProgress: React.FC<SyncProgressProps> = ({
  visible,
  progress,
  current,
  total,
  message,
  style,
}) => {
  const progressAnim = useRef(new Animated.Value(0)).current;
  const opacity = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    Animated.timing(progressAnim, {
      toValue: Math.max(0, Math.min(100, progress)),
      duration: 300,
      useNativeDriver: false,
    }).start();
  }, [progress, progressAnim]);

  useEffect(() => {
    Animated.timing(opacity, {
      toValue: visible ? 1 : 0,
      duration: 200,
      useNativeDriver: true,
    }).start();
  }, [visible, opacity]);

  if (!visible) return null;

  const progressWidth = progressAnim.interpolate({
    inputRange: [0, 100],
    outputRange: ['0%', '100%'],
  });

  const displayMessage = message || 'Syncing...';
  const itemCountText = current !== undefined && total !== undefined
    ? ` ${current} of ${total} items`
    : '';

  return (
    <Animated.View style={[styles.container, { opacity }, style]}>
      <View style={styles.content}>
        <View style={styles.textContainer}>
          <Text style={styles.message}>{displayMessage}</Text>
          {itemCountText && (
            <Text style={styles.itemCount}>{itemCountText}</Text>
          )}
        </View>
        <View style={styles.progressContainer}>
          <Animated.View
            style={[styles.progressBar, { width: progressWidth }]}
          />
        </View>
        <Text style={styles.percentage}>{Math.round(progress)}%</Text>
      </View>
    </Animated.View>
  );
};

const styles = StyleSheet.create({
  container: {
    backgroundColor: colors.surface,
    borderTopWidth: 1,
    borderTopColor: colors.border,
    borderBottomWidth: 1,
    borderBottomColor: colors.border,
  },
  content: {
    paddingVertical: spacing.md,
    paddingHorizontal: spacing.lg,
  },
  textContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: spacing.sm,
  },
  message: {
    fontSize: 13,
    fontWeight: '500',
    color: colors.text,
  },
  itemCount: {
    fontSize: 12,
    color: colors.textSecondary,
    marginLeft: spacing.xs,
  },
  progressContainer: {
    height: 6,
    backgroundColor: colors.gray[200],
    borderRadius: 3,
    overflow: 'hidden',
    marginBottom: spacing.xs,
  },
  progressBar: {
    height: '100%',
    backgroundColor: colors.primary,
    borderRadius: 3,
  },
  percentage: {
    fontSize: 11,
    color: colors.textSecondary,
    textAlign: 'right',
  },
});

export default SyncProgress;

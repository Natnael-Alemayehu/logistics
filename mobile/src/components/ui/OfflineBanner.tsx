import React, { useEffect, useRef } from 'react';
import {
  View,
  Text,
  TouchableOpacity,
  StyleSheet,
  Animated,
  Dimensions,
  ViewStyle,
} from 'react-native';
import { colors, spacing } from '../../utils/theme';

interface OfflineBannerProps {
  visible: boolean;
  onDismiss?: () => void;
  style?: ViewStyle;
}

const OfflineBanner: React.FC<OfflineBannerProps> = ({
  visible,
  onDismiss,
  style,
}) => {
  const translateY = useRef(new Animated.Value(-100)).current;
  const opacity = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    if (visible) {
      Animated.parallel([
        Animated.spring(translateY, {
          toValue: 0,
          useNativeDriver: true,
          tension: 100,
          friction: 10,
        }),
        Animated.timing(opacity, {
          toValue: 1,
          duration: 200,
          useNativeDriver: true,
        }),
      ]).start();
    } else {
      Animated.parallel([
        Animated.timing(translateY, {
          toValue: -100,
          duration: 200,
          useNativeDriver: true,
        }),
        Animated.timing(opacity, {
          toValue: 0,
          duration: 150,
          useNativeDriver: true,
        }),
      ]).start();
    }
  }, [visible, translateY, opacity]);

  if (!visible) return null;

  return (
    <Animated.View
      style={[
        styles.container,
        { transform: [{ translateY }], opacity },
        style,
      ]}
    >
      <View style={styles.content}>
        <View style={styles.iconContainer}>
          <Text style={styles.icon}>⚠️</Text>
        </View>
        <Text style={styles.message}>
          You're offline. Data will sync when connection is restored.
        </Text>
        {onDismiss && (
          <TouchableOpacity
            onPress={onDismiss}
            style={styles.closeButton}
            accessibilityLabel="Dismiss offline banner"
            accessibilityRole="button"
          >
            <Text style={styles.closeText}>×</Text>
          </TouchableOpacity>
        )}
      </View>
    </Animated.View>
  );
};

const styles = StyleSheet.create({
  container: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    zIndex: 1000,
    backgroundColor: '#fef3c7',
    borderBottomWidth: 1,
    borderBottomColor: '#f59e0b',
  },
  content: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: spacing.md,
    paddingHorizontal: spacing.lg,
    paddingTop: spacing.xl + spacing.sm,
  },
  iconContainer: {
    marginRight: spacing.sm,
  },
  icon: {
    fontSize: 16,
  },
  message: {
    flex: 1,
    fontSize: 13,
    fontWeight: '500',
    color: '#92400e',
    lineHeight: 18,
  },
  closeButton: {
    marginLeft: spacing.sm,
    padding: spacing.xs,
  },
  closeText: {
    fontSize: 20,
    fontWeight: 'bold',
    color: '#92400e',
  },
});

export default OfflineBanner;

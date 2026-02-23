import React, { useEffect, useState } from 'react';
import { View, Text, StyleSheet, Animated, Pressable } from 'react-native';
import { colors, spacing, typography, BorderRadius, Shadows } from '@/utils/theme';
import type { GeofenceEvent, Geofence } from '@/services/location/geofencing';

interface GeofenceNotificationProps {
  event: GeofenceEvent;
  onDismiss?: () => void;
  autoDismissMs?: number;
}

export function GeofenceNotification({
  event,
  onDismiss,
  autoDismissMs = 5000,
}: GeofenceNotificationProps) {
  const [opacity] = useState(new Animated.Value(0));
  const [slideY] = useState(new Animated.Value(-50));

  useEffect(() => {
    Animated.parallel([
      Animated.timing(opacity, {
        toValue: 1,
        duration: 300,
        useNativeDriver: true,
      }),
      Animated.timing(slideY, {
        toValue: 0,
        duration: 300,
        useNativeDriver: true,
      }),
    ]).start();

    let timer: ReturnType<typeof setTimeout> | null = null;
    if (autoDismissMs > 0) {
      timer = setTimeout(() => {
        handleDismiss();
      }, autoDismissMs);
    }

    return () => {
      if (timer) clearTimeout(timer);
    };
  }, []);

  const handleDismiss = () => {
    Animated.parallel([
      Animated.timing(opacity, {
        toValue: 0,
        duration: 200,
        useNativeDriver: true,
      }),
      Animated.timing(slideY, {
        toValue: -50,
        duration: 200,
        useNativeDriver: true,
      }),
    ]).start(() => {
      onDismiss?.();
    });
  };

  const getNotificationContent = (): { title: string; message: string; iconName: string } => {
    const { geofence, event: eventType } = event;

    if (eventType === 'dwell') {
      return {
        title: `${geofence.type.charAt(0).toUpperCase() + geofence.type.slice(1)} Area`,
        message: 'You have been in this area for a while',
        iconName: '⏱️',
      };
    }

    switch (geofence.type) {
      case 'origin':
        if (eventType === 'enter') {
          return {
            title: 'Origin Area',
            message: 'You have entered the origin pickup area',
            iconName: '📍',
          };
        } else if (eventType === 'exit') {
          return {
            title: 'Leaving Origin',
            message: 'You are leaving the origin area. Safe travels!',
            iconName: '🚚',
          };
        }
        break;

      case 'destination':
        if (eventType === 'enter') {
          return {
            title: 'Arrived at Destination',
            message: 'You have arrived at the delivery destination',
            iconName: '🎉',
          };
        } else if (eventType === 'exit') {
          return {
            title: 'Leaving Destination',
            message: 'You are leaving the destination area',
            iconName: '📍',
          };
        }
        break;

      case 'checkpoint':
        if (eventType === 'enter') {
          return {
            title: 'Approaching Checkpoint',
            message: 'You are approaching a checkpoint',
            iconName: '🏁',
          };
        } else if (eventType === 'exit') {
          return {
            title: 'Checkpoint Passed',
            message: 'You have passed the checkpoint',
            iconName: '✅',
          };
        }
        break;
    }

    return {
      title: 'Geofence Event',
      message: `Geofence ${eventType} detected`,
      iconName: '📍',
    };
  };

  const getBackgroundColor = (): string => {
    const { geofence, event: eventType } = event;

    if (geofence.type === 'destination' && eventType === 'enter') {
      return colors.success;
    }

    if (eventType === 'exit') {
      return colors.info;
    }

    if (eventType === 'dwell') {
      return colors.warning;
    }

    return colors.primary;
  };

  const content = getNotificationContent();
  const backgroundColor = getBackgroundColor();

  return (
    <Animated.View
      style={[
        styles.container,
        {
          opacity,
          transform: [{ translateY: slideY }],
          backgroundColor,
        },
      ]}
    >
      <Pressable style={styles.content} onPress={handleDismiss}>
        <Text style={styles.icon}>{content.iconName}</Text>
        <View style={styles.textContainer}>
          <Text style={styles.title}>{content.title}</Text>
          <Text style={styles.message}>{content.message}</Text>
        </View>
        <Pressable style={styles.dismissButton} onPress={handleDismiss}>
          <Text style={styles.dismissText}>✕</Text>
        </Pressable>
      </Pressable>
    </Animated.View>
  );
}

interface GeofenceNotificationContainerProps {
  events: GeofenceEvent[];
  onDismissEvent: (eventId: string) => void;
}

export function GeofenceNotificationContainer({
  events,
  onDismissEvent,
}: GeofenceNotificationContainerProps) {
  if (events.length === 0) return null;

  const latestEvent = events[0];

  return (
    <View style={styles.containerWrapper}>
      <GeofenceNotification
        event={latestEvent}
        onDismiss={() => onDismissEvent(latestEvent.geofenceId)}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  containerWrapper: {
    position: 'absolute' as const,
    top: spacing.lg,
    left: spacing.md,
    right: spacing.md,
    zIndex: 1000,
  },
  container: {
    borderRadius: BorderRadius.lg,
    ...Shadows.lg,
  },
  content: {
    flexDirection: 'row' as const,
    alignItems: 'center',
    padding: spacing.md,
  },
  icon: {
    fontSize: 24,
    marginRight: spacing.md,
  },
  textContainer: {
    flex: 1,
  },
  title: {
    ...typography.title,
    color: colors.surface,
    fontSize: 16,
  },
  message: {
    ...typography.caption,
    color: colors.surface,
    opacity: 0.9,
    marginTop: 2,
  },
  dismissButton: {
    padding: spacing.sm,
  },
  dismissText: {
    color: colors.surface,
    fontSize: 18,
    fontWeight: '600' as const,
  },
});

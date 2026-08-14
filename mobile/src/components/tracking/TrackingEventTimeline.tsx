import React, { useEffect, useState } from 'react';
import { View, Text, StyleSheet, FlatList } from 'react-native';
import { colors, spacing } from '@/utils/theme';
import { getByShipmentId, TrackingEvent } from '@/db/repositories/trackingEvents';

interface TrackingEventTimelineProps {
  shipmentId: string;
  limit?: number;
}

interface TimelineEvent {
  id: string;
  type: 'status_change' | 'location' | 'checkpoint' | 'geofence';
  title: string;
  description?: string;
  timestamp: string;
  icon: string;
}

function formatTimestamp(isoString: string): string {
  const date = new Date(isoString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return 'Just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;
  return date.toLocaleDateString();
}

function getEventIcon(eventType: string): string {
  switch (eventType) {
    case 'status_change':
      return '📋';
    case 'checkpoint':
      return '📍';
    case 'geofence':
      return '🏠';
    case 'gps_ping':
    default:
      return '📌';
  }
}

function getEventTitle(event: TrackingEvent): string {
  switch (event.event_type) {
    case 'status_change':
      return `Status: ${event.status || 'Unknown'}`;
    case 'checkpoint':
      return 'Checkpoint recorded';
    case 'geofence':
      return event.note || 'Geofence event';
    case 'gps_ping':
    default:
      return 'Location update';
  }
}

function getEventDescription(event: TrackingEvent): string | undefined {
  if (event.speed !== undefined && event.speed !== null) {
    return `Speed: ${event.speed.toFixed(1)} km/h`;
  }
  if (event.note) {
    return event.note;
  }
  return undefined;
}

export function TrackingEventTimeline({ shipmentId, limit = 10 }: TrackingEventTimelineProps) {
  const [events, setEvents] = useState<TimelineEvent[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const loadEvents = async () => {
      setIsLoading(true);
      try {
        const trackingEvents = await getByShipmentId(shipmentId);
        const timelineEvents: TimelineEvent[] = trackingEvents
          .slice(0, limit)
          .map((event) => ({
            id: event.id,
            type: event.event_type === 'status_change' ? 'status_change' : 
                  event.event_type === 'checkpoint' ? 'checkpoint' : 
                  event.event_type === 'geofence' ? 'geofence' : 'location',
            title: getEventTitle(event),
            description: getEventDescription(event),
            timestamp: event.recorded_at,
            icon: getEventIcon(event.event_type),
          }));
        setEvents(timelineEvents);
      } catch (error) {
        console.error('Failed to load tracking events:', error);
      } finally {
        setIsLoading(false);
      }
    };

    loadEvents();
  }, [shipmentId, limit]);

  if (isLoading) {
    return (
      <View style={styles.loadingContainer}>
        <Text style={styles.loadingText}>Loading events...</Text>
      </View>
    );
  }

  if (events.length === 0) {
    return (
      <View style={styles.emptyContainer}>
        <Text style={styles.emptyText}>No tracking events yet</Text>
      </View>
    );
  }

  const renderEvent = ({ item, index }: { item: TimelineEvent; index: number }) => (
    <View style={styles.eventItem}>
      <View style={styles.timelineColumn}>
        <View style={styles.iconContainer}>
          <Text style={styles.icon}>{item.icon}</Text>
        </View>
        {index < events.length - 1 && <View style={styles.timelineLine} />}
      </View>
      <View style={styles.eventContent}>
        <Text style={styles.eventTitle}>{item.title}</Text>
        {item.description && (
          <Text style={styles.eventDescription}>{item.description}</Text>
        )}
        <Text style={styles.eventTimestamp}>{formatTimestamp(item.timestamp)}</Text>
      </View>
    </View>
  );

  return (
    <View style={styles.container}>
      <FlatList
        data={events}
        renderItem={renderEvent}
        keyExtractor={(item) => item.id}
        scrollEnabled={false}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    marginTop: spacing.sm,
  },
  loadingContainer: {
    padding: spacing.md,
    alignItems: 'center',
  },
  loadingText: {
    color: colors.textSecondary,
    fontSize: 12,
  },
  emptyContainer: {
    padding: spacing.md,
    alignItems: 'center',
  },
  emptyText: {
    color: colors.textSecondary,
    fontSize: 12,
  },
  eventItem: {
    flexDirection: 'row',
    marginBottom: spacing.md,
  },
  timelineColumn: {
    width: 32,
    alignItems: 'center',
  },
  iconContainer: {
    width: 28,
    height: 28,
    borderRadius: 14,
    backgroundColor: colors.gray[100],
    justifyContent: 'center',
    alignItems: 'center',
  },
  icon: {
    fontSize: 14,
  },
  timelineLine: {
    width: 2,
    flex: 1,
    backgroundColor: colors.gray[200],
    marginTop: spacing.xs,
  },
  eventContent: {
    flex: 1,
    marginLeft: spacing.sm,
    paddingTop: 4,
  },
  eventTitle: {
    fontSize: 14,
    fontWeight: '500',
    color: colors.text,
  },
  eventDescription: {
    fontSize: 12,
    color: colors.textSecondary,
    marginTop: 2,
  },
  eventTimestamp: {
    fontSize: 11,
    color: colors.gray[500],
    marginTop: 2,
  },
});

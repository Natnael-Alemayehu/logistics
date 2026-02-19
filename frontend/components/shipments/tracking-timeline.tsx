'use client'

import { format } from 'date-fns'
import { cn } from '@/lib/utils'
import { StatusBadge } from './status-badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { MapPin, Clock, CheckCircle2 } from 'lucide-react'
import type { TrackingEvent } from '@/types'

interface TrackingTimelineProps {
  events: TrackingEvent[]
  className?: string
}

const eventTypeLabels: Record<TrackingEvent['event_type'], string> = {
  gps_ping: 'GPS Update',
  checkpoint: 'Checkpoint',
  status_change: 'Status Update',
}

export function TrackingTimeline({ events, className }: TrackingTimelineProps) {
  if (!events || events.length === 0) {
    return (
      <Card className={className}>
        <CardHeader>
          <CardTitle className="text-lg">Tracking History</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">No tracking events available.</p>
        </CardContent>
      </Card>
    )
  }

  const sortedEvents = [...events].sort(
    (a, b) => new Date(b.recorded_at).getTime() - new Date(a.recorded_at).getTime()
  )

  return (
    <Card className={className}>
      <CardHeader>
        <CardTitle className="text-lg">Tracking History</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="relative space-y-6">
          {sortedEvents.map((event, index) => (
            <div key={event.id} className="relative flex gap-4">
              <div className="flex flex-col items-center">
                <div
                  className={cn(
                    'flex h-8 w-8 items-center justify-center rounded-full',
                    event.event_type === 'status_change'
                      ? 'bg-primary text-primary-foreground'
                      : event.event_type === 'checkpoint'
                        ? 'bg-green-100 text-green-600'
                        : 'bg-muted text-muted-foreground'
                  )}
                >
                  {event.event_type === 'status_change' ? (
                    <CheckCircle2 className="h-4 w-4" />
                  ) : (
                    <MapPin className="h-4 w-4" />
                  )}
                </div>
                {index < sortedEvents.length - 1 && (
                  <div className="h-full w-px bg-border" />
                )}
              </div>
              <div className="flex-1 pb-6">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-medium">
                    {eventTypeLabels[event.event_type]}
                  </span>
                  {event.status && <StatusBadge status={event.status} />}
                </div>
                {event.note && (
                  <p className="mt-1 text-sm text-muted-foreground">{event.note}</p>
                )}
                <div className="mt-2 flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
                  <div className="flex items-center gap-1">
                    <Clock className="h-3 w-3" />
                    {format(new Date(event.recorded_at), 'MMM d, yyyy h:mm a')}
                  </div>
                  {event.speed_kph !== undefined && (
                    <span>{event.speed_kph.toFixed(1)} km/h</span>
                  )}
                  {event.accuracy_meters !== undefined && (
                    <span>±{event.accuracy_meters.toFixed(0)}m</span>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

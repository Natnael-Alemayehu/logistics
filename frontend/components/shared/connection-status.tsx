'use client'

import { Wifi, WifiOff, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { ConnectionStatus } from '@/hooks/use-websocket'

interface ConnectionStatusIndicatorProps {
  status: ConnectionStatus
  onReconnect?: () => void
  className?: string
}

const statusConfig = {
  connected: {
    dotColor: 'bg-green-500',
    icon: Wifi,
    label: 'Connected',
    description: 'Real-time updates active',
  },
  connecting: {
    dotColor: 'bg-yellow-500',
    icon: Loader2,
    label: 'Connecting',
    description: 'Establishing connection...',
    animate: true,
  },
  disconnected: {
    dotColor: 'bg-red-500',
    icon: WifiOff,
    label: 'Disconnected',
    description: 'Click to reconnect',
  },
  error: {
    dotColor: 'bg-red-500',
    icon: WifiOff,
    label: 'Connection Error',
    description: 'Click to reconnect',
  },
}

export function ConnectionStatusIndicator({
  status,
  onReconnect,
  className = '',
}: ConnectionStatusIndicatorProps) {
  const config = statusConfig[status]
  const Icon = config.icon

  return (
    <Button
      variant="ghost"
      size="sm"
      onClick={status !== 'connected' ? onReconnect : undefined}
      className={`h-8 gap-2 px-2 ${className}`}
      disabled={status === 'connecting'}
      title={`${config.label} - ${config.description}`}
    >
      <span className="relative flex h-2 w-2">
        <span
          className={`absolute inline-flex h-full w-full rounded-full ${config.dotColor} opacity-75 ${
            status === 'connecting' ? 'animate-ping' : ''
          }`}
        />
        <span
          className={`relative inline-flex h-2 w-2 rounded-full ${config.dotColor}`}
        />
      </span>
      <Icon
        className={`h-4 w-4 text-muted-foreground ${
          'animate' in config && config.animate ? 'animate-spin' : ''
        }`}
      />
      <span className="hidden text-xs text-muted-foreground sm:inline">
        {config.label}
      </span>
    </Button>
  )
}

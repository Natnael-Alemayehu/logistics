'use client'

import { useState } from 'react'
import { WifiOff, CloudOff, RefreshCw, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface OfflineIndicatorProps {
  isOffline: boolean
  pendingCount: number
  isSyncing: boolean
  onSync?: () => void
  className?: string
}

export function OfflineIndicator({
  isOffline,
  pendingCount,
  isSyncing,
  onSync,
  className,
}: OfflineIndicatorProps) {
  const [dismissed, setDismissed] = useState(false)

  if (!isOffline && pendingCount === 0) {
    return null
  }

  if (dismissed && !isOffline && !isSyncing) {
    return null
  }

  return (
    <div
      className={cn(
        'fixed top-0 left-0 right-0 z-50 flex items-center justify-center gap-2 px-4 py-2 text-sm font-medium',
        isOffline
          ? 'bg-amber-500 text-amber-950'
          : isSyncing
          ? 'bg-blue-500 text-white'
          : 'bg-amber-500 text-amber-950',
        className
      )}
    >
      {isOffline ? (
        <>
          <WifiOff className="h-4 w-4" />
          <span>Working offline</span>
          {pendingCount > 0 && (
            <span className="ml-1 rounded-full bg-amber-900/20 px-2 py-0.5 text-xs">
              {pendingCount} pending
            </span>
          )}
        </>
      ) : isSyncing ? (
        <>
          <RefreshCw className="h-4 w-4 animate-spin" />
          <span>Syncing...</span>
        </>
      ) : (
        <>
          <CloudOff className="h-4 w-4" />
          <span>{pendingCount} operation{pendingCount > 1 ? 's' : ''} pending sync</span>
          <Button
            variant="outline"
            size="sm"
            onClick={onSync}
            className="ml-2 h-6 border-amber-900/30 bg-amber-900/20 text-amber-950 hover:bg-amber-900/30"
          >
            Sync now
          </Button>
        </>
      )}
      
      {!isOffline && !isSyncing && (
        <button
          onClick={() => setDismissed(true)}
          className="ml-2 rounded-full p-1 hover:bg-amber-900/20"
        >
          <X className="h-3 w-3" />
        </button>
      )}
    </div>
  )
}

'use client'

import { useState } from 'react'
import { Cloud, CloudOff, RefreshCw, Trash2, Clock } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { offlineStorage, type PendingOperation } from '@/lib/offline-storage'
import { cn } from '@/lib/utils'

interface OfflineQueueBadgeProps {
  pendingCount: number
  isSyncing: boolean
  onSync: () => void
}

const operationTypeLabels: Record<PendingOperation['type'], string> = {
  create_shipment: 'Create Shipment',
  update_shipment: 'Update Shipment',
  update_status: 'Update Status',
  delete_shipment: 'Delete Shipment',
  assign_driver: 'Assign Driver',
  create_pod: 'Proof of Delivery',
}

export function OfflineQueueBadge({ pendingCount, isSyncing, onSync }: OfflineQueueBadgeProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [operations, setOperations] = useState<PendingOperation[]>([])

  const fetchOperations = async () => {
    const ops = await offlineStorage.getPendingOperations()
    setOperations(ops)
  }

  const handleOpen = (open: boolean) => {
    setIsOpen(open)
    if (open) {
      fetchOperations()
    }
  }

  const handleRemoveOperation = async (id: string) => {
    await offlineStorage.removePendingOperation(id)
    setOperations((prev) => prev.filter((op) => op.id !== id))
  }

  const handleClearAll = async () => {
    await offlineStorage.clearPendingOperations()
    setOperations([])
  }

  const formatTimestamp = (timestamp: number) => {
    const date = new Date(timestamp)
    return date.toLocaleString()
  }

  if (pendingCount === 0 && !isSyncing) {
    return (
      <Badge variant="outline" className="gap-1 text-green-600 border-green-200">
        <Cloud className="h-3 w-3" />
        Synced
      </Badge>
    )
  }

  return (
    <Dialog open={isOpen} onOpenChange={handleOpen}>
      <DialogTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          className={cn(
            'h-8 gap-2',
            pendingCount > 0 ? 'text-amber-600' : 'text-muted-foreground'
          )}
        >
          {isSyncing ? (
            <RefreshCw className="h-4 w-4 animate-spin" />
          ) : (
            <CloudOff className="h-4 w-4" />
          )}
          <Badge
            variant={pendingCount > 0 ? 'default' : 'secondary'}
            className="h-5 px-1.5 text-xs"
          >
            {pendingCount}
          </Badge>
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <CloudOff className="h-5 w-5" />
            Pending Operations
          </DialogTitle>
          <DialogDescription>
            These operations will be synced when you&apos;re back online.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          {operations.length === 0 ? (
            <p className="text-center text-muted-foreground py-4">
              No pending operations
            </p>
          ) : (
            <>
              <div className="max-h-64 space-y-2 overflow-y-auto">
                {operations.map((op) => (
                  <div
                    key={op.id}
                    className="flex items-center justify-between rounded-lg border p-3"
                  >
                    <div className="flex-1">
                      <p className="text-sm font-medium">
                        {operationTypeLabels[op.type]}
                      </p>
                      <p className="text-xs text-muted-foreground flex items-center gap-1">
                        <Clock className="h-3 w-3" />
                        {formatTimestamp(op.timestamp)}
                      </p>
                    </div>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => handleRemoveOperation(op.id)}
                      className="h-8 w-8 text-destructive hover:text-destructive"
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                ))}
              </div>
              <div className="flex justify-between">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleClearAll}
                  className="text-destructive"
                >
                  Clear All
                </Button>
                <Button
                  size="sm"
                  onClick={() => {
                    onSync()
                    setIsOpen(false)
                  }}
                  disabled={isSyncing}
                >
                  {isSyncing ? (
                    <RefreshCw className="h-4 w-4 animate-spin mr-2" />
                  ) : null}
                  Sync Now
                </Button>
              </div>
            </>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

'use client'

import { useState, useCallback, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { offlineStorage, type PendingOperation } from '@/lib/offline-storage'
import { api } from '@/lib/api-client'
import { toast } from 'sonner'

export function useOfflineSync() {
  const [isSyncing, setIsSyncing] = useState(false)
  const [syncProgress, setSyncProgress] = useState({ current: 0, total: 0 })
  const [lastSyncError, setLastSyncError] = useState<string | null>(null)
  const queryClient = useQueryClient()
  const syncInProgress = useRef(false)

  const processOperation = async (operation: PendingOperation): Promise<boolean> => {
    try {
      switch (operation.method) {
        case 'POST':
          await api.post(operation.endpoint, operation.data)
          break
        case 'PUT':
          await api.put(operation.endpoint, operation.data)
          break
        case 'PATCH':
          await api.patch(operation.endpoint, operation.data)
          break
        case 'DELETE':
          await api.delete(operation.endpoint)
          break
        default:
          console.warn(`Unknown method: ${operation.method}`)
      }
      return true
    } catch (error) {
      console.error(`Failed to process operation ${operation.id}:`, error)
      
      if (operation.retries < 3) {
        await offlineStorage.updatePendingOperation(operation.id, {
          retries: operation.retries + 1,
        })
      } else {
        await offlineStorage.removePendingOperation(operation.id)
      }
      return false
    }
  }

  const sync = useCallback(async () => {
    if (syncInProgress.current || !navigator.onLine) {
      return { success: false, reason: 'Sync already in progress or offline' }
    }

    syncInProgress.current = true
    setIsSyncing(true)
    setLastSyncError(null)

    try {
      const operations = await offlineStorage.getPendingOperations()
      
      if (operations.length === 0) {
        setIsSyncing(false)
        syncInProgress.current = false
        return { success: true, processed: 0 }
      }

      setSyncProgress({ current: 0, total: operations.length })

      let processed = 0
      let failed = 0

      for (const operation of operations) {
        const success = await processOperation(operation)
        
        if (success) {
          await offlineStorage.removePendingOperation(operation.id)
          processed++
        } else {
          failed++
        }
        
        setSyncProgress((prev) => ({ ...prev, current: prev.current + 1 }))
      }

      await queryClient.invalidateQueries()

      if (processed > 0) {
        toast.success(`Synced ${processed} operation${processed > 1 ? 's' : ''}`)
      }

      if (failed > 0) {
        setLastSyncError(`${failed} operation${failed > 1 ? 's' : ''} failed to sync`)
        toast.error(`${failed} operation${failed > 1 ? 's' : ''} failed to sync`)
      }

      return { success: true, processed, failed }
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Sync failed'
      setLastSyncError(errorMessage)
      toast.error(errorMessage)
      return { success: false, reason: errorMessage }
    } finally {
      setIsSyncing(false)
      syncInProgress.current = false
    }
  }, [queryClient])

  const addToQueue = useCallback(async (
    type: PendingOperation['type'],
    endpoint: string,
    method: PendingOperation['method'],
    data: unknown
  ) => {
    const id = await offlineStorage.addPendingOperation({
      type,
      endpoint,
      method,
      data,
    })
    
    if (!navigator.onLine) {
      toast.info('Operation queued for sync when online')
    }
    
    return id
  }, [])

  return {
    isSyncing,
    syncProgress,
    lastSyncError,
    sync,
    addToQueue,
  }
}

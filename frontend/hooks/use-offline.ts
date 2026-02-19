'use client'

import { useState, useEffect, useCallback } from 'react'
import { offlineStorage } from '@/lib/offline-storage'

function getInitialOnlineStatus(): boolean {
  if (typeof window === 'undefined') return true
  return navigator.onLine
}

export function useOffline() {
  const [isOnline, setIsOnline] = useState(getInitialOnlineStatus)
  const [pendingOperationsCount, setPendingOperationsCount] = useState(0)

  const updatePendingCount = useCallback(async () => {
    const count = await offlineStorage.getPendingOperationsCount()
    setPendingOperationsCount(count)
  }, [])

  useEffect(() => {
    const handleOnline = () => {
      setIsOnline(true)
    }

    const handleOffline = () => {
      setIsOnline(false)
    }

    window.addEventListener('online', handleOnline)
    window.addEventListener('offline', handleOffline)

    updatePendingCount()

    const interval = setInterval(updatePendingCount, 5000)

    return () => {
      window.removeEventListener('online', handleOnline)
      window.removeEventListener('offline', handleOffline)
      clearInterval(interval)
    }
  }, [updatePendingCount])

  return {
    isOnline,
    isOffline: !isOnline,
    pendingOperationsCount,
    refreshPendingCount: updatePendingCount,
  }
}

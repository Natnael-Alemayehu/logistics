import { useEffect, useState, useCallback } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useWebSocket, type DriverLocationUpdate } from './use-websocket'
import { useDriverLocations } from './use-drivers'
import type { DriverLocation } from '@/types'

interface RealtimeDriverLocation extends DriverLocation {
  isRealtime?: boolean
}

export function useDriverLocationsRealtime() {
  const queryClient = useQueryClient()
  const { data: initialData, isLoading, error } = useDriverLocations()
  const [realtimeLocations, setRealtimeLocations] = useState<Map<string, DriverLocationUpdate>>(new Map())
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null)

  const handleLocationUpdate = useCallback((data: unknown) => {
    const update = data as DriverLocationUpdate
    
    if (update.driver_id && update.lat != null && update.lng != null) {
      setRealtimeLocations((prev) => {
        const next = new Map(prev)
        next.set(update.driver_id, update)
        return next
      })
      setLastUpdate(new Date())
      
      queryClient.setQueryData<{ locations: DriverLocation[] }>(['driver-locations'], (old) => {
        if (!old?.locations) return old
        
        const locations = old.locations.map((loc) => {
          if (loc.driver_id === update.driver_id) {
            return {
              ...loc,
              lat: update.lat,
              lng: update.lng,
              speed_kph: update.speed,
              last_update: update.timestamp,
            }
          }
          return loc
        })
        
        return { locations }
      })
    }
  }, [queryClient])

  const { status, subscribe, reconnect } = useWebSocket({
    debug: process.env.NODE_ENV === 'development',
  })

  useEffect(() => {
    if (status === 'connected') {
      return subscribe('driver-locations', handleLocationUpdate)
    }
  }, [status, subscribe, handleLocationUpdate])

  const mergedLocations: RealtimeDriverLocation[] | undefined = initialData?.locations?.map((loc) => {
    const realtimeUpdate = realtimeLocations.get(loc.driver_id)
    if (realtimeUpdate) {
      return {
        ...loc,
        lat: realtimeUpdate.lat,
        lng: realtimeUpdate.lng,
        speed_kph: realtimeUpdate.speed ?? loc.speed_kph,
        last_update: realtimeUpdate.timestamp,
        isRealtime: true,
      } as RealtimeDriverLocation
    }
    return loc as RealtimeDriverLocation
  })

  return {
    locations: mergedLocations,
    isLoading,
    error,
    connectionStatus: status,
    lastUpdate,
    reconnect,
    realtimeUpdateCount: realtimeLocations.size,
  }
}

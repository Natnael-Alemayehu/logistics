'use client'

import { MapContainer, TileLayer, Marker, Popup, useMap } from 'react-leaflet'
import * as L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { useDriverLocations } from '@/hooks/use-drivers'
import { useDriverLocationsRealtime } from '@/hooks/use-driver-locations-realtime'
import { Skeleton } from '@/components/ui/skeleton'
import { Truck } from 'lucide-react'
import { useEffect, useRef } from 'react'
import type { DriverLocation } from '@/types'

const ADDIS_ABABA: [number, number] = [9.0, 38.7]

const truckIcon = L.divIcon({
  html: `<div style="font-size: 24px; display: flex; align-items: center; justify-content: center; transform: translate(-12px, -12px);">🚚</div>`,
  className: 'truck-marker',
  iconSize: [24, 24],
  iconAnchor: [12, 12],
  popupAnchor: [0, -12],
})

const realtimeTruckIcon = L.divIcon({
  html: `<div style="font-size: 24px; display: flex; align-items: center; justify-content: center; transform: translate(-12px, -12px);"><span style="animation: pulse 2s infinite;">🚚</span></div>
<style>@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.6; } }</style>`,
  className: 'truck-marker realtime',
  iconSize: [24, 24],
  iconAnchor: [12, 12],
  popupAnchor: [0, -12],
})

function MapSkeleton() {
  return (
    <div className="flex h-[300px] items-center justify-center rounded-lg border bg-muted">
      <div className="space-y-3 text-center">
        <Skeleton className="mx-auto h-12 w-12 rounded-full" />
        <Skeleton className="h-4 w-32" />
        <Skeleton className="h-3 w-24" />
      </div>
    </div>
  )
}

function EmptyMapState() {
  return (
    <div className="flex h-[300px] items-center justify-center rounded-lg border bg-muted">
      <div className="text-center">
        <Truck className="mx-auto h-8 w-8 text-muted-foreground" />
        <p className="mt-2 text-sm text-muted-foreground">
          No driver locations available
        </p>
        <p className="text-xs text-muted-foreground">
          Driver locations will appear here when they are on duty
        </p>
      </div>
    </div>
  )
}

function DriverMarkers() {
  const { data, isLoading, error } = useDriverLocations()

  if (isLoading) return null
  if (error || !data?.locations?.length) return null

  return (
    <>
      {data.locations.map((location) => {
        if (location.lat == null || location.lng == null) return null

        return (
          <Marker
            key={location.driver_id}
            position={[location.lat, location.lng]}
            icon={truckIcon}
          >
            <Popup>
              <div className="min-w-[150px]">
                <p className="font-semibold">{location.driver_name}</p>
                <p className="text-sm text-muted-foreground">
                  Status: {location.status}
                </p>
                {location.speed_kph !== undefined && (
                  <p className="text-xs text-muted-foreground">
                    Speed: {location.speed_kph.toFixed(1)} km/h
                  </p>
                )}
                {location.battery_level !== undefined && (
                  <p className="text-xs text-muted-foreground">
                    Battery: {location.battery_level}%
                  </p>
                )}
              </div>
            </Popup>
          </Marker>
        )
      })}
    </>
  )
}

interface RealtimeDriverLocation extends DriverLocation {
  isRealtime?: boolean
}

interface RealtimeDriverMarkersProps {
  locations: RealtimeDriverLocation[] | undefined
}

function RealtimeDriverMarkers({ locations }: RealtimeDriverMarkersProps) {
  const map = useMap()
  const prevPositionsRef = useRef<Map<string, [number, number]>>(new Map())

  useEffect(() => {
    if (!locations) return
    
    locations.forEach((loc) => {
      if (loc.lat == null || loc.lng == null) return
      
      const prevPos = prevPositionsRef.current.get(loc.driver_id)
      if (prevPos && (prevPos[0] !== loc.lat || prevPos[1] !== loc.lng)) {
        prevPositionsRef.current.set(loc.driver_id, [loc.lat, loc.lng])
      } else {
        prevPositionsRef.current.set(loc.driver_id, [loc.lat, loc.lng])
      }
    })
  }, [locations, map])

  if (!locations?.length) return null

  return (
    <>
      {locations.map((location) => {
        if (location.lat == null || location.lng == null) return null

        const isRealtime = location.isRealtime === true

        return (
          <Marker
            key={location.driver_id}
            position={[location.lat, location.lng]}
            icon={isRealtime ? realtimeTruckIcon : truckIcon}
          >
            <Popup>
              <div className="min-w-[150px]">
                <p className="font-semibold">{location.driver_name}</p>
                <p className="text-sm text-muted-foreground">
                  Status: {location.status}
                </p>
                {location.speed_kph !== undefined && (
                  <p className="text-xs text-muted-foreground">
                    Speed: {location.speed_kph.toFixed(1)} km/h
                  </p>
                )}
                {location.battery_level !== undefined && (
                  <p className="text-xs text-muted-foreground">
                    Battery: {location.battery_level}%
                  </p>
                )}
                {isRealtime && (
                  <p className="text-xs text-green-600 font-medium">
                    🟢 Live
                  </p>
                )}
              </div>
            </Popup>
          </Marker>
        )
      })}
    </>
  )
}

export function ActiveMap() {
  const { data, isLoading, error } = useDriverLocations()

  if (isLoading) return <MapSkeleton />

  const validLocations = data?.locations?.filter(
    (loc) => loc.lat != null && loc.lng != null
  )

  if (error || !validLocations?.length) {
    return <EmptyMapState />
  }

  return (
    <MapContainer
      center={ADDIS_ABABA}
      zoom={12}
      className="h-[300px] w-full rounded-lg"
      scrollWheelZoom={true}
    >
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      <DriverMarkers />
    </MapContainer>
  )
}

export function RealtimeActiveMap() {
  const { locations, isLoading, error } = useDriverLocationsRealtime()

  if (isLoading) return <MapSkeleton />

  const validLocations = locations?.filter(
    (loc) => loc.lat != null && loc.lng != null
  )

  if (error || !validLocations?.length) {
    return <EmptyMapState />
  }

  return (
    <MapContainer
      center={ADDIS_ABABA}
      zoom={12}
      className="h-[300px] w-full rounded-lg"
      scrollWheelZoom={true}
    >
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      <RealtimeDriverMarkers locations={validLocations as RealtimeDriverLocation[]} />
    </MapContainer>
  )
}

import { useState, useEffect, useCallback } from 'react';
import {
  Geofence,
  GeofenceEvent,
  addGeofence,
  removeGeofence,
  clearGeofences,
  getGeofences,
  getRecentEvents,
  checkGeofences,
  startGeofenceMonitoring,
  stopGeofenceMonitoring,
  isGeofenceMonitoring,
  onGeofenceEvent,
  createShipmentGeofences,
} from '@/services/location/geofencing';

export interface UseGeofencingResult {
  activeGeofences: Geofence[];
  recentEvents: GeofenceEvent[];
  isMonitoring: boolean;
  setupShipmentGeofences: (shipment: {
    id: string;
    origin_lat?: number;
    origin_lng?: number;
    destination_lat?: number;
    destination_lng?: number;
  }) => void;
  clearAllGeofences: () => void;
  addGeofence: (geofence: Geofence) => void;
  removeGeofence: (id: string) => void;
  checkPosition: (position: { latitude: number; longitude: number } | null) => void;
}

export function useGeofencing(): UseGeofencingResult {
  const [activeGeofences, setActiveGeofences] = useState<Geofence[]>([]);
  const [recentEvents, setRecentEvents] = useState<GeofenceEvent[]>([]);
  const [isMonitoring, setIsMonitoring] = useState(false);

  useEffect(() => {
    setActiveGeofences(getGeofences());
    setRecentEvents(getRecentEvents());
    setIsMonitoring(isGeofenceMonitoring());

    const unsubscribe = onGeofenceEvent((event) => {
      setRecentEvents(getRecentEvents());
      setActiveGeofences(getGeofences());
    });

    return () => {
      unsubscribe();
    };
  }, []);

  const setupShipmentGeofences = useCallback((shipment: {
    id: string;
    origin_lat?: number;
    origin_lng?: number;
    destination_lat?: number;
    destination_lng?: number;
  }) => {
    const geofences = createShipmentGeofences(shipment);
    geofences.forEach(addGeofence);
    setActiveGeofences(getGeofences());
    
    if (!isMonitoring) {
      startGeofenceMonitoring();
      setIsMonitoring(true);
    }
  }, [isMonitoring]);

  const clearAllGeofences = useCallback(() => {
    clearGeofences();
    setActiveGeofences([]);
    setRecentEvents([]);
  }, []);

  const addGeofenceHandler = useCallback((geofence: Geofence) => {
    addGeofence(geofence);
    setActiveGeofences(getGeofences());
  }, []);

  const removeGeofenceHandler = useCallback((id: string) => {
    removeGeofence(id);
    setActiveGeofences(getGeofences());
  }, []);

  const checkPosition = useCallback((position: { latitude: number; longitude: number } | null) => {
    checkGeofences(position);
    setRecentEvents(getRecentEvents());
  }, []);

  return {
    activeGeofences,
    recentEvents,
    isMonitoring,
    setupShipmentGeofences,
    clearAllGeofences,
    addGeofence: addGeofenceHandler,
    removeGeofence: removeGeofenceHandler,
    checkPosition,
  };
}

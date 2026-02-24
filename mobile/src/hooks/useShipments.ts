import { useCallback, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import * as shipmentsApi from '@/services/api/shipments';
import * as shipmentsDb from '@/db/repositories/shipments';
import * as syncMetadataRepo from '@/db/repositories/syncMetadata';
import { useConnectivity } from './useConnectivity';
import { useAuthStore } from '@/store/authStore';
import { useTrackingStore } from '@/store/trackingStore';
import { startLocationTracking, stopLocationTracking, getCurrentLocation } from '@/services/location';
import { getTrackingContext, clearTrackingContext } from '@/services/location/trackingContext';
import { insert as insertTrackingEvent } from '@/db/repositories/trackingEvents';
import type { Shipment } from '@/types/shipment';
import type { Shipment as DbShipment } from '@/db/repositories/shipments';

export type { Shipment };

export function useShipments() {
  const queryClient = useQueryClient();
  const { isOnline } = useConnectivity();
  const user = useAuthStore((state) => state.user);
  const isInitialSyncing = useAuthStore((state) => state.isInitialSyncing);
  const { 
    isTracking, 
    activeShipmentId, 
    startTracking: startTrackingStore, 
    stopTracking: stopTrackingStore 
  } = useTrackingStore();

  const {
    data: shipments,
    isLoading,
    error,
    refetch,
    isFetching,
    dataUpdatedAt,
  } = useQuery({
    queryKey: ['shipments'],
    queryFn: async () => {
      const cachedShipments = await shipmentsDb.getAll();
      const cached = cachedShipments.map(transformDbShipment);

      if (!isOnline || isInitialSyncing) {
        return cached;
      }

      try {
        const apiShipments = await shipmentsApi.getMyShipments();
        
        await shipmentsDb.upsertMany(
          apiShipments.map((s) => ({
            id: s.id,
            tracking_number: s.tracking_number,
            origin_address: s.origin_address,
            origin_lat: s.origin_lat,
            origin_lng: s.origin_lng,
            destination_address: s.destination_address,
            destination_lat: s.destination_lat,
            destination_lng: s.destination_lng,
            customer_name: s.customer_name,
            customer_phone: s.customer_phone,
            cargo_description: s.cargo_description,
            cargo_weight: s.cargo_weight,
            cargo_value: s.cargo_value,
            special_instructions: s.special_instructions,
            status: s.status,
            status_note: s.status_note,
            status_reason: s.status_reason,
            driver_id: s.driver_id,
            vehicle_id: s.vehicle_id,
            estimated_delivery: s.estimated_delivery,
            created_at: s.created_at,
            updated_at: s.updated_at,
          }))
        );

        if (apiShipments.length > 0) {
          await syncMetadataRepo.upsert({
            last_sync_at: new Date().toISOString(),
          });
        }

        return apiShipments;
      } catch (apiError) {
        if (cached.length > 0) {
          return cached;
        }
        throw apiError;
      }
    },
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: true,
    refetchOnReconnect: true,
  });

  useEffect(() => {
    if (isOnline && !isInitialSyncing && dataUpdatedAt) {
      const now = Date.now();
      const lastUpdate = dataUpdatedAt;
      const staleTime = 5 * 60 * 1000;
      
      if (now - lastUpdate > staleTime) {
        refetch();
      }
    }
  }, [isOnline, isInitialSyncing, dataUpdatedAt, refetch]);

  const updateStatusMutation = useMutation({
    mutationFn: async ({
      shipmentId,
      status,
      note,
      reason,
    }: {
      shipmentId: string;
      status: string;
      note?: string;
      reason?: string;
    }) => {
      if (isOnline) {
        await shipmentsApi.updateStatus(shipmentId, status, note, reason);
      }
      await shipmentsDb.update(shipmentId, { status });
    },
    onMutate: async ({ shipmentId, status }) => {
      await queryClient.cancelQueries({ queryKey: ['shipments'] });
      const previousShipments = queryClient.getQueryData<Shipment[]>(['shipments']);
      
      queryClient.setQueryData<Shipment[]>(['shipments'], (old) =>
        old?.map((s) => (s.id === shipmentId ? { ...s, status } : s))
      );
      
      return { previousShipments };
    },
    onError: (_err, _variables, context) => {
      if (context?.previousShipments) {
        queryClient.setQueryData(['shipments'], context.previousShipments);
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['shipments'] });
    },
  });

  const {
    data: shipment,
    isLoading: isLoadingShipment,
    error: shipmentError,
  } = useQuery({
    queryKey: ['shipment'],
    queryFn: async () => {
      const local = await shipmentsDb.getById(shipment?.id || '');
      return local ? transformDbShipment(local) : null;
    },
    enabled: false,
  });

  const fetchShipments = useCallback(async () => {
    return refetch();
  }, [refetch]);

  const refreshFromServer = useCallback(async () => {
    if (!isOnline) {
      throw new Error('Cannot refresh while offline');
    }
    return refetch();
  }, [isOnline, refetch]);

  const updateStatus = useCallback(
    (shipmentId: string, status: string, note?: string, reason?: string) => {
      return updateStatusMutation.mutateAsync({ shipmentId, status, note, reason });
    },
    [updateStatusMutation]
  );

  const startTrackingForShipment = useCallback(async (shipmentId: string) => {
    const driverId = user?.id;
    if (!driverId) {
      throw new Error('No driver ID available');
    }

    if (isTracking && activeShipmentId && activeShipmentId !== shipmentId) {
      await stopTrackingForShipment();
    }

    try {
      startTrackingStore(shipmentId, driverId);
      await startLocationTracking(shipmentId, driverId);
      
      const location = await getCurrentLocation();
      if (location) {
        await insertTrackingEvent({
          shipment_id: shipmentId,
          driver_id: driverId,
          latitude: location.latitude,
          longitude: location.longitude,
          accuracy: location.accuracy ?? undefined,
          speed: location.speed ? location.speed * 3.6 : undefined,
          heading: location.heading ?? undefined,
          event_type: 'status_change',
          status: 'in_transit',
          note: 'Tracking started',
          recorded_at: new Date().toISOString(),
        });
      }
      
      return true;
    } catch (error) {
      stopTrackingStore();
      throw error;
    }
  }, [user?.id, isTracking, activeShipmentId, startTrackingStore, stopTrackingStore]);

  const stopTrackingForShipment = useCallback(async () => {
    try {
      await stopLocationTracking();
      stopTrackingStore();
      return true;
    } catch (error) {
      console.error('Failed to stop tracking:', error);
      throw error;
    }
  }, [stopTrackingStore]);

  const getActiveTrackingShipment = useCallback(async () => {
    const context = await getTrackingContext();
    return context?.shipmentId ?? null;
  }, []);

  const restoreTrackingState = useCallback(async () => {
    const context = await getTrackingContext();
    if (context) {
      startTrackingStore(context.shipmentId, context.driverId);
    }
  }, [startTrackingStore]);

  return {
    shipments: shipments ?? [],
    shipment,
    isLoading,
    isLoadingShipment,
    isFetching,
    error,
    shipmentError,
    fetchShipments,
    refreshFromServer,
    updateStatus,
    updateStatusPending: updateStatusMutation.isPending,
    refetch,
    isTracking,
    activeShipmentId,
    startTrackingForShipment,
    stopTrackingForShipment,
    getActiveTrackingShipment,
    restoreTrackingState,
  };
}

function transformDbShipment(db: DbShipment): Shipment {
  return {
    id: db.id,
    tenant_id: db.tenant_id ?? '',
    tracking_number: db.tracking_number,
    origin_address: db.origin_address,
    origin_lat: db.origin_lat,
    origin_lng: db.origin_lng,
    destination_address: db.destination_address,
    destination_lat: db.destination_lat,
    destination_lng: db.destination_lng,
    customer_name: db.customer_name,
    customer_phone: db.customer_phone,
    cargo_description: db.cargo_description,
    cargo_weight: db.cargo_weight,
    cargo_value: db.cargo_value,
    special_instructions: db.special_instructions,
    driver_id: db.driver_id,
    vehicle_id: db.vehicle_id,
    status: db.status as Shipment['status'],
    status_note: db.status_note,
    status_reason: db.status_reason,
    estimated_delivery: db.estimated_delivery,
    actual_delivery: db.actual_delivery,
    created_at: db.created_at,
    updated_at: db.updated_at,
    created_by: db.created_by,
  };
}

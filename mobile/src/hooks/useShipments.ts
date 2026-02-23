import { useCallback } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import * as shipmentsApi from '@/services/api/shipments';
import * as shipmentsDb from '@/db/repositories/shipments';
import { useConnectivity } from './useConnectivity';

export interface Shipment {
  id: string;
  tracking_number: string;
  status: string;
  origin: string;
  destination: string;
  customer_name: string;
  customer_phone: string;
  scheduled_date: string;
  notes?: string;
  created_at: string;
  updated_at: string;
}

export function useShipments() {
  const queryClient = useQueryClient();
  const { isOnline } = useConnectivity();

  const {
    data: shipments,
    isLoading,
    error,
    refetch,
    isFetching,
  } = useQuery({
    queryKey: ['shipments'],
    queryFn: async () => {
      if (isOnline) {
        try {
          const apiShipments = await shipmentsApi.getMyShipments();
          await shipmentsDb.upsertMany(
            apiShipments.map((s) => ({
              id: s.id,
              tracking_number: s.tracking_number,
              origin_address: s.origin,
              destination_address: s.destination,
              customer_name: s.customer_name,
              customer_phone: s.customer_phone,
              status: s.status,
              created_at: s.created_at,
              updated_at: s.updated_at,
            }))
          );
          return apiShipments;
        } catch (error) {
          const localShipments = await shipmentsDb.getAll();
          return localShipments.map(transformDbShipment);
        }
      }
      const localShipments = await shipmentsDb.getAll();
      return localShipments.map(transformDbShipment);
    },
    staleTime: 5 * 60 * 1000,
  });

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
  };
}

function transformDbShipment(db: shipmentsDb.Shipment): Shipment {
  return {
    id: db.id,
    tracking_number: db.tracking_number,
    status: db.status,
    origin: db.origin_address,
    destination: db.destination_address,
    customer_name: db.customer_name,
    customer_phone: db.customer_phone,
    scheduled_date: db.estimated_delivery ?? '',
    notes: db.special_instructions,
    created_at: db.created_at,
    updated_at: db.updated_at,
  };
}

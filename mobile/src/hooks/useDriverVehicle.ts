import { useQuery } from '@tanstack/react-query';
import { getDriverVehicle } from '@/services/api/driver';
import { useConnectivity } from './useConnectivity';
import type { Vehicle } from '@/types/driver';

export function useDriverVehicle() {
  const { isOnline } = useConnectivity();

  const {
    data: vehicle,
    isLoading,
    error,
    refetch,
    isFetching,
  } = useQuery({
    queryKey: ['driverVehicle'],
    queryFn: async (): Promise<Vehicle | null> => {
      if (!isOnline) {
        return null;
      }
      
      try {
        return await getDriverVehicle();
      } catch {
        return null;
      }
    },
    staleTime: 10 * 60 * 1000,
    refetchOnWindowFocus: true,
    refetchOnReconnect: true,
    retry: 1,
  });

  return {
    vehicle,
    isLoading,
    isFetching,
    error,
    refetch,
  };
}

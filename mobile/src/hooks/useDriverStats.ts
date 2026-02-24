import { useQuery } from '@tanstack/react-query';
import { getDriverStats } from '@/services/api/driver';
import { useConnectivity } from './useConnectivity';
import type { DriverStats } from '@/types/driver';

const DEFAULT_STATS: DriverStats = {
  deliveriesToday: 0,
  deliveriesWeek: 0,
  deliveriesMonth: 0,
};

export function useDriverStats() {
  const { isOnline } = useConnectivity();

  const {
    data: stats,
    isLoading,
    error,
    refetch,
    isFetching,
  } = useQuery({
    queryKey: ['driverStats'],
    queryFn: async (): Promise<DriverStats> => {
      if (!isOnline) {
        return DEFAULT_STATS;
      }
      
      try {
        return await getDriverStats();
      } catch {
        return DEFAULT_STATS;
      }
    },
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: true,
    refetchOnReconnect: true,
    retry: 1,
  });

  return {
    stats: stats ?? DEFAULT_STATS,
    isLoading,
    isFetching,
    error,
    refetch,
  };
}

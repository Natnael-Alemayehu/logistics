import { useState, useEffect, useCallback } from 'react';
import { useConnectivity } from './useConnectivity';

export interface OfflineFirstResult<T> {
  data: T | null;
  isLoading: boolean;
  isStale: boolean;
  error: Error | null;
  refetch: () => Promise<void>;
  isFetching: boolean;
}

export interface OfflineFirstOptions<T> {
  cacheKey: string;
  fetchFromApi: () => Promise<T>;
  fetchFromLocal: () => Promise<T | null>;
  saveToLocal: (data: T) => Promise<void>;
  getCacheTimestamp?: () => Promise<string | null>;
  staleTime?: number;
}

export function useOfflineFirst<T>(options: OfflineFirstOptions<T>): OfflineFirstResult<T> {
  const {
    cacheKey,
    fetchFromApi,
    fetchFromLocal,
    saveToLocal,
    getCacheTimestamp,
    staleTime = 5 * 60 * 1000,
  } = options;

  const { isOnline } = useConnectivity();
  
  const [data, setData] = useState<T | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isFetching, setIsFetching] = useState(false);
  const [isStale, setIsStale] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const checkIfStale = useCallback(async (): Promise<boolean> => {
    if (!getCacheTimestamp) return false;
    
    const timestamp = await getCacheTimestamp();
    if (!timestamp) return true;
    
    const cachedTime = new Date(timestamp).getTime();
    const now = Date.now();
    
    return now - cachedTime > staleTime;
  }, [getCacheTimestamp, staleTime]);

  const fetchData = useCallback(async (forceApi = false) => {
    setIsFetching(true);
    setError(null);

    try {
      if (isOnline || forceApi) {
        try {
          const apiData = await fetchFromApi();
          await saveToLocal(apiData);
          setData(apiData);
          setIsStale(false);
          setIsFetching(false);
          return;
        } catch (apiError) {
          if (!isOnline) {
            throw apiError;
          }
        }
      }

      const localData = await fetchFromLocal();
      if (localData !== null) {
        setData(localData);
        setIsStale(await checkIfStale());
      } else {
        setError(new Error('No data available offline'));
      }
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch data'));
    } finally {
      setIsFetching(false);
      setIsLoading(false);
    }
  }, [isOnline, fetchFromApi, fetchFromLocal, saveToLocal, checkIfStale]);

  const refetch = useCallback(async () => {
    await fetchData(true);
  }, [fetchData]);

  useEffect(() => {
    fetchData();
  }, [cacheKey]);

  useEffect(() => {
    if (isOnline && isStale) {
      fetchData(true);
    }
  }, [isOnline, isStale, fetchData]);

  return {
    data,
    isLoading,
    isStale,
    error,
    refetch,
    isFetching,
  };
}

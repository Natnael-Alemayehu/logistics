'use client'

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { useState, useEffect } from 'react'
import { ApiError } from '@/lib/api-client'
import { offlineStorage } from '@/lib/offline-storage'

export function QueryProvider({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 5 * 60 * 1000,
            gcTime: 24 * 60 * 60 * 1000,
            retry: (failureCount, error) => {
              if (error instanceof ApiError && error.status === 401) return false
              if (!navigator.onLine) return false
              return failureCount < 3
            },
            refetchOnWindowFocus: true,
            refetchOnReconnect: true,
            networkMode: 'online',
          },
          mutations: {
            retry: false,
            networkMode: 'online',
          },
        },
      })
  )

  useEffect(() => {
    const persistCache = async () => {
      const cache = queryClient.getQueryCache()
      const queries = cache.findAll()
      
      for (const query of queries) {
        const data = query.state.data
        if (data) {
          const queryKey = query.queryKey
          if (queryKey[0] === 'shipments') {
            if (queryKey[1] === undefined || typeof queryKey[1] === 'object') {
              await offlineStorage.setShipments((data as { shipments: [] }).shipments || [])
            }
          }
        }
      }
    }

    const unsubscribe = queryClient.getQueryCache().subscribe(() => {
      persistCache()
    })

    return () => unsubscribe()
  }, [queryClient])

  return (
    <QueryClientProvider client={queryClient}>
      {children}
      <ReactQueryDevtools initialIsOpen={false} />
    </QueryClientProvider>
  )
}

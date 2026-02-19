import { create } from 'zustand'
import type { ShipmentStatus } from '@/types'

interface FilterState {
  shipmentStatus: ShipmentStatus | 'all'
  driverId: string | 'all'
  searchQuery: string
  dateFrom: string
  dateTo: string
  setStatus: (status: ShipmentStatus | 'all') => void
  setDriverId: (driverId: string | 'all') => void
  setSearchQuery: (query: string) => void
  setDateFrom: (date: string) => void
  setDateTo: (date: string) => void
  resetFilters: () => void
}

export const useFilterStore = create<FilterState>((set) => ({
  shipmentStatus: 'all',
  driverId: 'all',
  searchQuery: '',
  dateFrom: '',
  dateTo: '',
  setStatus: (status) => set({ shipmentStatus: status }),
  setDriverId: (driverId) => set({ driverId }),
  setSearchQuery: (searchQuery) => set({ searchQuery }),
  setDateFrom: (dateFrom) => set({ dateFrom }),
  setDateTo: (dateTo) => set({ dateTo }),
  resetFilters: () =>
    set({
      shipmentStatus: 'all',
      driverId: 'all',
      searchQuery: '',
      dateFrom: '',
      dateTo: '',
    }),
}))

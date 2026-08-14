import { create } from 'zustand';
import type { Shipment } from '@/types/shipment';

interface ShipmentsState {
  shipments: Shipment[];
  activeShipmentId: string | null;
  isLoading: boolean;
  lastFetched: string | null;

  setShipments: (shipments: Shipment[]) => void;
  updateShipment: (id: string, updates: Partial<Shipment>) => void;
  setActiveShipment: (id: string | null) => void;
  setLoading: (loading: boolean) => void;
  getShipmentById: (id: string) => Shipment | undefined;
}

export const useShipmentsStore = create<ShipmentsState>((set, get) => ({
  shipments: [],
  activeShipmentId: null,
  isLoading: false,
  lastFetched: null,

  setShipments: (shipments) => {
    set({
      shipments,
      lastFetched: new Date().toISOString(),
    });
  },

  updateShipment: (id, updates) => {
    set((state) => ({
      shipments: state.shipments.map((shipment) =>
        shipment.id === id ? { ...shipment, ...updates } : shipment
      ),
    }));
  },

  setActiveShipment: (id) => {
    set({ activeShipmentId: id });
  },

  setLoading: (loading) => {
    set({ isLoading: loading });
  },

  getShipmentById: (id) => {
    return get().shipments.find((shipment) => shipment.id === id);
  },
}));

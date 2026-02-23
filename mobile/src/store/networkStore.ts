import { create } from 'zustand';

interface NetworkState {
  isOnline: boolean;
  lastOnlineTime: Date | null;
  setOnline: (status: boolean) => void;
}

export const useNetworkStore = create<NetworkState>((set) => ({
  isOnline: true,
  lastOnlineTime: null,

  setOnline: (status: boolean) => {
    set({
      isOnline: status,
      lastOnlineTime: status ? new Date() : undefined,
    });
  },
}));

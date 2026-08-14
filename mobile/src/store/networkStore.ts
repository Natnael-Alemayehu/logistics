import { create } from 'zustand';

export type ConnectionQuality = 'fast' | 'medium' | 'slow' | 'offline';
export type ConnectionType = 'wifi' | 'cellular' | 'none' | 'unknown';

export interface ConnectionHistoryEntry {
  timestamp: Date;
  isOnline: boolean;
  connectionType: ConnectionType;
  connectionQuality: ConnectionQuality;
}

interface NetworkState {
  isOnline: boolean;
  lastOnlineTime: Date | null;
  lastOnlineAt: Date | null;
  lastOfflineAt: Date | null;
  connectionType: ConnectionType;
  connectionQuality: ConnectionQuality;
  isMetered: boolean;
  connectionHistory: ConnectionHistoryEntry[];
  setOnline: (status: boolean) => void;
  setConnectionType: (type: ConnectionType) => void;
  setConnectionQuality: (quality: ConnectionQuality) => void;
  setIsMetered: (metered: boolean) => void;
  addConnectionHistory: (entry: Omit<ConnectionHistoryEntry, 'timestamp'>) => void;
  getLastOnlineAt: () => Date | null;
  getLastOfflineAt: () => Date | null;
}

const MAX_HISTORY_ENTRIES = 10;

export const useNetworkStore = create<NetworkState>((set, get) => ({
  isOnline: true,
  lastOnlineTime: null,
  lastOnlineAt: null,
  lastOfflineAt: null,
  connectionType: 'unknown',
  connectionQuality: 'fast',
  isMetered: false,
  connectionHistory: [],

  setOnline: (status: boolean) => {
    set((state) => {
      const updates: Partial<NetworkState> = {
        isOnline: status,
        lastOnlineTime: status ? new Date() : state.lastOnlineTime,
      };

      if (status) {
        updates.lastOnlineAt = new Date();
        updates.connectionQuality = state.connectionType === 'none' ? 'fast' : state.connectionQuality;
      } else {
        updates.lastOfflineAt = new Date();
        updates.connectionQuality = 'offline';
      }

      return updates;
    });
  },

  setConnectionType: (type: ConnectionType) => {
    set({ connectionType: type });
  },

  setConnectionQuality: (quality: ConnectionQuality) => {
    set({ connectionQuality: quality });
  },

  setIsMetered: (metered: boolean) => {
    set({ isMetered: metered });
  },

  addConnectionHistory: (entry: Omit<ConnectionHistoryEntry, 'timestamp'>) => {
    set((state) => {
      const newEntry: ConnectionHistoryEntry = {
        ...entry,
        timestamp: new Date(),
      };

      const history = [newEntry, ...state.connectionHistory].slice(0, MAX_HISTORY_ENTRIES);

      return { connectionHistory: history };
    });
  },

  getLastOnlineAt: () => get().lastOnlineAt,
  getLastOfflineAt: () => get().lastOfflineAt,
}));

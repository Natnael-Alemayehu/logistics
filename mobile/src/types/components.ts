export type SyncStatus = 'synced' | 'pending' | 'syncing' | 'error';

export interface OfflineBannerProps {
  visible: boolean;
  onDismiss?: () => void;
}

export interface SyncIndicatorProps {
  status: SyncStatus;
  pendingCount: number;
  onPress?: () => void;
}

export interface SyncProgressProps {
  visible: boolean;
  progress: number;
  current?: number;
  total?: number;
  message?: string;
}

export interface PendingDataBadgeProps {
  count: number;
  hasError?: boolean;
}

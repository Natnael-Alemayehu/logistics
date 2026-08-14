import React, { useState, useCallback } from 'react';
import { View, StyleSheet, ViewStyle } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { useNetworkStore } from '@store/networkStore';
import { useSyncStore } from '@store/syncStore';
import OfflineBanner from '@components/ui/OfflineBanner';
import SyncProgress from '@components/ui/SyncProgress';
import PendingDataBadge from '@components/ui/PendingDataBadge';
import { spacing } from '@/utils/theme';

interface SyncStatusBarProps {
  style?: ViewStyle;
  showBadge?: boolean;
}

const SyncStatusBar: React.FC<SyncStatusBarProps> = ({
  style,
  showBadge = true,
}) => {
  const insets = useSafeAreaInsets();
  const isOnline = useNetworkStore((state) => state.isOnline);
  const isSyncing = useSyncStore((state) => state.isSyncing);
  const pendingCount = useSyncStore((state) => state.pendingCount);
  const syncProgress = useSyncStore((state) => state.syncProgress);
  const lastError = useSyncStore((state) => state.lastError);

  const [bannerDismissed, setBannerDismissed] = useState(false);

  const handleDismissBanner = useCallback(() => {
    setBannerDismissed(true);
  }, []);

  const showOfflineBanner = !isOnline && !bannerDismissed;
  const showSyncProgressBar = isSyncing && syncProgress.phase !== 'idle';
  const shouldShowBadge = showBadge && !isSyncing && pendingCount > 0;

  return (
    <View style={[styles.container, style]}>
      <OfflineBanner
        visible={showOfflineBanner}
        onDismiss={handleDismissBanner}
      />

      <SyncProgress
        visible={showSyncProgressBar}
        progress={syncProgress.percentage}
        current={syncProgress.itemsProcessed}
        total={syncProgress.totalItems}
        message={syncProgress.message}
      />

      {shouldShowBadge && (
        <View style={[styles.pendingContainer, { top: insets.top + 8 }]}>
          <PendingDataBadge
            count={pendingCount}
            hasError={!!lastError}
          />
        </View>
      )}
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    zIndex: 1000,
  },
  pendingContainer: {
    position: 'absolute',
    right: spacing.lg,
  },
});

export default SyncStatusBar;

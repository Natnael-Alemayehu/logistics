import React from 'react';
import { View, Text, StyleSheet, ViewStyle } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { useNetworkStore } from '@store/networkStore';
import { useSyncStore } from '@store/syncStore';
import { SyncStatus } from '@/types/components';
import SyncIndicator from '@components/ui/SyncIndicator';
import { colors, spacing } from '@/utils/theme';

interface AppHeaderProps {
  title: string;
  showSyncIndicator?: boolean;
  rightAction?: React.ReactNode;
  onSyncPress?: () => void;
  style?: ViewStyle;
}

const AppHeader: React.FC<AppHeaderProps> = ({
  title,
  showSyncIndicator = true,
  rightAction,
  onSyncPress,
  style,
}) => {
  const insets = useSafeAreaInsets();
  const isOnline = useNetworkStore((state) => state.isOnline);
  const isSyncing = useSyncStore((state) => state.isSyncing);
  const pendingCount = useSyncStore((state) => state.pendingCount);
  const lastError = useSyncStore((state) => state.lastError);

  const getSyncStatus = (): SyncStatus => {
    if (lastError) return 'error';
    if (isSyncing) return 'syncing';
    if (pendingCount > 0) return 'pending';
    return 'synced';
  };

  return (
    <View style={[styles.container, { paddingTop: insets.top }, style]}>
      <View style={styles.content}>
        <View style={styles.titleContainer}>
          <Text style={styles.title} numberOfLines={1}>{title}</Text>
          {!isOnline && (
            <View style={styles.offlineDot} />
          )}
        </View>

        {showSyncIndicator && (
          <SyncIndicator
            status={getSyncStatus()}
            pendingCount={pendingCount}
            onPress={onSyncPress}
          />
        )}

        {rightAction && (
          <View style={styles.rightAction}>
            {rightAction}
          </View>
        )}
      </View>
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    backgroundColor: colors.primary,
    borderBottomWidth: 1,
    borderBottomColor: 'rgba(255, 255, 255, 0.1)',
  },
  content: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.md,
    minHeight: 44,
  },
  titleContainer: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
  },
  title: {
    fontSize: 18,
    fontWeight: '600',
    color: '#ffffff',
  },
  offlineDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
    backgroundColor: colors.warning,
    marginLeft: spacing.sm,
  },
  rightAction: {
    marginLeft: spacing.sm,
  },
});

export default AppHeader;

import { useState, useCallback, useMemo } from 'react';
import {
  ResolutionStrategy,
  SyncConflict,
  ResolvedData,
  ResolvedBatch,
} from '@/types/conflict';
import { conflictResolver } from '@/services/sync/ConflictResolver';

interface UseConflictResolutionOptions {
  onConflictResolved?: (resolved: ResolvedData) => void;
  onAllConflictsResolved?: (batch: ResolvedBatch) => void;
  onManualRequired?: (conflicts: SyncConflict[]) => void;
  autoResolve?: boolean;
}

interface UseConflictResolutionReturn {
  conflicts: SyncConflict[];
  manualConflicts: SyncConflict[];
  isResolving: boolean;
  error: string | null;
  lastResolvedBatch: ResolvedBatch | null;
  hasConflicts: boolean;
  hasManualConflicts: boolean;
  detectConflict: (
    localData: Record<string, any>,
    serverData: Record<string, any>,
    entityType: 'shipment' | 'tracking_event' | 'proof_of_delivery',
    entityId: string
  ) => SyncConflict | null;
  addConflict: (conflict: SyncConflict) => void;
  addConflicts: (conflicts: SyncConflict[]) => void;
  resolveConflict: (conflict: SyncConflict, strategy?: ResolutionStrategy) => ResolvedData | null;
  resolveConflictWithChoice: (
    conflict: SyncConflict,
    fieldChoices: Record<string, 'local' | 'server' | 'merge'>
  ) => ResolvedData | null;
  resolveAllConflicts: () => ResolvedBatch;
  resolveFromDialog: (resolvedData: ResolvedData) => void;
  clearConflicts: () => void;
  clearError: () => void;
  dismissManualConflict: (conflictId: string) => void;
}

export function useConflictResolution(
  options: UseConflictResolutionOptions = {}
): UseConflictResolutionReturn {
  const { onConflictResolved, onAllConflictsResolved, onManualRequired, autoResolve = true } = options;

  const [conflicts, setConflicts] = useState<SyncConflict[]>([]);
  const [manualConflicts, setManualConflicts] = useState<SyncConflict[]>([]);
  const [isResolving, setIsResolving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastResolvedBatch, setLastResolvedBatch] = useState<ResolvedBatch | null>(null);

  const hasConflicts = useMemo(() => conflicts.length > 0, [conflicts]);
  const hasManualConflicts = useMemo(() => manualConflicts.length > 0, [manualConflicts]);

  const detectConflict = useCallback(
    (
      localData: Record<string, any>,
      serverData: Record<string, any>,
      entityType: 'shipment' | 'tracking_event' | 'proof_of_delivery',
      entityId: string
    ): SyncConflict | null => {
      return conflictResolver.detectConflict(localData, serverData, entityType, entityId);
    },
    []
  );

  const addConflict = useCallback((conflict: SyncConflict) => {
    setConflicts((prev) => {
      const exists = prev.some(
        (c) => c.entity_type === conflict.entity_type && c.entity_id === conflict.entity_id
      );
      if (exists) return prev;
      return [...prev, conflict];
    });
  }, []);

  const addConflicts = useCallback((newConflicts: SyncConflict[]) => {
    setConflicts((prev) => {
      const existingKeys = new Set(prev.map((c) => `${c.entity_type}:${c.entity_id}`));
      const uniqueNew = newConflicts.filter(
        (c) => !existingKeys.has(`${c.entity_type}:${c.entity_id}`)
      );
      return [...prev, ...uniqueNew];
    });
  }, []);

  const resolveConflict = useCallback(
    (conflict: SyncConflict, strategy?: ResolutionStrategy): ResolvedData | null => {
      setIsResolving(true);
      setError(null);

      try {
        const resolved = conflictResolver.resolveConflict(conflict, strategy);

        setConflicts((prev) => prev.filter((c) => c.id !== conflict.id));

        onConflictResolved?.(resolved);

        return resolved;
      } catch (err) {
        if (err instanceof Error && err.message.includes('Manual resolution required')) {
          setManualConflicts((prev) => {
            const exists = prev.some((c) => c.id === conflict.id);
            if (exists) return prev;
            return [...prev, conflict];
          });

          setConflicts((prev) => prev.filter((c) => c.id !== conflict.id));

          onManualRequired?.([conflict]);

          return null;
        }

        setError(err instanceof Error ? err.message : 'Failed to resolve conflict');
        return null;
      } finally {
        setIsResolving(false);
      }
    },
    [onConflictResolved, onManualRequired]
  );

  const resolveConflictWithChoice = useCallback(
    (
      conflict: SyncConflict,
      fieldChoices: Record<string, 'local' | 'server' | 'merge'>
    ): ResolvedData | null => {
      setIsResolving(true);
      setError(null);

      try {
        const resolved = conflictResolver.resolveWithManualChoice(conflict, fieldChoices);

        setManualConflicts((prev) => prev.filter((c) => c.id !== conflict.id));

        onConflictResolved?.(resolved);

        return resolved;
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to resolve conflict');
        return null;
      } finally {
        setIsResolving(false);
      }
    },
    [onConflictResolved]
  );

  const resolveAllConflicts = useCallback((): ResolvedBatch => {
    setIsResolving(true);
    setError(null);

    try {
      const batch = conflictResolver.resolveBatch(conflicts);

      setConflicts([]);
      setManualConflicts(batch.manual_conflicts);
      setLastResolvedBatch(batch);

      if (batch.manual_conflicts.length > 0) {
        onManualRequired?.(batch.manual_conflicts);
      }

      onAllConflictsResolved?.(batch);

      return batch;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to resolve conflicts');
      return {
        total_conflicts: conflicts.length,
        resolved_count: 0,
        manual_required_count: conflicts.length,
        resolutions: [],
        manual_conflicts: conflicts,
      };
    } finally {
      setIsResolving(false);
    }
  }, [conflicts, onAllConflictsResolved, onManualRequired]);

  const resolveFromDialog = useCallback(
    (resolvedData: ResolvedData) => {
      setManualConflicts((prev) => prev.filter((c) => c.id === resolvedData.conflict_id ? false : true));

      const conflict = manualConflicts.find((c) => c.id === resolvedData.conflict_id);
      if (conflict) {
        onConflictResolved?.(resolvedData);
      }
    },
    [manualConflicts, onConflictResolved]
  );

  const clearConflicts = useCallback(() => {
    setConflicts([]);
    setManualConflicts([]);
    setLastResolvedBatch(null);
  }, []);

  const clearError = useCallback(() => {
    setError(null);
  }, []);

  const dismissManualConflict = useCallback((conflictId: string) => {
    setManualConflicts((prev) => prev.filter((c) => c.id !== conflictId));
  }, []);

  return {
    conflicts,
    manualConflicts,
    isResolving,
    error,
    lastResolvedBatch,
    hasConflicts,
    hasManualConflicts,
    detectConflict,
    addConflict,
    addConflicts,
    resolveConflict,
    resolveConflictWithChoice,
    resolveAllConflicts,
    resolveFromDialog,
    clearConflicts,
    clearError,
    dismissManualConflict,
  };
}

export default useConflictResolution;

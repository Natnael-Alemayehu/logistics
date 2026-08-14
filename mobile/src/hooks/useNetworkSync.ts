import { useCallback, useEffect, useRef, useState } from 'react';
import { useConnectivity } from './useConnectivity';
import { SyncQueueManager, syncQueueManager, SyncQueueItem, SyncBatchResult } from '@/services/sync';

export interface QueuedOperation {
  id: string;
  entityType: string;
  entityId: string;
  operation: string;
  priority: number;
  attempts: number;
  lastError?: string;
  createdAt: string;
  retryAt?: string;
  status: string;
  payload?: unknown;
}

export interface NetworkSyncOptions {
  maxRetries?: number;
  retryDelay?: number;
  batchSize?: number;
  onSyncSuccess?: (operation: QueuedOperation) => void;
  onSyncError?: (operation: QueuedOperation, error: Error) => void;
  onBatchComplete?: (results: SyncBatchResult) => void;
}

const operationPayloads = new Map<string, unknown>();

function mapItemToOperation(item: SyncQueueItem): QueuedOperation {
  return {
    id: item.id.toString(),
    entityType: item.entity_type,
    entityId: item.entity_id,
    operation: item.operation,
    priority: item.priority,
    attempts: item.attempts,
    lastError: item.last_error,
    createdAt: item.created_at,
    retryAt: item.retry_at,
    status: item.status,
  };
}

export function useNetworkSync(options: NetworkSyncOptions = {}) {
  const {
    maxRetries = 5,
    retryDelay = 5000,
    batchSize = 10,
    onSyncSuccess,
    onSyncError,
    onBatchComplete,
  } = options;

  const { isOnline } = useConnectivity();
  const [pendingOperations, setPendingOperations] = useState<QueuedOperation[]>([]);
  const [pendingCount, setPendingCount] = useState(0);
  const [isSyncing, setIsSyncing] = useState(false);
  const [syncErrors, setSyncErrors] = useState<Map<string, string>>(new Map());
  const [queueManager] = useState(() => new SyncQueueManager({ maxAttempts: maxRetries }));
  
  const syncHandlers = useRef<Map<string, (entityId: string, payload?: unknown) => Promise<void>>>(new Map());
  const syncTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const loadPendingOperations = useCallback(async () => {
    try {
      const count = await queueManager.getCount();
      setPendingCount(count);
      
      const items = await queueManager.getReadyForSync();
      const operations = items.map(mapItemToOperation);
      setPendingOperations(operations);
    } catch (error) {
      console.error('Failed to load pending operations:', error);
    }
  }, [queueManager]);

  const registerHandler = useCallback(
    (operation: string, handler: (entityId: string, payload?: unknown) => Promise<void>) => {
      syncHandlers.current.set(operation, handler);
    },
    []
  );

  const unregisterHandler = useCallback((operation: string) => {
    syncHandlers.current.delete(operation);
  }, []);

  const queueOperation = useCallback(
    async (
      entityType: string,
      entityId: string,
      operation: string,
      payload?: unknown,
      priority = 0
    ): Promise<string> => {
      try {
        const id = await queueManager.add({
          entity_type: entityType,
          entity_id: entityId,
          operation,
          priority,
        });

        if (payload !== undefined) {
          operationPayloads.set(`${entityType}:${entityId}:${operation}`, payload);
        }

        await loadPendingOperations();

        if (isOnline) {
          processQueue();
        }

        return id.toString();
      } catch (error) {
        console.error('Failed to queue operation:', error);
        throw error;
      }
    },
    [isOnline, loadPendingOperations, queueManager]
  );

  const processOperation = useCallback(
    async (op: QueuedOperation): Promise<{ success: boolean; error?: string }> => {
      const handler = syncHandlers.current.get(op.operation);
      
      if (!handler) {
        console.warn(`No handler registered for operation: ${op.operation}`);
        return { success: false, error: `No handler for ${op.operation}` };
      }

      try {
        const payload = operationPayloads.get(`${op.entityType}:${op.entityId}:${op.operation}`);
        await handler(op.entityId, payload);
        
        await queueManager.markSynced(parseInt(op.id, 10));
        operationPayloads.delete(`${op.entityType}:${op.entityId}:${op.operation}`);
        
        setSyncErrors((prev) => {
          const next = new Map(prev);
          next.delete(op.id);
          return next;
        });
        
        onSyncSuccess?.(op);
        return { success: true };
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : 'Unknown error';
        
        setSyncErrors((prev) => {
          const next = new Map(prev);
          next.set(op.id, errorMessage);
          return next;
        });

        await queueManager.markFailed(parseInt(op.id, 10), errorMessage);
        
        if (op.attempts >= maxRetries - 1) {
          onSyncError?.(op, error instanceof Error ? error : new Error(errorMessage));
        }
        
        return { success: false, error: errorMessage };
      }
    },
    [maxRetries, onSyncSuccess, onSyncError, queueManager]
  );

  const processQueue = useCallback(async () => {
    if (!isOnline || isSyncing) return;

    setIsSyncing(true);

    try {
      const batch = await queueManager.getBatchForSync(batchSize);
      
      if (batch.items.length === 0) {
        return;
      }

      const results: SyncBatchResult = {
        succeeded: [],
        failed: [],
      };

      for (const item of batch.items) {
        const op = mapItemToOperation(item);
        const result = await processOperation(op);
        
        if (result.success) {
          results.succeeded.push(item.id);
        } else {
          results.failed.push({ id: item.id, error: result.error ?? 'Unknown error' });
        }
      }

      onBatchComplete?.(results);
      await loadPendingOperations();
    } catch (error) {
      console.error('Failed to process sync queue:', error);
    } finally {
      setIsSyncing(false);
    }
  }, [isOnline, isSyncing, batchSize, processOperation, loadPendingOperations, queueManager, onBatchComplete]);

  const retryOperation = useCallback(
    async (operationId: string) => {
      await queueManager.resetAttempts(parseInt(operationId, 10));
      await processQueue();
    },
    [processQueue, queueManager]
  );

  const removeOperation = useCallback(async (operationId: string) => {
    await queueManager.remove(parseInt(operationId, 10));
    await loadPendingOperations();
    
    setSyncErrors((prev) => {
      const next = new Map(prev);
      next.delete(operationId);
      return next;
    });
  }, [loadPendingOperations, queueManager]);

  const clearQueue = useCallback(async () => {
    await queueManager.clearAll();
    operationPayloads.clear();
    setPendingOperations([]);
    setPendingCount(0);
    setSyncErrors(new Map());
  }, [queueManager]);

  const compactQueue = useCallback(async () => {
    await queueManager.compactQueue();
    await loadPendingOperations();
  }, [loadPendingOperations, queueManager]);

  const getDeadLetterItems = useCallback(async () => {
    return await queueManager.getDeadLetterItems();
  }, [queueManager]);

  const retryFromDeadLetter = useCallback(
    async (id: number) => {
      const newId = await queueManager.retryFromDeadLetter(id);
      if (newId !== null) {
        await loadPendingOperations();
        if (isOnline) {
          processQueue();
        }
      }
      return newId;
    },
    [isOnline, loadPendingOperations, processQueue, queueManager]
  );

  useEffect(() => {
    loadPendingOperations();
  }, [loadPendingOperations]);

  useEffect(() => {
    if (isOnline && pendingCount > 0 && !isSyncing) {
      if (syncTimeoutRef.current) {
        clearTimeout(syncTimeoutRef.current);
      }
      
      syncTimeoutRef.current = setTimeout(() => {
        processQueue();
      }, retryDelay);
    }

    return () => {
      if (syncTimeoutRef.current) {
        clearTimeout(syncTimeoutRef.current);
      }
    };
  }, [isOnline, pendingCount, isSyncing, processQueue, retryDelay]);

  return {
    pendingOperations,
    pendingCount,
    isSyncing,
    syncErrors,
    registerHandler,
    unregisterHandler,
    queueOperation,
    processQueue,
    retryOperation,
    removeOperation,
    clearQueue,
    compactQueue,
    getDeadLetterItems,
    retryFromDeadLetter,
  };
}

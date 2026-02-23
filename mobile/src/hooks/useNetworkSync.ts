import { useCallback, useEffect, useRef, useState } from 'react';
import { useConnectivity } from './useConnectivity';
import * as syncQueue from '@/db/repositories/syncQueue';

export interface QueuedOperation {
  id: string;
  entityType: string;
  entityId: string;
  operation: string;
  priority: number;
  attempts: number;
  lastError?: string;
  createdAt: string;
  payload?: unknown;
}

export interface NetworkSyncOptions {
  maxRetries?: number;
  retryDelay?: number;
  onSyncSuccess?: (operation: QueuedOperation) => void;
  onSyncError?: (operation: QueuedOperation, error: Error) => void;
}

const operationPayloads = new Map<string, unknown>();

export function useNetworkSync(options: NetworkSyncOptions = {}) {
  const {
    maxRetries = 3,
    retryDelay = 5000,
    onSyncSuccess,
    onSyncError,
  } = options;

  const { isOnline } = useConnectivity();
  const [pendingOperations, setPendingOperations] = useState<QueuedOperation[]>([]);
  const [pendingCount, setPendingCount] = useState(0);
  const [isSyncing, setIsSyncing] = useState(false);
  const [syncErrors, setSyncErrors] = useState<Map<string, string>>(new Map());
  
  const syncHandlers = useRef<Map<string, (entityId: string, payload?: unknown) => Promise<void>>>(new Map());
  const syncTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const loadPendingOperations = useCallback(async () => {
    try {
      const count = await syncQueue.getCount();
      setPendingCount(count);
      
      const items = await syncQueue.getByPriority(50);
      const operations: QueuedOperation[] = items.map((item) => ({
        id: item.id.toString(),
        entityType: item.entity_type,
        entityId: item.entity_id,
        operation: item.operation,
        priority: item.priority,
        attempts: item.attempts,
        lastError: item.last_error,
        createdAt: item.created_at,
      }));
      setPendingOperations(operations);
    } catch (error) {
      console.error('Failed to load pending operations:', error);
    }
  }, []);

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
        const id = await syncQueue.add({
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

        return id;
      } catch (error) {
        console.error('Failed to queue operation:', error);
        throw error;
      }
    },
    [isOnline, loadPendingOperations]
  );

  const processOperation = useCallback(
    async (op: QueuedOperation): Promise<boolean> => {
      const handler = syncHandlers.current.get(op.operation);
      
      if (!handler) {
        console.warn(`No handler registered for operation: ${op.operation}`);
        return false;
      }

      try {
        const payload = operationPayloads.get(`${op.entityType}:${op.entityId}:${op.operation}`);
        await handler(op.entityId, payload);
        
        await syncQueue.remove(op.id);
        operationPayloads.delete(`${op.entityType}:${op.entityId}:${op.operation}`);
        
        setSyncErrors((prev) => {
          const next = new Map(prev);
          next.delete(op.id);
          return next;
        });
        
        onSyncSuccess?.(op);
        return true;
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : 'Unknown error';
        
        setSyncErrors((prev) => {
          const next = new Map(prev);
          next.set(op.id, errorMessage);
          return next;
        });

        if (op.attempts < maxRetries) {
          await syncQueue.incrementAttempts(op.id, errorMessage);
        } else {
          onSyncError?.(op, error instanceof Error ? error : new Error(errorMessage));
        }
        
        return false;
      }
    },
    [maxRetries, onSyncSuccess, onSyncError]
  );

  const processQueue = useCallback(async () => {
    if (!isOnline || isSyncing) return;

    setIsSyncing(true);

    try {
      const items = await syncQueue.getByPriority(50);
      
      for (const item of items) {
        const op: QueuedOperation = {
          id: item.id.toString(),
          entityType: item.entity_type,
          entityId: item.entity_id,
          operation: item.operation,
          priority: item.priority,
          attempts: item.attempts,
          lastError: item.last_error,
          createdAt: item.created_at,
        };
        
        await processOperation(op);
      }

      await loadPendingOperations();
    } catch (error) {
      console.error('Failed to process sync queue:', error);
    } finally {
      setIsSyncing(false);
    }
  }, [isOnline, isSyncing, processOperation, loadPendingOperations]);

  const retryOperation = useCallback(
    async (operationId: string) => {
      const operation = pendingOperations.find((op) => op.id === operationId);
      if (!operation) return;

      await syncQueue.incrementAttempts(operationId, undefined);
      await processQueue();
    },
    [pendingOperations, processQueue]
  );

  const removeOperation = useCallback(async (operationId: string) => {
    await syncQueue.remove(operationId);
    await loadPendingOperations();
    
    setSyncErrors((prev) => {
      const next = new Map(prev);
      next.delete(operationId);
      return next;
    });
  }, [loadPendingOperations]);

  const clearQueue = useCallback(async () => {
    await syncQueue.clearAll();
    operationPayloads.clear();
    setPendingOperations([]);
    setPendingCount(0);
    setSyncErrors(new Map());
  }, []);

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
  };
}

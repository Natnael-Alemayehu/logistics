import {
  ResolutionStrategy,
  SyncConflict,
  ResolvedData,
  ResolvedBatch,
  FieldConflict,
  ConflictLogEntry,
} from '@/types/conflict';
import { getStrategyForField, getDefaultStrategy } from './conflictStrategies';
import { ConflictStrategyConfig } from '@/types/conflict';

type EntityType = keyof ConflictStrategyConfig;

class ConflictResolver {
  private conflictLog: ConflictLogEntry[] = [];
  private logEnabled: boolean = true;

  enableLogging(enabled: boolean): void {
    this.logEnabled = enabled;
  }

  private log(
    conflictId: string,
    action: ConflictLogEntry['action'],
    details: string,
    strategy?: ResolutionStrategy
  ): void {
    if (!this.logEnabled) return;

    const entry: ConflictLogEntry = {
      id: `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
      conflict_id: conflictId,
      action,
      strategy,
      details,
      timestamp: new Date().toISOString(),
    };
    this.conflictLog.push(entry);
  }

  getConflictLog(): ConflictLogEntry[] {
    return [...this.conflictLog];
  }

  clearConflictLog(): void {
    this.conflictLog = [];
  }

  detectConflict(
    localData: Record<string, any>,
    serverData: Record<string, any>,
    entityType: EntityType,
    entityId: string
  ): SyncConflict | null {
    const conflicts: FieldConflict[] = [];
    const allKeys = new Set([
      ...Object.keys(localData),
      ...Object.keys(serverData),
    ]);

    const ignoredFields = ['id', 'created_at', 'synced_at'];

    for (const key of allKeys) {
      if (ignoredFields.includes(key)) continue;

      const localValue = localData[key];
      const serverValue = serverData[key];

      if (this.hasConflict(localValue, serverValue)) {
        conflicts.push({
          field_name: key,
          local_value: localValue,
          server_value: serverValue,
        });
      }
    }

    if (conflicts.length === 0) {
      return null;
    }

    const conflictId = `${entityType}-${entityId}-${Date.now()}`;

    this.log(conflictId, 'detected', `Found ${conflicts.length} conflicts for ${entityType}:${entityId}`);

    return {
      id: conflictId,
      entity_type: entityType,
      entity_id: entityId,
      local_value: localData,
      server_value: serverData,
      local_updated_at: localData.updated_at,
      server_updated_at: serverData.updated_at,
      field_conflicts: conflicts,
      detected_at: new Date().toISOString(),
    };
  }

  private hasConflict(localValue: any, serverValue: any): boolean {
    if (localValue === undefined && serverValue === undefined) return false;
    if (localValue === null && serverValue === null) return false;
    if (localValue === undefined || localValue === null) {
      return serverValue !== undefined && serverValue !== null;
    }
    if (serverValue === undefined || serverValue === null) {
      return localValue !== undefined && localValue !== null;
    }

    if (typeof localValue !== typeof serverValue) return true;

    if (Array.isArray(localValue) && Array.isArray(serverValue)) {
      return JSON.stringify([...localValue].sort()) !== JSON.stringify([...serverValue].sort());
    }

    if (typeof localValue === 'object' && typeof serverValue === 'object') {
      return JSON.stringify(localValue) !== JSON.stringify(serverValue);
    }

    return localValue !== serverValue;
  }

  resolveConflict(conflict: SyncConflict, overrideStrategy?: ResolutionStrategy): ResolvedData {
    const resolvedValue: Record<string, any> = {};
    const fieldResolutions: FieldConflict[] = [];

    const baseData = { ...conflict.server_value };
    Object.assign(baseData, conflict.local_value);

    const mergedKeys = new Set([
      ...Object.keys(conflict.local_value),
      ...Object.keys(conflict.server_value),
    ]);

    let strategyUsed = overrideStrategy || this.determineOverallStrategy(conflict);

    if (strategyUsed === ResolutionStrategy.MANUAL) {
      this.log(conflict.id, 'manual_required', `Manual resolution required for ${conflict.entity_type}:${conflict.entity_id}`);
      throw new Error(`Manual resolution required for conflict ${conflict.id}`);
    }

    for (const key of mergedKeys) {
      const localValue = conflict.local_value[key];
      const serverValue = conflict.server_value[key];

      if (!this.hasConflict(localValue, serverValue)) {
        resolvedValue[key] = localValue !== undefined ? localValue : serverValue;
        continue;
      }

      const fieldStrategy = overrideStrategy
        ? strategyUsed
        : getStrategyForField(conflict.entity_type as EntityType, key);

      const resolved = this.resolveField(localValue, serverValue, fieldStrategy);

      resolvedValue[key] = resolved.value;
      fieldResolutions.push({
        field_name: key,
        local_value: localValue,
        server_value: serverValue,
        resolved_value: resolved.value,
        strategy_used: resolved.strategy,
      });
    }

    this.log(conflict.id, 'resolved', `Resolved conflict for ${conflict.entity_type}:${conflict.entity_id}`, strategyUsed);

    return {
      conflict_id: conflict.id,
      entity_type: conflict.entity_type,
      entity_id: conflict.entity_id,
      resolved_value: resolvedValue,
      strategy_used: strategyUsed,
      resolved_at: new Date().toISOString(),
      field_resolutions: fieldResolutions,
    };
  }

  private determineOverallStrategy(conflict: SyncConflict): ResolutionStrategy {
    if (!conflict.field_conflicts || conflict.field_conflicts.length === 0) {
      return getDefaultStrategy(conflict.entity_type as EntityType);
    }

    const strategies = conflict.field_conflicts.map((fc) =>
      getStrategyForField(conflict.entity_type as EntityType, fc.field_name)
    );

    const uniqueStrategies = [...new Set(strategies)];

    if (uniqueStrategies.includes(ResolutionStrategy.MANUAL)) {
      return ResolutionStrategy.MANUAL;
    }

    return ResolutionStrategy.FIELD_LEVEL_MERGE;
  }

  private resolveField(
    localValue: any,
    serverValue: any,
    strategy: ResolutionStrategy
  ): { value: any; strategy: ResolutionStrategy } {
    switch (strategy) {
      case ResolutionStrategy.SERVER_WINS:
        return { value: serverValue, strategy };

      case ResolutionStrategy.CLIENT_WINS:
        return { value: localValue, strategy };

      case ResolutionStrategy.MERGE:
        return this.mergeValues(localValue, serverValue);

      case ResolutionStrategy.MERGE_CONCAT:
        return this.concatValues(localValue, serverValue);

      case ResolutionStrategy.FIELD_LEVEL_MERGE:
        return { value: serverValue, strategy: ResolutionStrategy.SERVER_WINS };

      case ResolutionStrategy.MANUAL:
        return { value: null, strategy };

      default:
        return { value: serverValue, strategy: ResolutionStrategy.SERVER_WINS };
    }
  }

  private mergeValues(localValue: any, serverValue: any): { value: any; strategy: ResolutionStrategy } {
    if (typeof localValue === 'object' && typeof serverValue === 'object') {
      if (Array.isArray(localValue) && Array.isArray(serverValue)) {
        const merged = [...new Set([...serverValue, ...localValue])];
        return { value: merged, strategy: ResolutionStrategy.MERGE };
      }

      if (!Array.isArray(localValue) && !Array.isArray(serverValue)) {
        const merged = { ...serverValue, ...localValue };
        return { value: merged, strategy: ResolutionStrategy.MERGE };
      }
    }

    return { value: serverValue, strategy: ResolutionStrategy.SERVER_WINS };
  }

  private concatValues(localValue: any, serverValue: any): { value: any; strategy: ResolutionStrategy } {
    if (typeof localValue === 'string' && typeof serverValue === 'string') {
      if (localValue === serverValue) {
        return { value: localValue, strategy: ResolutionStrategy.MERGE_CONCAT };
      }

      const separator = serverValue.endsWith('.') || localValue.startsWith('.') ? ' ' : '. ';
      const merged = `${serverValue}${separator}${localValue}`;
      return { value: merged, strategy: ResolutionStrategy.MERGE_CONCAT };
    }

    if (Array.isArray(localValue) && Array.isArray(serverValue)) {
      const merged = [...serverValue, ...localValue];
      return { value: merged, strategy: ResolutionStrategy.MERGE_CONCAT };
    }

    return { value: localValue || serverValue, strategy: ResolutionStrategy.CLIENT_WINS };
  }

  resolveBatch(conflicts: SyncConflict[]): ResolvedBatch {
    const resolutions: ResolvedData[] = [];
    const manualConflicts: SyncConflict[] = [];

    for (const conflict of conflicts) {
      try {
        const resolved = this.resolveConflict(conflict);
        resolutions.push(resolved);
      } catch (error) {
        if (error instanceof Error && error.message.includes('Manual resolution required')) {
          manualConflicts.push(conflict);
        } else {
          console.error(`Failed to resolve conflict ${conflict.id}:`, error);
          manualConflicts.push(conflict);
        }
      }
    }

    return {
      total_conflicts: conflicts.length,
      resolved_count: resolutions.length,
      manual_required_count: manualConflicts.length,
      resolutions,
      manual_conflicts: manualConflicts,
    };
  }

  resolveWithManualChoice(
    conflict: SyncConflict,
    fieldChoices: Record<string, 'local' | 'server' | 'merge'>
  ): ResolvedData {
    const resolvedValue: Record<string, any> = {};
    const fieldResolutions: FieldConflict[] = [];

    const mergedKeys = new Set([
      ...Object.keys(conflict.local_value),
      ...Object.keys(conflict.server_value),
    ]);

    for (const key of mergedKeys) {
      const localValue = conflict.local_value[key];
      const serverValue = conflict.server_value[key];
      const choice = fieldChoices[key];

      if (!this.hasConflict(localValue, serverValue)) {
        resolvedValue[key] = localValue !== undefined ? localValue : serverValue;
        continue;
      }

      let resolvedFieldValue: any;
      let strategy: ResolutionStrategy;

      switch (choice) {
        case 'local':
          resolvedFieldValue = localValue;
          strategy = ResolutionStrategy.CLIENT_WINS;
          break;
        case 'server':
          resolvedFieldValue = serverValue;
          strategy = ResolutionStrategy.SERVER_WINS;
          break;
        case 'merge':
          const merged = this.mergeValues(localValue, serverValue);
          resolvedFieldValue = merged.value;
          strategy = merged.strategy;
          break;
        default:
          resolvedFieldValue = serverValue;
          strategy = ResolutionStrategy.SERVER_WINS;
      }

      resolvedValue[key] = resolvedFieldValue;
      fieldResolutions.push({
        field_name: key,
        local_value: localValue,
        server_value: serverValue,
        resolved_value: resolvedFieldValue,
        strategy_used: strategy,
      });
    }

    this.log(conflict.id, 'resolved', `Manually resolved conflict for ${conflict.entity_type}:${conflict.entity_id}`, ResolutionStrategy.MANUAL);

    return {
      conflict_id: conflict.id,
      entity_type: conflict.entity_type,
      entity_id: conflict.entity_id,
      resolved_value: resolvedValue,
      strategy_used: ResolutionStrategy.MANUAL,
      resolved_at: new Date().toISOString(),
      field_resolutions: fieldResolutions,
    };
  }

  getConflictingFields(conflict: SyncConflict): string[] {
    if (!conflict.field_conflicts) {
      return this.detectConflict(
        conflict.local_value,
        conflict.server_value,
        conflict.entity_type as EntityType,
        conflict.entity_id
      )?.field_conflicts?.map((fc: FieldConflict) => fc.field_name) || [];
    }

    return conflict.field_conflicts.map((fc) => fc.field_name);
  }

  formatConflictForDisplay(conflict: SyncConflict): {
    localDisplay: Record<string, { value: any; changed: boolean }>;
    serverDisplay: Record<string, { value: any; changed: boolean }>;
  } {
    const allKeys = new Set([
      ...Object.keys(conflict.local_value),
      ...Object.keys(conflict.server_value),
    ]);

    const conflictingFields = new Set(this.getConflictingFields(conflict));

    const localDisplay: Record<string, { value: any; changed: boolean }> = {};
    const serverDisplay: Record<string, { value: any; changed: boolean }> = {};

    for (const key of allKeys) {
      const localValue = conflict.local_value[key];
      const serverValue = conflict.server_value[key];
      const hasConflict = conflictingFields.has(key);

      localDisplay[key] = {
        value: localValue,
        changed: hasConflict,
      };

      serverDisplay[key] = {
        value: serverValue,
        changed: hasConflict,
      };
    }

    return { localDisplay, serverDisplay };
  }
}

export const conflictResolver = new ConflictResolver();
export default ConflictResolver;

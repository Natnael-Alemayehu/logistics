export enum ResolutionStrategy {
  SERVER_WINS = 'server_wins',
  CLIENT_WINS = 'client_wins',
  MERGE = 'merge',
  MERGE_CONCAT = 'merge_concat',
  FIELD_LEVEL_MERGE = 'field_level_merge',
  MANUAL = 'manual',
}

export interface FieldConflict {
  field_name: string
  local_value: any
  server_value: any
  resolved_value?: any
  strategy_used?: ResolutionStrategy
}

export interface SyncConflict {
  id: string
  entity_type: 'shipment' | 'tracking_event' | 'proof_of_delivery'
  entity_id: string
  local_value: Record<string, any>
  server_value: Record<string, any>
  local_updated_at?: string
  server_updated_at?: string
  field_conflicts?: FieldConflict[]
  detected_at: string
}

export interface ResolvedData {
  conflict_id: string
  entity_type: string
  entity_id: string
  resolved_value: Record<string, any>
  strategy_used: ResolutionStrategy
  resolved_at: string
  field_resolutions?: FieldConflict[]
}

export interface ResolvedBatch {
  total_conflicts: number
  resolved_count: number
  manual_required_count: number
  resolutions: ResolvedData[]
  manual_conflicts: SyncConflict[]
}

export interface EntityStrategyConfig {
  [fieldName: string]: ResolutionStrategy
  default: ResolutionStrategy
}

export interface ConflictStrategyConfig {
  shipment: EntityStrategyConfig
  tracking_event: EntityStrategyConfig
  proof_of_delivery: EntityStrategyConfig
}

export interface ConflictLogEntry {
  id: string
  conflict_id: string
  action: 'detected' | 'resolved' | 'manual_required'
  strategy?: ResolutionStrategy
  details: string
  timestamp: string
}

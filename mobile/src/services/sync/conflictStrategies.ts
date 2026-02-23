import { ResolutionStrategy, ConflictStrategyConfig } from '@/types/conflict';

export const conflictStrategies: ConflictStrategyConfig = {
  shipment: {
    status: ResolutionStrategy.SERVER_WINS,
    status_note: ResolutionStrategy.SERVER_WINS,
    status_reason: ResolutionStrategy.SERVER_WINS,
    notes: ResolutionStrategy.MERGE_CONCAT,
    special_instructions: ResolutionStrategy.MERGE_CONCAT,
    estimated_delivery: ResolutionStrategy.SERVER_WINS,
    actual_delivery: ResolutionStrategy.SERVER_WINS,
    driver_id: ResolutionStrategy.SERVER_WINS,
    vehicle_id: ResolutionStrategy.SERVER_WINS,
    customer_name: ResolutionStrategy.CLIENT_WINS,
    customer_phone: ResolutionStrategy.CLIENT_WINS,
    destination_address: ResolutionStrategy.SERVER_WINS,
    destination_lat: ResolutionStrategy.SERVER_WINS,
    destination_lng: ResolutionStrategy.SERVER_WINS,
    default: ResolutionStrategy.SERVER_WINS,
  },
  tracking_event: {
    latitude: ResolutionStrategy.CLIENT_WINS,
    longitude: ResolutionStrategy.CLIENT_WINS,
    accuracy: ResolutionStrategy.CLIENT_WINS,
    speed: ResolutionStrategy.CLIENT_WINS,
    heading: ResolutionStrategy.CLIENT_WINS,
    event_type: ResolutionStrategy.CLIENT_WINS,
    status: ResolutionStrategy.SERVER_WINS,
    note: ResolutionStrategy.MERGE_CONCAT,
    recorded_at: ResolutionStrategy.CLIENT_WINS,
    battery_level: ResolutionStrategy.CLIENT_WINS,
    default: ResolutionStrategy.CLIENT_WINS,
  },
  proof_of_delivery: {
    recipient_name: ResolutionStrategy.CLIENT_WINS,
    recipient_phone: ResolutionStrategy.CLIENT_WINS,
    signature_data: ResolutionStrategy.CLIENT_WINS,
    photo_urls: ResolutionStrategy.CLIENT_WINS,
    photo_paths: ResolutionStrategy.CLIENT_WINS,
    delivery_address: ResolutionStrategy.CLIENT_WINS,
    delivery_lat: ResolutionStrategy.CLIENT_WINS,
    delivery_lng: ResolutionStrategy.CLIENT_WINS,
    delivery_notes: ResolutionStrategy.CLIENT_WINS,
    location_verified: ResolutionStrategy.CLIENT_WINS,
    location_mismatch_meters: ResolutionStrategy.CLIENT_WINS,
    recorded_at: ResolutionStrategy.CLIENT_WINS,
    default: ResolutionStrategy.CLIENT_WINS,
  },
};

export const getStrategyForField = (
  entityType: keyof ConflictStrategyConfig,
  fieldName: string
): ResolutionStrategy => {
  const entityConfig = conflictStrategies[entityType];
  if (!entityConfig) {
    return ResolutionStrategy.SERVER_WINS;
  }
  return entityConfig[fieldName] || entityConfig.default;
};

export const getDefaultStrategy = (
  entityType: keyof ConflictStrategyConfig
): ResolutionStrategy => {
  const entityConfig = conflictStrategies[entityType];
  if (!entityConfig) {
    return ResolutionStrategy.SERVER_WINS;
  }
  return entityConfig.default;
};

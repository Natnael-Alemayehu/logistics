/**
 * Wire contract for POST /api/v1/sync.
 *
 * Mirrors internal/model/sync.go on the backend. Keep the two in step: the
 * client may only delete a local record once the server has confirmed it, so a
 * drift here reintroduces silent data loss.
 */

/** Outcome the server reports for a single submitted item. */
export type SyncItemStatus = 'accepted' | 'duplicate' | 'rejected';

/** Reason codes accompanying a rejected item. */
export const SYNC_REJECT_CODES = {
  invalidShipmentId: 'invalid_shipment_id',
  notAssigned: 'not_assigned',
  stale: 'stale',
  invalidPayload: 'invalid_payload',
  storageFailed: 'storage_failed',
  internal: 'internal_error',
} as const;

export interface SyncItemResult {
  /** Echoes the client_id sent with the item. */
  client_id?: string;
  /** Position in the submitted collection; the fallback when client_id is absent. */
  index: number;
  status: SyncItemStatus;
  code?: string;
  message?: string;
}

export interface SyncResponse {
  server_time?: string;
  sync_time?: string;
  /** Count of accepted items. Superseded by the per-item arrays below. */
  events_received?: number;

  events?: SyncItemResult[];
  pods?: SyncItemResult[];
  statuses?: SyncItemResult[];

  deleted_shipment_ids?: string[];
  pull?: {
    shipments?: unknown[];
    deleted_shipment_ids?: string[];
  };
  shipments?: unknown[];
}

/**
 * True when the server confirmed it holds the item.
 *
 * `duplicate` counts as success: it means an earlier attempt landed and only the
 * response was lost, which is the normal outcome when an offline device replays
 * a batch. Treating it as a failure would strand delivered PODs forever.
 */
export function isPersisted(result: SyncItemResult): boolean {
  return result.status === 'accepted' || result.status === 'duplicate';
}

/**
 * Indexes results by the client_id that was sent, falling back to position for
 * servers that do not echo it.
 */
export function indexResults(
  results: SyncItemResult[] | undefined,
  localIds: string[]
): Map<string, SyncItemResult> {
  const byLocalId = new Map<string, SyncItemResult>();
  if (!results) {
    return byLocalId;
  }

  for (const result of results) {
    const localId = result.client_id ?? localIds[result.index];
    if (localId !== undefined) {
      byLocalId.set(localId, result);
    }
  }

  return byLocalId;
}

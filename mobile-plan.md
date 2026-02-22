# Mobile Application Implementation Plan

## Executive Summary

This plan outlines the development of the **Driver Mobile App** - a critical component of the Ethiopian Logistics Tracking Platform. The app is designed for Ethiopian truck drivers who operate in areas with unreliable network connectivity, requiring robust offline-first architecture.

---

## 1. Technology Stack Decision

### Recommended: **React Native (Expo)**

| Factor | React Native | Flutter |
|--------|--------------|---------|
| Team Familiarity | Matches existing React/Next.js patterns | New learning curve |
| Code Reuse | ~40% shared logic with frontend | Minimal |
| Ecosystem | Mature, Expo simplifies development | Growing, good performance |
| Offline Storage | SQLite, AsyncStorage | sqflite, Hive |
| Hot Reload | Yes | Yes |
| Native Modules | Good ecosystem | Good ecosystem |
| Web Code Sharing | Shared utilities, types, hooks possible | No |

**Rationale**: Since the frontend uses React, Zustand, React Query, and TypeScript, React Native allows significant code reuse (types, API client patterns, state management concepts, business logic). Expo provides managed workflow for faster development.

---

## 2. Project Structure

```
mobile/
├── app.json                    # Expo config
├── package.json
├── tsconfig.json
├── babel.config.js
├── eas.json                    # EAS Build config
├── src/
│   ├── app/                    # Navigation & screens (Expo Router)
│   │   ├── _layout.tsx
│   │   ├── index.tsx           # Splash/auth redirect
│   │   ├── (auth)/
│   │   │   ├── login.tsx
│   │   │   └── forgot-pin.tsx
│   │   ├── (main)/
│   │   │   ├── _layout.tsx     # Tab navigator
│   │   │   ├── index.tsx       # Shipments list (home)
│   │   │   ├── shipment/[id].tsx
│   │   │   ├── map.tsx         # Full map view
│   │   │   ├── profile.tsx
│   │   │   └── settings.tsx
│   │   └── pod/
│   │       └── [shipmentId].tsx
│   ├── components/
│   │   ├── ui/                 # Base components (Button, Card, Input...)
│   │   ├── shipments/          # Shipment-related
│   │   ├── tracking/           # Map, location components
│   │   ├── pod/                # Signature pad, photo capture
│   │   └── shared/             # Reusable utilities
│   ├── hooks/
│   │   ├── useAuth.ts
│   │   ├── useShipments.ts
│   │   ├── useSync.ts
│   │   ├── useLocation.ts
│   │   ├── useConnectivity.ts
│   │   └── useNotifications.ts
│   ├── services/
│   │   ├── api/
│   │   │   ├── client.ts       # HTTP client
│   │   │   ├── auth.ts
│   │   │   ├── shipments.ts
│   │   │   └── sync.ts
│   │   ├── websocket.ts
│   │   ├── storage.ts          # SQLite/AsyncStorage wrapper
│   │   ├── location.ts         # GPS tracking service
│   │   └── backgroundSync.ts
│   ├── store/
│   │   ├── authStore.ts
│   │   ├── shipmentsStore.ts
│   │   ├── syncStore.ts
│   │   └── settingsStore.ts
│   ├── db/
│   │   ├── schema.ts           # SQLite schema
│   │   ├── migrations.ts
│   │   └── repositories/       # Data access layer
│   │       ├── shipments.ts
│   │       ├── trackingEvents.ts
│   │       └── pods.ts
│   ├── types/
│   │   ├── shipment.ts
│   │   ├── user.ts
│   │   ├── tracking.ts
│   │   └── api.ts
│   ├── utils/
│   │   ├── constants.ts
│   │   ├── validators.ts
│   │   ├── formatters.ts
│   │   └── compression.ts
│   └── i18n/
│       ├── config.ts
│       ├── en.json
│       └── am.json
├── assets/
│   ├── images/
│   └── fonts/
└── android/                    # Native config (if needed)
```

---

## 3. Core Features Implementation

### 3.1 Authentication (FR-001 to FR-005)

**Driver Login Flow:**
```
Phone Number (Ethiopian format) + 4-digit PIN
    ↓
POST /api/v1/auth/login/driver
    ↓
Store tokens (access + refresh) in SecureStore
    ↓
Fetch assigned shipments
    ↓
Navigate to main app
```

**Implementation:**
- Phone validation: `/^(\+251|0)[1-9]\d{8}$/`
- PIN: 4-6 digit numeric input
- SecureStore for tokens (encrypted)
- Biometric unlock option (FaceID/Fingerprint)
- Auto-refresh token mechanism
- Session management (single device per driver)

### 3.2 Offline Data Storage (FR-014)

**SQLite Schema:**

```sql
-- Shipments (synced from server)
CREATE TABLE shipments (
  id TEXT PRIMARY KEY,
  tracking_number TEXT UNIQUE NOT NULL,
  origin_address TEXT NOT NULL,
  origin_lat REAL,
  origin_lng REAL,
  destination_address TEXT NOT NULL,
  destination_lat REAL,
  destination_lng REAL,
  customer_name TEXT NOT NULL,
  customer_phone TEXT NOT NULL,
  cargo_description TEXT,
  status TEXT NOT NULL,
  driver_id TEXT,
  vehicle_id TEXT,
  estimated_delivery TEXT,
  special_instructions TEXT,
  synced_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

-- Tracking events (generated locally)
CREATE TABLE tracking_events (
  id TEXT PRIMARY KEY,
  shipment_id TEXT NOT NULL,
  driver_id TEXT NOT NULL,
  latitude REAL NOT NULL,
  longitude REAL NOT NULL,
  accuracy REAL,
  speed REAL,
  heading REAL,
  event_type TEXT NOT NULL, -- 'gps_ping', 'checkpoint', 'status_change'
  status TEXT,
  note TEXT,
  recorded_at TEXT NOT NULL,
  synced_at TEXT,
  device_id TEXT,
  battery_level INTEGER,
  sync_priority INTEGER DEFAULT 0
);

-- Proof of delivery (generated locally)
CREATE TABLE proof_of_delivery (
  id TEXT PRIMARY KEY,
  shipment_id TEXT UNIQUE NOT NULL,
  driver_id TEXT NOT NULL,
  recipient_name TEXT NOT NULL,
  recipient_phone TEXT,
  signature_data TEXT, -- Base64 or file path
  photo_paths TEXT, -- JSON array of local paths
  delivery_address TEXT,
  delivery_lat REAL,
  delivery_lng REAL,
  delivery_notes TEXT,
  location_verified INTEGER DEFAULT 0,
  location_mismatch_meters INTEGER,
  recorded_at TEXT NOT NULL,
  synced_at TEXT,
  sync_status TEXT DEFAULT 'pending' -- 'pending', 'syncing', 'synced', 'failed'
);

-- Sync queue
CREATE TABLE sync_queue (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  entity_type TEXT NOT NULL, -- 'tracking_event', 'pod', 'status'
  entity_id TEXT NOT NULL,
  operation TEXT NOT NULL, -- 'create', 'update'
  priority INTEGER DEFAULT 0, -- Higher = sync first
  attempts INTEGER DEFAULT 0,
  last_error TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY (entity_id) REFERENCES tracking_events(id) OR proof_of_delivery(id)
);

-- Sync metadata
CREATE TABLE sync_metadata (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  driver_id TEXT NOT NULL,
  last_sync_at TEXT,
  last_event_id TEXT,
  pending_count INTEGER DEFAULT 0,
  storage_used_kb INTEGER DEFAULT 0,
  device_id TEXT
);

-- Cached map tiles (optional, for offline maps)
CREATE TABLE cached_tiles (
  id TEXT PRIMARY KEY, -- z/x/y format
  tile_data BLOB NOT NULL,
  cached_at TEXT NOT NULL,
  last_accessed TEXT NOT NULL
);
```

**Data Flow:**
1. On login: Pull assigned shipments via `/api/v1/my-shipments`
2. Store in SQLite with sync timestamp
3. All local operations write to SQLite + sync_queue
4. Background sync processes queue when online

### 3.3 Connectivity & Sync (FR-015, FR-039, FR-040)

**Connectivity Detection:**
```typescript
// using @react-native-community/netinfo
NetInfo.addEventListener(state => {
  const isConnected = state.isConnected;
  const isInternetReachable = state.isInternetReachable;
  const connectionType = state.type; // wifi, cellular, none
  
  // Trigger sync when connectivity restored
  if (isConnected && wasOffline) {
    syncService.sync();
  }
});
```

**Sync Protocol:**

```typescript
interface SyncRequest {
  device_id: string;
  last_sync_at: string | null;
  events: TrackingEvent[];
  pods: ProofOfDelivery[];
  statuses: StatusUpdate[];
  battery_level: number;
  storage_remaining_kb: number;
}

// Priority order:
// 1. PODs (delivery confirmations)
// 2. Status changes
// 3. Tracking events (batched, compressed)
```

**Sync Strategy:**
- Triggered on: app launch, connectivity gained, manual pull-to-refresh, after POD
- Background sync: every 5 minutes when app is foregrounded
- Batch size: max 50 events per request
- Compression: gzip for large payloads
- Retry: exponential backoff, max 3 retries per item
- Conflict resolution: last-writer-wins for tracking, driver-wins for status

### 3.4 GPS Tracking (FR-019)

**Adaptive Tracking Algorithm:**

```typescript
const getTrackingConfig = (battery: number, network: boolean) => {
  if (network && battery > 50) {
    return { interval: 5 * 60 * 1000, accuracy: 'high' }; // 5 min
  } else if (battery > 20) {
    return { interval: 15 * 60 * 1000, accuracy: 'balanced' }; // 15 min
  } else {
    return { interval: null, accuracy: 'low', checkpointOnly: true };
  }
};
```

**Implementation:**
- Use `expo-location` for GPS
- Foreground service with notification (required for Android)
- Background task using `expo-task-manager`
- Store each ping with: coordinates, accuracy, speed, heading, battery, timestamp
- Movement detection to pause tracking when stationary

**Location Permissions:**
- Foreground: Required
- Background: Required for continuous tracking
- Android: `ACCESS_FINE_LOCATION`, `ACCESS_BACKGROUND_LOCATION`
- iOS: `location` background mode

### 3.5 Shipment Management (FR-016 to FR-018)

**Shipment List Screen:**
- Grouped by status: In Transit, Pending, Completed
- Sort by: estimated delivery, priority flag
- Pull-to-refresh triggers sync
- Each item shows: tracking number, destination, customer, status badge, sync indicator
- Offline indicator when data may be stale

**Shipment Detail Screen:**
- Full shipment info
- Map showing destination (cached offline)
- One-tap call customer button
- Navigation button (opens Google Maps/Apple Maps)
- Status update buttons (context-aware)
- Action buttons: Start Trip, Mark Arrived, Mark Delayed, Complete Delivery

**Status Update Flow:**
```
Tap "Mark Arrived"
    ↓
Capture GPS location automatically
    ↓
Record status change in SQLite + sync_queue
    ↓
Attempt sync if online
    ↓
Show confirmation toast
```

**Delay Reasons (FR-018.3):**
- Road conditions
- Weather
- Security checkpoint
- Mechanical issue
- Traffic
- Other (free text)

### 3.6 Proof of Delivery (FR-020 to FR-023)

**POD Flow:**
```
"Complete Delivery" button
    ↓
POD Screen
    ├── Recipient Name (required)
    ├── Recipient Phone (optional)
    ├── Signature Pad (required)
    ├── Photo Capture (optional, up to 3)
    ├── Location Verification (automatic)
    ├── Delivery Notes (optional)
    └── Submit Button
    ↓
Validate location (warn if > 500m from destination)
    ↓
Save to SQLite + sync_queue (high priority)
    ↓
Attempt immediate sync
    ↓
Navigate to success screen
```

**Signature Pad:**
- Use `react-native-skia` or `@shopify/react-native-skia`
- Responsive drawing canvas
- Clear button for retry
- Export as base64 PNG or SVG

**Photo Capture:**
- Direct camera access via `expo-camera`
- Geotag automatically with current location
- Compress: max 800x600, 70% JPEG quality, target < 500KB
- Store locally in app's document directory
- Upload thumbnails first, full photos later

**Location Verification (FR-022):**
- Compare delivery GPS with destination coordinates
- If distance > 500m, show warning dialog:
  - "Your location doesn't match the destination. Proceed anyway?"
  - Options: "Proceed" (require reason) or "Re-attempt"
- Log mismatch meters in POD record

### 3.7 Offline Maps (FR-025)

**Implementation Options:**

1. **Vector Tiles (Recommended):** Use `@maplibre/maplibre-react-native` with downloaded style
2. **Raster Tiles:** Cache OpenStreetMap tiles locally

**Offline Map Strategy:**
- Download map region when WiFi available
- Cache regions for active shipment routes
- Limit storage to 100MB with LRU eviction
- Show cached destination markers offline
- Display current GPS location (doesn't require network)

### 3.8 Notifications (FR-033)

**Local Notifications:**
- New shipment assigned (synced from server)
- Reminder to complete POD
- Low battery warning (tracking mode change)

**Push Notifications (FCM/APNs):**
- New shipment assignment
- Shipment update/cancellation
- Urgent message from dispatcher

**Implementation:**
- `expo-notifications` for both local and push
- Handle notification tap to navigate to relevant screen

---

## 4. Screen-by-Screen Specification

### 4.1 Splash Screen
- App logo animation
- Check authentication status
- Redirect to Login or Main

### 4.2 Login Screen
- Phone number input (Ethiopian format validation)
- PIN input (4-digit, secure)
- "Forgot PIN?" link
- Language toggle (EN/AM)
- Loading state during authentication

### 4.3 Shipments List (Home)
- Tab bar: Active, Completed, All
- List of shipment cards
- Status badge (color-coded)
- Sync status indicator (green check = synced, yellow clock = pending)
- Pull-to-refresh
- FAB for quick actions (if applicable)
- Offline banner when no connectivity

### 4.4 Shipment Detail
- Header: Tracking number, status badge
- Customer info card (name, phone - tappable)
- Origin/destination addresses
- Cargo details (if any)
- Special instructions (highlighted)
- Map preview (destination pin)
- Status history timeline
- Action buttons:
  - "Start Trip" (if assigned)
  - "Mark In Transit"
  - "Mark Delayed" (requires reason)
  - "Mark Arrived"
  - "Complete Delivery" (leads to POD)

### 4.5 POD Screen
- Header: "Proof of Delivery"
- Recipient name input (required, autocomplete from history)
- Recipient phone input (optional)
- Signature pad canvas
  - Clear button
  - "Sign here" placeholder
- Photo section
  - Camera button to add photos
  - Thumbnail gallery of captured photos
  - Delete photo option
- Delivery notes input
  - Quick-select chips for common notes
  - Free text field
- Location verification alert (if mismatch)
- Submit button (disabled until signature captured)
- Success screen after submission

### 4.6 Map Screen
- Full-screen map
- Current location marker
- Active shipment destinations (pins)
- Tap pin to see shipment preview
- Navigate button (opens external maps)

### 4.7 Profile Screen
- Driver name and phone
- Vehicle assignment
- Statistics (deliveries today, this week, this month)
- Sync status: last sync time, pending items count
- Storage usage
- Logout button

### 4.8 Settings Screen
- Language selection (English, Amharic)
- Notification preferences
- Tracking settings (enable/disable background tracking)
- Data usage: WiFi-only sync option
- Clear cache (with confirmation)
- About: app version, support contact
- Debug info (build number, device ID)

---

## 5. State Management Architecture

### Stores (Zustand)

```typescript
// authStore.ts
interface AuthState {
  user: Driver | null;
  accessToken: string | null;
  refreshToken: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  // Actions
  login: (phone: string, pin: string) => Promise<void>;
  logout: () => Promise<void>;
  refreshTokens: () => Promise<void>;
}

// shipmentsStore.ts
interface ShipmentsState {
  shipments: Shipment[];
  activeShipmentId: string | null;
  isLoading: boolean;
  lastFetched: string | null;
  // Actions
  fetchShipments: () => Promise<void>;
  updateShipmentStatus: (id: string, status: string, note?: string) => Promise<void>;
  getShipmentById: (id: string) => Shipment | undefined;
}

// syncStore.ts
interface SyncState {
  isSyncing: boolean;
  lastSyncAt: string | null;
  pendingCount: number;
  syncProgress: number; // 0-100
  lastError: string | null;
  // Actions
  sync: () => Promise<void>;
  getPendingCount: () => number;
}

// settingsStore.ts
interface SettingsState {
  language: 'en' | 'am';
  trackingEnabled: boolean;
  wifiOnlySync: boolean;
  // Actions
  setLanguage: (lang: string) => void;
  toggleTracking: () => void;
}
```

### React Query (for server state)

```typescript
// Shipments query
const useShipments = () => {
  return useQuery({
    queryKey: ['shipments'],
    queryFn: () => shipmentsApi.getMyShipments(),
    staleTime: 5 * 60 * 1000, // 5 minutes
    gcTime: 24 * 60 * 60 * 1000, // 24 hours
    networkMode: 'offlineFirst',
  });
};
```

---

## 6. Background Services

### 6.1 Location Tracking Service

```typescript
// services/location.ts
import * as Location from 'expo-location';
import * as TaskManager from 'expo-task-manager';

const LOCATION_TASK = 'background-location-tracking';

TaskManager.defineTask(LOCATION_TASK, async ({ data, error }) => {
  if (error) return;
  
  const { locations } = data as { locations: Location.LocationObject[] };
  const location = locations[0];
  
  // Store in SQLite
  await db.trackingEvents.insert({
    latitude: location.coords.latitude,
    longitude: location.coords.longitude,
    accuracy: location.coords.accuracy,
    speed: location.coords.speed,
    recorded_at: new Date().toISOString(),
    event_type: 'gps_ping',
  });
  
  // Queue for sync
  await syncQueue.add('tracking_event', trackingEvent.id);
});

export const startTracking = async () => {
  await Location.startLocationUpdatesAsync(LOCATION_TASK, {
    accuracy: Location.Accuracy.Balanced,
    timeInterval: 5 * 60 * 1000, // 5 minutes
    distanceInterval: 100, // 100 meters
    foregroundService: {
      notificationTitle: 'Logistics Tracking Active',
      notificationBody: 'Your location is being tracked for active shipments',
    },
  });
};
```

### 6.2 Background Sync Service

```typescript
// services/backgroundSync.ts
import * as BackgroundFetch from 'expo-background-fetch';
import * as TaskManager from 'expo-task-manager';

const SYNC_TASK = 'background-sync';

TaskManager.defineTask(SYNC_TASK, async () => {
  const netInfo = await NetInfo.fetch();
  if (!netInfo.isConnected) {
    return BackgroundFetch.Result.NoData;
  }
  
  const result = await syncService.sync();
  return result.success 
    ? BackgroundFetch.Result.NewData 
    : BackgroundFetch.Result.Failed;
});

export const registerBackgroundSync = async () => {
  await BackgroundFetch.registerTaskAsync(SYNC_TASK, {
    minimumInterval: 15 * 60, // 15 minutes
    stopOnTerminate: false,
    startOnBoot: true,
  });
};
```

---

## 7. Dependencies

### Core Dependencies

```json
{
  "dependencies": {
    "expo": "~51.0.0",
    "expo-router": "~3.5.0",
    "expo-status-bar": "~1.12.0",
    "expo-secure-store": "~12.8.0",
    "expo-location": "~17.0.0",
    "expo-camera": "~15.0.0",
    "expo-notifications": "~0.28.0",
    "expo-task-manager": "~11.8.0",
    "expo-background-fetch": "~12.0.0",
    "expo-image-manipulator": "~12.0.0",
    "expo-file-system": "~17.0.0",
    
    "react-native": "0.74.5",
    "react": "18.2.0",
    
    "@react-native-community/netinfo": "^11.3.0",
    "@shopify/react-native-skia": "^1.2.0",
    "@maplibre/maplibre-react-native": "^9.1.0",
    
    "zustand": "^4.5.0",
    "@tanstack/react-query": "^5.0.0",
    
    "i18next": "^23.0.0",
    "react-i18next": "^14.0.0",
    
    "react-hook-form": "^7.50.0",
    "zod": "^3.22.0",
    
    "expo-sqlite": "~14.0.0"
  },
  "devDependencies": {
    "@types/react": "~18.2.0",
    "typescript": "~5.3.0"
  }
}
```

---

## 8. Implementation Phases

### Phase 1: Foundation (Week 1-2)

**Goal:** Core app structure, authentication, basic shipment viewing

| Task | Description | Est. Hours |
|------|-------------|------------|
| Project setup | Expo init, TypeScript, ESLint, folder structure | 4 |
| Navigation setup | Expo Router, auth flow, tab navigator | 8 |
| Auth store | Zustand auth state, SecureStore integration | 6 |
| Login screen | Phone/PIN input, validation, API integration | 8 |
| API client | HTTP client with auth interceptors | 6 |
| SQLite setup | Database initialization, schema, basic repository | 8 |
| Shipment types | TypeScript interfaces matching backend | 2 |
| Shipments list | Fetch and display assigned shipments | 8 |
| Shipment detail | View shipment details | 6 |
| **Total** | | **56** |

### Phase 2: Offline Core (Week 3-4)

**Goal:** Offline data storage, sync mechanism, connectivity handling

| Task | Description | Est. Hours |
|------|-------------|------------|
| SQLite repositories | Full CRUD for all entities | 12 |
| Sync queue | Queue management, priority handling | 8 |
| Sync service | API integration, batch processing | 12 |
| Connectivity detection | NetInfo integration, state management | 4 |
| Offline indicators | UI components for sync status | 4 |
| Data persistence | Shipments caching on login | 6 |
| Conflict resolution | Handle sync conflicts | 6 |
| **Total** | | **52** |

### Phase 3: GPS Tracking (Week 5-6)

**Goal:** Location tracking, background services, battery optimization

| Task | Description | Est. Hours |
|------|-------------|------------|
| Location permissions | Request and handle permissions | 4 |
| Foreground tracking | GPS updates when app open | 8 |
| Background tracking | TaskManager, foreground service | 12 |
| Adaptive tracking | Battery-aware interval adjustment | 8 |
| Tracking events storage | Store and queue GPS data | 6 |
| Tracking UI | Map view, current location | 8 |
| **Total** | | **46** |

### Phase 4: Proof of Delivery (Week 7-8)

**Goal:** Signature capture, photo evidence, POD submission

| Task | Description | Est. Hours |
|------|-------------|------------|
| POD screen UI | Form layout, inputs | 8 |
| Signature pad | Drawing canvas, export | 12 |
| Camera integration | Photo capture, gallery | 8 |
| Image compression | Resize, compress before storage | 4 |
| Location verification | Distance check, warning dialog | 4 |
| POD storage | SQLite storage, sync queue | 6 |
| POD sync | Priority upload, retry logic | 6 |
| Success screen | Confirmation, next steps | 2 |
| **Total** | | **50** |

### Phase 5: Polish & Testing (Week 9-10)

**Goal:** Internationalization, notifications, testing, bug fixes

| Task | Description | Est. Hours |
|------|-------------|------------|
| i18n setup | i18next, translation files | 6 |
| Amharic translations | Complete translation | 8 |
| Push notifications | FCM setup, notification handling | 8 |
| Local notifications | Reminders, alerts | 4 |
| Settings screen | Language, preferences | 4 |
| Profile screen | Driver info, stats | 4 |
| Error handling | Global error boundaries, messages | 6 |
| Unit tests | Core logic tests | 8 |
| Integration tests | E2E critical flows | 8 |
| Bug fixes | Issues from testing | 8 |
| Performance optimization | Startup time, memory | 4 |
| **Total** | | **68** |

### Phase 6: Offline Maps & Final Polish (Week 11-12)

**Goal:** Offline map support, final testing, app store preparation

| Task | Description | Est. Hours |
|------|-------------|------------|
| MapLibre setup | Map component, markers | 8 |
| Offline map download | Region caching | 12 |
| Map caching logic | Storage limits, eviction | 6 |
| Final UI polish | Animations, transitions | 8 |
| App icons & splash | Assets preparation | 4 |
| EAS build setup | Build configuration | 4 |
| Internal testing | Full regression | 8 |
| App store submission | Play Store preparation | 4 |
| Documentation | README, build instructions | 4 |
| **Total** | | **58** |

---

## 9. Testing Strategy

### Unit Tests
- **State stores**: Test actions, state changes
- **Utilities**: Formatters, validators, compression
- **Repository layer**: SQLite operations

### Integration Tests
- **Sync flow**: Offline → Online sync
- **Authentication**: Login, token refresh, logout
- **POD flow**: Capture → Store → Sync

### E2E Tests (Detox or Maestro)
- Login with valid credentials
- View shipments list
- Update shipment status
- Complete delivery with POD
- Offline mode handling

### Manual Testing Checklist
- [ ] Login with correct/incorrect credentials
- [ ] Pull-to-refresh shipments list
- [ ] View shipment details offline
- [ ] Update status offline (verify syncs when online)
- [ ] Capture signature
- [ ] Take POD photo
- [ ] Complete delivery (verify POD syncs)
- [ ] Background tracking (minimize app, verify GPS continues)
- [ ] Low battery mode (verify tracking adjusts)
- [ ] Airplane mode toggle (verify sync triggers)
- [ ] Language switch (verify all text updates)
- [ ] App kill and restart (verify data persists)

---

## 10. Security Considerations

### Data Security
- **SQLite encryption**: Use `expo-sqlite` with SQLCipher if available, or encrypt sensitive fields
- **Token storage**: `expo-secure-store` (uses Android Keystore / iOS Keychain)
- **API communication**: HTTPS only, certificate pinning for production

### Authentication
- JWT tokens with short expiration (15 min access, 7 day refresh)
- Biometric unlock option after initial PIN login
- Session invalidation on password/PIN change

### Privacy
- Location data stored locally, synced only when user initiates delivery
- Customer phone numbers: display masked option for privacy
- Data retention: Local data cleared on logout

---

## 11. Performance Targets

| Metric | Target |
|--------|--------|
| App cold start | < 3 seconds |
| Shipment list load | < 1 second (from cache) |
| Sync time (50 events) | < 5 seconds on 3G |
| GPS battery drain | < 15% per 8-hour shift |
| Storage usage (offline) | < 200MB typical |
| Photo compression | < 500KB per photo |
| APK size | < 30MB |

---

## 12. Success Metrics

| Metric | Target |
|--------|--------|
| Driver adoption | > 80% actively using after training |
| POD completion rate | > 95% of deliveries have digital POD |
| Sync success rate | > 99% within 1 hour of connectivity |
| App crash rate | < 0.5% of sessions |
| Play Store rating | > 4.0 stars |

---

## 13. Risk Mitigation

| Risk | Mitigation |
|------|------------|
| GPS inaccuracy in rural areas | Manual checkpoint override, accuracy display |
| Battery drain complaints | Clear settings for tracking, battery-aware mode |
| Sync failures on poor network | Chunked uploads, resume capability, retry logic |
| Photo upload failures | Thumbnail-first strategy, WiFi preference option |
| Driver resistance to technology | Simple UI, training program, quick wins |
| Device compatibility issues | Support Android 8+ (API 26+), test on low-end devices |

---

## 14. Open Questions for Discussion

1. **Device provisioning**: Will companies provide devices or should the app work on driver's personal phones?
2. **Minimum Android version**: Support Android 8+ (covers 90%+ of devices in Ethiopia) or newer?
3. **Offline maps**: Priority level - essential for MVP or post-launch?
4. **USSD fallback**: Should we build USSD support for feature phone users in Phase 1?
5. **Biometric authentication**: Include in MVP or post-launch enhancement?
6. **Multi-language audio**: Voice instructions in Amharic for accessibility?

---

## 15. Next Steps

1. **Immediate**: Review and approve this plan
2. **Week 1**: Project setup, begin Phase 1 development
3. **Stakeholder alignment**: Confirm answers to open questions
4. **Design review**: UI/UX mockups for key screens
5. **Backend coordination**: Ensure API endpoints are mobile-ready

---

## 16. Appendix A: API Endpoints Reference

### Driver-Specific Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/auth/login/driver` | Driver login (phone + PIN) |
| POST | `/api/v1/auth/logout` | Logout and revoke session |
| POST | `/api/v1/auth/refresh` | Refresh access token |
| POST | `/api/v1/auth/change-password` | Change password/PIN |
| GET | `/api/v1/my-shipments` | List driver's active shipments |
| POST | `/api/v1/sync` | Sync offline data |
| GET | `/api/v1/ws` | WebSocket connection |

### Sync Request Format

```json
{
  "device_id": "string",
  "last_sync_at": "2024-01-15T08:00:00Z",
  "events": [
    {
      "shipment_id": "uuid",
      "latitude": 9.0320,
      "longitude": 38.7635,
      "accuracy": 15.5,
      "speed": 45.2,
      "heading": 180,
      "event_type": "gps_ping",
      "recorded_at": "2024-01-15T10:30:00Z",
      "battery_level": 75
    }
  ],
  "pods": [
    {
      "shipment_id": "uuid",
      "recipient_name": "Abebe Bikila",
      "recipient_phone": "+251911234567",
      "signature_data": "base64...",
      "photo_urls": ["url1", "url2"],
      "delivery_lat": 9.0320,
      "delivery_lng": 38.7635,
      "delivery_notes": "Left with security guard",
      "location_verified": true,
      "recorded_at": "2024-01-15T14:30:00Z"
    }
  ],
  "statuses": [
    {
      "shipment_id": "uuid",
      "status": "arrived",
      "note": "Arrived at destination",
      "recorded_at": "2024-01-15T14:00:00Z"
    }
  ],
  "battery_level": 65,
  "storage_remaining_kb": 512000
}
```

### Sync Response Format

```json
{
  "server_time": "2024-01-15T14:30:05Z",
  "events_received": 47,
  "conflicts": [],
  "pull": {
    "shipments": [
      {
        "id": "uuid",
        "tracking_number": "ET-20240115-0001",
        "destination_address": "Bole, Addis Ababa",
        "customer_name": "Tigist Haile",
        "customer_phone": "+251922345678",
        "status": "assigned",
        "estimated_delivery": "2024-01-15T16:00:00Z"
      }
    ],
    "messages": ["Dispatcher: Please call office when arrived"],
    "updates": ["Shipment ET-20240114-0023 has been cancelled"]
  }
}
```

---

## 17. Appendix B: Status State Machine

```
                    ┌─────────────┐
                    │   Pending   │
                    └──────┬──────┘
                           │ Assign
                           ▼
                    ┌─────────────┐
            ┌──────│   Assigned  │──────┐
            │      └──────┬──────┘      │
            │             │ Start      │ Cancel
            │             ▼             │
            │      ┌─────────────┐      │
            │      │  In Transit │──────┤
            │      └──────┬──────┘      │
            │             │             │
            │    ┌────────┼────────┐    │
            │    │        │        │    │
            │    ▼        ▼        │    │
            │ ┌───────┐ ┌───────┐ │    │
            │ │Delayed│ │ Issue │─┤    │
            │ └───┬───┘ └───────┘ │    │
            │     │               │    │
            │     │ Resume        ▼    ▼
            │     └────────► ┌─────────────┐
            │                 │   Arrived   │
            │                 └──────┬──────┘
            │                        │ Deliver
            │                        ▼
            │                 ┌─────────────┐
            └────────────────►│  Delivered  │
                              └─────────────┘

                              ┌─────────────┐
                              │  Cancelled  │
                              └─────────────┘
```

---

## 18. Appendix C: Ethiopian Phone Number Validation

```typescript
// Valid formats:
// +251911234567 (international)
// +251711234567 (international)
// 0911234567 (local)
// 0711234567 (local)

const ETHIOPIAN_PHONE_REGEX = /^(\+251|0)[1-9]\d{8}$/;

const validateEthiopianPhone = (phone: string): boolean => {
  const cleaned = phone.replace(/\s|-/g, '');
  return ETHIOPIAN_PHONE_REGEX.test(cleaned);
};

const formatEthiopianPhone = (phone: string): string => {
  const cleaned = phone.replace(/\s|-/g, '');
  if (cleaned.startsWith('0')) {
    return '+251' + cleaned.slice(1);
  }
  return cleaned.startsWith('+') ? cleaned : '+251' + cleaned;
};
```

---

## 19. Appendix D: Error Handling Codes

| Code | Description | User Message |
|------|-------------|--------------|
| AUTH_001 | Invalid credentials | "Invalid phone number or PIN" |
| AUTH_002 | Account locked | "Account locked. Try again in 15 minutes" |
| AUTH_003 | Session expired | "Session expired. Please login again" |
| SYNC_001 | Sync failed | "Sync failed. Will retry automatically" |
| SYNC_002 | Conflict detected | "Data conflict. Please review" |
| GPS_001 | Permission denied | "Location permission required for tracking" |
| GPS_002 | GPS unavailable | "GPS signal weak. Move to open area" |
| CAM_001 | Camera permission denied | "Camera permission required for POD photos" |
| NET_001 | No connection | "No internet connection. Data will sync later" |

---

*Document Version: 1.0*
*Last Updated: February 2026*
*Author: Engineering Team*

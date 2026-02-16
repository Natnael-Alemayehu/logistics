# Logistics SAAS Platform - Ethiopia

## Overview

A logistics tracking and proof-of-delivery platform designed specifically for the Ethiopian market, addressing the unique challenges of unstable network connectivity in rural areas while providing trucking companies visibility into their fleet operations.

---

## Problem Statement

### Current Pain Points

1. **Lack of Visibility**: Trucking companies have no real-time knowledge of where their shipments are
2. **No Proof of Delivery**: Disputes arise between drivers, companies, and customers about whether deliveries were completed
3. **Communication Gaps**: Drivers cannot easily update status when traveling through areas with no connectivity
4. **Manual Processes**: Phone calls and paper-based tracking lead to errors and delays
5. **Customer Anxiety**: End customers have no visibility into their shipment status

### Market Context - Ethiopia

- **Network Reliability**: Stable in Addis Ababa and major cities (Dire Dawa, Mekelle, Bahir Dar, Hawassa). Unreliable or non-existent in rural areas and along major transit corridors
- **Coverage**: Ethio Telecom has ~90% GSM coverage but data coverage is significantly lower
- **Power**: Inconsistent electricity in rural areas affects device charging
- **Smartphone Penetration**: Growing but many drivers use basic feature phones
- **Major Industries Served**: Agriculture (coffee, teff, flowers), Manufacturing, Construction materials, Import/Export through Djibouti corridor

---

## Core Product Features

### 1. Shipment Tracking

- Real-time GPS tracking when connectivity is available
- Checkpoint-based tracking for offline scenarios
- Automatic status updates based on location
- Route deviation alerts
- ETA calculations

### 2. Proof of Delivery (POD)

- Digital signature capture
- Photo evidence with geolocation and timestamp
- Recipient verification via phone number
- Optional: Fingerprint verification for repeat customers
- Offline POD storage with automatic sync

### 3. Driver Communication

- Simple status updates (En Route, Delayed, Arrived, Delivered)
- Pre-defined reason codes for delays (road conditions, weather, checkpoint, mechanical)
- Voice note support for hands-free updates
- SMS fallback for critical communications

### 4. Dispatcher Dashboard

- Fleet overview map
- Individual shipment tracking
- Exception alerts and notifications
- Delivery confirmation workflow
- Analytics and reporting

### 5. Customer Portal

- Track shipment by tracking number or phone number
- SMS notifications for status changes
- Delivery confirmation

---

## Technical Architecture

### Design Principles

1. **Offline-First**: The application must function fully without network connectivity
2. **Sync-When-Available**: Data synchronizes automatically when connection is detected
3. **Minimal Data Usage**: Compressed payloads, delta syncs, no unnecessary transfers
4. **Graceful Degradation**: Feature phones supported via SMS/USSD
5. **Battery Conscious**: Efficient GPS usage, background optimization

### System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        CLOUD INFRASTRUCTURE                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │   API Layer   │  │  Sync Server │  │   SMS Gateway        │  │
│  │   (REST/WS)   │  │  (Conflict   │  │   (Ethio Telecom     │  │
│  │               │  │   Resolution)│  │    Integration)      │  │
│  └──────┬───────┘  └──────┬───────┘  └──────────┬───────────┘  │
│         │                 │                      │              │
│  ┌──────▼─────────────────▼──────────────────────▼───────────┐  │
│  │                     PostgreSQL Database                    │  │
│  │            (Shipments, Tracking Events, PODs)             │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                    ┌─────────┴─────────┐
                    │                   │
              ┌─────▼─────┐      ┌──────▼──────┐
              │  Driver   │      │ Dispatcher  │
              │ Mobile App│      │  Web App    │
              │ (Offline) │      │  (Online)   │
              └───────────┘      └─────────────┘
```

### Mobile App (Driver) - Offline-First Architecture

#### Local Data Storage

```
SQLite Database Schema:
├── shipments (assigned to driver, downloaded at start of day)
├── tracking_events (GPS coordinates, timestamps, stored locally)
├── delivery_proofs (signatures, photos, stored locally)
├── sync_queue (pending operations, ordered by priority)
└── sync_metadata (last sync timestamp, conflict resolution data)
```

#### Sync Strategy

```
Sync Protocol:
1. On app start: Pull assigned shipments for next 24-48 hours
2. On connectivity detected:
   a. Upload pending tracking events (batched)
   b. Upload pending PODs (priority queue)
   c. Pull any new/updated shipment assignments
   d. Pull any dispatcher messages/updates
3. Conflict Resolution:
   - Last-writer-wins for tracking events (based on server timestamp)
   - Manual review queue for conflicting delivery statuses
```

#### GPS Tracking Strategy

```
Adaptive Tracking Algorithm:
┌─────────────────────────────────────────────────┐
│ IF network_available AND battery > 30%:         │
│   - High frequency GPS (every 5 minutes)        │
│   - Real-time sync to server                    │
│ ELIF battery > 20%:                             │
│   - Medium frequency GPS (every 15 minutes)     │
│   - Store locally, sync when available          │
│ ELSE:                                           │
│   - Checkpoint-based only (at stops)            │
│   - Manual status updates                       │
└─────────────────────────────────────────────────┘
```

### SMS Fallback System

For drivers without smartphones or in areas with no data:

```
SMS Commands:
- SEND <shipment_id> <status> → Update shipment status
- POD <shipment_id> <recipient_phone> → Confirm delivery
- LOC <shipment_id> → Request location (dispatcher query)

Status Codes:
1 = En Route
2 = Delayed (reason via follow-up)
3 = Arrived at destination
4 = Delivered
5 = Issue/Problem
```

### Web Dashboard (Dispatcher)

- Real-time map view with last-known locations
- Shipment list with status indicators
- Sync status per driver (green = synced, yellow = pending sync, red = offline > 4 hours)
- Exception queue for manual review
- Reporting and analytics

---

## Data Models

### Shipment

```json
{
  "id": "string",
  "tracking_number": "string",
  "origin": {
    "address": "string",
    "coordinates": { "lat": "number", "lng": "number" }
  },
  "destination": {
    "address": "string",
    "coordinates": { "lat": "number", "lng": "number" }
  },
  "customer": {
    "name": "string",
    "phone": "string"
  },
  "driver_id": "string",
  "vehicle_id": "string",
  "status": "pending | in_transit | delayed | arrived | delivered | issue",
  "estimated_delivery": "timestamp",
  "actual_delivery": "timestamp | null",
  "created_at": "timestamp",
  "updated_at": "timestamp"
}
```

### Tracking Event

```json
{
  "id": "string",
  "shipment_id": "string",
  "driver_id": "string",
  "coordinates": { "lat": "number", "lng": "number" },
  "accuracy": "number",
  "speed": "number | null",
  "heading": "number | null",
  "event_type": "gps_ping | checkpoint | status_change",
  "status": "string | null",
  "note": "string | null",
  "recorded_at": "timestamp",
  "synced_at": "timestamp | null",
  "device_id": "string"
}
```

### Proof of Delivery

```json
{
  "id": "string",
  "shipment_id": "string",
  "driver_id": "string",
  "recipient_name": "string",
  "recipient_phone": "string",
  "recipient_signature": "base64 | null",
  "photo_evidence": "base64 | null",
  "photo_coordinates": { "lat": "number", "lng": "number" },
  "delivery_location": {
    "address": "string",
    "coordinates": { "lat": "number", "lng": "number" }
  },
  "delivery_notes": "string | null",
  "recorded_at": "timestamp",
  "synced_at": "timestamp | null"
}
```

---

## Engineering Challenges & Solutions

### Challenge 1: Offline GPS Accuracy

**Problem**: GPS can be inaccurate indoors or in remote areas. Without real-time correction, location data may be wrong.

**Solutions**:
- Store GPS accuracy metadata with each point
- Implement confidence scoring based on accuracy radius
- Allow manual location override at checkpoints
- Use geofencing with generous radius for key locations (warehouses, checkpoints)

### Challenge 2: Large Sync Queues

**Problem**: Driver in remote area for extended period accumulates large amount of data. First sync after reconnection could take long.

**Solutions**:
- Compress payloads using gzip/brotli
- Implement delta encoding for location data (only store changes)
- Priority-based sync (PODs first, then tracking events, then metadata)
- Background sync with retry logic
- Chunked uploads with resume capability

### Challenge 3: Photo Storage and Sync

**Problem**: POD photos are large (2-5MB each). Slow upload on poor connections.

**Solutions**:
- Resize/compress photos locally before storage (max 800x600, 70% JPEG quality)
- Store thumbnail (for quick viewing) and full photo separately
- Sync thumbnails first, full photos when WiFi available
- Option to defer photo sync to end of day

### Challenge 4: Conflict Resolution

**Problem**: Dispatcher updates shipment while driver's app has pending updates. Conflicts arise.

**Solutions**:
- Operational Transform for status changes
- Server-authoritative for assignment changes
- Manual review queue for critical conflicts (e.g., both mark as delivered vs issue)
- Audit log for all changes with attribution

### Challenge 5: Battery Drain

**Problem**: Constant GPS tracking kills phone battery, especially on older Android devices common in Ethiopia.

**Solutions**:
- Adaptive tracking frequency based on:
  - Movement detection (accelerometer to detect if vehicle moving)
  - Battery level
  - Network availability
- Use Android's FusedLocationProvider for efficiency
- Allow driver to manually trigger location ping
- Checkpoint-based mode for extended offline periods

### Challenge 6: Data Costs

**Problem**: Mobile data is expensive for drivers. Company may not reimburse.

**Solutions**:
- Minimal data payloads (target < 10MB/day active use)
- WiFi sync encouragement at company depots
- SMS bundle for critical updates (cheaper than data)
- Company-pays model: data costs billed to company, not driver

### Challenge 7: Feature Phone Support

**Problem**: Many drivers use feature phones, not smartphones.

**Solutions**:
- USSD menu for basic status updates
- SMS-based status reporting
- IVR (Interactive Voice Response) for complex interactions
- Low-end Android app for budget smartphones (under $50 devices)

### Challenge 8: Timestamp Drift

**Problem**: Driver device clock may be inaccurate. Timestamps for offline events need to be reliable.

**Solutions**:
- Store device timestamp AND server-estimated timestamp
- Use NTP sync when online
- Apply clock drift correction during sync
- For legal POD purposes, rely on server timestamp

---

## Technology Stack Recommendations

### Mobile App (Driver)

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Framework | React Native or Flutter | Cross-platform, large ecosystem |
| Local DB | SQLite (react-native-quick-sqlite / sqflite) | Reliable offline storage |
| State Management | Redux / Riverpod | Predictable state for offline/online transitions |
| GPS | React Native Geolocation / geolocator | Fused location provider |
| Background Tasks | react-native-background-fetch / workmanager | Sync when app backgrounded |
| Images | react-native-image-resizer / image_picker | Compress before storage |

### Backend

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Runtime | Node.js or Go | High concurrency for real-time features |
| API | REST + WebSockets | REST for CRUD, WS for real-time updates |
| Database | PostgreSQL + PostGIS | Spatial queries for location data |
| Cache | Redis | Session management, pub/sub for real-time |
| Queue | Redis or RabbitMQ | Async job processing for sync, notifications |
| File Storage | S3-compatible (Wasabi, MinIO) | Cost-effective for photos |

### SMS Gateway

| Option | Provider | Notes |
|--------|----------|-------|
| Primary | Ethio Telecom Enterprise API | Direct integration, most reliable |
| Backup | Twilio (via international gateway) | Higher cost, backup option |
| Alternative | Local aggregators (Santo, others) | May have better enterprise terms |

### Hosting

| Option | Recommendation |
|--------|----------------|
| Cloud | AWS (eu-south-1, Stockholm) or Azure (South Africa North) - closest regions with good latency to Ethiopia |
| Alternative | Local hosting partner for data residency requirements |

---

## Security Considerations

### Data Security

- End-to-end encryption for sensitive data (recipient details, signatures)
- TLS 1.3 for all API communications
- Encrypted local storage on device (SQLCipher)
- Secure key storage using platform keychain/keystore

### Authentication

- JWT-based authentication with refresh tokens
- Device binding (one active session per driver)
- SMS OTP for low-security scenarios
- Optional biometric for app unlock

### Privacy

- Location data retention policy (delete after 90 days unless needed for audit)
- Customer phone numbers masked for drivers (last 4 digits only)
- GDPR-inspired data rights (export, delete requests)

---

## Implementation Phases

### Phase 1: MVP (Months 1-3)

**Goal**: Core tracking with offline support for single trucking company pilot

**Features**:
- Driver Android app with offline GPS tracking
- Basic sync functionality
- Simple dispatcher web dashboard
- Proof of delivery with signature only
- SMS status update fallback

**Technical**:
- React Native app
- Node.js backend
- PostgreSQL database
- Manual SMS sending (no gateway yet)

### Phase 2: Enhanced Offline (Months 4-6)

**Goal**: Robust offline support, photo POD, conflict resolution

**Features**:
- Photo evidence capture
- Advanced conflict resolution
- Chunked sync with resume
- Adaptive GPS tracking
- Battery optimization

**Technical**:
- Redis queue for sync management
- S3-compatible storage for photos
- Enhanced sync protocol

### Phase 3: Multi-Tenant SAAS (Months 7-9)

**Goal**: Launch as commercial product

**Features**:
- Company onboarding flow
- Role-based access control
- Customer tracking portal
- Analytics dashboard
- Billing integration

**Technical**:
- Multi-tenant database architecture
- Customer-facing web app
- Usage-based billing system

### Phase 4: Enterprise Features (Months 10-12)

**Goal**: Enterprise sales, integrations

**Features**:
- SMS gateway integration (Ethio Telecom)
- USSD support for feature phones
- API for ERP integration
- White-label option
- Advanced analytics

**Technical**:
- SMS gateway integration
- Public API with documentation
- Custom domain support

---

## Success Metrics

### Technical KPIs

- Offline capability: App functions fully after 24 hours offline
- Sync success rate: > 99% of events sync within 1 hour of connectivity
- Battery impact: < 15% battery drain per 8-hour shift with active tracking
- Data usage: < 10MB per driver per day average

### Business KPIs

- Driver adoption: > 80% of drivers actively using app after training
- POD completion: > 95% of deliveries have digital POD
- Customer visibility: > 90% of shipments trackable within 1 hour
- Dispatch efficiency: 50% reduction in phone calls to drivers

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Ethio Telecom API unavailability | Build SMS parsing fallback; have aggregator backup |
| Driver resistance to new technology | Simple UI; training program; gamification |
| Data costs discourage adoption | Company pays model; optimize data usage |
| Power outages affect driver devices | Encourage power banks; low-power mode |
| GPS unavailable in certain areas | Manual checkpoint override; network-based location |
| Regulatory changes | Modular architecture; local hosting option |

---

## Open Questions

1. **Pricing Model**: Per-shipment, per-driver, or per-company subscription?
2. **Device Provisioning**: Company provides devices or driver uses personal phone?
3. **Integration Priority**: Which ERP/accounting systems do target customers use?
4. **Insurance**: Does digital POD satisfy insurance requirements for claims?
5. **Cross-Border**: Support for shipments to/from Djibouti, Kenya, Sudan?

---

## Appendix A: Major Transit Corridors in Ethiopia

| Corridor | Route | Network Status | Notes |
|----------|-------|----------------|-------|
| Djibouti Corridor | Addis → Djibouti | Variable | Most important for imports; significant gaps |
| North Corridor | Addis → Mekelle → Aksum | Variable | Conflict-affected areas; unstable |
| South Corridor | Addis → Hawassa → Moyale | Poor | Main export route to Kenya |
| West Corridor | Addis → Bahir Dar → Gondora | Moderate | Agricultural heartland |
| East Corridor | Addis → Dire Dawa → Jijiga | Variable | Pastoral regions, minimal coverage |

## Appendix B: Sample Sync Protocol

```
Driver App → Server (on connectivity detected):

POST /api/v1/sync/batch
{
  "device_id": "device_123",
  "last_sync": "2024-01-15T08:00:00Z",
  "events": [
    {
      "type": "tracking",
      "data": { /* tracking event */ }
    },
    {
      "type": "pod",
      "priority": "high",
      "data": { /* POD data */ }
    }
  ],
  "metadata": {
    "device_time": "2024-01-15T14:30:00Z",
    "battery_level": 45,
    "storage_remaining_mb": 512
  }
}

Server Response:

{
  "status": "accepted",
  "events_received": 47,
  "conflicts": [],
  "server_time": "2024-01-15T14:30:05Z",
  "pull": {
    "shipments": [ /* new/updated shipments */ ],
    "messages": [ /* dispatcher messages */ ],
    "updates": [ /* status updates from dispatcher */ ]
  }
}
```

---

*Document Version: 1.0*
*Last Updated: February 2026*
*Author: Engineering Team, Addis Ababa*
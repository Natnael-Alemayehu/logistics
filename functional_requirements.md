# Functional Requirements Document (FRD)
## Ethiopian Logistics Tracking Platform

**Version**: 1.0  
**Date**: February 2026  
**Status**: Draft for Team Review

---

## 1. Introduction

### 1.1 Purpose

This document defines the functional requirements for the Ethiopian Logistics Tracking Platform—a SAAS product enabling trucking companies to track shipments and capture proof of delivery, with offline-first capabilities designed for Ethiopia's challenging network infrastructure.

### 1.2 Scope

The platform consists of three main components:
- **Driver Mobile App** (Android, offline-first)
- **Dispatcher Web Dashboard** (browser-based)
- **Customer Tracking Portal** (browser-based + SMS)

### 1.3 Definitions

| Term | Definition |
|------|------------|
| POD | Proof of Delivery - evidence that a shipment was delivered to the intended recipient |
| Sync | The process of reconciling local device data with server data |
| Offline-first | Architecture that allows full functionality without network connectivity |
| Checkpoint | A designated location where shipment status should be updated |
| Driver | The operator of a vehicle transporting shipments |
| Dispatcher | A company employee managing fleet operations |
| Tenant | A trucking company subscribed to the platform |

### 1.4 User Roles

| Role | Description | Access Level |
|------|-------------|--------------|
| Driver | Vehicle operator, uses mobile app | Limited to assigned shipments |
| Dispatcher | Fleet coordinator, uses web dashboard | Company-wide shipment access |
| Fleet Manager | Senior dispatcher with admin rights | Company-wide + user management |
| Company Admin | Company owner or designated admin | Full company access + billing |
| Customer | End recipient of shipment | Limited to own shipments |
| Platform Admin | SAAS operator staff | Cross-tenant admin access |

---

## 2. Authentication & Authorization

### FR-001: User Registration

**Priority**: P1 (MVP)

**Description**: The system shall allow company administrators to create user accounts for dispatchers and drivers within their tenant.

**Acceptance Criteria**:
- AC-001.1: Admin can create driver accounts with: full name, phone number, vehicle assignment
- AC-001.2: Admin can create dispatcher accounts with: full name, email, phone number
- AC-001.3: System generates temporary credentials sent via SMS
- AC-001.4: User must change password on first login
- AC-001.5: System validates phone number format (Ethiopian: +251 or 09/07 prefix)

---

### FR-002: User Authentication

**Priority**: P1 (MVP)

**Description**: The system shall authenticate users via phone number/email and password.

**Acceptance Criteria**:
- AC-002.1: Drivers authenticate using phone number + password or PIN (4-6 digits)
- AC-002.2: Dispatchers authenticate using email or phone + password
- AC-002.3: System supports "Remember me" for 30 days (web only)
- AC-002.4: System locks account after 5 failed attempts for 15 minutes
- AC-002.5: System logs all authentication attempts with timestamp and IP

---

### FR-003: Session Management

**Priority**: P1 (MVP)

**Description**: The system shall manage user sessions securely across devices.

**Acceptance Criteria**:
- AC-003.1: Driver app maintains session for 7 days without re-authentication
- AC-003.2: Web sessions timeout after 8 hours of inactivity
- AC-003.3: User can view active sessions and revoke others
- AC-003.4: Single active session per driver device (new login invalidates previous)
- AC-003.5: Session tokens expire and must be refreshed via refresh token

---

### FR-004: Role-Based Access Control

**Priority**: P1 (MVP)

**Description**: The system shall enforce access controls based on user roles.

**Acceptance Criteria**:
- AC-004.1: Drivers can only view shipments assigned to them
- AC-004.2: Dispatchers can view all shipments within their company
- AC-004.3: Fleet Managers can manage driver/dispatcher accounts
- AC-004.4: Company Admins can manage billing and company settings
- AC-004.5: Customers can only view their own shipments via tracking number or phone

---

### FR-005: Multi-Tenant Isolation

**Priority**: P1 (MVP)

**Description**: The system shall ensure complete data isolation between tenant companies.

**Acceptance Criteria**:
- AC-005.1: Users from one company cannot access data from another company
- AC-005.2: All API queries automatically filter by tenant_id based on authenticated user
- AC-005.3: Database queries are scoped to tenant context
- AC-005.4: Cross-tenant references are prevented at database level
- AC-005.5: Platform admins have explicit override capability with audit logging

---

## 3. Shipment Management

### FR-006: Shipment Creation

**Priority**: P1 (MVP)

**Description**: The system shall allow dispatchers to create new shipments with all required information.

**Acceptance Criteria**:
- AC-006.1: Shipment requires: origin address, destination address, customer name, customer phone
- AC-006.2: System auto-generates unique tracking number (format: ET-YYYYMMDD-XXXX)
- AC-006.3: Dispatcher can optionally add: cargo description, weight, value, special instructions
- AC-006.4: Dispatcher can assign driver and vehicle at creation or later
- AC-006.5: System calculates estimated delivery date based on route and historical data
- AC-006.6: System validates addresses against known Ethiopian locations database
- AC-006.7: Shipment status is initialized to "pending"

---

### FR-007: Shipment Assignment

**Priority**: P1 (MVP)

**Description**: The system shall allow dispatchers to assign shipments to drivers and vehicles.

**Acceptance Criteria**:
- AC-007.1: Dispatcher can assign one or multiple shipments to a driver
- AC-007.2: Assignment creates a notification to the driver (if online, queued if offline)
- AC-007.3: System checks driver availability (not over-assigned based on configurable limit)
- AC-007.4: Assignment updates shipment status to "assigned"
- AC-007.5: Dispatcher can reassign shipments between drivers
- AC-007.6: Reassignment logs previous assignment in history

---

### FR-008: Shipment Status Updates

**Priority**: P1 (MVP)

**Description**: The system shall track and display shipment status throughout the delivery lifecycle.

**Acceptance Criteria**:
- AC-008.1: Valid statuses: Pending → Assigned → In Transit → Delayed → Arrived → Delivered → Issue → Cancelled
- AC-008.2: Each status change records: timestamp, user, location (if available), notes
- AC-008.3: Status history is preserved and viewable
- AC-008.4: System allows status notes for context (e.g., delay reason)
- AC-008.5: Status can be updated by: driver (via app), dispatcher (via dashboard), system (via automation)
- AC-008.6: Status changes trigger notifications to relevant parties

---

### FR-009: Shipment Search & Filtering

**Priority**: P1 (MVP)

**Description**: The system shall provide search and filtering capabilities for shipments.

**Acceptance Criteria**:
- AC-009.1: Search by: tracking number, customer name, customer phone, driver name
- AC-009.2: Filter by: status, date range, assigned driver, origin, destination
- AC-009.3: Results are paginated (default 25 per page)
- AC-009.4: Filters can be combined (AND logic)
- AC-009.5: Search results highlight matching text
- AC-009.6: Dispatcher can save filter presets for quick access

---

### FR-010: Shipment Details View

**Priority**: P1 (MVP)

**Description**: The system shall display comprehensive shipment details.

**Acceptance Criteria**:
- AC-010.1: View includes all shipment metadata
- AC-010.2: View displays current status with timestamp
- AC-010.3: View shows assigned driver and vehicle information
- AC-010.4: View displays full status history timeline
- AC-010.5: View shows tracking events (location pings) on map
- AC-010.6: View displays proof of delivery when completed
- AC-010.7: View shows any attached documents or notes

---

### FR-011: Shipment Editing

**Priority**: P2 (Post-MVP)

**Description**: The system shall allow dispatchers to edit shipment details.

**Acceptance Criteria**:
- AC-011.1: Editable fields: destination, customer info, cargo details, instructions
- AC-011.2: Non-editable once delivered: origin, tracking number
- AC-011.3: All edits are logged with editor, timestamp, old value, new value
- AC-011.4: Driver receives notification of significant changes (destination change)
- AC-011.5: System prevents editing shipments from other tenants

---

### FR-012: Shipment Cancellation

**Priority**: P2 (Post-MVP)

**Description**: The system shall allow cancellation of shipments.

**Acceptance Criteria**:
- AC-012.1: Dispatcher can cancel shipments with status "pending" or "assigned"
- AC-012.2: Cancellation requires reason selection from predefined list
- AC-012.3: Cancelled shipments are retained in history (not deleted)
- AC-012.4: Customer receives SMS notification of cancellation
- AC-012.5: Driver assignment is released on cancellation

---

### FR-013: Bulk Shipment Operations

**Priority**: P3 (Future)

**Description**: The system shall support bulk operations for efficiency.

**Acceptance Criteria**:
- AC-013.1: Bulk import via CSV upload (validate format before processing)
- AC-013.2: Bulk assign multiple shipments to single driver
- AC-013.3: Bulk status update with single confirmation
- AC-013.4: Export shipments to CSV/Excel with applied filters
- AC-013.5: Progress indicator for long-running bulk operations

---

## 4. Driver Mobile App

### FR-014: Offline Data Storage

**Priority**: P1 (MVP) - Critical

**Description**: The driver app shall store all necessary data locally for offline operation.

**Acceptance Criteria**:
- AC-014.1: App stores assigned shipments locally on first sync
- AC-014.2: App stores tracking events locally when offline
- AC-014.3: App stores POD data (signatures, photos) locally when offline
- AC-014.4: Local storage is encrypted using SQLCipher or equivalent
- AC-014.5: App maintains sync queue of pending uploads
- AC-014.6: App displays storage usage and warns when approaching limit
- AC-014.7: Data persists across app restarts and device reboots

---

### FR-015: Connectivity Detection & Handling

**Priority**: P1 (MVP) - Critical

**Description**: The app shall detect network connectivity and adapt behavior accordingly.

**Acceptance Criteria**:
- AC-015.1: App detects network state: online (good), online (poor), offline
- AC-015.2: UI clearly indicates current connectivity status (icon + text)
- AC-015.3: When transitioning from offline to online, sync initiates automatically
- AC-015.4: User can manually trigger sync via button
- AC-015.5: Sync runs in background without blocking user interaction
- AC-015.6: App handles interrupted sync gracefully (resume on next attempt)

---

### FR-016: Shipment List View (Driver)

**Priority**: P1 (MVP)

**Description**: The driver app shall display assigned shipments.

**Acceptance Criteria**:
- AC-016.1: List shows: tracking number, destination, customer name, status, priority flag
- AC-016.2: List is sorted by: priority first, then by estimated delivery date
- AC-016.3: Driver can filter by status: all, pending pickup, in transit, completed
- AC-016.4: List displays sync status indicator per shipment (synced/pending)
- AC-016.5: Tapping shipment opens detail view
- AC-016.6: List updates in real-time when data changes (if online)
- AC-016.7: Pull-to-refresh triggers sync and list update

---

### FR-017: Shipment Detail View (Driver)

**Priority**: P1 (MVP)

**Description**: The driver app shall display shipment details and action buttons.

**Acceptance Criteria**:
- AC-017.1: View shows: full address, customer phone (tappable to call), cargo details, instructions
- AC-017.2: View displays destination on map (cached for offline)
- AC-017.3: View shows navigation button (opens Google Maps or similar)
- AC-017.4: Action buttons available: Start Trip, Mark Arrived, Mark Delayed, Complete Delivery
- AC-017.5: Buttons are context-aware (only valid actions shown based on status)
- AC-017.6: View shows customer phone with one-tap dialing
- AC-017.7: View displays any dispatcher notes or updates

---

### FR-018: Status Update (Driver)

**Priority**: P1 (MVP)

**Description**: The driver shall be able to update shipment status.

**Acceptance Criteria**:
- AC-018.1: Single tap status update buttons (minimize clicks)
- AC-018.2: Status change captures current GPS location automatically
- AC-018.3: For "Delayed" status, driver must select reason from list:
  - Road conditions
  - Weather
  - Security checkpoint
  - Mechanical issue
  - Traffic
  - Other (with text input)
- AC-018.4: Optional note field for additional context
- AC-018.5: Status updates are stored locally if offline
- AC-018.6: Confirmation feedback on status update (toast notification)
- AC-018.7: Status updates trigger sync attempt if online

---

### FR-019: GPS Tracking

**Priority**: P1 (MVP)

**Description**: The app shall track driver location for shipment visibility.

**Acceptance Criteria**:
- AC-019.1: GPS tracking runs in foreground when app is open
- AC-019.2: GPS tracking continues in background when app is minimized (with notification)
- AC-019.3: Tracking frequency adapts based on:
  - Network availability: 5 min (online), 15 min (offline)
  - Battery level: 5 min (>50%), 15 min (20-50%), checkpoint-only (<20%)
  - Movement detection: pause tracking when stationary for >10 min
- AC-019.4: Each GPS ping captures: coordinates, accuracy, speed, heading, timestamp, battery level
- AC-019.5: Location data is stored locally and synced in batches
- AC-019.6: Driver can pause tracking (indicates "tracking paused" to dispatcher)
- AC-019.7: Tracking resumes automatically when app is opened

---

### FR-020: Proof of Delivery - Signature

**Priority**: P1 (MVP)

**Description**: The driver shall capture recipient signature as proof of delivery.

**Acceptance Criteria**:
- AC-020.1: Signature capture screen appears after "Complete Delivery" action
- AC-020.2: Signature pad is responsive and accurate (minimum 100 DPI equivalent)
- AC-020.3: Signature pad includes "Clear" button to retry
- AC-020.4: Recipient name field is required (free text, auto-suggest from previous)
- AC-020.5: Recipient phone field is optional
- AC-020.6: Confirmation button saves signature and completes delivery
- AC-020.7: Signature is stored as vector data (SVG) or compressed image (PNG)
- AC-020.8: Signature capture works offline (no network required)

---

### FR-021: Proof of Delivery - Photo

**Priority**: P1 (MVP)

**Description**: The driver shall capture photo evidence of delivery.

**Acceptance Criteria**:
- AC-021.1: Photo capture option on POD screen
- AC-021.2: App accesses device camera directly (not external camera app)
- AC-021.3: Photo is automatically geotagged with current location
- AC-021.4: Photo is automatically timestamped
- AC-021.5: Photo is compressed before storage (max 800x600, 70% JPEG quality, target <500KB)
- AC-021.6: Driver can take multiple photos (up to 3 per delivery)
- AC-021.7: Photo preview allows retake before confirmation
- AC-021.8: Photo is stored locally and synced when connectivity available
- AC-021.9: Photo works offline (no network required)

---

### FR-022: Delivery Location Verification

**Priority**: P2 (Post-MVP)

**Description**: The system shall verify delivery location matches expected destination.

**Acceptance Criteria**:
- AC-022.1: System compares delivery GPS with destination coordinates
- AC-022.2: If distance > 500m, warning is shown to driver with option to:
  - Proceed anyway (with reason)
  - Re-attempt at correct location
- AC-022.3: Location mismatch is flagged in POD record
- AC-022.4: Dispatcher sees location mismatch indicator in dashboard
- AC-022.5: System logs actual delivery coordinates regardless of match

---

### FR-023: Delivery Notes

**Priority**: P1 (MVP)

**Description**: The driver shall be able to add notes to proof of delivery.

**Acceptance Criteria**:
- AC-023.1: Notes field available on POD screen
- AC-023.2: Notes field accepts free text up to 500 characters
- AC-023.3: Common notes available as quick-select buttons:
  - "Left with security guard"
  - "Left at reception"
  - "Partial delivery"
  - "Package damaged upon arrival"
- AC-023.4: Notes are stored with POD and visible in shipment history

---

### FR-024: SMS Fallback for Status Updates

**Priority**: P1 (MVP)

**Description**: The system shall accept status updates via SMS for drivers without app access.

**Acceptance Criteria**:
- AC-024.1: SMS format: `ET <tracking_number> <status_code> [note]`
- AC-024.2: Status codes: 1=In Transit, 2=Delayed, 3=Arrived, 4=Delivered, 5=Issue
- AC-024.3: System parses incoming SMS from registered driver phone numbers
- AC-024.4: System acknowledges SMS with auto-reply confirmation
- AC-024.5: Invalid format receives error reply with correct format example
- AC-024.6: SMS updates are logged with sender phone, timestamp, parsed data
- AC-024.7: SMS fallback works when app is not installed or not functional

---

### FR-025: Offline Map Support

**Priority**: P2 (Post-MVP)

**Description**: The app shall provide basic map functionality offline.

**Acceptance Criteria**:
- AC-025.1: App caches map tiles for assigned shipment routes
- AC-025.2: Offline map shows current location (GPS-based)
- AC-025.3: Offline map shows destination markers
- AC-025.4: Offline map shows saved checkpoints
- AC-025.5: Maps are downloaded when WiFi available (optional)
- AC-025.6: Map cache is limited to 100MB with LRU eviction

---

### FR-026: App Settings

**Priority**: P2 (Post-MVP)

**Description**: The app shall provide configurable settings.

**Acceptance Criteria**:
- AC-026.1: Settings include: language (Amharic, English), notification preferences, sync frequency
- AC-026.2: Settings persist across app updates
- AC-026.3: Driver can view sync status and storage usage
- AC-026.4: Driver can clear local cache (with confirmation)
- AC-026.5: Driver can view app version and contact support
- AC-026.6: Settings synced to server for cross-device consistency

---

## 5. Dispatcher Web Dashboard

### FR-027: Dashboard Overview

**Priority**: P1 (MVP)

**Description**: The dispatcher dashboard shall provide an overview of fleet operations.

**Acceptance Criteria**:
- AC-027.1: Dashboard shows: active shipments count, drivers on duty, deliveries today, issues count
- AC-027.2: Dashboard displays map with all active vehicle locations
- AC-027.3: Dashboard shows recent activity feed (last 20 events)
- AC-027.4: Dashboard displays alerts panel (exceptions requiring attention)
- AC-027.5: Dashboard updates in real-time via WebSocket when online
- AC-027.6: Dashboard loads within 3 seconds on standard connection

---

### FR-028: Live Map View

**Priority**: P1 (MVP)

**Description**: The dashboard shall display a live map of fleet locations.

**Acceptance Criteria**:
- AC-028.1: Map shows vehicle markers with driver name and status color
- AC-028.2: Markers show last-known location timestamp (sync indicator)
- AC-028.3: Clicking marker shows driver details and assigned shipments
- AC-028.4: Map supports zoom and pan
- AC-028.5: Map can be filtered by driver, status, or region
- AC-028.6: Map shows shipment routes (origin to destination lines)
- AC-028.7: Map supports satellite view toggle
- AC-028.8: Map auto-centers on user's company primary operating region

---

### FR-029: Shipment List View (Dispatcher)

**Priority**: P1 (MVP)

**Description**: The dispatcher dashboard shall display a comprehensive shipment list.

**Acceptance Criteria**:
- AC-029.1: List shows: tracking number, status, driver, origin, destination, customer, ETA
- AC-029.2: List is sortable by any column
- AC-029.3: List is filterable by status, driver, date range
- AC-029.4: List supports full-text search
- AC-029.5: Status is color-coded for quick scanning
- AC-029.6: Sync status indicator shows if driver data is stale
- AC-029.7: List supports column visibility toggle
- AC-029.8: List supports export to CSV

---

### FR-030: Shipment Detail View (Dispatcher)

**Priority**: P1 (MVP)

**Description**: The dispatcher dashboard shall display detailed shipment information.

**Acceptance Criteria**:
- AC-030.1: View shows all shipment metadata
- AC-030.2: View displays tracking timeline with map
- AC-030.3: View shows driver contact info (call/message buttons)
- AC-030.4: View displays proof of delivery when complete
- AC-030.5: View allows status override (with audit log)
- AC-030.6: View allows adding dispatcher notes
- AC-030.7: View shows all tracking events with coordinates

---

### FR-031: Driver Management

**Priority**: P1 (MVP)

**Description**: The dashboard shall allow management of drivers.

**Acceptance Criteria**:
- AC-031.1: View list of all drivers with status (active/inactive, on-duty/off-duty)
- AC-031.2: View driver details: name, phone, assigned vehicle, current location
- AC-031.3: View driver's assigned shipments
- AC-031.4: View driver's delivery history and performance metrics
- AC-031.5: Create, edit, deactivate driver accounts
- AC-031.6: Reset driver password (sends SMS with new temporary password)
- AC-031.7: Assign/unassign vehicles to drivers

---

### FR-032: Vehicle Management

**Priority**: P2 (Post-MVP)

**Description**: The dashboard shall allow management of vehicles.

**Acceptance Criteria**:
- AC-032.1: View list of all vehicles with: plate number, type, status
- AC-032.2: Create, edit, deactivate vehicles
- AC-032.3: Assign vehicles to drivers
- AC-032.4: View vehicle's shipment history
- AC-032.5: Track vehicle maintenance schedule (optional integration)

---

### FR-033: Notifications & Alerts

**Priority**: P1 (MVP)

**Description**: The system shall provide notifications and alerts to dispatchers.

**Acceptance Criteria**:
- AC-033.1: Real-time notifications for:
  - Delivery completed
  - Delivery failed/issue reported
  - Driver offline > 4 hours during active shipment
  - Route deviation detected
  - ETA exceeded
- AC-033.2: Notifications appear in dashboard notification center
- AC-033.3: Notifications can trigger browser push (with permission)
- AC-033.4: Notifications can trigger email (configurable per notification type)
- AC-033.5: Notification history is retained for 30 days
- AC-033.6: Notifications can be marked as read/unread, acknowledged

---

### FR-034: Reports & Analytics

**Priority**: P2 (Post-MVP)

**Description**: The dashboard shall provide reporting and analytics.

**Acceptance Criteria**:
- AC-034.1: Reports: deliveries per day/week/month, on-time rate, average delivery time
- AC-034.2: Driver performance report: deliveries, on-time rate, issues
- AC-034.3: Route analysis: most common routes, average times
- AC-034.4: Customer report: top customers, delivery volumes
- AC-034.5: Reports can be filtered by date range, driver, route
- AC-034.6: Reports can be exported as PDF or CSV
- AC-034.7: Dashboard visualizations: charts, graphs for key metrics

---

### FR-035: Dispatcher Messaging

**Priority**: P2 (Post-MVP)

**Description**: The dashboard shall allow dispatchers to communicate with drivers.

**Acceptance Criteria**:
- AC-035.1: Send message to driver via app (if online) or SMS (fallback)
- AC-035.2: Message is stored and shown in shipment history
- AC-035.3: Driver can respond (if app installed)
- AC-035.4: Bulk message to all drivers on active shipments
- AC-035.5: Predefined quick messages (e.g., "Call office immediately")

---

## 6. Customer Portal

### FR-036: Shipment Tracking (Customer)

**Priority**: P1 (MVP)

**Description**: Customers shall be able to track their shipments.

**Acceptance Criteria**:
- AC-036.1: Track by entering tracking number
- AC-036.2: Track by entering customer phone number (shows all shipments to that phone)
- AC-036.3: Display current status and last known location
- AC-036.4: Display estimated delivery date/time
- AC-036.5: Display status history timeline
- AC-036.6: No authentication required for tracking
- AC-036.7: Rate limiting prevents abuse (max 10 queries per minute per IP)

---

### FR-037: SMS Notifications (Customer)

**Priority**: P1 (MVP)

**Description**: Customers shall receive SMS notifications for shipment status.

**Acceptance Criteria**:
- AC-037.1: SMS sent on: shipment dispatched, in transit (optional), out for delivery, delivered
- AC-037.2: SMS includes: tracking number, status, link to tracking portal
- AC-037.3: Company can configure which notifications to send
- AC-037.4: SMS respects opt-out (customer can reply STOP to unsubscribe)
- AC-037.5: Delivery confirmation SMS includes: recipient name, delivery time

---

### FR-038: Delivery Confirmation (Customer)

**Priority**: P2 (Post-MVP)

**Description**: Customers shall be able to confirm delivery receipt.

**Acceptance Criteria**:
- AC-038.1: Customer receives SMS with confirmation link
- AC-038.2: Link opens page showing POD details
- AC-038.3: Customer can confirm receipt or report issue
- AC-038.4: Confirmation updates shipment with customer acknowledgment
- AC-038.5: Issue report creates ticket for dispatcher review

---

## 7. Sync & Data Management

### FR-039: Automatic Sync Protocol

**Priority**: P1 (MVP) - Critical

**Description**: The system shall automatically synchronize data between driver app and server.

**Acceptance Criteria**:
- AC-039.1: Sync triggers on:
  - App launch
  - Connectivity gained after offline period
  - Periodic interval when online (default: 5 minutes)
  - Manual user request
  - After critical action (POD completion)
- AC-039.2: Sync is delta-based (only changes since last sync)
- AC-039.3: Sync batches data for efficiency (max 50 events per batch)
- AC-039.4: Sync uses compression for large payloads
- AC-039.5: Sync resumes from interruption point
- AC-039.6: Sync shows progress indicator for large syncs
- AC-039.7: Sync logs success/failure with details

---

### FR-040: Conflict Resolution

**Priority**: P1 (MVP)

**Description**: The system shall resolve conflicts between local and server data.

**Acceptance Criteria**:
- AC-040.1: For tracking events: last-timestamp-wins
- AC-040.2: For status changes: most recent status from driver takes precedence
- AC-040.3: For assignment changes: server is authoritative (dispatcher wins)
- AC-040.4: For POD data: once delivered, cannot be overwritten by driver
- AC-040.5: Conflicts requiring review are flagged for dispatcher attention
- AC-040.6: All conflict resolutions are logged for audit

---

### FR-041: Data Retention

**Priority**: P2 (Post-MVP)

**Description**: The system shall manage data retention according to policy.

**Acceptance Criteria**:
- AC-041.1: Tracking events retained for 90 days
- AC-041.2: POD data retained for 1 year (legal requirement)
- AC-041.3: Shipment metadata retained for 1 year
- AC-041.4: Aggregate analytics data retained indefinitely
- AC-041.5: User can request data export (GDPR-inspired)
- AC-041.6: User can request account deletion (anonymizes personal data)

---

### FR-042: Audit Logging

**Priority**: P1 (MVP)

**Description**: The system shall maintain audit logs for all significant actions.

**Acceptance Criteria**:
- AC-042.1: Log all authentication events (login, logout, failed attempt)
- AC-042.2: Log all status changes with: user, timestamp, old value, new value, source (app/dashboard)
- AC-042.3: Log all shipment modifications
- AC-042.4: Log all user account changes
- AC-042.5: Logs are immutable (append-only)
- AC-042.6: Logs are retained for 1 year minimum
- AC-042.7: Platform admins can query logs for investigation

---

## 8. Billing & Subscription

### FR-043: Subscription Plans

**Priority**: P2 (Post-MVP) - Required for SAAS Launch

**Description**: The system shall manage subscription plans for companies.

**Acceptance Criteria**:
- AC-043.1: Plans: Starter (up to 5 drivers), Business (up to 50), Enterprise (unlimited)
- AC-043.2: Each plan defines: driver limit, feature access, SMS quota
- AC-043.3: Company admin can view current plan and usage
- AC-043.4: Company admin can upgrade/downgrade plan
- AC-043.5: Plan changes take effect at next billing cycle (upgrade: immediate, downgrade: end of cycle)

---

### FR-044: Billing Management

**Priority**: P2 (Post-MVP)

**Description**: The system shall manage billing and payments.

**Acceptance Criteria**:
- AC-044.1: Billing cycle: monthly or annual (discount for annual)
- AC-044.2: Payment methods: mobile money (CBE Birr, telebirr), bank transfer, card (future)
- AC-044.3: Invoices generated automatically at billing cycle
- AC-044.4: Invoices sent via email
- AC-044.5: Invoice history available in dashboard
- AC-044.6: Overdue accounts are suspended (read-only mode)
- AC-044.7: Grace period of 7 days before suspension

---

### FR-045: Usage Tracking

**Priority**: P2 (Post-MVP)

**Description**: The system shall track usage for billing and analytics.

**Acceptance Criteria**:
- AC-045.1: Track: active drivers, shipments created, deliveries completed, SMS sent
- AC-045.2: Usage dashboard for company admin
- AC-045.3: Alerts when approaching plan limits (80% threshold)
- AC-045.4: Overage charges clearly defined and communicated
- AC-045.5: Usage data retained for billing dispute resolution

---

## 9. API & Integrations

### FR-046: Public API

**Priority**: P3 (Future)

**Description**: The system shall provide a public API for integrations.

**Acceptance Criteria**:
- AC-046.1: RESTful API with OpenAPI documentation
- AC-046.2: API key authentication
- AC-046.3: Rate limiting per API key
- AC-046.4: Endpoints: shipments CRUD, tracking, drivers, webhooks
- AC-046.5: Versioned API (v1, v2, etc.)
- AC-046.6: API access is plan-dependent (Enterprise only initially)

---

### FR-047: Webhooks

**Priority**: P3 (Future)

**Description**: The system shall support webhooks for event notifications.

**Acceptance Criteria**:
- AC-047.1: Webhooks for: shipment created, status changed, delivered, issue reported
- AC-047.2: Configurable webhook URLs per company
- AC-047.3: Retry logic for failed webhook deliveries
- AC-047.4: Webhook logs for debugging
- AC-047.5: Signature verification for security

---

## 10. Admin & Platform Management

### FR-048: Company Onboarding

**Priority**: P2 (Post-MVP) - Required for SAAS

**Description**: The system shall support self-service company onboarding.

**Acceptance Criteria**:
- AC-048.1: Registration form: company name, admin name, email, phone, password
- AC-048.2: Email verification required
- AC-048.3: Phone verification via SMS OTP
- AC-048.4: Trial period of 14 days with Starter plan features
- AC-048.5: Guided setup wizard: add drivers, add vehicles, create first shipment
- AC-048.6: Skip trial and subscribe immediately option

---

### FR-049: Platform Admin Dashboard

**Priority**: P2 (Post-MVP)

**Description**: The platform shall have an admin dashboard for SAAS operators.

**Acceptance Criteria**:
- AC-049.1: View all companies with status (active, trial, suspended)
- AC-049.2: View platform-wide metrics: total drivers, shipments, revenue
- AC-049.3: Manage company accounts: activate, suspend, delete
- AC-049.4: View billing and payment history
- AC-049.5: Support ticket management
- AC-049.6: System health monitoring

---

---

## Appendix A: Priority Definitions

| Priority | Definition | MVP Inclusion |
|----------|------------|---------------|
| P1 | Must Have - Critical for core functionality | Yes |
| P2 | Should Have - Important but not critical | Post-MVP (Months 4-6) |
| P3 | Nice to Have - Enhances product | Future (Months 7+) |

---

## Appendix B: Status State Machine

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

## Appendix C: Data Model Summary

### Shipment
- id (UUID)
- tracking_number (unique string)
- tenant_id (foreign key)
- origin (address + coordinates)
- destination (address + coordinates)
- customer_name
- customer_phone
- cargo_description (optional)
- status (enum)
- driver_id (foreign key, nullable)
- vehicle_id (foreign key, nullable)
- estimated_delivery (timestamp)
- actual_delivery (timestamp, nullable)
- created_at, updated_at
- created_by (user id)

### TrackingEvent
- id (UUID)
- shipment_id (foreign key)
- driver_id (foreign key)
- coordinates (lat, lng)
- accuracy (meters)
- speed (optional)
- heading (optional)
- event_type (enum: gps_ping, checkpoint, status_change)
- status (nullable, for status_change events)
- note (nullable)
- recorded_at (device timestamp)
- synced_at (server timestamp, nullable)
- device_id

### ProofOfDelivery
- id (UUID)
- shipment_id (foreign key)
- driver_id (foreign key)
- recipient_name
- recipient_phone (optional)
- signature_data (base64 or URL)
- photo_urls (array)
- delivery_location (address + coordinates)
- delivery_notes (optional)
- recorded_at (device timestamp)
- synced_at (server timestamp, nullable)

### User
- id (UUID)
- tenant_id (foreign key)
- role (enum: driver, dispatcher, fleet_manager, admin)
- full_name
- phone (unique)
- email (unique, nullable for drivers)
- password_hash
- is_active
- created_at, updated_at

---

*Document End*
*Total Requirements: 49*
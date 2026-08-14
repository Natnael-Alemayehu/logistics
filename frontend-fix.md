frontend-fix.md - Document Plan
Document Structure
 Frontend Fix Document
 Ethiopian Logistics Platform - Frontend Audit & Fixes
 Executive Summary
- Total Issues Found: 72
- Critical: 8 | High: 18 | Medium: 26 | Low: 20
- Estimated Fix Time: 8-12 hours
---
 Section 1: Type System Fixes
 1.1 Vehicle Type (Critical)
- Location: `frontend/types/vehicle.ts`
- Problem: Frontend type has `make`, `model`, `year`, `status`, `capacity_kg` but backend only has:
  - `id`, `tenant_id`, `plate_number`, `vehicle_type`, `is_active`, `created_at`, `updated_at`
- Fix: Align Vehicle interface with backend model
 1.2 Driver Type (Critical)
- Location: `frontend/types/driver.ts`
- Problem: Frontend expects fields that don't exist (`assigned_vehicle_id`, `current_lat/lng`, etc.)
- Fix: Driver is actually a User with role="driver", use User type
 1.3 TrackingEvent Coordinates (Critical)
- Location: `frontend/types/shipment.ts`
- Problem: Uses `lat/lng`, backend uses `latitude/longitude`
- Fix: Rename coordinate properties
 1.4 Missing Types (High)
- Add `LoginOutput`, `SessionOutput`, `PaginatedResponse<T>`, `DriverLocationOutput`, `PODOutput`
---
 Section 2: API Endpoint Fixes
 2.1 Non-Existent Endpoints (Critical) - MARK AS TODO
These endpoints are called but don't exist in backend:
- `/api/v1/tenants/*` - Tenant management
- `/api/v1/dashboard/alerts` - Dashboard alerts
- `/api/v1/dashboard/activity` - Recent activity
- `/api/v1/reports/*` - Reports
- `/api/v1/admin/stats` - Platform stats
- `/api/v1/drivers/locations` - Bulk driver locations
Fix: Mark hooks and components as TODO, add comments
 2.2 Wrong HTTP Methods/Endpoints (Critical)
- `useDeleteShipment` - Backend has no DELETE, use POST `/shipments/{id}/cancel`
- `useDeleteDriver` - Backend has no DELETE endpoint for drivers
- `useUpdateDriver` - Backend has no PUT for drivers
- `useDriverLocations` - Endpoint doesn't exist, use individual `/drivers/{id}/location`
 2.3 Missing Request Body (Critical)
- `useRevokeOtherSessions` - Must send `current_session_id` in body
 2.4 Missing Hooks (High)
- `useCancelShipment` - POST `/shipments/{id}/cancel`
- `useChangePassword` - POST `/auth/change-password`
- `useForgotPIN` - POST `/auth/forgot-pin`
- `useSearchShipments` - GET `/shipments/search`
---
 Section 3: Auth & Store Fixes
 3.1 Auth Store Persistence (Critical)
- Location: `frontend/stores/auth-store.ts`
- Problem: Only tokens persisted, user object lost on refresh
- Fix: Either persist user object OR add initialization from token
 3.2 Session Type Mismatch (High)
- Frontend uses `is_current_session`, backend uses `is_current`
 3.3 Redundant Methods (Low)
- `logout` and `clearAuth` are identical
---
 Section 4: Page & Component Fixes
 4.1 Broken Navigation (Critical)
- `/shipments/[id]/edit` - Page doesn't exist, remove links
- `/settings/profile` - Page doesn't exist, remove links
 4.2 Role-Based Access (High)
Add role checks to pages (not just sidebar):
- `/reports` - admin, fleet_manager only
- `/vehicles` - admin, fleet_manager only
- `/drivers` - admin, fleet_manager, dispatcher
- `/settings` - admin only
 4.3 Error Handling (High)
All pages need error handling for data fetching:
- Dashboard, Shipments, Drivers, Vehicles, Reports, Settings, Admin pages
 4.4 Login Redirect (High)
- Authenticated users should be redirected away from `/login`
 4.5 Data Display Issues (Medium)
- Shipment detail shows `driver_id` UUID instead of driver name
- Settings company tab shows hardcoded values
---
 Section 5: Form Fixes
 5.1 Vehicle Form (Critical)
- Location: `frontend/components/vehicles/vehicle-form.tsx`
- Problem: Sends `make`, `model`, `year`, `status` - backend ignores these
- Fix: Send only `plate_number` (required) and `vehicle_type` (optional)
 5.2 Driver Form (Critical)
- Location: `frontend/components/drivers/driver-form.tsx`
- Problem: `pin` is optional but backend requires it
- Fix: Make `pin` required for new drivers
 5.3 Missing Cancel Buttons (Low)
Add cancel buttons to: vehicle-form, driver-form, shipment-form
---
 Section 6: Admin Section (Keep Minimal)
 6.1 Remove Tenant Management (High)
- Delete `/admin/tenants` page
- Remove `useTenants`, `useCreateTenant`, `useUpdateTenant` hooks
- Remove `tenant-form.tsx` component
- Mark tenant-related code as TODO
 6.2 Keep User Management (Partial Backend Support)
- Backend has `GET /users`, `POST /users`, `PUT /users/{id}`
- Keep `/admin/users` page but mark as limited functionality
 6.3 Admin Dashboard (TODO)
- Mark `/admin` dashboard as TODO since `/admin/stats` doesn't exist
- Show placeholder message
---
 Section 7: Reports Section (TODO)
 7.1 Mark as TODO
All reports functionality depends on non-existent endpoints:
- `useDeliveryReport`
- `useDriverPerformanceReport`
- `useReportsStats`
- `/reports` page
Fix: Add TODO comments, show "Coming Soon" placeholder
---
 Section 8: Implementation Order
 Phase 1: Critical Fixes (2-3 hours)
1. Fix Vehicle type and form
2. Fix Driver type and form
3. Fix TrackingEvent coordinates
4. Fix Auth store persistence
5. Fix useDeleteShipment → useCancelShipment
6. Fix useRevokeOtherSessions
 Phase 2: High Priority (3-4 hours)
1. Remove broken navigation links
2. Add missing hooks (cancel shipment, change password, forgot PIN)
3. Add role-based access to pages
4. Add error handling to all pages
5. Fix login redirect
 Phase 3: Medium Priority (2-3 hours)
1. Add missing types
2. Mark non-existent endpoints as TODO
3. Mark admin section as minimal/TODO
4. Mark reports as TODO
5. Fix data display issues
 Phase 4: Low Priority (1-2 hours)
1. Add store persistence
2. Add cancel buttons to forms
3. Remove redundant methods
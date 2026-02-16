 Project Overview
Ethiopian Logistics SAAS - An offline-first mobile logistics platform for Ethiopian trucking companies that provides:
- Real-time fleet tracking and visibility
- Proof of delivery capture (photo, signature, GPS)
- Offline-first architecture that works in low-connectivity areas
- SMS fallback for critical operations
 Key Reference Files
When working on this project, reference these files for context:
| Need | File | Description |
|------|------|-------------|
| Business context | `./business_model.md` | Business model, pricing, market strategy |
| Functional requirements | `./functional_requirements.md` | What the system must do |
| Non-functional requirements | `./non_functional_requirements.md` | Performance, security, scalability constraints |
| Implementation plan | `./plan.md` | MVP build plan with tasks and priorities |
| Database schema | `./schema.md` | PostgreSQL schema with PostGIS |
| Architecture details | `./logistics.md` | Technical architecture and design decisions |
**Always read `plan.md` first to understand the current task context and priorities.**
 Tech Stack
- **Backend:** Golang
- **Database:** PostgreSQL 14+ with PostGIS
- **Messaging:** RabbitMQ
- **Mobile App:** Flutter or React Native (Android-first)
- **Infrastructure:** Multi-tenant SaaS with company isolation
- **Timezone:** Africa/Addis_Ababa (UTC+3)
 Architecture Principles
 Offline-First Design
- All critical operations must work offline
- Local SQLite on mobile with sync queue
- Background sync with priority levels:
  - **Critical:** Delivery confirmations, emergency alerts (sync immediately)
  - **High:** Status updates, GPS breadcrumbs (sync on 3G+)
  - **Normal:** Photos, signatures, analytics (sync on WiFi/strong 4G)
- SMS fallback when no data connection for >30 minutes
 Multi-Tenancy
- `company_id` on ALL tenant-scoped tables
- Row Level Security (RLS) for data isolation
- Subscription tiers: small, medium, large, enterprise
 Database Conventions
- **Primary keys:** UUID using `gen_random_uuid()`
- **Soft deletes:** `deleted_at TIMESTAMP WITH TIME ZONE`
- **Timestamps:** Always `TIMESTAMP WITH TIME ZONE`
- **Flexible fields:** Use `JSONB DEFAULT '{}'`
- **Spatial data:** Use PostGIS `GEOGRAPHY(POINT, 4326)`
- **Currency:** All amounts in ETB (Ethiopian Birr)
 Domain Terms
| Term | Definition |
|------|------------|
| POD | Proof of Delivery - photo, signature, GPS confirmation |
| Route | Daily assignment of deliveries to a driver |
| Stop | Waypoint in a route (pickup, delivery, fuel, rest) |
| Delivery | Individual package/job being transported |
| ETB | Ethiopian Birr (currency) |
| Sync Queue | Offline operation buffer for later upload |
 Coding Standards
 Go Backend
- Use standard project layout (`cmd/`, `internal/`, `pkg/`)
- Context propagation for all database operations
- Structured logging (slog or zerolog)
- Unit tests for business logic
 Database
- All migrations in `migrations/` directory
- Never modify existing migrations - create new ones
- Always include `updated_at` trigger
- Index foreign keys and frequently queried columns
 API Design
- RESTful endpoints under `/api/v1/`
- JWT authentication with company context
- Standard error response format:
    {
    "error": {
      "code": "ERROR_CODE",
      "message": "Human readable message"
    }
  }
Workflow
1. Before starting any task: Read plan.md to understand context
2. For business context: Read ./business_model.md
3. For requirements: Check functional_requirements.md and non_functional_requirements.md
4. For database changes: Reference schema.md and follow conventions
Do's
- Consider offline scenarios in ALL code
- Handle timezone conversions properly (UTC in DB, local display)
- Design for low-bandwidth, high-latency environments
- Test with realistic Ethiopian network conditions
- Follow the existing schema patterns in schema.md
- Use the sync queue pattern for mobile-to-server operations
Don'ts
- Don't assume constant network connectivity
- Don't hardcode currencies, locales, or timezones
- Don't skip company_id on new tables
- Don't ignore multi-tenant isolation requirements
- Don't create tables without proper indexes
- Don't use synchronous operations for non-critical syncs

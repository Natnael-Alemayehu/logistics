# Phase 4: Production Readiness & GCP Deployment
## Ethiopian Logistics Tracking Platform - Backend

**Status**: Ready for Planning
**Prerequisites**: Phase 1-3 completed (MVP backend, SMS, Storage, WebSocket, Integration Tests)
**Date**: February 2026
**Target**: Deploy to GCP VM with production-ready infrastructure

---

## Executive Summary

This phase focuses on making the backend production-ready for deployment to Google Cloud Platform. Key areas include:

1. **Redis Integration** - Distributed caching, sessions, rate limiting, pub/sub
2. **Observability** - Prometheus metrics, structured logging, request tracing
3. **Background Jobs** - Async processing for photos, SMS, reports
4. **Deployment** - Docker production setup, GCP VM configuration
5. **Security Hardening** - HTTPS, secure headers, secrets management

---

## Current Infrastructure State

### Completed
| Component | Status | Notes |
|-----------|--------|-------|
| PostgreSQL Database | Done | PostGIS enabled, migrations working |
| JWT Authentication | Done | RS256 signing, refresh tokens |
| REST API | Done | Chi router, all MVP endpoints |
| File Storage | Done | Local + S3-compatible |
| SMS Service | Done | Mock/Twilio/Ethio Telecom |
| WebSocket | Done | Real-time updates |
| Integration Tests | Done | testcontainers, auth/shipment/sync |

### Missing for Production
| Component | Gap | Impact |
|-----------|-----|--------|
| Redis | No distributed caching/sessions | Cannot scale horizontally |
| Metrics | No observability | Cannot monitor performance |
| Background Jobs | Synchronous heavy operations | Poor UX for uploads |
| Rate Limiting | IP-based only | No per-user limits |
| API Versioning | No /api/v1/ prefix | Breaking changes difficult |
| HTTPS | No SSL termination | Insecure in production |

---

## Part A: Redis Integration

### A.1 Redis Client Package

**File**: `pkg/redis/redis.go`

**Purpose**: Centralized Redis client with connection pooling

**Features**:
- Connection pool management
- Health check support
- Context-aware operations
- Automatic reconnection

**Configuration**:
```go
type RedisConfig struct {
    Addr         string // host:port
    Password     string
    DB           int
    PoolSize     int    // default: 10
    MinIdleConns int    // default: 5
    MaxRetries   int    // default: 3
}
```

---

### A.2 Session Storage Migration

**File**: `internal/service/session.go`

**Changes**:
- Move session storage from PostgreSQL to Redis
- TTL-based automatic expiration
- Faster session lookups
- Reduce database load

**Redis Key Schema**:
```
session:{session_id}           -> JSON session data (TTL: 30 days)
user_sessions:{user_id}        -> SET of session_ids
```

---

### A.3 Distributed Rate Limiting

**File**: `internal/middleware/ratelimit.go`

**Purpose**: Per-user, per-IP rate limiting that works across multiple API instances

**Limits**:
| Scope | Limit | Window |
|-------|-------|--------|
| Per IP | 100 requests | 1 minute |
| Per User | 200 requests | 1 minute |
| Per API Key | 1000 requests | 1 minute |
| Auth endpoints | 10 requests | 1 minute |

**Redis Key Schema**:
```
ratelimit:ip:{ip}              -> counter
ratelimit:user:{user_id}       -> counter
ratelimit:api_key:{key}        -> counter
```

---

### A.4 WebSocket Pub/Sub

**File**: `pkg/websocket/redis_hub.go`

**Purpose**: Enable WebSocket message broadcasting across multiple API instances

**Changes**:
- Replace in-memory hub with Redis pub/sub
- Channel: `ws:tenant:{tenant_id}`
- Support horizontal scaling of API servers

---

## Part B: Observability

### B.1 Prometheus Metrics

**File**: `pkg/metrics/metrics.go`

**Metrics to Collect**:

| Category | Metric | Type | Labels |
|----------|--------|------|--------|
| HTTP | `http_requests_total` | Counter | method, path, status |
| HTTP | `http_request_duration_seconds` | Histogram | method, path |
| Database | `db_query_duration_seconds` | Histogram | query, operation |
| Business | `shipments_created_total` | Counter | tenant_id |
| Business | `deliveries_completed_total` | Counter | tenant_id |
| Business | `sync_events_total` | Counter | event_type |
| SMS | `sms_sent_total` | Counter | provider, status |
| WebSocket | `ws_connections_active` | Gauge | tenant_id |
| Queue | `queue_jobs_pending` | Gauge | job_type |
| Queue | `queue_jobs_processed_total` | Counter | job_type, status |

**Endpoint**: `GET /metrics` (Prometheus format)

---

### B.2 Metrics Middleware

**File**: `internal/middleware/metrics.go`

**Features**:
- Record request duration
- Count requests by status code
- Track in-flight requests
- Exclude health/metrics endpoints

---

### B.3 Structured Logging Enhancement

**File**: `pkg/logger/logger.go`

**Changes**:
- JSON output for production
- Log level configuration
- Request ID in all logs
- Trace ID for distributed tracing

**Log Format**:
```json
{
  "timestamp": "2026-02-18T10:30:00Z",
  "level": "info",
  "message": "request completed",
  "request_id": "abc123",
  "trace_id": "def456",
  "tenant_id": "tenant-uuid",
  "user_id": "user-uuid",
  "method": "POST",
  "path": "/shipments",
  "status": 201,
  "duration_ms": 45
}
```

---

### B.4 Grafana Dashboards

**File**: `deployments/grafana/dashboards/`

**Dashboards**:
1. **API Overview** - Request rate, latency, errors
2. **Business Metrics** - Shipments, deliveries, active drivers
3. **Infrastructure** - CPU, memory, database connections

---

## Part C: Background Jobs

### C.1 Job Queue System

**File**: `pkg/queue/queue.go`

**Interface**:
```go
type Queue interface {
    Enqueue(ctx context.Context, job Job) error
    Dequeue(ctx context.Context, jobType string) (*Job, error)
    Ack(ctx context.Context, jobID string) error
    Nack(ctx context.Context, jobID string, err error) error
}
```

**Redis Implementation**: `pkg/queue/redis.go`

**Job Types**:
| Job Type | Description | Priority |
|----------|-------------|----------|
| `photo_compress` | Compress POD photos | Normal |
| `sms_send` | Send SMS notifications | High |
| `sms_batch` | Batch SMS sending | Normal |
| `report_generate` | Generate analytics reports | Low |
| `data_archive` | Archive old tracking events | Low |

---

### C.2 Photo Compression Worker

**File**: `internal/jobs/photo_compress.go`

**Flow**:
1. Receive job with photo path
2. Download from storage
3. Compress to target size (<500KB)
4. Upload compressed version
5. Update database with new URL

---

### C.3 SMS Batch Worker

**File**: `internal/jobs/sms_batch.go`

**Purpose**: Batch SMS notifications for efficiency

**Flow**:
1. Collect pending SMS from queue
2. Batch send via provider
3. Update audit log with results

---

### C.4 Worker Process

**File**: `cmd/worker/main.go`

**Features**:
- Configurable concurrency
- Graceful shutdown
- Job retry with exponential backoff
- Dead letter queue for failed jobs

---

## Part D: Security Hardening

### D.1 HTTPS with Let's Encrypt

**File**: `deployments/nginx/nginx.conf`

**Setup**:
- Nginx reverse proxy
- Certbot for SSL certificates
- Automatic renewal via cron
- HTTP to HTTPS redirect

**Alternative**: Caddy (automatic HTTPS)

---

### D.2 Security Headers Middleware

**File**: `internal/middleware/security.go`

**Headers**:
```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Strict-Transport-Security: max-age=31536000; includeSubDomains
Content-Security-Policy: default-src 'self'
Referrer-Policy: strict-origin-when-cross-origin
```

---

### D.3 Secrets Management

**Options**:
1. **Environment variables** (simplest, good for single VM)
2. **GCP Secret Manager** (recommended for production)
3. **HashiCorp Vault** (enterprise)

**Implementation**: Start with env vars, plan migration to GCP Secret Manager

**Secrets to manage**:
- Database URL
- Redis URL
- JWT private key
- SMS API keys
- S3 credentials

---

### D.4 Input Validation Audit

**Files to review**:
- All handler input structs
- All service input structs
- SQL query parameterization (already using sqlc)

---

## Part E: GCP Deployment

### E.1 Architecture

```
+---------------------------------------------------------+
|                     GCP VM (e2-micro)                    |
|  +---------------------------------------------------+  |
|  |                   Nginx (SSL)                     |  |
|  |                    :443                          |  |
|  +---------------------+---------------------------+  |
|                        |                              |
|  +---------------------v---------------------------+  |
|  |              Logistics API                       |  |
|  |                  :8080                           |  |
|  +----------+---------------------+----------------+  |
|             |                     |                  |
|  +----------v--------+  +----------v----------+       |
|  |   PostgreSQL      |  |      Redis         |       |
|  |   (Docker)        |  |     (Docker)       |       |
|  |     :5432         |  |      :6379         |       |
|  +-------------------+  +-------------------+       |
|                                                         |
|  +---------------------------------------------------+  |
|  |              Prometheus + Grafana                 |  |
|  |              (Optional, or remote)                |  |
|  +---------------------------------------------------+  |
+---------------------------------------------------------+
```

---

### E.2 GCP Setup Steps

**1. Create VM Instance**:
```bash
# Instance: e2-micro
# Region: us-central1 (or closest to Ethiopia)
# Disk: 20GB SSD
# OS: Ubuntu 22.04
# Network: Allow HTTP, HTTPS, SSH
```

**2. Install Dependencies**:
```bash
# Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
```

**3. Deploy Application**:
```bash
# Clone repository
git clone <repo-url> /opt/logistics
cd /opt/logistics

# Create environment file
cp .env.example .env
# Edit .env with production values

# Start services
docker-compose -f docker-compose.prod.yml up -d
```

**4. Setup SSL**:
```bash
# Install Certbot
sudo apt install certbot python3-certbot-nginx

# Obtain certificate
sudo certbot --nginx -d api.yourdomain.com

# Auto-renewal
sudo systemctl enable certbot.timer
```

---

### E.3 Production Docker Compose

**File**: `docker-compose.prod.yml`

**Services**:
| Service | Image | Resources | Purpose |
|---------|-------|-----------|---------|
| api | logistics-api | 512MB | Main API server |
| postgres | postgres:15-alpine | 512MB | Database |
| redis | redis:7-alpine | 128MB | Cache/sessions |
| nginx | nginx:alpine | 64MB | Reverse proxy, SSL |
| prometheus | prom/prometheus | 256MB | Metrics (optional) |
| grafana | grafana/grafana | 256MB | Dashboards (optional) |

**Total RAM**: ~1.5GB (fits e2-micro with 1GB swap)

---

### E.4 Database Backup Strategy

**Automated Backups**:
```bash
# Daily backup script
pg_dump -Fc logistics > /backups/logistics_$(date +%Y%m%d).dump

# Retain 7 days locally
# Optional: Upload to GCS
gsutil cp /backups/logistics_*.dump gs://logistics-backups/
```

**Cron**: `0 2 * * * /opt/logistics/scripts/backup.sh`

---

### E.5 Cost Estimate

| Item | Configuration | Monthly Cost |
|------|---------------|--------------|
| e2-micro VM | 2 vCPU, 1GB RAM | $3.88 |
| Persistent Disk | 20GB SSD | $0.85 |
| External IP | Static | $0.00 (included) |
| Network Egress | ~10GB/month | $0.00 (free tier) |
| **Total** | | **~$4.73** |

**Within $5 credit!**

---

## Part F: Configuration Changes

### F.1 Environment Variables

**File**: `.env.example`

```bash
# Server
PORT=8080
ENVIRONMENT=production
LOG_LEVEL=info

# Database
DATABASE_URL=postgres://logistics:password@postgres:5432/logistics?sslmode=disable

# Redis
REDIS_URL=redis:6379

# JWT
JWT_PRIVATE_KEY_PATH=/app/keys/private.pem
JWT_PUBLIC_KEY_PATH=/app/keys/public.pem
JWT_ACCESS_TTL=3600
JWT_REFRESH_TTL=2592000
JWT_ISSUER=logistics.et

# Storage
STORAGE_TYPE=s3
S3_BUCKET=logistics-pod
S3_REGION=auto
S3_ENDPOINT=https://s3.amazonaws.com
S3_ACCESS_KEY=your-access-key
S3_SECRET_KEY=your-secret-key

# SMS
SMS_PROVIDER=mock
TRACKING_URL_BASE=https://api.yourdomain.com/track

# Rate Limiting
RATE_LIMIT_IP=100
RATE_LIMIT_USER=200
RATE_LIMIT_AUTH=10
```

### F.2 Config Struct Updates

**File**: `internal/config/config.go`

**Add fields**:
- Redis configuration
- Rate limit configuration
- Storage configuration (already added)
- SMS configuration (already added)

---

## Part G: API Versioning

### G.1 Route Restructure

**Current**: `/auth/login`, `/shipments`, etc.

**Target**: `/api/v1/auth/login`, `/api/v1/shipments`, etc.

**Changes**:
- Update `routes.go` to prefix all routes with `/api/v1`
- Keep `/health`, `/metrics`, `/swagger` at root
- Update Swagger documentation

---

## Timeline

| Week | Tasks |
|------|-------|
| **Week 1** | Redis integration, session migration, distributed rate limiting |
| **Week 2** | Prometheus metrics, structured logging, Grafana dashboards |
| **Week 3** | Background jobs system, photo compression worker, SMS batch worker |
| **Week 4** | Security hardening, API versioning, production Docker setup |
| **Week 5** | GCP deployment, SSL setup, backup configuration |
| **Week 6** | Testing, monitoring, documentation, polish |

---

## Files to Create

### New Files (18)

```
pkg/redis/redis.go
pkg/metrics/metrics.go
pkg/queue/queue.go
pkg/queue/redis.go
pkg/logger/logger.go
internal/middleware/ratelimit.go
internal/middleware/metrics.go
internal/middleware/security.go
internal/service/session.go
internal/jobs/photo_compress.go
internal/jobs/sms_batch.go
cmd/worker/main.go
deployments/docker-compose.prod.yml
deployments/nginx/nginx.conf
deployments/grafana/dashboards/api.json
deployments/grafana/dashboards/business.json
scripts/backup.sh
docs/deployment.md
```

### Modified Files (10)

```
internal/config/config.go
internal/handler/routes.go
internal/middleware/auth.go
internal/service/auth.go
pkg/websocket/hub.go
cmd/api/main.go
docker-compose.yml
Makefile
.env.example
README.md
```

---

## Success Criteria

### Deployment Checklist

- [ ] API accessible via HTTPS
- [ ] All endpoints return expected responses
- [ ] Database migrations run successfully
- [ ] Redis connection working
- [ ] WebSocket connections stable
- [ ] Prometheus metrics available
- [ ] Rate limiting enforced
- [ ] SSL certificate valid
- [ ] Backups configured
- [ ] Monitoring alerts set up

### Performance Targets

| Metric | Target |
|--------|--------|
| API response time (P95) | < 500ms |
| Database query time (P95) | < 100ms |
| WebSocket connection time | < 100ms |
| Memory usage | < 700MB |
| CPU usage (idle) | < 20% |

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| VM runs out of memory | Add 1GB swap, monitor usage |
| Single point of failure | Plan for managed services upgrade |
| Database backup failure | Test restore weekly |
| SSL certificate expiry | Certbot auto-renewal |
| Cost overrun | Set up GCP budget alerts |

---

## Future Considerations

After Phase 4, consider:

1. **Managed PostgreSQL** (Cloud SQL) when budget allows
2. **Managed Redis** (Memorystore) for reliability
3. **CDN** for static assets and API caching
4. **Kubernetes** for horizontal scaling
5. **CI/CD Pipeline** for automated deployments

---

*Document End*
*Phase 4 - Production Readiness & GCP Deployment*
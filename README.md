# Incident Management System

A backend-focused Incident Management System built in Go for an internship assignment. The project demonstrates async signal ingestion, Redis-backed queuing and debounce, MongoDB audit storage, PostgreSQL work items, RCA validation, MTTR calculation, and a small HTML dashboard for reviewers.

## Architecture

```text
                  HTTP
Reviewer/API ---> Gin handlers
                    |
                    v
                 Services
                    |
        +-----------+------------+
        |           |            |
        v           v            v
      Redis      MongoDB     PostgreSQL
   queue/cache   raw audit   work_items + rca
        |
        v
   Worker pool
```

Code follows a clean, simple layering:

```text
cmd/api                 application entrypoint
internal/config         environment configuration
internal/domain         core domain models and state machine
internal/handlers       HTTP request/response layer
internal/services       business logic
internal/repositories   Redis, MongoDB, PostgreSQL implementations
internal/queues         queue abstraction
internal/ratelimit      Redis rate limiter
internal/middleware     logging, CORS, rate limiting
internal/routes         route registration
internal/server         Gin server assembly
internal/worker         async signal worker pool
index.html              lightweight dashboard UI
```

## What It Does

- Accepts incident signals through `POST /ingest`.
- Pushes accepted signals to Redis so ingestion stays async.
- Uses a fixed-size worker pool to process queued signals.
- Stores every processed signal in MongoDB as an audit log.
- Uses Redis debounce to create only one work item per component per 10 seconds.
- Stores durable work items and RCA records in PostgreSQL.
- Enforces work item lifecycle transitions:

```text
OPEN -> INVESTIGATING -> RESOLVED -> CLOSED
```

- Requires RCA before closing a work item.
- Calculates MTTR as `end_time - start_time` when RCA is submitted.
- Exposes a dashboard API and a plain HTML dashboard.

## Async Ingestion

`POST /ingest` validates the payload, applies Redis-backed rate limiting, and enqueues the signal into Redis. The API returns `202 Accepted` after queue publish, so clients are not blocked by MongoDB or PostgreSQL writes.

The worker pool continuously consumes from Redis, retries bounded processing failures, sends permanently failed payloads to a DLQ, and logs throughput every few seconds.

## Debounce Logic

Redis stores a key per component:

```text
debounce:{component_id} = work_item_id
```

The key has a 10-second TTL. The first signal for a component creates a new work item ID. Further signals during the TTL reuse the same ID, so all raw signals are linked while only one work item is created for the burst.

The Redis debounce operation is atomic through a Lua script, avoiding duplicate work item IDs during concurrent worker processing.

## Storage Choices

MongoDB is used for raw signals because incoming signal documents are append-only audit records.

PostgreSQL is used for work items and RCA because they are relational business records with lifecycle state, uniqueness, foreign keys, and MTTR updates.

Redis is used for queueing, rate limiting, dashboard cache, and debounce coordination.

## RCA And MTTR

RCA requires:

- `work_item_id`
- `start_time`
- `end_time`
- `root_cause`
- `fix`
- `prevention`

`end_time` must be after `start_time`. After RCA creation, MTTR is stored on the work item in nanoseconds.

## API Endpoints

### Health

```bash
curl http://localhost:8080/health
```

```json
{
  "status": "ok",
  "service": "ims-backend",
  "time": "2026-05-06T10:00:00Z"
}
```

### Ingest Signal

```bash
curl -X POST http://localhost:8080/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "component_id": "CACHE_CLUSTER_01",
    "severity": "P2",
    "message": "High latency detected",
    "timestamp": "2026-05-03T10:15:00Z"
  }'
```

```json
{
  "status": "accepted"
}
```

### List Active Work Items

```bash
curl http://localhost:8080/work-items
```

```json
{
  "status": "success",
  "data": [
    {
      "work_item_id": "11111111-1111-1111-1111-111111111111",
      "component_id": "CACHE_CLUSTER_01",
      "severity": "P0",
      "status": "OPEN",
      "created_at": "2026-05-03T10:15:00Z"
    }
  ]
}
```

### Get Work Item

```bash
curl http://localhost:8080/work-items/{work_item_id}
```

```json
{
  "status": "success",
  "data": {
    "id": "11111111-1111-1111-1111-111111111111",
    "component_id": "CACHE_CLUSTER_01",
    "severity": "P0",
    "status": "OPEN",
    "created_at": "2026-05-03T10:15:00Z",
    "updated_at": "2026-05-03T10:15:00Z"
  }
}
```

### Transition Work Item

```bash
curl -X PATCH http://localhost:8080/work-items/{work_item_id}/status \
  -H "Content-Type: application/json" \
  -d '{
    "status": "INVESTIGATING"
  }'
```

### Create RCA

```bash
curl -X POST http://localhost:8080/rca \
  -H "Content-Type: application/json" \
  -d '{
    "work_item_id": "11111111-1111-1111-1111-111111111111",
    "start_time": "2026-05-03T10:15:00Z",
    "end_time": "2026-05-03T10:45:00Z",
    "root_cause": "Cache node memory exhaustion",
    "fix": "Replaced unhealthy node",
    "prevention": "Added memory saturation alert"
  }'
```

## Setup

### Option 1: Run Everything With Docker Compose

```bash
docker-compose up -d
```

API:

```text
http://localhost:8080
```

Dashboard:

```text
index.html
```

Open `index.html` in a browser after the backend is running.

### Option 2: Run Dependencies In Docker And API Locally

```bash
docker-compose up -d postgres mongodb redis
go run cmd/api/main.go
```

Local defaults:

```text
REDIS_ADDR=localhost:6379
MONGO_URI=mongodb://localhost:27017
POSTGRES_DSN=postgres://postgres:postgres@localhost:5432/ims?sslmode=disable
```

## Configuration

```text
APP_NAME=ims-backend
APP_ENV=development
HTTP_ADDR=:8080
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
MONGO_URI=mongodb://localhost:27017
MONGO_DATABASE=ims
MONGO_COLLECTION=raw_signals
POSTGRES_DSN=postgres://postgres:postgres@localhost:5432/ims?sslmode=disable
SIGNAL_QUEUE_NAME=ims:signals
SIGNAL_DLQ_NAME=ims:signals:dlq
WORKER_COUNT=5
RATE_LIMIT_REQUESTS=100
RATE_LIMIT_WINDOW=1s
```

## Tests

```bash
go test ./...
```

## Frontend Dashboard

The dashboard is a single self-contained HTML file:

```text
index.html
```

It fetches:

```text
GET http://localhost:8080/work-items
```

It displays component ID, severity, status, and created time, with severity badges:

- `P0`: red
- `P1`: orange
- `P2`: yellow

## Screenshots

### Dashboard

`docs/screenshots/dashboard.png`

### Docker Containers

`docs/screenshots/docker.png`

### Health Endpoint

`docs/screenshots/health-endpoint.png`

### Work Items API

`docs/screenshots/work-items-api.png`

### Architecture Diagram
`docs/screenshots/architecture.png`


## Known Limitations

- Redis list queue uses simple push/pop semantics and is suitable for assignment demo scope, not full production-grade acknowledgement semantics.
- Schema is bootstrapped at startup for local development; production systems should use migrations.
- RCA creation and MTTR update are separate repository calls.
- Graceful shutdown is minimal; the app relies on process termination behavior.
- The dashboard is intentionally read-only.

## Future Improvements

- Add database migrations.
- Add queue depth and DLQ metrics.
- Add graceful HTTP and worker shutdown.
- Add DLQ replay tooling.
- Add authentication for write APIs.

## Submission Notes

Generated artifacts such as `bin/`, `.gocache/`, `.cursor/`, test binaries, logs, and local env files are ignored. The repository is intended to be reviewed from source plus Docker Compose.

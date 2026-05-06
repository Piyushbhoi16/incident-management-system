# Prompts Used During Implementation

## Architecture
- Design a clean Go backend structure using handlers -> services -> repositories -> domain with Redis, MongoDB, and PostgreSQL responsibilities separated.

## Worker
- Implement an async worker pool that consumes queued signals, retries bounded failures, and routes failed payloads to a DLQ.

## Debounce
- Enforce one work item per component within a 10-second burst using Redis atomic operations and shared work_item_id linking.

## Frontend
- Build a single-file HTML dashboard with plain CSS/JS that fetches active incidents and presents reviewer-friendly incident status visibility.

## RCA And State Machine
- Enforce strict work item transitions `OPEN -> INVESTIGATING -> RESOLVED -> CLOSED`, require RCA before close, and compute MTTR from RCA start/end timestamps.

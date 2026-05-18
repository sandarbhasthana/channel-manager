# PMS Inbound API Ingestion — Implementation Guide

This document describes the **MyPMS webhook / booking-engine ingestion** built for the Channel Manager platform. It implements [PMS_API_REFERENCE.md §1 — Webhook / Booking Engine APIs](./PMS_API_REFERENCE.md#1-webhook--booking-engine-apis): the channel manager acts as an **HTTP client** that calls your PMS, persists catalog data locally, and writes availability into the inventory service.

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Repository layout](#repository-layout)
4. [PMS API mapping](#pms-api-mapping)
5. [Connect-RPC surface](#connect-rpc-surface)
6. [Data flow](#data-flow)
7. [Configuration](#configuration)
8. [Prerequisites](#prerequisites)
9. [Testing](#testing)
10. [Known limitations](#known-limitations)
11. [Future work](#future-work)

---

## Overview

| Concern                    | Implementation                                                                 |
| -------------------------- | ------------------------------------------------------------------------------ |
| **PMS provider**           | `mypms` (MyPMS booking-engine webhooks)                                        |
| **Auth to PMS**            | `Authorization: Bearer <token>` per connection                                 |
| **Credentials storage**    | `base_url` + `bearer_token` via `ConnectPms` → in-memory secret resolver (dev) |
| **Catalog sync**           | `SyncCatalog` → `pms.properties` + `pms.room_types`                            |
| **Availability ingestion** | `IngestAvailability` → `inventory.inventory_days`                              |
| **Booking passthrough**    | Quote / create / get / update / cancel proxied to PMS                          |
| **Tenant isolation**       | Postgres RLS via `db.Pool.WithTenant` + WorkOS JWT                             |

The channel manager does **not** expose the PMS webhook URLs publicly. It **calls** them outbound using stored credentials.

---

## Architecture

```
┌─────────────────┐     Connect-RPC (JWT)      ┌──────────────────┐
│  Dashboard /    │ ─────────────────────────► │   apps/api       │
│  API clients    │                            │   PmsService     │
└─────────────────┘                            └────────┬─────────┘
                                                      │
                        ┌─────────────────────────────┼─────────────────────────────┐
                        ▼                             ▼                             ▼
               ┌────────────────┐           ┌─────────────────┐           ┌─────────────────┐
               │  PmsService    │           │ Postgres repos  │           │ InventoryService│
               │  (usecases)    │           │ pms.* schema    │           │ inventory_days  │
               └───────┬────────┘           └─────────────────┘           └─────────────────┘
                       │
                       ▼
               ┌────────────────┐     Bearer HTTP    ┌─────────────────────────┐
               │ mypms.Adapter  │ ─────────────────► │  MyPMS (your PMS)       │
               │ + HTTP Client  │   /api/webhooks/…  │  §1 Webhook APIs        │
               └────────────────┘                    └─────────────────────────┘
```

### Layering (hexagonal)

| Layer         | Package                   | Role                                                           |
| ------------- | ------------------------- | -------------------------------------------------------------- |
| **Domain**    | `services/pms/domain`     | Models: `Connection`, `Property`, `RoomType`, bookings, quotes |
| **Ports**     | `services/pms/ports`      | `BookingEngineClient`, repositories, `InventoryWriter`         |
| **Use cases** | `services/pms/usecases`   | `SyncCatalog`, `IngestAvailability`, booking proxy methods     |
| **Adapters**  | `services/pms/adapters/*` | HTTP client, Postgres, Connect-RPC, inventory bridge           |

---

## Repository layout

```
services/pms/
├── domain/models.go              # Domain types + capabilities
├── ports/ports.go                # Interfaces (BookingEngineClient, repos)
├── usecases/service.go           # Orchestration + ingestion logic
├── sqlc.yaml
├── adapters/
│   ├── mypms/
│   │   ├── client.go             # HTTP client for §1 endpoints
│   │   ├── types.go              # Request/response DTOs
│   │   ├── adapter.go            # ports.BookingEngineClient implementation
│   │   └── client_test.go        # Unit tests (httptest)
│   ├── postgres/
│   │   ├── schema.sql            # sqlc schema mirror
│   │   ├── queries/*.sql         # sqlc queries
│   │   ├── pgstore/              # Generated sqlc code
│   │   ├── connection_repo.go
│   │   ├── property_repo.go
│   │   └── room_type_repo.go
│   ├── connect/
│   │   ├── handler.go            # Connect-RPC handlers
│   │   └── mappers.go            # Proto ↔ domain
│   ├── inventory/writer.go       # Writes to inventory service
│   └── infra/secrets.go          # In-memory credential store (dev)
proto/pms/v1/pms.proto            # PmsService RPC definitions
apps/api/main.go                  # Wires PMS + registers RPC routes
```

**Database schemas** (migrations already present):

- `pms.connections` — org-level PMS link + `secret_ref`
- `pms.properties` — canonical properties (`external_id` from PMS)
- `pms.room_types` — canonical room types per property
- `inventory.inventory_days` — updated by `IngestAvailability`

---

## PMS API mapping

Reference: [PMS_API_REFERENCE.md §1](./PMS_API_REFERENCE.md#1-webhook--booking-engine-apis).

| §    | PMS endpoint                          | HTTP                         | Client method               | Channel Manager use               |
| ---- | ------------------------------------- | ---------------------------- | --------------------------- | --------------------------------- |
| 1.1  | `/api/webhooks/bookings`              | `GET`                        | `Client.OrgHealth`          | `OrgHealth` RPC                   |
| 1.2  | `/api/webhooks/bookings`              | `POST` `search_properties`   | `Client.SearchProperties`   | **`SyncCatalog`** (property list) |
| 1.3  | `/api/webhooks/bookings/{propertyId}` | `GET`                        | `Client.PropertyHealth`     | `PropertyHealth` RPC              |
| 1.4  | `…/{propertyId}`                      | `POST` `search_availability` | `Client.SearchAvailability` | **`IngestAvailability`**          |
| 1.5  | `…/{propertyId}`                      | `POST` `get_room_details`    | `Client.GetRoomDetails`     | **`SyncCatalog`** (room types)    |
| 1.6  | `…/{propertyId}`                      | `POST` `get_quote`           | `Client.GetQuote`           | `GetQuote` RPC                    |
| 1.7  | `…/{propertyId}`                      | `POST` `create_booking`      | `Client.CreateBooking`      | `CreateBooking` RPC               |
| 1.8  | `…/{propertyId}`                      | `POST` `get_booking`         | `Client.GetBooking`         | `GetBooking` RPC                  |
| 1.9  | `…/{propertyId}`                      | `POST` `update_booking`      | `Client.UpdateBooking`      | `UpdateBooking` RPC               |
| 1.10 | `…/{propertyId}`                      | `POST` `cancel_booking`      | `Client.CancelBooking`      | `CancelBooking` RPC               |

All `POST` bodies include `"action": "<action_name>"` as documented in the PMS reference.

### Credential keys (`ConnectPms`)

Stored in the secret resolver (not in Postgres):

| Key            | Required | Description                                               |
| -------------- | -------- | --------------------------------------------------------- |
| `base_url`     | Yes      | PMS base URL, e.g. `http://pms.local` (no trailing slash) |
| `bearer_token` | Yes      | Bearer token for `Authorization` header                   |

Aliases accepted: `baseUrl`, `bearerToken`, `token`.

---

## Connect-RPC surface

Base URL: `http://localhost:8080` (default API port).

All procedures require authentication (JWT in `Authorization: Bearer …` or `access_token` cookie from WorkOS login).

| Procedure                               | Purpose                                                  |
| --------------------------------------- | -------------------------------------------------------- |
| `/pms.v1.PmsService/ListConnections`    | List org PMS connections                                 |
| `/pms.v1.PmsService/ConnectPms`         | Register MyPMS + credentials                             |
| `/pms.v1.PmsService/DisconnectPms`      | Disable a connection                                     |
| `/pms.v1.PmsService/ListProperties`     | List synced properties (`pms_id` = connection id filter) |
| `/pms.v1.PmsService/GetProperty`        | Property + room types                                    |
| `/pms.v1.PmsService/ListRoomTypes`      | Room types for a property                                |
| `/pms.v1.PmsService/SyncCatalog`        | **Ingest** properties + room types from PMS              |
| `/pms.v1.PmsService/IngestAvailability` | **Ingest** availability → inventory                      |
| `/pms.v1.PmsService/OrgHealth`          | PMS org health check                                     |
| `/pms.v1.PmsService/PropertyHealth`     | PMS property health check                                |
| `/pms.v1.PmsService/GetQuote`           | Price quote from PMS                                     |
| `/pms.v1.PmsService/CreateBooking`      | Create booking in PMS                                    |
| `/pms.v1.PmsService/GetBooking`         | Fetch booking from PMS                                   |
| `/pms.v1.PmsService/UpdateBooking`      | Update booking in PMS                                    |
| `/pms.v1.PmsService/CancelBooking`      | Cancel booking in PMS                                    |

`PmsKind` enum includes `PMS_KIND_MYPMS = 5`.

---

## Data flow

### 1. Connect PMS

1. Client calls `ConnectPms` with `kind: PMS_KIND_MYPMS`, label, and credentials.
2. Credentials are stored in the secret resolver; `secret_ref` is saved on `pms.connections`.
3. Row is inserted under the current org (RLS).

### 2. Sync catalog

1. `SyncCatalog(connection_id)` resolves credentials → builds `mypms.Client`.
2. Calls PMS `search_properties` (optional city/country/name filters).
3. For each property: upsert `pms.properties`.
4. For each property: calls `get_room_details` → upsert `pms.room_types`.
5. Updates `connections.last_sync_at`.

### 3. Ingest availability

1. `IngestAvailability(property_id, date range, guests)` loads property + connection.
2. Calls PMS `search_availability` for the property’s `external_id`.
3. Maps PMS `room_type_id` → internal `pms.room_types.id` (requires prior **SyncCatalog**).
4. Builds per-night `inventory_days` rows (one row per room type per night in range).
5. Calls `InventoryService.BulkUpsertInventory` (Redis idempotency + in-proc events).

### 4. Booking operations

Quote and booking RPCs resolve the property’s PMS `external_id` and proxy the corresponding `action` to the PMS. Responses are mapped to `PmsBooking` proto messages.

---

## Configuration

### Environment (`.env`)

```dotenv
# Optional default; per-connection base_url is set in ConnectPms credentials
PMS_BASE_URL=http://localhost:3000

# Standard API / DB / auth (see .env.example)
DB_HOST=localhost
APP_DB_USER=app
APP_DB_PASSWORD=app_dev
WORKOS_API_KEY=...
WORKOS_CLIENT_ID=...
REDIS_ADDR=localhost:6379
```

On the **PMS side**, webhook auth uses `PMS_WEBHOOK_SECRET` as a JSON map of org → token (see PMS reference). The channel manager stores the matching bearer token per connection via `ConnectPms`.

### Codegen

After changing `proto/pms/v1/pms.proto`:

```bash
# Install plugins once
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest

make proto
```

Regenerate sqlc for PMS queries:

```bash
cd services/pms && sqlc generate
```

---

## Prerequisites

1. **Go 1.22+**, Docker, Postgres, Redis (see root [README.md](../README.md)).
2. **Migrations applied:** `make migrate-up`
3. **PMS running** and reachable at `base_url` with a valid bearer token.
4. **API running:** `make api`
5. **Authenticated session** for Connect-RPC (WorkOS login or Bearer JWT).

---

## Testing

### 1. Unit tests (MyPMS HTTP client)

Runs mocked PMS server tests — no live PMS required.

```bash
cd services/pms
go test ./adapters/mypms/... -v
```

**Covers:**

- `TestClient_OrgHealth` — `GET /api/webhooks/bookings`, Bearer header
- `TestClient_SearchProperties` — `POST` with `action: search_properties`
- `TestClient_SearchAvailability` — property-scoped availability POST

**Expected:** all tests `PASS`.

Run the full PMS module:

```bash
cd services/pms
go test ./...
```

### 2. Build verification

```bash
cd apps/api
go build -o bin/api .
```

**Expected:** compiles without errors.

### 3. Integration test — end-to-end flow

#### Step A: Start infrastructure

```bash
make docker-up
make migrate-up
make api
```

#### Step B: Authenticate

Log in via dashboard (`make dev` → http://localhost:3000/login) **or** use a WorkOS JWT:

```bash
export TOKEN="<your-access-token>"
export API=http://localhost:8080
```

Verify session:

```bash
curl -s -b "access_token=$TOKEN" "$API/me" | jq .
```

#### Step C: Connect MyPMS

```bash
curl -s -X POST "$API/pms.v1.PmsService/ConnectPms" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "kind": "PMS_KIND_MYPMS",
    "label": "Local MyPMS",
    "credentials": {
      "base_url": "http://localhost:3000",
      "bearer_token": "YOUR_PMS_WEBHOOK_TOKEN"
    }
  }' | jq .
```

Save `connection.id` from the response as `CONNECTION_ID`.

#### Step D: Org health (optional sanity check)

```bash
curl -s -X POST "$API/pms.v1.PmsService/OrgHealth" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"connectionId\": \"$CONNECTION_ID\"}" | jq .
```

**Expected:** `status`, `organizationId`, `availableActions` from your PMS.

#### Step E: Sync catalog

```bash
curl -s -X POST "$API/pms.v1.PmsService/SyncCatalog" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"connectionId\": \"$CONNECTION_ID\"
  }" | jq .
```

**Expected:**

```json
{
  "propertiesSynced": 1,
  "roomTypesSynced": 3
}
```

(Counts depend on your PMS data.)

List properties:

```bash
curl -s -X POST "$API/pms.v1.PmsService/ListProperties" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"pmsId\": \"$CONNECTION_ID\"}" | jq .
```

Save a property `id` as `PROPERTY_ID`.

#### Step F: Ingest availability

```bash
curl -s -X POST "$API/pms.v1.PmsService/IngestAvailability" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"propertyId\": \"$PROPERTY_ID\",
    \"checkin\": {\"year\": 2026, \"month\": 6, \"day\": 1},
    \"checkout\": {\"year\": 2026, \"month\": 6, \"day\": 5},
    \"adults\": 2,
    \"rooms\": 1
  }" | jq .
```

**Expected:**

```json
{
  "inventoryRowsAffected": 12,
  "eventId": "<uuid>"
}
```

Verify inventory (use a `roomTypeId` from `GetProperty`):

```bash
curl -s -X POST "$API/inventory.v1.InventoryService/GetInventory" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"propertyId\": \"$PROPERTY_ID\",
    \"roomTypeId\": \"<ROOM_TYPE_UUID>\",
    \"range\": {
      \"start\": {\"year\": 2026, \"month\": 6, \"day\": 1},
      \"end\": {\"year\": 2026, \"month\": 6, \"day\": 5}
    }
  }" | jq .
```

#### Step G: Booking passthrough (optional)

**Get quote:**

```bash
curl -s -X POST "$API/pms.v1.PmsService/GetQuote" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"propertyId\": \"$PROPERTY_ID\",
    \"roomId\": \"<PMS_ROOM_ID>\",
    \"checkin\": {\"year\": 2026, \"month\": 6, \"day\": 10},
    \"checkout\": {\"year\": 2026, \"month\": 6, \"day\": 12},
    \"adults\": 2
  }" | jq .
```

**Create booking:**

```bash
curl -s -X POST "$API/pms.v1.PmsService/CreateBooking" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"propertyId\": \"$PROPERTY_ID\",
    \"roomId\": \"<PMS_ROOM_ID>\",
    \"checkin\": {\"year\": 2026, \"month\": 6, \"day\": 10},
    \"checkout\": {\"year\": 2026, \"month\": 6, \"day\": 12},
    \"guestName\": \"Test Guest\",
    \"email\": \"guest@example.com\",
    \"adults\": 2
  }" | jq .
```

**Get / update / cancel** — use `GetBooking`, `UpdateBooking`, `CancelBooking` with `bookingId` from the create response.

### 4. Database verification

Connect as the app role and set tenant context (or query via API only):

```sql
-- Properties synced for your org
SELECT id, external_id, name, connection_id
  FROM pms.properties;

-- Room types
SELECT id, property_id, code, name, external_id
  FROM pms.room_types;

-- Inventory after ingestion
SELECT room_type_id, stay_date, available, stop_sell
  FROM inventory.inventory_days
 ORDER BY stay_date;
```

### 5. Troubleshooting

| Symptom                                         | Likely cause                                    | Fix                                               |
| ----------------------------------------------- | ----------------------------------------------- | ------------------------------------------------- |
| `401 unauthorized` on RPC                       | Missing/invalid JWT                             | Log in again; pass `Authorization: Bearer`        |
| `organization not registered`                   | WorkOS org not mirrored                         | Complete SSO + webhook sync                       |
| `unsupported provider`                          | Wrong `kind` on connect                         | Use `PMS_KIND_MYPMS`                              |
| `credentials require base_url and bearer_token` | Incomplete `ConnectPms` body                    | Include both credential keys                      |
| PMS HTTP 401/403                                | Wrong bearer token                              | Match PMS `PMS_WEBHOOK_SECRET` for org            |
| `unknown room type from PMS` on ingest          | Catalog not synced                              | Run `SyncCatalog` first                           |
| `inventory writer not configured`               | API wiring issue                                | Ensure `apps/api` includes PMS + inventory writer |
| Zero `inventoryRowsAffected`                    | No matching room types / empty PMS availability | Check PMS data and date range                     |

### 6. Test checklist

- [ ] `go test ./services/pms/adapters/mypms/...` passes
- [ ] `go build ./apps/api` succeeds
- [ ] `ConnectPms` returns connection with `id`
- [ ] `OrgHealth` returns PMS status
- [ ] `SyncCatalog` reports `propertiesSynced` > 0
- [ ] `ListProperties` / `GetProperty` show synced data
- [ ] `IngestAvailability` reports `inventoryRowsAffected` > 0
- [ ] `GetInventory` returns days for synced room type
- [ ] (Optional) `CreateBooking` + `GetBooking` round-trip works

---

## Known limitations

1. **In-memory secrets** — `InMemorySecretResolver` is for local dev only; production needs Vault/AWS Secrets Manager (same pattern as `services/channel/adapters/infra`).
2. **Availability granularity** — `search_availability` returns stay-level offers; ingestion expands the same availability across each night in `[checkin, checkout)`. Refine if your PMS returns per-night rows.
3. **Transactional outbox** — Inventory events use the in-proc bus after commit, not `ops.outbox` (see platform roadmap).
4. **Other PMS vendors** — Cloudbeds/Mews/CSV adapters remain stubs; only `mypms` implements `BookingEngineClient`.
5. **No dashboard UI yet** — PMS flows are API-only; connectors UI is for OTAs, not PMS.

---

## Future work

- [ ] Dashboard: Connect PMS, Sync catalog, Ingest availability buttons
- [ ] Background worker: scheduled `SyncCatalog` + `IngestAvailability` (Asynq)
- [ ] Production secret backend + credential rotation
- [ ] Transactional outbox for inventory events after PMS ingest
- [ ] Map PMS reservations into `reservations` schema on `create_booking` / webhooks
- [ ] Per-night availability when PMS API supports it

---

## Related documents

- [PMS_API_REFERENCE.md](./PMS_API_REFERENCE.md) — Full PMS API (§1–§12)
- [README.md](../README.md) — Monorepo setup, auth, migrations
- [workos-sso-setup.md](./workos-sso-setup.md) — Authentication for API testing

# PMS Outbound Integration — Implementation Guide

This document describes **outbound APIs** on Channel Manager that your PMS (MyPMS) calls to read canonical inventory, OTA/channel reservations, and channel status, and to trigger sync actions. It complements [PMS_INBOUND_INGESTION.md](./PMS_INBOUND_INGESTION.md) (CM → PMS).

---

## Overview

| Concern              | Implementation                                           |
| -------------------- | -------------------------------------------------------- |
| **Auth**             | `Authorization: Bearer <api_key>` (org-scoped)           |
| **Phase A keys**     | `CM_INTEGRATION_SECRETS` env JSON map                    |
| **Phase B keys**     | `tenancy.integration_api_keys` + admin HTTP routes       |
| **API style**        | REST JSON, action-based `POST` (mirrors PMS §1 webhooks) |
| **Tenant isolation** | API key → `TenantContext` → Postgres RLS                 |

```text
MyPMS  ──Bearer API key──▶  Channel Manager /api/integrations/pms
                              ├── inventory (canonical)
                              ├── reservations (OTA-ingested)
                              ├── channel status / sync jobs
                              └── trigger: fetch OTA reservations, push availability
```

---

## Authentication

### Do you need API keys?

**Yes**, for machine-to-machine calls from PMS to Channel Manager.

| Mechanism                               | Use                                 |
| --------------------------------------- | ----------------------------------- |
| WorkOS JWT                              | Dashboard users only                |
| PMS bearer (`ConnectPms` credentials)   | Channel Manager → PMS inbound       |
| OTA credentials (`channel.connections`) | Channel Manager → Booking.com, etc. |
| **Integration API keys**                | **PMS → Channel Manager outbound**  |

### Phase A — Environment secrets (dev/staging)

Set in `.env`:

```dotenv
CM_INTEGRATION_SECRETS='{"<your-local-org-uuid>":"dev-pms-integration-token"}'
```

Use the **local org UUID** from `tenancy.organizations` (after WorkOS login / webhook sync), not the WorkOS org id.

### Phase B — Generated keys (production)

1. Log in to the dashboard and obtain a WorkOS JWT (or use cookie session).
2. As **owner** or **admin**, create a key:

```bash
curl -s -X POST http://localhost:8080/admin/integration-keys \
  -H "Authorization: Bearer $WORKOS_JWT" \
  -H "Content-Type: application/json" \
  -d '{"name":"MyPMS Production"}' | jq .
```

3. Save `secret_key` from the response — it is shown **once** (`cm_live_<prefix>_<secret>`).
4. Configure MyPMS with that value as `channel_manager_api_key`.

**Admin routes:**

| Method   | Path                           | Description           |
| -------- | ------------------------------ | --------------------- |
| `GET`    | `/admin/integration-keys`      | List keys (no secret) |
| `POST`   | `/admin/integration-keys`      | Create key            |
| `DELETE` | `/admin/integration-keys/{id}` | Revoke key            |

---

## API endpoints

Base URL: `http://localhost:8080` (default).

All integration routes require: `Authorization: Bearer <api_key>`.

### Org health

`GET /api/integrations/pms`

**Response:**

```json
{
  "status": "ok",
  "service": "channel-manager-integration",
  "organization_id": "<org-uuid>",
  "available_actions": [
    "list_channels",
    "get_inventory",
    "get_rates",
    "list_reservations",
    "fetch_channel_reservations",
    "push_availability",
    "push_rates",
    "get_sync_jobs"
  ]
}
```

### Property health

`GET /api/integrations/pms/{propertyId}`

`propertyId` = internal `pms.properties.id` from inbound `SyncCatalog`.

### Property actions

`POST /api/integrations/pms/{propertyId}`

**Request body:** JSON with `"action": "<action_name>"` plus action-specific fields.

| Action                       | Required fields                                  | Description                                        |
| ---------------------------- | ------------------------------------------------ | -------------------------------------------------- |
| `list_channels`              | —                                                | OTA channels linked to property                    |
| `get_inventory`              | `checkin`, `checkout`, `room_type_id`            | Canonical `inventory_days` (YYYY-MM-DD)            |
| `get_rates`                  | —                                                | Returns empty array until pricing service is wired |
| `list_reservations`          | —                                                | Reservations stored in CM (incl. OTA fetch)        |
| `fetch_channel_reservations` | optional `since` (YYYY-MM-DD), `idempotency_key` | Pull from OTAs → persist                           |
| `push_availability`          | `checkin`, `checkout`, `room_type_id`            | Push inventory to all active channels              |
| `push_rates`                 | —                                                | No-op until pricing wired                          |
| `get_sync_jobs`              | optional `limit`                                 | Recent channel sync jobs                           |

**Example — get inventory:**

```bash
curl -s -X POST "http://localhost:8080/api/integrations/pms/$PROPERTY_ID" \
  -H "Authorization: Bearer $CM_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "action": "get_inventory",
    "room_type_id": "<room-type-uuid>",
    "checkin": "2026-06-01",
    "checkout": "2026-06-05"
  }' | jq .
```

**Example — fetch OTA reservations:**

```bash
curl -s -X POST "http://localhost:8080/api/integrations/pms/$PROPERTY_ID" \
  -H "Authorization: Bearer $CM_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "action": "fetch_channel_reservations",
    "since": "2026-05-01",
    "idempotency_key": "fetch-2026-05-17"
  }' | jq .
```

**Example — push availability to channels:**

```bash
curl -s -X POST "http://localhost:8080/api/integrations/pms/$PROPERTY_ID" \
  -H "Authorization: Bearer $CM_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "action": "push_availability",
    "room_type_id": "<room-type-uuid>",
    "checkin": "2026-06-01",
    "checkout": "2026-06-05"
  }' | jq .
```

---

## Repository layout

```text
platform/integration/          # Bearer auth, env secrets, DB keystore
services/integration/
  domain/actions.go              # Action name constants
  usecases/service.go            # Orchestration
  adapters/http/
    handler.go                   # PMS REST routes
    admin_keys.go                # Key management (WorkOS JWT)
services/reservations/           # Postgres repo + IngestReservation
migrations/tenancy/0005_*        # integration_api_keys table
```

---

## Testing

### 1. Unit tests

```bash
cd platform/integration && go test ./...
```

### 2. Build

```bash
cd apps/api && go build -o bin/api .
```

### 3. Migrations

```bash
make migrate-up
```

Applies `tenancy.integration_api_keys` (migration `0005`).

### 4. End-to-end checklist

- [ ] Set `CM_INTEGRATION_SECRETS` with valid org UUID
- [ ] `make api` running; PMS inbound sync completed (`SyncCatalog`, channels connected)
- [ ] `GET /api/integrations/pms` returns `200` with Bearer token
- [ ] `POST` `list_channels` returns channel rows
- [ ] `POST` `get_inventory` returns inventory days
- [ ] `POST` `list_reservations` returns array (may be empty)
- [ ] `POST` `fetch_channel_reservations` returns `ingested` count (OTA adapters may report not implemented)
- [ ] `POST /admin/integration-keys` with owner JWT creates `cm_live_…` key
- [ ] Revoked / wrong token returns `401`

### 5. Configure MyPMS

Store on the PMS side (symmetric to inbound):

```json
{
  "channel_manager_base_url": "http://localhost:8080",
  "channel_manager_api_key": "<from CM_INTEGRATION_SECRETS or generated key>"
}
```

---

## Known limitations

1. **OTA adapters** — `fetch_channel_reservations` and `push_availability` call stub adapters; responses include `fetch_errors` / `errors` until real OTA HTTP is implemented.
2. **Pricing** — `get_rates` and `push_rates` are placeholders.
3. **Idempotency** — Reservation ingest uses in-memory idempotency in dev; use Redis in production.
4. **Guest names** — Listed reservations may not include guest name until guest join is added to list query.

---

## Related documents

- [PMS_INBOUND_INGESTION.md](./PMS_INBOUND_INGESTION.md)
- [PMS_API_REFERENCE.md](./PMS_API_REFERENCE.md)
- [README.md](../README.md)

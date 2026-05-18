# Channel Connectors — Plug-and-Play OTA Integration Guide

> Companion to [`# Channel Manager Platform — Engineering.md`](./#%20Channel%20Manager%20Platform%20—%20Engineering.md) §8.
> Authoritative source for **how a new OTA is onboarded, contracted, tested, and shipped** on this platform.

---

## 1. Purpose

Every OTA — Airbnb, Booking.com, Expedia, Agoda, Priceline, and whatever ships next — must plug into the
same **`ChannelAdapter`** contract. The platform never branches on `if channel == "bookingcom"`. The
connector framework guarantees:

- **One contract.** A new OTA is a new package under `services/channel/adapters/<ota>/` and a registry entry.
- **Capability negotiation.** Adapters advertise what they can do (`push_availability`, `push_rates`,
  `fetch_reservations`, `push_reservations`); the sync engine never calls something the adapter does not declare.
- **Tenant-safe by construction.** All persisted state lives in `channel.*` tables under Postgres RLS keyed
  on `app.current_org_id`. Credentials are stored as a `secret_ref` only — never plaintext.
- **Idempotent, outbox-driven writes.** Outbound work is dispatched via `channel.sync_jobs` with an
  `event_id` reference; retries are exactly-once at the consumer.
- **Shadow-first rollout.** Every new tenant — and every brand-new connector — runs in shadow mode for at
  least 24 hours before any real write hits an OTA.

---

## 2. Where Connectors Live

```
services/channel/
├── domain/        # value objects: AvailabilityUpdate, RateUpdate, FetchedReservation, ChannelCapability
├── ports/         # the contract — ChannelAdapter + AvailabilityPusher + RatePusher + ReservationFetcher
├── usecases/      # ChannelService — registry + orchestration
└── adapters/
    ├── airbnb/
    ├── bookingcom/
    ├── expedia/
    ├── agoda/
    └── priceline/        # ← new OTA goes here
```

Cross-references:

| Concern                 | File                                                                              |
| ----------------------- | --------------------------------------------------------------------------------- |
| Adapter contract        | [`services/channel/ports/ports.go`](../services/channel/ports/ports.go)           |
| Domain value objects    | [`services/channel/domain/models.go`](../services/channel/domain/models.go)       |
| Registry / orchestrator | [`services/channel/usecases/service.go`](../services/channel/usecases/service.go) |
| Protobuf API            | [`proto/channel/v1/channel.proto`](../proto/channel/v1/channel.proto)             |
| DB schema               | [`migrations/channel/0001_init.up.sql`](../migrations/channel/0001_init.up.sql)   |

---

## 3. The `ChannelAdapter` Contract

The contract uses **interface segregation** — adapters implement only the roles they support.

```go
// services/channel/ports/ports.go

type ChannelAdapter interface {
    ChannelID() string                          // stable id: "airbnb", "bookingcom", "expedia", ...
    Capabilities() []domain.ChannelCapability   // declared role set
}

type AvailabilityPusher interface {
    PushAvailability(ctx context.Context, updates []domain.AvailabilityUpdate) error
}

type RatePusher interface {
    PushRates(ctx context.Context, updates []domain.RateUpdate) error
}

type ReservationFetcher interface {
    FetchReservations(ctx context.Context, propertyID string, since time.Time) ([]domain.FetchedReservation, error)
}
```

### 3.1 Capabilities (the negotiation surface)

```go
// services/channel/domain/models.go
const (
    CapabilityPushAvailability  ChannelCapability = "push_availability"
    CapabilityPushRates         ChannelCapability = "push_rates"
    CapabilityFetchReservations ChannelCapability = "fetch_reservations"
    CapabilityPushReservations  ChannelCapability = "push_reservations"
)
```

The orchestrator inspects `Capabilities()` before dispatching a job. An adapter that does not declare a
capability **must not** implement the corresponding role interface — `go-arch-lint` enforces this in CI.

### 3.2 Channel kinds (the wire identity)

```protobuf
// proto/channel/v1/channel.proto
enum ChannelKind {
  CHANNEL_KIND_UNSPECIFIED  = 0;
  CHANNEL_KIND_BOOKING_COM  = 1;
  CHANNEL_KIND_AIRBNB       = 2;
  CHANNEL_KIND_EXPEDIA      = 3;
  CHANNEL_KIND_AGODA        = 4;
  CHANNEL_KIND_DIRECT       = 5;
  // Add new values at the end; never renumber.
}
```

The Go `ChannelID()` string and the proto `ChannelKind` enum are paired one-to-one and both must change in
the same PR when a new OTA is added.

---

## 4. Supported & Planned OTAs

| OTA         | `ChannelID()` | Proto enum                 | Push avail. | Push rates | Fetch res. | Status         |
| ----------- | ------------- | -------------------------- | :---------: | :--------: | :--------: | -------------- |
| Airbnb      | `airbnb`      | `CHANNEL_KIND_AIRBNB`      |     ✅      |     ✅     |     ✅     | 🟡 Skeleton    |
| Booking.com | `bookingcom`  | `CHANNEL_KIND_BOOKING_COM` |     ✅      |     ✅     |     ✅     | 🟡 Skeleton    |
| Expedia     | `expedia`     | `CHANNEL_KIND_EXPEDIA`     |     ✅      |     ✅     |     ✅     | 🟡 Skeleton    |
| Agoda       | `agoda`       | `CHANNEL_KIND_AGODA`       |     ✅      |     ✅     |     ✅     | 🟡 Skeleton    |
| Priceline   | `priceline`   | _to be added_              |     ✅      |     ✅     |     ✅     | 🔲 Not started |
| Priceline   | `direct`      | `CHANNEL_KIND_DIRECT`      |     ❎      |     ❎     |     ❎     | 🔲 Not started |

> **Legend.** ✅ Done · 🟡 Stub committed, no real network calls · 🔲 Not started.

---

## 5. Phase-Wise Onboarding Plan

Every new OTA traverses **eight gated phases**. A phase cannot start until the previous one has merged and
been signed off by both engineering and the partner-success owner. Each phase is a separate PR.

### Phase 0 — Discovery & Contract Sign-off

Owner: **PM + Solutions Eng** · Output: a one-pager in `docs/ota/<ota>.md`.

- Confirm partner API access (sandbox + production credentials).
- Document the **resource model**: how listings, rooms, rate plans, and reservations map between OTA and
  our domain.
- Capture the **rate-limit envelope** (RPS, burst, daily quotas, error backoff guidance).
- Identify **webhook vs polling** boundaries — push from the OTA wherever offered.
- List **required scopes / permissions** for the API key or OAuth flow.
- Decide the `ChannelID()` slug (lowercase, ASCII, no spaces) and reserve the next `ChannelKind` proto enum.

**Exit criteria:** signed partner contract, sandbox credentials in Secrets Manager, slug + enum reserved.

### Phase 1 — Skeleton & Registration

Owner: **Backend** · Output: `services/channel/adapters/<ota>/adapter.go` + registry wiring.

```go
// services/channel/adapters/<ota>/adapter.go
package <ota>

type Adapter struct {
    http   *http.Client      // OTA HTTP client (or SDK)
    secret SecretResolver    // resolves channel.connections.secret_ref → live creds
    log    *slog.Logger
}

func NewAdapter(deps Deps) *Adapter { /* ... */ }

func (a *Adapter) ChannelID() string { return "<ota>" }

func (a *Adapter) Capabilities() []domain.ChannelCapability {
    return []domain.ChannelCapability{
        // start empty; add capabilities one phase at a time.
    }
}
```

Then register it once at process start:

```go
// apps/api/main.go
channelSvc.RegisterAdapter(<ota>.NewAdapter(deps))
```

**Exit criteria:** package compiles, `go test ./services/channel/...` passes, adapter appears in
`/internal/admin/connectors` listing.

### Phase 2 — Sandbox Adapter (no real writes)

Owner: **Backend** · Output: working HTTP client wired against the OTA sandbox, **read paths only**.

- Implement the OTA HTTP transport (request signing, OAuth refresh, retry middleware).
- Add **golden-file tests** that record fixtures from the sandbox and replay them in CI.
- Stub all `Push*` methods to return `domain.ErrNotImplemented` while iterating on the read path.

**Exit criteria:** golden tests green; sandbox round-trip recorded for at least the auth handshake and the
`FetchReservations` call.

### Phase 3 — Read Path (`FetchReservations`)

Owner: **Backend + QA** · Add `CapabilityFetchReservations`.

- Implement `FetchReservations(ctx, propertyID, since)` end-to-end.
- Persist results idempotently — use the OTA confirmation id as the natural key, never our internal id.
- Emit `reservation.fetched` events through the in-proc bus.
- Dashboard surfaces the count on the **Connector Health** page.

**Exit criteria:** sandbox reservations land in our DB with no duplicates across two consecutive sync runs.

### Phase 4 — Write Path: Availability

Owner: **Backend** · Add `CapabilityPushAvailability`.

- Implement `PushAvailability(ctx, updates)` against the OTA sandbox.
- Stamp every outbound request with an **idempotency key** = `sha256(orgID|connectionID|propertyID|date|version)`.
- Wire through the **outbox**: writes to `channel.sync_jobs` and the domain mutation share one DB transaction.
- Add **circuit breaker** (`platform/circuit`) keyed on `(orgID, channelID)`.
- Token-bucket the call site to respect the OTA's published rate-limit envelope.

**Exit criteria:** 1 000 synthetic availability updates pushed to sandbox with zero duplicates, p99 < 2 s,
circuit opens correctly under injected 5xx storms.

### Phase 5 — Write Path: Rates

Owner: **Backend** · Add `CapabilityPushRates`.

- Implement `PushRates(ctx, updates)` with the same idempotency / outbox / circuit-breaker discipline as Phase 4.
- Resolve rate plans through the **mapping** service (`channel.connections.config.rate_plan_map`); never
  assume a 1:1 between internal rate plans and OTA rate plans.
- Reject any update whose currency does not match the property's configured currency — surface as a
  `mapping_error` on the connector health page rather than failing the sync silently.

**Exit criteria:** sandbox accepts a full week of rates across at least three rate plans; reconciliation
job diffs internal vs. OTA-reported rates with zero drift.

### Phase 6 — Hardening & Observability

Owner: **Backend + SRE**.

- Add **OpenTelemetry spans** around every outbound call (`http.client`, `channel.push.<capability>`).
- Export Prometheus metrics: `channel_sync_total{channel,capability,outcome}`,
  `channel_sync_duration_seconds`, `channel_circuit_state{channel,org}`.
- Add **structured logs** with `channel_id`, `org_id`, `property_id`, `connection_id`, `idempotency_key`,
  and the OTA's request id (do **not** log credentials or guest PII).
- Wire the dashboard's **Connector Health** view: success rate, last sync, queue depth, breaker state.
- Add a **runbook** at `docs/runbooks/connectors/<ota>.md` covering: common error codes, escalation
  contact at the OTA, how to replay failed `sync_jobs`, how to rotate credentials.

**Exit criteria:** dashboard shows live metrics for the new connector; on-call can resolve a simulated
outage using only the runbook.

### Phase 7 — Shadow Mode → Production

Owner: **Backend + Partner Success**.

1. Enable the connector for a single pilot tenant in **shadow mode** (`channel.connections.status =
'shadow'`). All pushes are computed and logged but never sent. A diff report runs nightly comparing
   what we would have sent vs. what the OTA shows.
2. After 24 h of green diffs, flip to `status = 'active'` for the pilot tenant.
3. Observe for one full week. If `channel_sync_total{outcome="error"}` stays under 0.5 %, mark the
   connector **GA** and remove the feature flag.
4. Document the GA decision in `docs/ota/<ota>.md` with the metrics snapshot.

**Exit criteria:** connector marked GA in the platform-status table at the top of this document.

---

## 6. Credentials & Secret Handling

Credentials never live in Git, env vars, or DB columns. The only thing stored in
`channel.connections.secret_ref` is an opaque pointer (e.g. `aws-secrets://channel/<orgID>/<channelID>`)
that the runtime resolves through `platform/secrets`.

| Concern              | Rule                                                                                            |
| -------------------- | ----------------------------------------------------------------------------------------------- |
| Storage              | AWS Secrets Manager in prod; `platform/secrets/file` for local dev only.                        |
| Rotation             | Connectors must re-resolve the secret on every breaker half-open; never cache beyond 5 minutes. |
| OAuth refresh tokens | Stored alongside the access token under the same `secret_ref`; adapter handles refresh.         |
| Webhook signing      | Each connector exposes a verification helper; the API gateway calls it before queuing jobs.     |
| Audit                | Every secret read emits an `audit.secret_accessed` event with `channel_id`, `org_id`, actor.    |

---

## 7. The Sync Engine (How Adapters Are Called)

```
┌──────────────────────────────────────────────────────────────────────────┐
│                       services/channel/usecases                          │
│                                                                          │
│  RegisterAdapter(a) ──► registry[a.ChannelID()] = a                      │
│                                                                          │
│  Push(orgID, channelID, payload)                                         │
│     1. SET LOCAL app.current_org_id = orgID                              │
│     2. lookup adapter by channelID                                       │
│     3. verify capability is declared                                     │
│     4. acquire circuit-breaker token                                     │
│     5. write sync_jobs row + outbox event (one TX)                       │
│     6. dispatch to AvailabilityPusher / RatePusher                       │
│     7. on success → mark sync_jobs row succeeded, emit channel.synced    │
│     8. on failure → exponential backoff, retry up to N, then dead-letter │
└──────────────────────────────────────────────────────────────────────────┘
```

Adapters **never** open DB connections or touch other services. They only translate domain objects to
OTA wire formats and back. All persistence happens in `usecases/`.

---

## 8. Mapping (Internal ↔ OTA Identifiers)

Each `channel.connections.config` JSONB stores per-tenant mappings:

```jsonc
{
  "property_map": { "<internal_property_id>": "<ota_property_id>" },
  "room_map": { "<internal_room_id>": "<ota_room_id>" },
  "rate_plan_map": { "<internal_rate_plan_id>": "<ota_rate_plan_id>" },
  "currency": "INR",
  "timezone": "Asia/Kolkata",
}
```

Rules:

- Adapters **must** treat missing map entries as a hard error (`domain.ErrMappingMissing`); they must not
  guess.
- Mapping mutations go through the mapping service so they are auditable; adapters are read-only over the
  `config` blob.
- Mapping suggestions can be AI-assisted (embedding similarity over listing titles), but every suggestion
  requires explicit user confirmation before it is written.

---

## 9. Webhooks vs. Polling

Prefer webhooks. The pattern for any OTA that supports them:

1. Each connector registers a public endpoint at
   `POST /webhooks/channels/<channel_id>/{connection_id}`.
2. The handler verifies the OTA signature **before** any DB work, then enqueues a `sync_jobs` row of type
   `reservation_pull` (or whichever event applies).
3. The job worker invokes the same `FetchReservations` code path the polling fallback uses — webhooks are
   a trigger, not a separate data path.

Polling fallback (used when an OTA does not offer webhooks, or as a safety net):

- Default interval: 5 minutes for reservations, 15 minutes for availability reconciliation.
- Interval is per-connection and stored in `channel.connections.config.poll_interval_seconds`.
- The worker holds a Redis lease (`channel:poll:<connection_id>`) so a horizontally-scaled fleet never
  double-polls.

---

## 10. Error Taxonomy

All adapter errors must wrap one of the sentinels in `services/channel/domain/errors.go`:

| Sentinel            | Meaning                                    | Default action                    |
| ------------------- | ------------------------------------------ | --------------------------------- |
| `ErrAuth`           | Credentials invalid / revoked.             | Pause connector, alert on-call.   |
| `ErrRateLimited`    | OTA returned 429 or equivalent.            | Backoff + retry.                  |
| `ErrMappingMissing` | No mapping for the resource.               | Surface on health page, no retry. |
| `ErrValidation`     | OTA rejected payload (e.g. negative rate). | Dead-letter, alert engineering.   |
| `ErrTransient`      | 5xx / network error.                       | Retry with jitter.                |
| `ErrNotImplemented` | Capability not declared.                   | Programmer error → CI catches.    |

The orchestrator branches on the sentinel only. Adapters never decide retry policy themselves.

---

## 11. Testing Matrix

| Layer              | Tool                      | Mandatory for                               |
| ------------------ | ------------------------- | ------------------------------------------- |
| Unit               | `testify`                 | All payload mappers, signature helpers      |
| Golden / fixture   | `go-vcr` or recorded JSON | Every OTA HTTP round-trip                   |
| Contract           | `buf breaking`            | Proto changes — must be backward-compatible |
| Integration (sqlc) | `testcontainers/postgres` | Usecases that touch `channel.*` tables      |
| End-to-end         | Playwright (dashboard)    | Connect-OTA flow + Connector Health page    |
| Load / chaos       | `vegeta` + `toxiproxy`    | Phase 6 sign-off                            |

New connectors **must not** depend on the OTA's live sandbox in CI — record fixtures and replay.

---

## 12. Per-OTA Notes (Implementation Pointers)

These are starting points; the per-OTA one-pagers in `docs/ota/<ota>.md` carry the full detail.

### 12.1 Airbnb

- API: Airbnb Partner API (OAuth 2.0, listings + reservations).
- Webhooks: `reservation.created`, `reservation.cancelled`, `listing.updated`.
- Notes: No native rate-plan concept — single nightly price per listing. Map all internal rate plans onto
  the base price; surcharges go through `pricing_rules.modifiers`.

### 12.2 Booking.com

- API: Booking.com Connectivity API (XML + JSON variants; pick JSON when available).
- Webhooks: Push notifications on reservations and modifications.
- Notes: Requires content certification before production access. Strict on idempotency — duplicate keys
  return 200 OK and are silently ignored, so log the OTA-reported request id for replay debugging.

### 12.3 Expedia

- API: Expedia Partner Solutions (EPS) Rapid + EQC for content.
- Webhooks: Booking notifications via EQC.
- Notes: Distinct endpoints for rates vs. availability. Rate plans are LOS-based — our mapping config must
  capture `los_min`, `los_max` per rate plan.

### 12.4 Agoda

- API: Agoda YCS (Yield Control System).
- Webhooks: Limited; expect to poll for reservations every 5 minutes.
- Notes: Heavy use of long-form XML; budget for a thicker mapper.

### 12.5 Priceline

- API: Priceline Partner Network (PPN).
- Status: not yet started. Phase 0 discovery pending — owner to be assigned.

---

## 13. Adding a New OTA — Checklist

Print this and tick every box before merging the GA PR.

- [ ] Phase 0 one-pager committed at `docs/ota/<ota>.md`.
- [ ] `ChannelKind` enum value added at the end of `proto/channel/v1/channel.proto`; `buf breaking` green.
- [ ] Adapter package created under `services/channel/adapters/<ota>/`.
- [ ] `RegisterAdapter` call added in `apps/api/main.go`.
- [ ] `channel.connections.provider` value documented in this file's table (§4).
- [ ] Capabilities declared honestly — no role interface is implemented without the matching capability.
- [ ] Golden-file tests for every OTA round-trip.
- [ ] Idempotency keys + outbox + circuit breaker wired.
- [ ] OpenTelemetry spans + Prometheus metrics emitted.
- [ ] Runbook at `docs/runbooks/connectors/<ota>.md`.
- [ ] Shadow mode validated for ≥ 24 h on a pilot tenant.
- [ ] Error rate < 0.5 % over one full week post-activation.
- [ ] Status table in §4 updated to ✅.

---

## 14. References

- [`docs/# Channel Manager Platform — Engineering.md`](./#%20Channel%20Manager%20Platform%20—%20Engineering.md) §8 — original `ChannelAdapter` contract description.
- [`services/channel/ports/ports.go`](../services/channel/ports/ports.go) — contract source of truth.
- [`proto/channel/v1/channel.proto`](../proto/channel/v1/channel.proto) — wire contract.
- [`migrations/channel/0001_init.up.sql`](../migrations/channel/0001_init.up.sql) — `connections` + `sync_jobs` schema.

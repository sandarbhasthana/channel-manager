# Channel Manager Platform — Engineering Reference Guide

> _Consolidated reference distilled from the architecture and planning conversation. Treat this as the canonical source for stack choices, conventions, and the do’s and don’ts your team should hold itself to._
> ***Version:*** *2.1 — April 2026*
> ***Audience:*** *Engineering Team, AI coding agents, Technical Reviewers*
> ***Companion document:*** *`Channel_Manager_Implementation_Plan.docx`* *(the narrative blueprint)*

---

## **Build Status** _(last updated: May 2026)_

### Infrastructure & Platform

| Component                                               | Status  | Notes                                                                                                                         |
| ------------------------------------------------------- | ------- | ----------------------------------------------------------------------------------------------------------------------------- |
| Monorepo scaffold (`go.work`, `buf.yaml`, `turbo.json`) | ✅ Done | All service dirs present                                                                                                      |
| Per-service `go.mod` modules                            | ✅ Done | `services/*`, `platform/*`, `apps/api`                                                                                        |
| Postgres connection pool (`platform/db`)                | ✅ Done | pgx v5, RLS-aware runtime role                                                                                                |
| Multi-tenant schema + RLS (`migrations/tenancy`)        | ✅ Done | `tenancy.orgs`, `tenancy.org_members`, RLS policies                                                                           |
| Per-service migrations                                  | ✅ Done | `migrations/{inventory,pricing,reservations,…}`                                                                               |
| In-process event bus (`platform/events/inproc`)         | ✅ Done | Typed Go channels; NATS upgrade path ready                                                                                    |
| Protobuf contracts (`proto/*`)                          | ✅ Done | inventory, pricing, reservations, channel, pms, audit                                                                         |
| Generated Go + TS types (`gen/go`)                      | ✅ Done | `buf generate` wired                                                                                                          |
| Observability (`platform/observability`)                | ✅ Done | `Init()` / `Shutdown()` implemented — OTel TracerProvider + MeterProvider (OTLP/gRPC) + Sentry; wired into `apps/api/main.go` |
| Config loader (`platform/config`)                       | ✅ Done | env-based, typed struct                                                                                                       |

### Auth Platform (`platform/auth`)

| Component                                             | Status       | Notes                                                     |
| ----------------------------------------------------- | ------------ | --------------------------------------------------------- |
| JWT verifier — RS256 + JWKS auto-refresh              | ✅ Done      | 8/8 tests passing                                         |
| WorkOS login redirect (`GET /auth/login`)             | ✅ Done      | SSO + email/password via AuthKit                          |
| OAuth callback + code exchange (`GET /auth/callback`) | ✅ Done      | Sets HttpOnly `access_token` + `refresh_token` cookies    |
| WorkOS webhook receiver (`POST /auth/webhook`)        | ✅ Done      | HMAC-verified; org/user mirroring                         |
| Org + user mirroring into `tenancy` schema            | ✅ Done      | `UpsertOrg` / `UpsertUser`                                |
| HTTP middleware — cookie + bearer fallback            | ✅ Done      | Reads cookie first, then `Authorization` header           |
| Connect-RPC unary interceptor                         | ✅ Done      | JWT verify → tenant resolve → Casbin enforce              |
| Casbin RBAC enforcer (Postgres adapter)               | ✅ Done      | Domain-scoped `g(sub, role, dom)`; wired into interceptor |
| `TenantContext` propagated to all RPC handlers        | ✅ Done      | `context.go`                                              |
| `GET /me` session endpoint                            | ✅ Done      | Returns `user_id`, `org_id`, `role` as JSON               |
| Token refresh endpoint (`POST /auth/refresh`)         | ❌ Not built | Sessions expire without renewal path                      |
| Middleware + interceptor tests                        | ❌ Not built | Only `verifier_test.go` exists                            |

### Inventory Service (`services/inventory`)

| Component                            | Status       | Notes                                                       |
| ------------------------------------ | ------------ | ----------------------------------------------------------- |
| Domain models                        | ✅ Done      | `domain/models.go`                                          |
| Port interfaces                      | ✅ Done      | `ports/ports.go`                                            |
| Use-case service                     | ✅ Done      | 9/9 unit tests passing                                      |
| Postgres repository (sqlc)           | ✅ Done      | `adapters/postgres/repo.go`                                 |
| Redis idempotency store              | ✅ Done      | `adapters/redis/idempotency.go` — 24 h TTL                  |
| In-proc event publisher              | ✅ Done      | `adapters/events/publisher.go`                              |
| Connect-RPC handler                  | ✅ Done      | `adapters/connect/handler.go` — wired in `apps/api/main.go` |
| Integration tests (Postgres + Redis) | ❌ Not built | Tier 3 tests pending                                        |

### Other Services

| Service                                | Status  | Notes                                                                  |
| -------------------------------------- | ------- | ---------------------------------------------------------------------- |
| Pricing (`services/pricing`)           | ⚠️ Stub | Domain + ports + stub usecase only; no Postgres adapter or RPC handler |
| Reservations (`services/reservations`) | ⚠️ Stub | Domain + ports + stub usecase only                                     |
| Mapping (`services/mapping`)           | ⚠️ Stub | Skeleton only                                                          |
| Channel (`services/channel`)           | ⚠️ Stub | Skeleton only; no OTA adapters                                         |
| PMS (`services/pms`)                   | ⚠️ Stub | Skeleton only; no PMS adapters                                         |
| Audit (`services/audit`)               | ⚠️ Stub | Skeleton only                                                          |

### Applications

| App                              | Status       | Notes                                                              |
| -------------------------------- | ------------ | ------------------------------------------------------------------ |
| API server (`apps/api`)          | ✅ Done      | Compiles and runs; inventory RPC live behind auth                  |
| Sync worker (`apps/sync-worker`) | ❌ Not built | Binary dir exists, nothing inside                                  |
| Dashboard (`apps/dashboard`)     | ⚠️ Stub      | Next.js scaffold only — single placeholder `page.tsx`; no login UI |

---

## **1\. Architectural Pillars**

The seven non-negotiables. Every decision in this guide traces back to one of them.

1. **PMS-agnostic core.** Any PMS — including your own — plugs in through one Go interface. The domain never sees a vendor.
2. **Channel-agnostic distribution.** [Booking.com](http://Booking.com), Airbnb, Expedia, Agoda, direct, and future GDS/metasearch all sit behind the same `ChannelAdapter` contract.
3. **Multi-tenant isolation as a database invariant.** Postgres Row-Level Security is the last line of defense; application code is the first.
4. **Event-driven sync with idempotency.** Every write to an OTA carries an idempotency key. Every domain mutation publishes a domain event.
5. **Audit log for anything touching money, inventory, or credentials.** No silent state changes.
6. **Modular monolith with split-ready discipline.** One or two binaries today, extractable to services tomorrow as a deploy change, not a rewrite.
7. **AI-assisted development with human-reviewed gates.** Every AI-authored PR passes the same CI and review bar as any other PR.

---

## **2\. Three Decisions to Unbundle**

These three are constantly conflated. Hold them apart.

| Decision               | Question                 | Choice                 | Rationale                                                           |
| ---------------------- | ------------------------ | ---------------------- | ------------------------------------------------------------------- |
| Repo layout            | One repo or many?        | Monorepo               | Atomic cross-module changes, shared tooling, single source of truth |
| Deployment topology    | One binary or many?      | Modular monolith first | Operational simplicity at MVP; split when triggers appear           |
| Engineering discipline | Prepared to split later? | Yes, from day one      | Per-module Go modules, schema-per-service, protobuf contracts       |

Trigger conditions for splitting a service out (Phase 4):

- One channel saturating the shared worker pool
- One subsystem requiring an independent deploy cadence
- Blast radius from a bad deploy becomes unacceptable
- A team boundary forms around a subsystem

---

## **3\. Backend Architecture**

### **3.1 Language and runtime**

- **Go 1.22+** for all backend services and adapters.
- Static binary deploys, low memory footprint, predictable GC, native concurrency for I/O-bound fan-out.
- Easy for AI agents to generate consistently and for humans to review.

### **3.2 Service topology (current and future)**

```gherkin
[Clients] Dashboard  Booking Engine  Partner APIs
                            |
                    [API Gateway + Auth]
                            |
  +-------------+-----------+-----------+---------------+
  |             |                       |               |
Inventory   Pricing Engine        Reservations      Mapping
Module      (Strategy + AI)         Module          Module
  |             |                       |               |
  +------+------+------+---------+------+-------+-------+
         |             |         |              |
   EventBus (in-proc today; NATS / Kafka after split)
         |             |         |              |
   Channel Sync Engine           PMS Adapter Hub
   (Saga Orchestrator)           (PMS-agnostic)
         |                               |
   Channel Adapters                 PMS Adapters
   Booking.com, Airbnb,            Cloudbeds, Mews,
   Agoda, Expedia, direct          Your PMS, CSV
         |                               |
   [Outbound OTA APIs]            [PMS APIs / Webhooks]
```

### **3.3 Domain modules (each is its own Go module)**

- `services/inventory/` — availability, stop-sell, min/max stay
- `services/pricing/` — base rates, derivations, strategy engine
- `services/reservations/` — booking lifecycle, modifications, cancellations
- `services/mapping/` — internal ↔ external schema translation, version history
- `services/channel/` — `ChannelAdapter` contract + per-OTA adapters
- `services/pms/` — `PmsAdapter` contract + per-PMS adapters
- `services/audit/` — append-only audit log

Each module is structured as:

```gherkin
services/<name>/
  domain/      # pure structs and rules, no external deps
  ports/       # interfaces this module needs from others
  usecases/    # application services / orchestration
  adapters/    # concrete: HTTP handlers, postgres repos, queue
  go.mod
```

###

### **3.4 Platform (cross-cutting) libraries**

- `platform/events/` — `EventBus` interface; `inproc`, `nats`, `kafka` implementations
- `platform/auth/` — JWT verification, tenant context middleware, Casbin enforcer
- `platform/db/` — pgx connection pool, sqlc wiring, migration runner
- `platform/http/` — Chi server scaffolding, middleware chains
- `platform/observability/` — OpenTelemetry SDK, structured logging
- `platform/config/` — env loading, secrets resolution

### **3.5 Data layer**

- **PostgreSQL 16** (Neon for serverless, RDS/Aurora for AWS-native).
- **sqlc + pgx** — type-safe SQL, no ORM magic. Queries are explicit, reviewable, and AI-friendly.
- **Schema-per-service.** Each service owns its schema; cross-schema foreign keys are forbidden by lint.
- **golang-migrate or Atlas** for migrations, one directory per service.
- **Redis** for inventory locks, idempotency keys, rate-limit tokens, session cache.
- **ClickHouse** for revenue analytics (RevPAR, ADR, channel contribution).
- **Meilisearch** for ops search (instant booking lookup).

### **3.6 Event bus and job queue**

- **In-process EventBus** today (typed Go channels under the hood). Same interface upgraded to **NATS JetStream** post-split, **Kafka** when volume demands replay and partitioning.
- **Asynq** (Redis-backed) for background jobs: sync work, scheduled rate updates, retries.
- **Outbox pattern** — DB writes and event publishes are atomic. No lost updates when a queue blips.

### **3.7 Authentication and authorization**

A two-layer model. Each layer solves a different problem.

| Layer            | Purpose                            | Tool                              |
| ---------------- | ---------------------------------- | --------------------------------- |
| Identity         | Who is this user?                  | Clerk or WorkOS (SSO, SAML, SCIM) |
| Tenant isolation | Are they in the right org?         | Postgres Row-Level Security       |
| Authorization    | What can they do on this resource? | Casbin (embedded Go library)      |

- Identity layer issues JWTs; Go middleware verifies and extracts `(userId, orgId, role)`.
- Middleware sets `SET LOCAL app.current_org_id = '...'` on the DB connection. RLS policies enforce tenant scope at the database — bug-resistant by construction.
- Casbin answers “given role and resource, allowed?” with sub-millisecond, in-process decisions. RBAC with domains plus light ABAC.
- Defer managed authz services ([Permit.io](http://Permit.io), Oso Cloud, Auth0 FGA). Re-evaluate at Phase 3 only if policy authoring becomes a customer-success burden.
- Phase 4 upgrade path: **OpenFGA** for relationship-graph permissions (reseller white-label, marketplace), **OPA / Cerbos** for cross-service policy decisions.

### **3.8 Observability**

- **OpenTelemetry (Go SDK)** for traces, metrics, and logs — instrumented at the platform layer so domain code stays clean.
- **Grafana Cloud** as the backend.
- **Sentry** for error tracking with source maps for the frontend.
- Structured logs only (`slog` or `zap`), JSON-formatted, with `trace_id`, `org_id`, `request_id` on every line.
- Alerts on sync failure rates and OTA latency, not just HTTP 5xx.

---

## **4\. Frontend Architecture**

### **4.1 Framework**

- **Next.js 15** (App Router) + **React 19** + **TypeScript 5**.
- Server components by default, client components when interactivity demands.
- Same monorepo, under `apps/dashboard/`.

### **4.2 Component system**

- **Tailwind CSS** for utilities.
- **shadcn/ui** as the component baseline (copy-paste, owned, unforked).
- **Lucide React** for icons.
- **Recharts** or **Tremor** for analytics dashboards.

### **4.3 State management**

- **TanStack Query (React Query)** for server state — caching, refetching, optimistic updates.
- **Zustand** for small slices of client state when needed.
- No Redux. No client-side caches of server data outside TanStack Query.

### **4.4 Type sharing with the backend**

- **Protobuf** is the source of truth.
- **buf** generates both Go types (for services) and TypeScript types (for the dashboard).
- Frontend imports types from `proto/` via a generated package — no hand-written DTOs that duplicate the domain.
- **Connect-RPC** browser client for typed calls into the backend.

### **4.5 Frontend testing**

- **Vitest** for unit tests.
- **Playwright** for end-to-end and visual regression.
- **Storybook** for component development and documentation.
- **MSW** (Mock Service Worker) for component-level integration tests against generated proto types.

---

## **5\. Monorepo Layout**

### **5.1 Directory structure**

```coffeescript
channel-manager/
├── go.work # Go workspace — binds all modules
├── buf.yaml # Protobuf contracts + code generation
├── turbo.json # Turborepo for cross-language tasks
├──Makefile
├── docker-compose.yml # Local stack: postgres, redis, nats
│
├── apps/# DEPLOYABLES — each has a main.go or package.json
│├── api/# Starts as THE binary; later the HTTP gateway
│├── sync-worker/# Background worker (same binary at first)
│└── dashboard/# Next.js + TypeScript UI
│
├── services/# DOMAIN MODULES — each an isolated Go module
│├── inventory/
│├── pricing/
│├── reservations/
│├── mapping/
│├── channel/
││└── adapters/# One adapter per OTA
││├── bookingcom/
││├── airbnb/
││├── expedia/
││└── agoda/
│├── pms/
││└── adapters/
││├── mypms/# Your own PMS — customer zero
││├── cloudbeds/
││├── mews/
││└── csv/
│└── audit/
│
├── platform/# SHARED LIBS — cross-cutting, stable
│├── events/
│├── auth/
│├── db/
│├── http/
│├── observability/
│└── config/
│
├── proto/# CONTRACTS — protobuf schemas
│├── channel/v1/
│├── pricing/v1/
│└── reservation/v1/
│
├── migrations/# SQL per-service schema
└── tools/# Codegen, seed, onboarding CLI
```

### **5.2 Six enforcement disciplines**

Without these, “modular monolith” becomes a mud ball and the split never happens.

1. [**`go.work`**](http://go.work) **+ per-service** **`go.mod`\*\***.\*\* Each `services/<name>/` is a separate Go module.
2. **`internal/`** **packages** for everything not meant for cross-service consumption (compiler-enforced).
3. **`depguard`** **or** **`go-arch-lint`** **in CI.** Build fails if `services/pricing` imports `services/channel`directly.
4. **Protobuf as the cross-service contract.** Anything that might go over the wire someday lives in `proto/`.
5. **Schema-per-service from day one.** Cross-schema foreign keys forbidden by lint. This is the single most important discipline.
6. **Per-service migrations.** `migrations/<service>/` directories own their schema lifecycle independently.

---

## **6\. Libraries — Complete List**

### **6.1 Backend (Go)**

**Web and RPC:**

- [`github.com/go-chi/chi/v5`](http://github.com/go-chi/chi/v5) — HTTP router
- [`connectrpc.com/connect`](http://connectrpc.com/connect) — Connect-RPC (gRPC-compatible, browser-friendly)
- [`google.golang.org/grpc`](http://google.golang.org/grpc) — gRPC for service-to-service later
- [`github.com/grpc-ecosystem/go-grpc-middleware/v2`](http://github.com/grpc-ecosystem/go-grpc-middleware/v2) — auth, logging, recovery interceptors
  **Database:**
- [`github.com/jackc/pgx/v5`](http://github.com/jackc/pgx/v5) — Postgres driver
- [`github.com/sqlc-dev/sqlc`](http://github.com/sqlc-dev/sqlc) — type-safe query generator (binary, used at build time)
- [`github.com/golang-migrate/migrate/v4`](http://github.com/golang-migrate/migrate/v4) — migrations
- [`github.com/redis/go-redis/v9`](http://github.com/redis/go-redis/v9) — Redis client
  **Jobs and events:**
- [`github.com/hibiken/asynq`](http://github.com/hibiken/asynq) — Redis-backed job queue
- [`github.com/nats-io/nats.go`](http://github.com/nats-io/nats.go) — NATS client (post-split)
- [`github.com/segmentio/kafka-go`](http://github.com/segmentio/kafka-go) — Kafka client (at scale)
  **Auth and authz:**
- [`github.com/clerkinc/clerk-sdk-go/v2`](http://github.com/clerkinc/clerk-sdk-go/v2) or [`github.com/workos/workos-go/v4`](http://github.com/workos/workos-go/v4) — identity
- [`github.com/casbin/casbin/v2`](http://github.com/casbin/casbin/v2) — RBAC/ABAC enforcer
- [`github.com/golang-jwt/jwt/v5`](http://github.com/golang-jwt/jwt/v5) — JWT verification
  **Observability:**
- [`go.opentelemetry.io/otel`](http://go.opentelemetry.io/otel) and friends — OpenTelemetry SDK
- [`github.com/getsentry/sentry-go`](http://github.com/getsentry/sentry-go) — error tracking
- `log/slog` (stdlib) — structured logging
  **Resilience:**
- [`github.com/sony/gobreaker`](http://github.com/sony/gobreaker) — circuit breaker
- [`github.com/cenkalti/backoff/v4`](http://github.com/cenkalti/backoff/v4) — exponential backoff
- [`golang.org/x/time/rate`](http://golang.org/x/time/rate) — token bucket rate limiting
  **Validation and utilities:**
- [`github.com/go-playground/validator/v10`](http://github.com/go-playground/validator/v10) — struct validation
- [`github.com/oklog/ulid/v2`](http://github.com/oklog/ulid/v2) — time-sortable IDs
- [`github.com/spf13/viper`](http://github.com/spf13/viper) — config loading
- [`github.com/stretchr/testify`](http://github.com/stretchr/testify) — test assertions
- [`go.uber.org/mock`](http://go.uber.org/mock) or [`github.com/vektra/mockery/v2`](http://github.com/vektra/mockery/v2) — mocks
- [`github.com/testcontainers/testcontainers-go`](http://github.com/testcontainers/testcontainers-go) — real Postgres/Redis in CI
- [`github.com/pact-foundation/pact-go/v2`](http://github.com/pact-foundation/pact-go/v2) — OTA contract tests
  **AI / ML:**
- [`github.com/tmc/langchaingo`](http://github.com/tmc/langchaingo) — LangChain in Go (or run a Python sidecar)
- [`github.com/sashabaranov/go-openai`](http://github.com/sashabaranov/go-openai) — OpenAI client (embeddings)
- [`github.com/anthropics/anthropic-sdk-go`](http://github.com/anthropics/anthropic-sdk-go) — Anthropic Claude client
- [`github.com/pgvector/pgvector-go`](http://github.com/pgvector/pgvector-go) — pgvector for embeddings storage

### **6.2 Frontend (TypeScript / Next.js)**

**Core:**

- `next` (15.x), `react` (19.x), `react-dom`, `typescript` (5.x)
- `tailwindcss`, [`@tailwindcss`](https://github.com/tailwindcss)`/typography`, [`@tailwindcss`](https://github.com/tailwindcss)`/forms`
- `tailwind-merge`, `clsx`
  **UI:**
- `shadcn/ui` (copy-paste, not an npm dep)
- [`@radix`](https://github.com/radix)`-ui/*` (underlies shadcn)
- `lucide-react` — icons
- `recharts` or [`@tremor`](https://github.com/tremor)`/react` — charts
- `react-hook-form` + `zod` — forms and validation
- `sonner` — toasts
- `cmdk` — command palette
- `vaul` — drawers
  **Data:**
- [`@tanstack`](https://github.com/tanstack)`/react-query` — server state
- `zustand` — small client state
- [`@connectrpc`](https://github.com/connectrpc)`/connect-web` — typed RPC client
- [`@bufbuild`](https://github.com/bufbuild)`/protobuf` — protobuf runtime types
  **Testing:**
- `vitest`, [`@testing`](https://github.com/testing)`-library/react`, [`@testing`](https://github.com/testing)`-library/user-event`
- [`@playwright`](https://github.com/playwright)`/test` — E2E
- `msw` — Mock Service Worker
- `storybook` ([`@storybook`](https://github.com/storybook)`/nextjs`)
  **Auth:**
- [`@clerk`](https://github.com/clerk)`/nextjs` or [`@workos`](https://github.com/workos)`-inc/authkit-nextjs`

### **6.3 Infrastructure and tooling**

- **Container:** Docker, multi-stage builds, distroless base for Go binaries
- **Orchestration:** ECS Fargate (AWS) or [Fly.io](http://Fly.io), with Kubernetes only after meaningful service split
- **IaC:** Terraform with the AWS provider; Atmos or Terragrunt only if multi-account
- **CI/CD:** GitHub Actions; **goreleaser** for binary builds; **Trivy** for image scanning
- **Build orchestration:** Turborepo (cross-language tasks); `buf` for proto codegen; `make` for one-liners
- **Secrets:** AWS Secrets Manager or Doppler; never in env vars committed anywhere
- **DNS / CDN:** Cloudflare in front of everything

### **6.4 AI / ML stack**

- **Reasoning model:** Anthropic Claude (recommendations, copilot, anomaly explanations)
- **Embeddings:** OpenAI `text-embedding-3-large` or Voyage AI
- **Vector store:** pgvector inside the same Postgres
- **Orchestration:** LangGraph (Python sidecar via gRPC) or `langchaingo` (in-process)
- **Eval / monitoring:** Langfuse or Helicone

---

## **7\. PMS Integration**

### **7.1 The** **`PmsAdapter`** **contract**

```haskell
type PmsAdapterinterface{
Capabilities()[]PmsCapability// push, pull, webhook, bulk

ListProperties(ctx context.Context)([]Property, error)
ListRoomTypes(ctx context.Context, propertyID string)([]RoomType, error)
GetInventory(ctx context.Context, q InventoryQuery)([]InventoryDay, error)
GetRates(ctx context.Context, q RateQuery)([]RateDay, error)
GetReservations(ctx context.Context, q ReservationQuery)([]Reservation, error)
}

// Optional capability interfaces — Interface Segregation in action
type ReservationPusherinterface{
PushReservation(ctx context.Context, r Reservation)(Ack, error)
}
type InventoryPusherinterface{
PushInventory(ctx context.Context, items []InventoryDay)(Ack, error)
}
type ChangeFeedinterface{
Subscribe(ctx context.Context, h ChangeHandler)(Subscription, error)
}
```

### **7.2 Ingestion modes (in order of preference)**

1. **Webhook-first** — PMS pushes, we ack and persist
2. **Polling fallback** — per-tenant scheduler with ETag / If-Modified-Since
3. **Bulk import** — CSV / Excel / JSON for seeding and long-tail PMSs
4. **Direct DB read-replica** — for chains that grant access; treated as just another adapter
5. **Custom adapter SDK** — public Go module so partners can build their own

### **7.3 Your own PMS — three integration options**

| Option | Mode                          | When                                                     |
| ------ | ----------------------------- | -------------------------------------------------------- |
| A      | HTTP adapter                  | Start here. Loosest coupling, validates the contract     |
| B      | Native gRPC / Connect-RPC     | Once both systems share infra and the API has stabilized |
| C      | Shared event bus (NATS/Kafka) | Once the canonical event schema is proven by production  |

Multi-PMS coexistence comes free: each `Property` row carries a `pmsType`, the `AdapterRegistry` resolves the right adapter per tenant, and tenants on your PMS, Cloudbeds, and Mews all run side by side.

---

## **8\. Channel (OTA) Integration**

### **8.1 The** **`ChannelAdapter`** **contract**

Same Interface Segregation pattern as PMS — small role-based interfaces, channels implement only what they support.

```haskell
type ChannelAdapterinterface{
ChannelID()string
Capabilities()[]ChannelCapability
}

type AvailabilityPusherinterface{
PushAvailability(ctx context.Context, items []InventoryDay)(Ack, error)
}
type RatePusherinterface{
PushRates(ctx context.Context, items []RateDay)(Ack, error)
}
type ReservationFetcherinterface{
FetchReservations(ctx context.Context, q ReservationQuery)([]Reservation, error)
}
```

### **8.2 Sync engine**

- **Saga orchestrator** for multi-channel writes; compensating actions on failure.
- **Idempotency keys** on every outbound write.
- **Inventory locks** in Redis with TTL to prevent double-bookings.
- **Outbox pattern** for DB-and-event atomicity.
- **Per-channel circuit breaker** + token bucket; failures don’t cascade.
- **Exponential backoff with jitter** for retries.
- **Shadow mode** for the first 24 hours of any new tenant — observe, diff, report, but don’t write.

---

## **9\. Onboarding**

### **9.1 Six-step wizard (target: under 30 minutes to first sync)**

1. **Sign up** — email/password or SSO
2. **Property setup** — rooms and rate plans (CSV upload supported)
3. **Connect PMS** — OAuth or API key, autodiscovery of properties
4. **Connect channels** — OAuth to [Booking.com](http://Booking.com) / Airbnb / Expedia
5. **Mapping review** — AI-suggested room/listing mappings, user confirms
6. **Go live** — shadow mode for 24 hours, then full sync

### **9.2 AI-assisted onboarding**

- CSV / Excel autopilot — column-mapping with synonym handling
- PMS autodiscovery — list properties and rooms the credentials see
- Embedding-based mapping suggestions — match OTA listings to internal rooms
- 24-hour shadow mode with a diff report
- In-app copilot for sync-failure triage
- Industry presets (boutique hotel, vacation rental, hostel, serviced apartments)
- Headless onboarding CLI for chains and partners

### **9.3 Onboarding metrics (instrument from day 1)**

- Time-to-first-sync (target median < 30 minutes)
- Onboarding completion rate (signup → “Go live” within 7 days)
- First-week sync success rate
- Support tickets per onboarded property in the first 30 days

---

## **10\. Implementation Phases (summary)**

| Phase                | Weeks | Status         | What ships                                                                                                                                     |
| -------------------- | ----- | -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| Kickoff              | 0     | ✅ Done        | Monorepo, per-service Go modules, Postgres/Redis, migrations, proto contracts, in-proc event bus                                               |
| Auth (Phase 3)       | —     | ✅ Done        | WorkOS JWT + JWKS, Casbin RBAC, HttpOnly cookies, Connect-RPC interceptor, RLS tenant context                                                  |
| Inventory (Phase 4)  | —     | ✅ Done        | Domain → usecase → Postgres/Redis/events → Connect-RPC handler wired in API                                                                    |
| Alpha                | 8     | 🔄 In Progress | One PMS + [Booking.com](http://Booking.com) end-to-end on a test tenant — observability, pricing, channel adapters, dashboard login UI pending |
| MVP / Phase 1 beta   | 13    | ❌ Not started | Onboarding wizard, audit log, dashboard, 3–5 design partners live                                                                              |
| Phase 2 GA           | 22    | ❌ Not started | Airbnb + Expedia + Agoda + pricing engine v1 + second PMS + partner API                                                                        |
| Phase 3 GA           | 32    | ❌ Not started | Revenue intelligence, direct booking engine, competitor scraping, payments                                                                     |
| Phase 4 early access | 44    | ❌ Not started | AI pricing optimizer, offline sync, first service splits, marketplace infra                                                                    |

---

## **11\. AI-Assisted Development Discipline**

What AI does well in this codebase, and what humans must still own.
**AI does well:**

- OTA HTTP client scaffolds
- proto-to-Go and proto-to-TS code generation
- sqlc query drafts from schema
- Test scaffolds (testify, gomock, Playwright)
- Onboarding copy, tooltips, empty states
- Runbook drafts, changelogs, release notes
- Migration scripts, log triage, SQL for analytics
  **Humans still own:**
- Architecture trade-offs (including when to actually split a service)
- OTA contract edge cases — the dragons live in the specifics
- Security and compliance decisions
- Pricing-engine business rules
- Customer conversations, incident command
- Sign-off on any AI-authored change touching money, inventory, or credentials
  **Review discipline:** every AI-authored PR passes the same CI gates and human review. No bypass for AI-authored code. Higher scrutiny on critical paths.

---

## **12\. Do’s and Don’ts (consolidated)**

### **12.1 Architecture**

**Do**

- Design ports and adapters from day one
- Enforce module boundaries with `depguard` or `go-arch-lint` in CI
- Use protobuf as the source of truth for cross-service contracts
- Isolate each domain module in its own Go module with its own schema
- Implement Postgres RLS for tenant isolation
- Keep an audit log for everything that touches money, inventory, or credentials
- Use idempotency keys on all writes to channels
- Use the outbox pattern for atomic DB + event writes
- Use circuit breakers for outbound calls
- Shadow-mode every new tenant for the first 24 hours
  **Don’t**
- Don’t conflate monorepo with microservices (they are independent decisions)
- Don’t split into microservices on day one
- Don’t share databases across services (or across your own products)
- Don’t bypass the adapter layer, even for your own PMS
- Don’t add foreign keys across service schemas
- Don’t poll when webhooks are available
- Don’t make rate or inventory changes without idempotency
- Don’t store raw OTA credentials in Git, env vars, or DB tables in plaintext

### **12.2 Backend / Go**

**Do**

- Use small, role-based interfaces (Interface Segregation via Go’s structural typing)
- Keep domain logic in pure structs with no framework imports
- Use sqlc for type-safe SQL — no ORM magic
- Pass `context.Context` through every I/O call
- Wire dependencies in `main.go` (the only place that knows concrete types)
- Use `internal/` for non-exported packages
- Use [`errors.Is`](http://errors.Is) / [`errors.As`](http://errors.As) for error inspection
- Prefer the standard library where it’s reasonable
  **Don’t**
- Don’t use ORMs that hide SQL (e.g., GORM) — they obscure performance and reviewability
- Don’t define fat interfaces (one `PmsAdapter` with 30 methods); segregate
- Don’t put config-dependent work in `init()`
- Don’t ignore errors
- Don’t share state between goroutines without channels or sync primitives
- Don’t use global state for anything that depends on tenant context

### **12.3 Database**

**Do**

- Schema-per-service, even within a single Postgres instance
- Use Row-Level Security on every tenant-scoped table
- Version migrations per service
- Use JSONB for flexible OTA payloads — preserve unknown fields, never discard
- Use ULIDs (time-sortable, distributed-safe) instead of integer IDs
- Use connection pooling (pgx native pool or PgBouncer at scale)
  **Don’t**
- Don’t add foreign keys across service schemas
- Don’t put business logic in Postgres triggers (visibility tax)
- Don’t share a single migration directory across services
- Don’t store secrets in DB tables (use Secrets Manager)
- Don’t use auto-incrementing integer IDs in distributed contexts

### **12.4 Authentication and authorization**

**Do**

- Use Clerk or WorkOS for identity (don’t build it)
- Use Casbin for authorization decisions
- Enforce Postgres RLS as a backstop, regardless of application authz
- Use typed permission constants (`ChannelWrite`, `RateEdit`, etc.) — grep-able and refactor-safe
- Scope every API call by tenant context from the JWT
- Treat policy as code: version-controlled, reviewed, tested
  **Don’t**
- Don’t roll your own RBAC engine from scratch
- Don’t conflate identity (who) with authorization (what they can do)
- Don’t skip RLS because “we have application authz” — both are required
- Don’t put role logic scattered through application code; centralize in policy
- Don’t adopt a managed authz service ([Permit.io](http://Permit.io), Oso Cloud, Auth0 FGA) before Phase 3

### **12.5 Channel adapters (OTAs)**

**Do**

- Implement `ChannelAdapter` plus only the capability interfaces an OTA actually supports
- Use Pact contract tests for every OTA in CI
- Wrap every outbound call with a circuit breaker and a token bucket
- Persist raw OTA responses in JSONB for debugging
- Version your adapter against OTA API versions
- Make every write idempotent
- Use exponential backoff with jitter for retries
  **Don’t**
- Don’t share state between channel adapters
- Don’t let one OTA’s failure cascade to others
- Don’t tightly couple to OTA-specific schema in domain logic — translate at the adapter
- Don’t skip retry logic
- Don’t deploy an adapter without a canary tenant first

### **12.6 PMS integration**

**Do**

- Use webhook-first when the PMS supports it
- Publish a public Go SDK so partners can build their own adapter
- Preserve all unknown fields in JSONB
- Track field-level mapping versions for debugging
- Use your own PMS as customer zero — the reference implementation
  **Don’t**
- Don’t read the PMS database directly, even your own
- Don’t share a database schema with your own PMS
- Don’t skip the canonical model for your own PMS — that turns you back into a point integration
- Don’t couple the channel manager’s release cadence to the PMS’s

### **12.7 Frontend**

**Do**

- Generate TypeScript types from protobuf — never hand-write DTOs
- Use shadcn/ui as the component baseline
- Use Next.js App Router with server components by default
- Use Tailwind for styling
- Use TanStack Query for server state
- Use Playwright for end-to-end tests
- Use Storybook for component development
  **Don’t**
- Don’t roll your own design system in Phase 1
- Don’t store server data in `useState` or context — use TanStack Query
- Don’t duplicate domain types — generate them from proto
- Don’t use `localStorage` or `sessionStorage` for sensitive data
- Don’t ship without an accessibility baseline (WCAG AA)
- Don’t use Redux

### **12.8 Testing**

**Do**

- Contract-test every OTA with Pact in CI
- Use `testcontainers-go` for real Postgres and Redis in CI (not mocks)
- Unit-test domain logic with no I/O
- Load-test the sync engine before launch (target a realistic worst-case tenant)
- Chaos-test OTA failure scenarios (timeouts, 500s, malformed responses)
- Run a cross-tenant read test that must fail (validates RLS)
  **Don’t**
- Don’t mock everything — integration tests catch what unit tests miss
- Don’t skip integration tests because “they’re slow” — they’re the most valuable tier
- Don’t ship a new OTA without contract tests
- Don’t accept coverage as a quality metric in isolation

### **12.9 Operations**

**Do**

- Use OpenTelemetry from day one, instrumented at the platform layer
- Alert on sync failure rate, OTA latency, and queue depth — not just HTTP 5xx
- Encrypt OTA credentials at rest (Secrets Manager + envelope encryption)
- Rotate credentials regularly
- Have a documented rollback path for every deploy
- Run regular backup-restore drills, not just backups
  **Don’t**
- Don’t deploy without observability
- Don’t store credentials in environment variables committed to Git
- Don’t deploy without a rollback path
- Don’t skip backups (and don’t trust untested backups)
- Don’t run a single-AZ database in production

### **12.10 AI-assisted development**

**Do**

- Use AI for boilerplate, scaffolds, tests, docs, log triage, SQL drafts
- Require human review for every AI-authored PR — same CI gates as any other PR
- Apply higher scrutiny to anything touching money, inventory, or credentials
- Gate AI pricing recommendations behind explicit user opt-in
- Maintain an audit trail for every AI-applied action
  **Don’t**
- Don’t bypass CI or review for AI-authored code
- Don’t have AI make architectural decisions unilaterally
- Don’t auto-apply AI pricing without user opt-in and a shadow-mode comparison period
- Don’t use AI for security-sensitive code without extra human scrutiny
- Don’t ship AI-authored code that hasn’t been read end-to-end by a human

---

## **13\. Risk Register (summary)**

| Risk                                | Mitigation                                                               |
| ----------------------------------- | ------------------------------------------------------------------------ |
| OTA API drift                       | Pact contract tests in CI, canary tenant, schema version tracking        |
| Double-booking on race              | Redis inventory locks with TTL, outbox pattern, idempotency keys         |
| OTA rate limits / bans              | Per-channel token bucket, circuit breaker, exponential backoff           |
| Modular monolith becomes a mud ball | `go.work`, `internal/`, depguard CI, schema-per-service, proto contracts |
| Premature modularization            | Start with 4 coarse modules, split only on friction                      |
| PMS variance explodes adapters      | Public PmsAdapter SDK, partners build their own, canonical model         |
| Multi-tenant data leak              | Postgres RLS + tenant context middleware + cross-tenant read test        |
| AI mispricing                       | Suggestions only by default, audit trail, shadow A/B before auto-apply   |
| AI-authored code ships unreviewed   | No CI/review bypass, higher scrutiny on critical paths                   |
| Phase 1 scope creep                 | Hard cut: one PMS, one OTA, no AI pricing — everything else is Phase 2+  |

---

## **14\. Quick Reference**

### **One-line stack**

> ***Go*** *monolith on* ***Postgres + sqlc*** *with* ***Redis*** *locks,* ***Asynq*** *jobs,* ***in-proc EventBus*** *(NATS later),* ***Chi + Connect-RPC*** *APIs,* ***Casbin*** *authz,* ***Clerk/WorkOS*** *identity,* **\*Postgres RLS\*\***for tenant isolation,\* ***Next.js + shadcn*** *frontend,* ***buf*** *for proto,* ***OpenTelemetry + Grafana + Sentry*** *observability, in a* ***monorepo*** *enforced by* ***depguard***_._

### **When to split a service**

When at least one is true:

- A channel saturates the shared worker pool
- A subsystem requires a different deploy cadence
- Blast radius from a bad deploy is unacceptable
- A team boundary forms around the subsystem
  Not before.

### **When to upgrade the event bus**

- **In-proc → NATS JetStream:** when the first service splits out
- **NATS → Kafka:** when you need durable replay, partitioning, or cross-DC fan-out

### **When to add managed authz**

- Defer indefinitely. Re-evaluate at Phase 3 only if non-engineers need to author policy through a UI.

### **When to use AI vs human work**

- **AI:** boilerplate, scaffolds, tests, docs, log triage, SQL drafts, codegen
- **Human:** architecture, OTA edge cases, security, pricing rules, customer ops, incident command, money/inventory/credential sign-off

---

_End of_ [_guide._](http://guide.Channel)[**Channel**](http://guide.Channel) **Manager Platform — Engineering Reference Guide**

> _Consolidated reference distilled from the architecture and planning conversation. Treat this as the canonical source for stack choices, conventions, and the do’s and don’ts your team should hold itself to._
> ***Version:*** *2.1 — April 2026*
> ***Audience:*** *Engineering team, AI coding agents, technical reviewers*
> ***Companion document:*** *`Channel_Manager_Implementation_Plan.docx`* *(the narrative blueprint)*

---

**1\. Architectural Pillars**
The seven non-negotiables. Every decision in this guide traces back to one of them.

1. **PMS-agnostic core.** Any PMS — including your own — plugs in through one Go interface. The domain never sees a vendor.
2. **Channel-agnostic distribution.** [Booking.com](http://Booking.com), Airbnb, Expedia, Agoda, direct, and future GDS/metasearch all sit behind the same `ChannelAdapter` contract.
3. **Multi-tenant isolation as a database invariant.** Postgres Row-Level Security is the last line of defense; application code is the first.
4. **Event-driven sync with idempotency.** Every write to an OTA carries an idempotency key. Every domain mutation publishes a domain event.
5. **Audit log for anything touching money, inventory, or credentials.** No silent state changes.
6. **Modular monolith with split-ready discipline.** One or two binaries today, extractable to services tomorrow as a deploy change, not a rewrite.
7. **AI-assisted development with human-reviewed gates.** Every AI-authored PR passes the same CI and review bar as any other PR.

---

**2\. Three Decisions to Unbundle**
These three are constantly conflated. Hold them apart.

| Decision               | Question                 | Choice                 | Rationale                                                           |
| ---------------------- | ------------------------ | ---------------------- | ------------------------------------------------------------------- |
| Repo layout            | One repo or many?        | Monorepo               | Atomic cross-module changes, shared tooling, single source of truth |
| Deployment topology    | One binary or many?      | Modular monolith first | Operational simplicity at MVP; split when triggers appear           |
| Engineering discipline | Prepared to split later? | Yes, from day one      | Per-module Go modules, schema-per-service, protobuf contracts       |

Trigger conditions for splitting a service out (Phase 4):

- One channel saturating the shared worker pool
- One subsystem requiring an independent deploy cadence
- Blast radius from a bad deploy becomes unacceptable
- A team boundary forms around a subsystem

---

**3\. Backend Architecture**
**3.1 Language and runtime**

- **Go 1.22+** for all backend services and adapters.
- Static binary deploys, low memory footprint, predictable GC, native concurrency for I/O-bound fan-out.
- Easy for AI agents to generate consistently and for humans to review.
  **3.2 Service topology (current and future)**

```gherkin
[Clients]DashboardBookingEnginePartnerAPIs
|
[API Gateway+Auth]
|
+-------------+-----------+-----------+---------------+
||||
InventoryPricingEngineReservationsMapping
Module(Strategy+ AI)ModuleModule
||||
+------+------+------+---------+------+-------+-------+
||||
EventBus(in-proc today; NATS /Kafka after split)
||||
ChannelSyncEngine PMS AdapterHub
(SagaOrchestrator)(PMS-agnostic)
||
ChannelAdapters PMS Adapters
Booking.com,Airbnb,Cloudbeds,Mews,
Agoda,Expedia, direct Your PMS, CSV
||
[Outbound OTA APIs][PMS APIs/Webhooks]
```

**3.3 Domain modules (each is its own Go module)**

- `services/inventory/` — availability, stop-sell, min/max stay
- `services/pricing/` — base rates, derivations, strategy engine
- `services/reservations/` — booking lifecycle, modifications, cancellations
- `services/mapping/` — internal ↔ external schema translation, version history
- `services/channel/` — `ChannelAdapter` contract + per-OTA adapters
- `services/pms/` — `PmsAdapter` contract + per-PMS adapters
- `services/audit/` — append-only audit log
  Each module is structured as:

```gherkin
services/<name>/
 domain/# pure structs and rules, no external deps
 ports/# interfaces this module needs from others
 usecases/# application services / orchestration
 adapters/# concrete: HTTP handlers, postgres repos, queue
 go.mod
```

**3.4 Platform (cross-cutting) libraries**

- `platform/events/` — `EventBus` interface; `inproc`, `nats`, `kafka` implementations
- `platform/auth/` — JWT verification, tenant context middleware, Casbin enforcer
- `platform/db/` — pgx connection pool, sqlc wiring, migration runner
- `platform/http/` — Chi server scaffolding, middleware chains
- `platform/observability/` — OpenTelemetry SDK, structured logging
- `platform/config/` — env loading, secrets resolution
  **3.5 Data layer**
- **PostgreSQL 16** (Neon for serverless, RDS/Aurora for AWS-native).
- **sqlc + pgx** — type-safe SQL, no ORM magic. Queries are explicit, reviewable, and AI-friendly.
- **Schema-per-service.** Each service owns its schema; cross-schema foreign keys are forbidden by lint.
- **golang-migrate or Atlas** for migrations, one directory per service.
- **Redis** for inventory locks, idempotency keys, rate-limit tokens, session cache.
- **ClickHouse** for revenue analytics (RevPAR, ADR, channel contribution).
- **Meilisearch** for ops search (instant booking lookup).
  **3.6 Event bus and job queue**
- **In-process EventBus** today (typed Go channels under the hood). Same interface upgraded to **NATS JetStream** post-split, **Kafka** when volume demands replay and partitioning.
- **Asynq** (Redis-backed) for background jobs: sync work, scheduled rate updates, retries.
- **Outbox pattern** — DB writes and event publishes are atomic. No lost updates when a queue blips.
  **3.7 Authentication and authorization**
  A two-layer model. Each layer solves a different problem.

| Layer            | Purpose                            | Tool                              |
| ---------------- | ---------------------------------- | --------------------------------- |
| Identity         | Who is this user?                  | Clerk or WorkOS (SSO, SAML, SCIM) |
| Tenant isolation | Are they in the right org?         | Postgres Row-Level Security       |
| Authorization    | What can they do on this resource? | Casbin (embedded Go library)      |

- Identity layer issues JWTs; Go middleware verifies and extracts `(userId, orgId, role)`.
- Middleware sets `SET LOCAL app.current_org_id = '...'` on the DB connection. RLS policies enforce tenant scope at the database — bug-resistant by construction.
- Casbin answers “given role and resource, allowed?” with sub-millisecond, in-process decisions. RBAC with domains plus light ABAC.
- Defer managed authz services ([Permit.io](http://Permit.io), Oso Cloud, Auth0 FGA). Re-evaluate at Phase 3 only if policy authoring becomes a customer-success burden.
- Phase 4 upgrade path: **OpenFGA** for relationship-graph permissions (reseller white-label, marketplace), **OPA / Cerbos** for cross-service policy decisions.
  **3.8 Observability**
- **OpenTelemetry (Go SDK)** for traces, metrics, and logs — instrumented at the platform layer so domain code stays clean.
- **Grafana Cloud** as the backend.
- **Sentry** for error tracking with source maps for the frontend.
- Structured logs only (`slog` or `zap`), JSON-formatted, with `trace_id`, `org_id`, `request_id` on every line.
- Alerts on sync failure rates and OTA latency, not just HTTP 5xx.

---

**4\. Frontend Architecture**
**4.1 Framework**

- **Next.js 15** (App Router) + **React 19** + **TypeScript 5**.
- Server components by default, client components when interactivity demands.
- Same monorepo, under `apps/dashboard/`.
  **4.2 Component system**
- **Tailwind CSS** for utilities.
- **shadcn/ui** as the component baseline (copy-paste, owned, unforked).
- **Lucide React** for icons.
- **Recharts** or **Tremor** for analytics dashboards.
  **4.3 State management**
- **TanStack Query (React Query)** for server state — caching, refetching, optimistic updates.
- **Zustand** for small slices of client state when needed.
- No Redux. No client-side caches of server data outside TanStack Query.
  **4.4 Type sharing with the backend**
- **Protobuf** is the source of truth.
- **buf** generates both Go types (for services) and TypeScript types (for the dashboard).
- Frontend imports types from `proto/` via a generated package — no hand-written DTOs that duplicate the domain.
- **Connect-RPC** browser client for typed calls into the backend.
  **4.5 Frontend testing**
- **Vitest** for unit tests.
- **Playwright** for end-to-end and visual regression.
- **Storybook** for component development and documentation.
- **MSW** (Mock Service Worker) for component-level integration tests against generated proto types.

---

**5\. Monorepo Layout**
**5.1 Directory structure**

```coffeescript
channel-manager/
├── go.work # Go workspace — binds all modules
├── buf.yaml # Protobuf contracts + code generation
├── turbo.json # Turborepo for cross-language tasks
├──Makefile
├── docker-compose.yml # Local stack: postgres, redis, nats
│
├── apps/# DEPLOYABLES — each has a main.go or package.json
│├── api/# Starts as THE binary; later the HTTP gateway
│├── sync-worker/# Background worker (same binary at first)
│└── dashboard/# Next.js + TypeScript UI
│
├── services/# DOMAIN MODULES — each an isolated Go module
│├── inventory/
│├── pricing/
│├── reservations/
│├── mapping/
│├── channel/
││└── adapters/# One adapter per OTA
││├── bookingcom/
││├── airbnb/
││├── expedia/
││└── agoda/
│├── pms/
││└── adapters/
││├── mypms/# Your own PMS — customer zero
││├── cloudbeds/
││├── mews/
││└── csv/
│└── audit/
│
├── platform/# SHARED LIBS — cross-cutting, stable
│├── events/
│├── auth/
│├── db/
│├── http/
│├── observability/
│└── config/
│
├── proto/# CONTRACTS — protobuf schemas
│├── channel/v1/
│├── pricing/v1/
│└── reservation/v1/
│
├── migrations/# SQL per-service schema
└── tools/# Codegen, seed, onboarding CLI
```

**5.2 Six enforcement disciplines**
Without these, “modular monolith” becomes a mud ball and the split never happens.

1. [**`go.work`**](http://go.work) **+ per-service** **`go.mod`\*\***.\*\* Each `services/<name>/` is a separate Go module.
2. **`internal/`** **packages** for everything not meant for cross-service consumption (compiler-enforced).
3. **`depguard`** **or** **`go-arch-lint`** **in CI.** Build fails if `services/pricing` imports `services/channel`directly.
4. **Protobuf as the cross-service contract.** Anything that might go over the wire someday lives in `proto/`.
5. **Schema-per-service from day one.** Cross-schema foreign keys forbidden by lint. This is the single most important discipline.
6. **Per-service migrations.** `migrations/<service>/` directories own their schema lifecycle independently.

---

**6\. Libraries — Complete List**
**6.1 Backend (Go)**
**Web and RPC:**

- [`github.com/go-chi/chi/v5`](http://github.com/go-chi/chi/v5) — HTTP router
- [`connectrpc.com/connect`](http://connectrpc.com/connect) — Connect-RPC (gRPC-compatible, browser-friendly)
- [`google.golang.org/grpc`](http://google.golang.org/grpc) — gRPC for service-to-service later
- [`github.com/grpc-ecosystem/go-grpc-middleware/v2`](http://github.com/grpc-ecosystem/go-grpc-middleware/v2) — auth, logging, recovery interceptors
  **Database:**
- [`github.com/jackc/pgx/v5`](http://github.com/jackc/pgx/v5) — Postgres driver
- [`github.com/sqlc-dev/sqlc`](http://github.com/sqlc-dev/sqlc) — type-safe query generator (binary, used at build time)
- [`github.com/golang-migrate/migrate/v4`](http://github.com/golang-migrate/migrate/v4) — migrations
- [`github.com/redis/go-redis/v9`](http://github.com/redis/go-redis/v9) — Redis client
  **Jobs and events:**
- [`github.com/hibiken/asynq`](http://github.com/hibiken/asynq) — Redis-backed job queue
- [`github.com/nats-io/nats.go`](http://github.com/nats-io/nats.go) — NATS client (post-split)
- [`github.com/segmentio/kafka-go`](http://github.com/segmentio/kafka-go) — Kafka client (at scale)
  **Auth and authz:**
- [`github.com/clerkinc/clerk-sdk-go/v2`](http://github.com/clerkinc/clerk-sdk-go/v2) or [`github.com/workos/workos-go/v4`](http://github.com/workos/workos-go/v4) — identity
- [`github.com/casbin/casbin/v2`](http://github.com/casbin/casbin/v2) — RBAC/ABAC enforcer
- [`github.com/golang-jwt/jwt/v5`](http://github.com/golang-jwt/jwt/v5) — JWT verification
  **Observability:**
- [`go.opentelemetry.io/otel`](http://go.opentelemetry.io/otel) and friends — OpenTelemetry SDK
- [`github.com/getsentry/sentry-go`](http://github.com/getsentry/sentry-go) — error tracking
- `log/slog` (stdlib) — structured logging
  **Resilience:**
- [`github.com/sony/gobreaker`](http://github.com/sony/gobreaker) — circuit breaker
- [`github.com/cenkalti/backoff/v4`](http://github.com/cenkalti/backoff/v4) — exponential backoff
- [`golang.org/x/time/rate`](http://golang.org/x/time/rate) — token bucket rate limiting
  **Validation and utilities:**
- [`github.com/go-playground/validator/v10`](http://github.com/go-playground/validator/v10) — struct validation
- [`github.com/oklog/ulid/v2`](http://github.com/oklog/ulid/v2) — time-sortable IDs
- [`github.com/spf13/viper`](http://github.com/spf13/viper) — config loading
- [`github.com/stretchr/testify`](http://github.com/stretchr/testify) — test assertions
- [`go.uber.org/mock`](http://go.uber.org/mock) or [`github.com/vektra/mockery/v2`](http://github.com/vektra/mockery/v2) — mocks
- [`github.com/testcontainers/testcontainers-go`](http://github.com/testcontainers/testcontainers-go) — real Postgres/Redis in CI
- [`github.com/pact-foundation/pact-go/v2`](http://github.com/pact-foundation/pact-go/v2) — OTA contract tests
  **AI / ML:**
- [`github.com/tmc/langchaingo`](http://github.com/tmc/langchaingo) — LangChain in Go (or run a Python sidecar)
- [`github.com/sashabaranov/go-openai`](http://github.com/sashabaranov/go-openai) — OpenAI client (embeddings)
- [`github.com/anthropics/anthropic-sdk-go`](http://github.com/anthropics/anthropic-sdk-go) — Anthropic Claude client
- [`github.com/pgvector/pgvector-go`](http://github.com/pgvector/pgvector-go) — pgvector for embeddings storage
  **6.2 Frontend (TypeScript / Next.js)**
  **Core:**
- `next` (15.x), `react` (19.x), `react-dom`, `typescript` (5.x)
- `tailwindcss`, [`@tailwindcss`](https://github.com/tailwindcss)`/typography`, [`@tailwindcss`](https://github.com/tailwindcss)`/forms`
- `tailwind-merge`, `clsx`
  **UI:**
- `shadcn/ui` (copy-paste, not an npm dep)
- [`@radix`](https://github.com/radix)`-ui/*` (underlies shadcn)
- `lucide-react` — icons
- `recharts` or [`@tremor`](https://github.com/tremor)`/react` — charts
- `react-hook-form` + `zod` — forms and validation
- `sonner` — toasts
- `cmdk` — command palette
- `vaul` — drawers
  **Data:**
- [`@tanstack`](https://github.com/tanstack)`/react-query` — server state
- `zustand` — small client state
- [`@connectrpc`](https://github.com/connectrpc)`/connect-web` — typed RPC client
- [`@bufbuild`](https://github.com/bufbuild)`/protobuf` — protobuf runtime types
  **Testing:**
- `vitest`, [`@testing`](https://github.com/testing)`-library/react`, [`@testing`](https://github.com/testing)`-library/user-event`
- [`@playwright`](https://github.com/playwright)`/test` — E2E
- `msw` — Mock Service Worker
- `storybook` ([`@storybook`](https://github.com/storybook)`/nextjs`)
  **Auth:**
- [`@clerk`](https://github.com/clerk)`/nextjs` or [`@workos`](https://github.com/workos)`-inc/authkit-nextjs`
  **6.3 Infrastructure and tooling**
- **Container:** Docker, multi-stage builds, distroless base for Go binaries
- **Orchestration:** ECS Fargate (AWS) or [Fly.io](http://Fly.io), with Kubernetes only after meaningful service split
- **IaC:** Terraform with the AWS provider; Atmos or Terragrunt only if multi-account
- **CI/CD:** GitHub Actions; **goreleaser** for binary builds; **Trivy** for image scanning
- **Build orchestration:** Turborepo (cross-language tasks); `buf` for proto codegen; `make` for one-liners
- **Secrets:** AWS Secrets Manager or Doppler; never in env vars committed anywhere
- **DNS / CDN:** Cloudflare in front of everything
  **6.4 AI / ML stack**
- **Reasoning model:** Anthropic Claude (recommendations, copilot, anomaly explanations)
- **Embeddings:** OpenAI `text-embedding-3-large` or Voyage AI
- **Vector store:** pgvector inside the same Postgres
- **Orchestration:** LangGraph (Python sidecar via gRPC) or `langchaingo` (in-process)
- **Eval / monitoring:** Langfuse or Helicone

---

**7\. PMS Integration**
**7.1 The** **`PmsAdapter`** **contract**

```haskell
type PmsAdapterinterface{
Capabilities()[]PmsCapability// push, pull, webhook, bulk

ListProperties(ctx context.Context)([]Property, error)
ListRoomTypes(ctx context.Context, propertyID string)([]RoomType, error)
GetInventory(ctx context.Context, q InventoryQuery)([]InventoryDay, error)
GetRates(ctx context.Context, q RateQuery)([]RateDay, error)
GetReservations(ctx context.Context, q ReservationQuery)([]Reservation, error)
}

// Optional capability interfaces — Interface Segregation in action
type ReservationPusherinterface{
PushReservation(ctx context.Context, r Reservation)(Ack, error)
}
type InventoryPusherinterface{
PushInventory(ctx context.Context, items []InventoryDay)(Ack, error)
}
type ChangeFeedinterface{
Subscribe(ctx context.Context, h ChangeHandler)(Subscription, error)
}
```

**7.2 Ingestion modes (in order of preference)**

1. **Webhook-first** — PMS pushes, we ack and persist
2. **Polling fallback** — per-tenant scheduler with ETag / If-Modified-Since
3. **Bulk import** — CSV / Excel / JSON for seeding and long-tail PMSs
4. **Direct DB read-replica** — for chains that grant access; treated as just another adapter
5. **Custom adapter SDK** — public Go module so partners can build their own
   **7.3 Your own PMS — three integration options**

| S.No | Mode                          | When                                                     |
| ---- | ----------------------------- | -------------------------------------------------------- |
| A    | HTTP adapter                  | Start here. Loosest coupling, validates the contract     |
| B    | Native gRPC / Connect-RPC     | Once both systems share infra and the API has stabilized |
| C    | Shared event bus (NATS/Kafka) | Once the canonical event schema is proven by production  |

Multi-PMS coexistence comes free: each `Property` row carries a `pmsType`, the `AdapterRegistry` resolves the right adapter per tenant, and tenants on your PMS, Cloudbeds, and Mews all run side by side.

---

**8\. Channel (OTA) Integration**
**8.1 The** **`ChannelAdapter`** **contract**
Same Interface Segregation pattern as PMS — small role-based interfaces, channels implement only what they support.

```haskell
type ChannelAdapterinterface{
ChannelID()string
Capabilities()[]ChannelCapability
}

type AvailabilityPusherinterface{
PushAvailability(ctx context.Context, items []InventoryDay)(Ack, error)
}
type RatePusherinterface{
PushRates(ctx context.Context, items []RateDay)(Ack, error)
}
type ReservationFetcherinterface{
FetchReservations(ctx context.Context, q ReservationQuery)([]Reservation, error)
}
```

**8.2 Sync engine**

- **Saga orchestrator** for multi-channel writes; compensating actions on failure.
- **Idempotency keys** on every outbound write.
- **Inventory locks** in Redis with TTL to prevent double-bookings.
- **Outbox pattern** for DB-and-event atomicity.
- **Per-channel circuit breaker** + token bucket; failures don’t cascade.
- **Exponential backoff with jitter** for retries.
- **Shadow mode** for the first 24 hours of any new tenant — observe, diff, report, but don’t write.

---

**9\. Onboarding**
**9.1 Six-step wizard (target: under 30 minutes to first sync)**

1. **Sign up** — email/password or SSO
2. **Property setup** — rooms and rate plans (CSV upload supported)
3. **Connect PMS** — OAuth or API key, autodiscovery of properties
4. **Connect channels** — OAuth to [Booking.com](http://Booking.com) / Airbnb / Expedia
5. **Mapping review** — AI-suggested room/listing mappings, user confirms
6. **Go live** — shadow mode for 24 hours, then full sync
   **9.2 AI-assisted onboarding**

- CSV / Excel autopilot — column-mapping with synonym handling
- PMS autodiscovery — list properties and rooms the credentials see
- Embedding-based mapping suggestions — match OTA listings to internal rooms
- 24-hour shadow mode with a diff report
- In-app copilot for sync-failure triage
- Industry presets (boutique hotel, vacation rental, hostel, serviced apartments)
- Headless onboarding CLI for chains and partners
  **9.3 Onboarding metrics (instrument from day 1)**
- Time-to-first-sync (target median < 30 minutes)
- Onboarding completion rate (signup → “Go live” within 7 days)
- First-week sync success rate
- Support tickets per onboarded property in the first 30 days

---

**10\. Implementation Phases (summary)**

| Phase                | Weeks | What ships                                                                  |
| -------------------- | ----- | --------------------------------------------------------------------------- |
| Kickoff              | 0     | Monorepo, CI, observability baseline, skeleton in staging                   |
| Alpha                | 8     | One PMS + [Booking.com](http://Booking.com) end-to-end on a test tenant     |
| MVP / Phase 1 beta   | 13    | Onboarding wizard, audit log, dashboard, 3–5 design partners live           |
| Phase 2 GA           | 22    | Airbnb + Expedia + Agoda + pricing engine v1 + second PMS + partner API     |
| Phase 3 GA           | 32    | Revenue intelligence, direct booking engine, competitor scraping, payments  |
| Phase 4 early access | 44    | AI pricing optimizer, offline sync, first service splits, marketplace infra |

---

**11\. AI-Assisted Development Discipline**
What AI does well in this codebase, and what humans must still own.
**AI does well:**

- OTA HTTP client scaffolds
- proto-to-Go and proto-to-TS code generation
- sqlc query drafts from schema
- Test scaffolds (testify, gomock, Playwright)
- Onboarding copy, tooltips, empty states
- Runbook drafts, changelogs, release notes
- Migration scripts, log triage, SQL for analytics
  **Humans still own:**
- Architecture trade-offs (including when to actually split a service)
- OTA contract edge cases — the dragons live in the specifics
- Security and compliance decisions
- Pricing-engine business rules
- Customer conversations, incident command
- Sign-off on any AI-authored change touching money, inventory, or credentials
  **Review discipline:** every AI-authored PR passes the same CI gates and human review. No bypass for AI-authored code. Higher scrutiny on critical paths.

---

**12\. Do’s and Don’ts (consolidated)**
**12.1 Architecture**
**Do**

- Design ports and adapters from day one
- Enforce module boundaries with `depguard` or `go-arch-lint` in CI
- Use protobuf as the source of truth for cross-service contracts
- Isolate each domain module in its own Go module with its own schema
- Implement Postgres RLS for tenant isolation
- Keep an audit log for everything that touches money, inventory, or credentials
- Use idempotency keys on all writes to channels
- Use the outbox pattern for atomic DB + event writes
- Use circuit breakers for outbound calls
- Shadow-mode every new tenant for the first 24 hours
  **Don’t**
- Don’t conflate monorepo with microservices (they are independent decisions)
- Don’t split into microservices on day one
- Don’t share databases across services (or across your own products)
- Don’t bypass the adapter layer, even for your own PMS
- Don’t add foreign keys across service schemas
- Don’t poll when webhooks are available
- Don’t make rate or inventory changes without idempotency
- Don’t store raw OTA credentials in Git, env vars, or DB tables in plaintext
  **12.2 Backend / Go**
  **Do**
- Use small, role-based interfaces (Interface Segregation via Go’s structural typing)
- Keep domain logic in pure structs with no framework imports
- Use sqlc for type-safe SQL — no ORM magic
- Pass `context.Context` through every I/O call
- Wire dependencies in `main.go` (the only place that knows concrete types)
- Use `internal/` for non-exported packages
- Use [`errors.Is`](http://errors.Is) / [`errors.As`](http://errors.As) for error inspection
- Prefer the standard library where it’s reasonable
  **Don’t**
- Don’t use ORMs that hide SQL (e.g., GORM) — they obscure performance and reviewability
- Don’t define fat interfaces (one `PmsAdapter` with 30 methods); segregate
- Don’t put config-dependent work in `init()`
- Don’t ignore errors
- Don’t share state between goroutines without channels or sync primitives
- Don’t use global state for anything that depends on tenant context
  **12.3 Database**
  **Do**
- Schema-per-service, even within a single Postgres instance
- Use Row-Level Security on every tenant-scoped table
- Version migrations per service
- Use JSONB for flexible OTA payloads — preserve unknown fields, never discard
- Use ULIDs (time-sortable, distributed-safe) instead of integer IDs
- Use connection pooling (pgx native pool or PgBouncer at scale)
  **Don’t**
- Don’t add foreign keys across service schemas
- Don’t put business logic in Postgres triggers (visibility tax)
- Don’t share a single migration directory across services
- Don’t store secrets in DB tables (use Secrets Manager)
- Don’t use auto-incrementing integer IDs in distributed contexts
  **12.4 Authentication and authorization**
  **Do**
- Use Clerk or WorkOS for identity (don’t build it)
- Use Casbin for authorization decisions
- Enforce Postgres RLS as a backstop, regardless of application authz
- Use typed permission constants (`ChannelWrite`, `RateEdit`, etc.) — grep-able and refactor-safe
- Scope every API call by tenant context from the JWT
- Treat policy as code: version-controlled, reviewed, tested
  **Don’t**
- Don’t roll your own RBAC engine from scratch
- Don’t conflate identity (who) with authorization (what they can do)
- Don’t skip RLS because “we have application authz” — both are required
- Don’t put role logic scattered through application code; centralize in policy
- Don’t adopt a managed authz service ([Permit.io](http://Permit.io), Oso Cloud, Auth0 FGA) before Phase 3
  **12.5 Channel adapters (OTAs)**
  **Do**
- Implement `ChannelAdapter` plus only the capability interfaces an OTA actually supports
- Use Pact contract tests for every OTA in CI
- Wrap every outbound call with a circuit breaker and a token bucket
- Persist raw OTA responses in JSONB for debugging
- Version your adapter against OTA API versions
- Make every write idempotent
- Use exponential backoff with jitter for retries
  **Don’t**
- Don’t share state between channel adapters
- Don’t let one OTA’s failure cascade to others
- Don’t tightly couple to OTA-specific schema in domain logic — translate at the adapter
- Don’t skip retry logic
- Don’t deploy an adapter without a canary tenant first
  **12.6 PMS integration**
  **Do**
- Use webhook-first when the PMS supports it
- Publish a public Go SDK so partners can build their own adapter
- Preserve all unknown fields in JSONB
- Track field-level mapping versions for debugging
- Use your own PMS as customer zero — the reference implementation
  **Don’t**
- Don’t read the PMS database directly, even your own
- Don’t share a database schema with your own PMS
- Don’t skip the canonical model for your own PMS — that turns you back into a point integration
- Don’t couple the channel manager’s release cadence to the PMS’s
  **12.7 Frontend**
  **Do**
- Generate TypeScript types from protobuf — never hand-write DTOs
- Use shadcn/ui as the component baseline
- Use Next.js App Router with server components by default
- Use Tailwind for styling
- Use TanStack Query for server state
- Use Playwright for end-to-end tests
- Use Storybook for component development
  **Don’t**
- Don’t roll your own design system in Phase 1
- Don’t store server data in `useState` or context — use TanStack Query
- Don’t duplicate domain types — generate them from proto
- Don’t use `localStorage` or `sessionStorage` for sensitive data
- Don’t ship without an accessibility baseline (WCAG AA)
- Don’t use Redux
  **12.8 Testing**
  **Do**
- Contract-test every OTA with Pact in CI
- Use `testcontainers-go` for real Postgres and Redis in CI (not mocks)
- Unit-test domain logic with no I/O
- Load-test the sync engine before launch (target a realistic worst-case tenant)
- Chaos-test OTA failure scenarios (timeouts, 500s, malformed responses)
- Run a cross-tenant read test that must fail (validates RLS)
  **Don’t**
- Don’t mock everything — integration tests catch what unit tests miss
- Don’t skip integration tests because “they’re slow” — they’re the most valuable tier
- Don’t ship a new OTA without contract tests
- Don’t accept coverage as a quality metric in isolation
  **12.9 Operations**
  **Do**
- Use OpenTelemetry from day one, instrumented at the platform layer
- Alert on sync failure rate, OTA latency, and queue depth — not just HTTP 5xx
- Encrypt OTA credentials at rest (Secrets Manager + envelope encryption)
- Rotate credentials regularly
- Have a documented rollback path for every deploy
- Run regular backup-restore drills, not just backups
  **Don’t**
- Don’t deploy without observability
- Don’t store credentials in environment variables committed to Git
- Don’t deploy without a rollback path
- Don’t skip backups (and don’t trust untested backups)
- Don’t run a single-AZ database in production
  **12.10 AI-assisted development**
  **Do**
- Use AI for boilerplate, scaffolds, tests, docs, log triage, SQL drafts
- Require human review for every AI-authored PR — same CI gates as any other PR
- Apply higher scrutiny to anything touching money, inventory, or credentials
- Gate AI pricing recommendations behind explicit user opt-in
- Maintain an audit trail for every AI-applied action
  **Don’t**
- Don’t bypass CI or review for AI-authored code
- Don’t have AI make architectural decisions unilaterally
- Don’t auto-apply AI pricing without user opt-in and a shadow-mode comparison period
- Don’t use AI for security-sensitive code without extra human scrutiny
- Don’t ship AI-authored code that hasn’t been read end-to-end by a human

---

**13\. Risk Register (summary)**

| Risk                                | Mitigation                                                               |
| ----------------------------------- | ------------------------------------------------------------------------ |
| OTA API drift                       | Pact contract tests in CI, canary tenant, schema version tracking        |
| Double-booking on race              | Redis inventory locks with TTL, outbox pattern, idempotency keys         |
| OTA rate limits / bans              | Per-channel token bucket, circuit breaker, exponential backoff           |
| Modular monolith becomes a mud ball | `go.work`, `internal/`, depguard CI, schema-per-service, proto contracts |
| Premature modularization            | Start with 4 coarse modules, split only on friction                      |
| PMS variance explodes adapters      | Public PmsAdapter SDK, partners build their own, canonical model         |
| Multi-tenant data leak              | Postgres RLS + tenant context middleware + cross-tenant read test        |
| AI mispricing                       | Suggestions only by default, audit trail, shadow A/B before auto-apply   |
| AI-authored code ships unreviewed   | No CI/review bypass, higher scrutiny on critical paths                   |
| Phase 1 scope creep                 | Hard cut: one PMS, one OTA, no AI pricing — everything else is Phase 2+  |

---

**14\. Quick Reference**
**One-line stack**

> ***Go*** *monolith on* ***Postgres + sqlc*** *with* ***Redis*** *locks,* ***Asynq*** *jobs,* ***in-proc EventBus*** *(NATS later),* ***Chi + Connect-RPC*** *APIs,* ***Casbin*** *authz,* ***Clerk/WorkOS*** *identity,* **\*Postgres RLS\*\***for tenant isolation,\* ***Next.js + shadcn*** *frontend,* ***buf*** *for proto,* ***OpenTelemetry + Grafana + Sentry*** *observability, in a* ***monorepo*** *enforced by* ***depguard***_._
> **When to split a service**
> When at least one is true:

- A channel saturates the shared worker pool
- A subsystem requires a different deploy cadence
- Blast radius from a bad deploy is unacceptable
- A team boundary forms around the subsystem
  Not before.
  **When to upgrade the event bus**
- **In-proc → NATS JetStream:** when the first service splits out
- **NATS → Kafka:** when you need durable replay, partitioning, or cross-DC fan-out
  **When to add managed authz**
- Defer indefinitely. Re-evaluate at Phase 3 only if non-engineers need to author policy through a UI.
  **When to use AI vs human work**
- **AI:** boilerplate, scaffolds, tests, docs, log triage, SQL drafts, codegen
- **Human:** architecture, OTA edge cases, security, pricing rules, customer ops, incident command, money/inventory/credential sign-off

---

_End of guide._

# Channel Manager Platform

A multi-tenant hotel channel manager built as a **modular monolith** — one deployable binary today, split-ready services tomorrow.

Connects any PMS (Property Management System) to any OTA (Booking.com, Airbnb, Expedia, Agoda) through a unified adapter contract. Manages availability, pricing, and reservations across all channels with idempotent sync, inventory locking, and a full audit trail.

---

## Stack

| Layer                | Technology                                                |
| -------------------- | --------------------------------------------------------- |
| **API**              | Go 1.22+, Chi router, Connect-RPC (gRPC-compatible)       |
| **Auth**             | WorkOS (SSO, SAML, SCIM), Casbin RBAC, RS256 JWT          |
| **Database**         | PostgreSQL 16, pgx v5, sqlc, golang-migrate               |
| **Tenant isolation** | Postgres Row-Level Security (RLS)                         |
| **Cache / Locks**    | Redis 7                                                   |
| **Events**           | In-process typed bus (NATS JetStream upgrade path)        |
| **Background jobs**  | Asynq (Redis-backed)                                      |
| **Observability**    | OpenTelemetry (traces + metrics), Sentry, slog            |
| **Dashboard**        | Next.js 15, React 19, TypeScript, Tailwind CSS, shadcn/ui |
| **Contracts**        | Protobuf + buf (generates Go + TypeScript types)          |
| **Build**            | Turborepo + Make, Go workspaces (`go.work`)               |

---

## Repository Layout

```
channel-manager/
├── apps/
│   ├── api/              # Main HTTP binary (auth + Connect-RPC gateway)
│   ├── sync-worker/      # Background sync worker (future)
│   └── dashboard/        # Next.js 15 dashboard
├── services/             # Domain modules — each an isolated Go module
│   ├── inventory/        # Availability, stop-sell, min/max stay  ✅
│   ├── pricing/          # Rate strategies (stub)
│   ├── reservations/     # Booking lifecycle (stub)
│   ├── mapping/          # OTA ↔ internal schema translation (stub)
│   ├── channel/          # ChannelAdapter contract + OTA adapters (stub)
│   ├── pms/              # PmsAdapter contract + PMS adapters (stub)
│   └── audit/            # Append-only audit log (stub)
├── platform/             # Cross-cutting shared libraries
│   ├── auth/             # JWT verify, Casbin, middleware, WorkOS handlers
│   ├── config/           # Env-based typed config
│   ├── db/               # pgx pool, sqlc wiring, migration runner
│   ├── events/           # EventBus interface + in-process impl
│   ├── http/             # Chi scaffolding, middleware chains
│   └── observability/    # OTel TracerProvider + MeterProvider + Sentry
├── proto/                # Protobuf contracts (inventory, pricing, channel…)
├── gen/go/               # buf-generated Go types
├── migrations/           # Per-service SQL migrations
│   ├── tenancy/          # Orgs, members, RLS policies, Casbin rules
│   └── inventory/        # Inventory schema
├── docs/                 # Engineering reference and setup guides
├── docker-compose.yml    # Local: Postgres, Redis, NATS
├── go.work               # Go workspace (binds all modules)
├── buf.yaml              # Protobuf codegen config
├── turbo.json            # Turborepo task graph
└── Makefile              # One-liner commands
```

---

## Prerequisites

- **Go 1.22+**
- **Node.js 20+** and **pnpm**
- **Docker** (for local Postgres, Redis, NATS)
- **buf** — `brew install bufbuild/buf/buf`
- **golangci-lint** — `brew install golangci-lint`
- **WorkOS account** — see [docs/workos-sso-setup.md](docs/workos-sso-setup.md)

---

## Local Setup

### 1. Clone and install

```bash
git clone https://github.com/your-org/channel-manager.git
cd channel-manager
cd apps/dashboard && pnpm install && cd ../..
```

### 2. Configure environment

```bash
cp .env.example .env
```

Fill in `.env`:

```dotenv
# Database (Postgres — local Docker)
DB_HOST=localhost
DB_PORT=5432
DB_NAME=channel
DB_USER=postgres
DB_PASSWORD=your_password

# Runtime role (RLS-enforced)
APP_DB_USER=app
APP_DB_PASSWORD=app_dev

# Redis
REDIS_ADDR=localhost:6379

# WorkOS
WORKOS_API_KEY=sk_test_…
WORKOS_CLIENT_ID=client_…
WORKOS_REDIRECT_URI=http://localhost:8080/auth/callback
WORKOS_WEBHOOK_SECRET=your_webhook_secret
WORKOS_COOKIE_PASSWORD=32_char_random_string

# Dashboard (for CORS)
DASHBOARD_URL=http://localhost:3000
NEXT_PUBLIC_API_URL=http://localhost:8080

# Observability (optional for local)
OTEL_SERVICE_NAME=channel-manager
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
SENTRY_DSN=
```

### 3. Start infrastructure

```bash
make docker-up
```

### 4. Run migrations

```bash
make migrate-up
```

### 5. Start the API

```bash
make api
```

### 6. Start the dashboard

```bash
make dev
```

Dashboard → http://localhost:3000  
API → http://localhost:8080

---

## Make Targets

| Command                | Description                                             |
| ---------------------- | ------------------------------------------------------- |
| `make docker-up`       | Start Postgres, Redis, NATS containers                  |
| `make docker-down`     | Stop containers                                         |
| `make migrate-up`      | Apply all pending migrations                            |
| `make migrate-down`    | Roll back last migration                                |
| `make migrate-version` | Print current migration version                         |
| `make api`             | Run API server (loads `.env`)                           |
| `make dev`             | Run Next.js dashboard (`pnpm dev`)                      |
| `make build`           | Compile `bin/api` and `bin/sync-worker`                 |
| `make test`            | Run all Go tests                                        |
| `make lint`            | Run golangci-lint                                       |
| `make proto`           | Regenerate Go + TS types from protobuf (`buf generate`) |

---

## API Endpoints

| Method | Path                               | Auth                   | Description                                                  |
| ------ | ---------------------------------- | ---------------------- | ------------------------------------------------------------ |
| `GET`  | `/auth/login`                      | Public                 | Redirect to WorkOS SSO (`?provider=GoogleOAuth\|AppleOAuth`) |
| `GET`  | `/auth/callback`                   | Public                 | WorkOS OAuth callback — sets HttpOnly cookies                |
| `POST` | `/auth/password`                   | Public                 | Email/password login — sets HttpOnly cookies                 |
| `POST` | `/auth/webhook`                    | HMAC                   | WorkOS webhook receiver — mirrors orgs/users                 |
| `GET`  | `/me`                              | Cookie/Bearer          | Returns `{user_id, org_id, role}`                            |
| `POST` | `/inventory.v1.InventoryService/*` | Cookie/Bearer + Casbin | Connect-RPC inventory handlers                               |

---

## Authentication Flow

```
Browser ──GET /auth/login?provider=GoogleOAuth──▶ API
API ──redirect──▶ WorkOS AuthKit ──▶ Google / Apple / Email
WorkOS ──redirect──▶ GET /auth/callback
API ── exchange code ──▶ WorkOS
     ── verify JWT ──────────────────
     ── mirror org + user ────────── tenancy schema
     ── set HttpOnly cookies ──────▶ Browser
Browser ──credentialed requests──▶ /me, /inventory.v1…
```

Cookie-based sessions use `access_token` + `refresh_token` (HttpOnly, SameSite=Lax). Bearer token fallback supported for API clients.

---

## Multi-Tenancy

Every tenant (org) is isolated at the **database level** using Postgres Row-Level Security. The auth middleware:

1. Verifies the JWT (RS256, JWKS auto-refresh)
2. Resolves `org_id` from the token
3. Sets `SET LOCAL app.current_org_id = '…'` on the DB connection
4. RLS policies filter every query to that org — bugs in application code cannot leak cross-tenant data

---

## Authorization

Casbin RBAC with domain scoping: `g(subject, role, domain)`.

Roles: `owner`, `admin`, `member`. Policies are stored in Postgres (`tenancy.casbin_rule`) and enforced in-process via a Connect-RPC unary interceptor before any handler runs.

---

## Build Status

| Component                                     | Status |
| --------------------------------------------- | ------ |
| Monorepo scaffold, Go workspace               | ✅     |
| Postgres pool + RLS + migrations              | ✅     |
| Auth (WorkOS JWT + Casbin + HttpOnly cookies) | ✅     |
| Google / Apple / Email SSO (AuthKit)          | ✅     |
| Observability (OTel + Sentry)                 | ✅     |
| Inventory service (domain → RPC)              | ✅     |
| Dashboard login UI                            | ✅     |
| Dashboard session provider + route guards     | ⏳     |
| Token refresh endpoint                        | ⏳     |
| Pricing service                               | 🔲     |
| Reservations service                          | 🔲     |
| Channel adapters (Booking.com, Airbnb…)       | 🔲     |
| PMS adapters                                  | 🔲     |
| Sync worker + saga orchestrator               | 🔲     |
| Onboarding wizard                             | 🔲     |

---

## Docs

- [Engineering Reference Guide](docs/%23%20Channel%20Manager%20Platform%20%E2%80%94%20Engineering.md) — architecture, conventions, do's and don'ts
- [WorkOS SSO Setup](docs/workos-sso-setup.md) — step-by-step Google, Apple, and email/password configuration

---

## Development Conventions

- **No cross-service DB imports** — each service owns its schema; cross-schema foreign keys are forbidden
- **Protobuf as the contract** — anything that might go over the wire lives in `proto/`
- **sqlc, not an ORM** — queries are explicit, reviewable, and type-safe
- **`internal/` enforced** — non-exported packages stay non-exported (compiler-enforced)
- **Idempotency keys on every OTA write** — no duplicate channel updates
- **Outbox pattern** — DB writes and event publishes are atomic

---

## License

Private — all rights reserved.

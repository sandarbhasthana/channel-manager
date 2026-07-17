# Allotment + Cutover Plan

**Self-contained handoff.** Everything needed to resume this work in a fresh
session is in this document. No prior conversation context required.

**Status: all decisions settled (D1–D10). Phase 0 ready to start.**

---

## ⚠️ READ FIRST — pick these up before anything else

### 1. G1 is a LIVE BUG, not a planning item

**Any property already at `route=cm, percent=100` is losing every preference
booking right now, and telling the guest it succeeded.**

BE sends `room_id: null` for any booking where the guest filled a room
preference. CM's storefront `createBooking` hard-rejects it
(`"room_id or hold_token is required"`). BE catches the error, logs it, and
**still returns `success: true` to the guest.** The booking exists in the BE's own
DB, never reaches CM or the PMS, and nobody finds out until the guest arrives.

This is why it is the first item in Phase 0 (§4, G1). It is not cutover risk —
it is happening in production-shaped environments today.

### 2. The D8 backfill will blank distribution

Backfilling `Room.syncToChannelManager → false` (D8, §9) means **CM and BE sell
nothing until rooms are explicitly opted in, per property** — including the
Grand Palace test property.

That is intended and correct (fail closed for CM, never for the front desk), but
it must be **coordinated, not discovered mid-test**. Plan the opt-in for each
property in the same change as the backfill.

---

## 0. Orientation

### The three repos

| Repo | Path | What it is |
|---|---|---|
| **PMS** | `~/Desktop/Study/pms-app` | Next.js + Prisma + Postgres. System of record for properties, roomTypes, rooms, reservations. Front desk, calendar, housekeeping. |
| **CM** | `~/Desktop/Study/channel-manager` | Go. Connect-RPC services + a Next.js dashboard (`apps/dashboard`). Distribution hub between the PMS and all channels. |
| **BE** | `~/Desktop/Study/aura-hospitality` | Booking engine. Next.js guest site (`apps/web`) + Express API (`apps/api`). The direct sales channel. |

### Running the stack

- **CM API**: `cd ~/Desktop/Study/channel-manager && make api` → `:8080`.
  Loads `.env`. Needs `REDIS_ADDR`, `REDIS_PASSWORD`, `REDIS_DB`, `DATABASE_URL`.
- **CM dashboard**: `apps/dashboard`, Next.js → `:3000`.
- **BE**: `apps/web` → `:3001` (bumped from 3000 because the CM dashboard holds it);
  `apps/api` → `:5001`. BE API CORS allowlists both 3000 and 3001.
- **CM migrations**: `make migrate-up`.
- **Redis** requires AUTH (`requirepass`). CM reads the password via
  `cfg.Redis.Password` in `apps/api/main.go`. A client built with only `Addr`
  fails with `NOAUTH Authentication required` on the storefront's hold lookup.

### Conventions

- **Every new finding goes into ClickUp task `86d3ndpvb`** (the findings
  register). Do not open new tasks for findings.
- **Prisma/sqlc verification must be run by the user** — blocked in the agent
  sandbox.
- **Proto codegen: `buf` is blocked.** Use the pure-Go replacement
  (protocompile + plugins, `/tmp/gentool`); output is byte-identical to
  `buf generate`.
- Go and Postgres **do** run in the agent sandbox despite older handoff notes
  saying otherwise.
- RLS applies via `WithTenant` (sets `app.current_org_id`) and requires the
  non-superuser `app` role. Roles are `admin` / `property_manager`; superuser is
  not an intended route.
- Connect-RPC services register on `rpcMux` but must **also** be mounted into the
  auth-protected mux per path in `apps/api/main.go`. A missing
  `protected.Handle(xRPCPath, rpcMux)` line manifests as a **404 on every
  procedure of that service** — it has bitten once already.

---

## 1. Target architecture

```
PMS: roomType has N rooms
  │
  ├─ hotel selects WHICH rooms go to CM  ──────────► level 1  (Room.syncToChannelManager)
  │    └─ exclusivity expires N days out ─────────► release   (D10)
  │
  └────────────► CM  ──► splits its pool across channels ──► level 2 (channel_allotments)
                          ├─ Booking Engine   (provider = direct)
                          ├─ Booking.com
                          └─ Expedia

Bookings:  Channel ──► CM ──► PMS      (always — no canary, no direct path)
```

CM is the only thing that talks to the PMS. Every channel — including the Booking
Engine — talks to CM and nothing else. **BE is not special; it is a channel that
happens to be ours.**

**Allotment is two levels:**

- **Level 1 — PMS → CM.** The hotel picks *which rooms* are available to CM
  (`Room.syncToChannelManager`). Those rooms are CM's pool. **Hard carve-out**
  (D7): the PMS front desk does not sell them — until **release** (D10).
- **Level 2 — CM → channels.** CM splits its pool across its channels. BE is
  subject to this exactly like Booking.com, because BE is part of CM and cannot
  sell beyond what the PMS gave CM.

Load-bearing consequences:

- **`BE → PMS direct` is not a feature.** Strangler scaffolding that violates the
  topology. Deleted in Phase 1. **No canary** (D9) — every BE booking goes via CM.
- **BE must become a channel** before it can be allotted (Phase 2).
- **CM traffics in roomTypes, never room numbers** (D6).
- **CM can never sell beyond its pool; no channel beyond CM.**

---

## 2. Ground truth (verified in code)

### PMS

| Fact | Location |
|---|---|
| `Room.syncToChannelManager Boolean @default(true)` — the room-level carve-out flag. **Level 1 exists.** Default flips to `false` + backfill (D8). | `prisma/schema.prisma:288` |
| Room Sync UI (toggle rooms in/out of CM) — PMS > Settings > Channel Manager | `src/components/settings/channels/PropertyMappingTab.tsx` |
| `ChannelManagerRoomTypeMapping` — maps PMS roomType → CM roomType | `prisma/schema.prisma:1831` |
| `get_room_types` / room details **do** filter `syncToChannelManager: true` | `src/app/api/webhooks/bookings/route.ts:263, 341` |
| **`getPropertyAvailabilitySnapshot` does NOT filter `syncToChannelManager`** — CM receives the property's *entire* availability. **This is the level-1 gap.** | `src/lib/webhooks/availability-engine.ts:300` |
| That engine is called **only** from the webhook route (4 sites) — CM-facing only, so adding the filter cannot disturb the front desk | `src/app/api/webhooks/bookings/route.ts:190, 363, 480, 786` |
| `handleSearchAvailability` | `src/app/api/webhooks/bookings/route.ts:175` |
| `RoomType` has occupancy fields only — no room-count column | `prisma/schema.prisma:228` |
| Calendar renders a per-booking **note/message icon** — the surface D6 relies on for preferences | `src/app/calendar/page.tsx` |

### CM

| Fact | Location |
|---|---|
| `inventory.inventory_days` — PK `(org_id, room_type_id, stay_date)`, already per-date; `available`, `sold`, `blocked`, `stop_sell`, `version` | `services/inventory/adapters/postgres/schema.sql` |
| `IngestAvailability` writes `avail := o.AvailableUnits` — a **live mirror**, not an allotment | `services/pms/usecases/service.go:293` |
| PMS→inventory writer sets **only** `Available` + `StopSell` | `services/pms/adapters/inventory/writer.go` |
| **Upsert clobbers accounting**: `sold = EXCLUDED.sold` + zero-valued writer ⇒ **every sync resets `sold` to 0** | `services/inventory/adapters/postgres/queries/inventory.sql:23-45` |
| **Nothing increments `sold` on a booking.** `sold`/`blocked` are dead columns | — |
| `channels` `(org_id, property_id, connection_id, provider, external_property_id, status)`; adapters: `bookingcom`, `expedia`, `airbnb`. **No row for BE.** | `services/channel/adapters/postgres/schema.sql:21` |
| Storefront `searchAvailability` **proxies live to the PMS**, subtracts Redis holds, never reads `inventory_days` | `services/storefront/usecases/service.go:315` |
| **Holds are keyed by `room_id`** — `heldRooms` returns `map[roomID]bool`, availability filters `held[o.RoomID]`. Must become per-roomType counts (Phase 2). | `services/storefront/usecases/service.go` (`heldRooms`) |
| Storefront `createBooking` — **requires `room_id` or `hold_token`** (this is G1) | `services/storefront/usecases/service.go:455` |
| `persistReservation` → `IngestReservation`, `source: "direct"` into `RawPayload` → `metadata` column | `services/storefront/usecases/service.go` |
| `DirectChannel = "direct"` | `services/storefront/domain/actions.go:53` |
| `requireBookingEngine` gate | `services/storefront/usecases/service.go:305` |
| `ListDirectReservations` — `WHERE metadata->>'source' = 'direct'`; `domain.Source = "direct"` | `services/bookingengine/adapters/postgres/repository.go:44`; `services/bookingengine/domain:19` |
| `UpdateSettings` writes `pms.properties.booking_engine_enabled` | `services/bookingengine/adapters/postgres/repository.go:122` |
| `BookingEngineEnabled` reads the same column | `services/pms/adapters/postgres/property_repo.go:132` |
| `reservations.reservations` carries `channel_id`, `room_type_id`, `check_in`, `check_out`, `status` | `services/reservations/domain/models.go` |

### BE

| Fact | Location |
|---|---|
| `resolveRoute` — reads go to CM whenever `route=cm`; **`create_booking` obeys `percent`**. `percent=0` ⇒ bookings still land in the PMS. **All deleted in Phase 1** (D9). | `apps/api/src/bookingRoute.ts:104` |
| `create_booking` sends `room_id: assignRoom ? (stay.roomId \|\| null) : null` — **null for any preference booking**, never a `hold_token` | `apps/api/src/routes/checkout.ts:291` |
| BE **never sends `total_amount`** ⇒ CM persists `total_amount_minor = 0` | `apps/api/src/routes/checkout.ts:287` |
| On upstream failure BE logs and **still returns `success: true`** to the guest | `apps/api/src/routes/checkout.ts:324-329` |

---

## 3. The correction: routing ≠ allotment

Not the same thing, must not share a field:

- **Routing (`booking_route`, `booking_route_percent`)** — *which code path
  handles a request.* Temporary scaffolding. **Deleted entirely** in Phase 1.
- **Allotment** — *how much inventory may be sold, and by whom.* Permanent.
  Phases 3–4.

Because routing dies, **"both 0 → PMS direct" cannot mean what it originally
meant** — there is no PMS-direct path to fall back to. `0` gets an explicit,
non-magic meaning (D1).

---

## 4. Phase 0 — CM-path parity (blocks everything)

- **G1 — unassigned bookings are rejected. ⚠️ LIVE BUG — see READ FIRST.** BE
  sends `room_id: null` for any preference booking; storefront `createBooking`
  hard-fails; BE logs it and **still returns success to the guest**. Silently
  lost.
  → Fix per D6: CM accepts `room_type_id`; delete the `room_id` requirement.
- **G2 — totals are lost.** BE never sends `total_amount`; every direct
  reservation in CM has `total_amount_minor = 0`.
  → BE sends `total_amount` + `currency`; CM rejects a create with no total
  rather than silently writing 0.
- **G3 — no parity proof** for `get_quote`, `cancel_booking`, `get_booking` on
  the CM path.

**Exit criterion:** a preference booking and a no-preference booking both
complete end-to-end through CM, appear in the BE table with correct totals, and
match what the PMS recorded.

## 5. Phase 1 — Hard cut to CM, delete routing

No canary (D9). Once Phase 0 passes:

1. **Delete**: `BOOKING_ROUTE` env, `booking_route` + `booking_route_percent`
   columns, `resolveRoute`, `rollBucket`, the shadow-compare, and the routing
   half of `fetchChannelConfig`.
2. `bookingRoute.ts` collapses to "post this action to CM."
3. The existing shadow-compare may be used **once** as a pre-cut verification
   before it is removed.

`booking_engine_enabled` survives Phase 1, then folds into `channels.status` in
Phase 2.

> **Interim note:** until Phase 1 lands, a property must be set
> `route=cm, percent=100` for BE bookings to reach CM at all. `route=cm,
> percent=0` sends availability to CM but books into the PMS — which is why
> direct bookings do not appear in the CM Booking Engine table.

## 6. Phase 2 — BE becomes a channel

- Channel row per property: `provider = 'direct'`, via a synthetic `internal`
  connection (D4).
- `pms.properties.booking_engine_enabled` → `channels.status` (`active`/
  `inactive`), so every channel has one on/off mechanism.
- Storefront offers move room-level → roomType-level (D6).
- **Holds must move to roomType-level.** `heldRooms` currently returns
  `map[roomID]bool` and availability filters by `held[o.RoomID]`. At roomType
  granularity a hold becomes a **count per roomType per night**, subtracted from
  `base`. A real change, not a rename — a room-keyed hold cannot express "one
  Deluxe is held" once offers stop naming rooms.

## 7. Phase 3 — Level 1: make the room carve-out real

The flag, the UI, and the roomType mapping already exist. **The availability path
ignores them.** Work:

1. **Filter CM availability to CM rooms.** Add `syncToChannelManager: true` to
   the room query in `getPropertyAvailabilitySnapshot`
   (`availability-engine.ts:300`). Safe: that engine is called only from the
   CM-facing webhook route.
2. **Enforce the hard carve-out on the PMS side** (D7). The front desk must not
   sell rooms marked `syncToChannelManager: true`, **except inside the release
   window** (D10). The front-desk availability path does **not** use
   `getPropertyAvailabilitySnapshot` — locate it and apply the predicate there.
3. **Add release** (D10): `Property.cmReleaseDays Int @default(0)` + the
   front-desk predicate. Surface it in the Room Sync settings UI.
4. **Flip the default to `false`** and **backfill existing rows to `false`**
   (D8). ⚠️ See READ FIRST — this blanks distribution until rooms are opted in.
5. **Leave `inventory_days` alone.** It is a live mirror and unused by the
   storefront. Do not repurpose it as the pool — see §8 `base`.

## 8. Phase 4 — Level 2: the channel split

### Data model

```sql
CREATE TABLE inventory.channel_allotments (
    org_id       UUID NOT NULL,
    property_id  UUID NOT NULL,
    channel_id   UUID NOT NULL,
    room_type_id UUID NOT NULL,
    mode         TEXT NOT NULL DEFAULT 'unlimited'
                 CHECK (mode IN ('unlimited','rooms','percent')),
    rooms        INT  CHECK (rooms   >= 0),
    percent      INT  CHECK (percent BETWEEN 0 AND 100),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, channel_id, room_type_id),
    CHECK ( (mode='rooms'   AND rooms   IS NOT NULL)
         OR (mode='percent' AND percent IS NOT NULL)
         OR  mode='unlimited' )
);
```

No dates (D3). Explicit `mode` (D1).

### Resolution

```
cm_capacity(roomType)  = COUNT(rooms WHERE roomTypeId = rt AND syncToChannelManager)
                         -- static; the size of CM's carve-out

base(roomType, date)   = live PMS availability for CM-synced rooms of that roomType,
                         minus roomType-level holds for that night
                         -- already net of EVERY booking (front desk + all channels)

allot_rooms(channel, roomType) =
    unlimited -> cm_capacity                         -- no channel cap; the pool still binds
    rooms     -> rooms
    percent   -> floor(cm_capacity * percent / 100)  -- stable denominator, does not erode

sold_via_channel(channel, roomType, date)
                       = CM reservations for that channel/roomType overlapping that
                         night WHERE status <> 'cancelled'

effective(channel, roomType, date) =
    max(0, min( allot_rooms - sold_via_channel, base ))
```

A stay is sellable only if `effective(...) > 0` for **every** night in the range.

**Why `base` is the live proxy, not `inventory_days`:** serving from
`inventory_days` would make `IngestAvailability` lag directly cause overselling.
The live proxy is already net of front-desk sales *and* CM's own bookings (they
all land in the PMS), which makes both the front-desk case and release (D10)
correct for free.

**Why counts are derived, not stored:** a `sold` counter needs an owner, is wiped
by the sync upsert (`sold = EXCLUDED.sold`), and drifts silently. Reservations are
the source of truth we already trust. Needs an index on
`(org_id, property_id, room_type_id, check_in, check_out)`; revisit with a
materialised counter only if measured slow.

**Cancellations must be excluded** from `sold_via_channel` (`status <>
'cancelled'`). Without it a cancelled booking permanently consumes a channel's
allotment — the channel slowly starves and nothing in the UI explains why.

### The invariants

> 1. **No channel can sell beyond CM's pool** — `min(..., base)` enforces it.
> 2. **CM cannot sell beyond its carve-out** — `base` only ever counts
>    `syncToChannelManager` rooms.
> 3. **A booking on any channel — or the front desk — immediately reduces every
>    channel**, because `base` is the live shared pool, not a per-channel copy.

Invariant 3 is why the formula is `min(rooms - sold, base)` and **not**
`min(rooms, base) - sold`: the latter hides rooms a channel is entitled to sell
once the pool shrinks.

Worked example — CM carve-out = 10 Deluxe; BE `rooms=8`, BKG `rooms=8`:

| Event | base | BE shows | BKG shows |
|---|---|---|---|
| start | 10 | min(8−0, 10) = **8** | min(8−0, 10) = **8** |
| BE sells 1 | 9 | min(8−1, 9) = **7** | min(8−0, 9) = **8** |
| BKG sells 6 | 3 | min(8−1, 3) = **3** | min(8−6, 3) = **2** |
| BE sells 2 | 1 | min(8−3, 1) = **1** | min(8−6, 1) = **1** |

Both advertise 8 at the start (16 > 10 — deliberate free-sell, D2), but no
channel ever shows more than CM actually holds, and the last row correctly offers
the single remaining room on both channels. First to book wins; the loser is
rejected by the booking-time re-check.

### Enforcement — two points, both required

- **Availability** (`searchAvailability`): stop proxying blind; cap offers by
  `effective(...)` per roomType per night.
- **Booking** (`createBooking`): re-check `effective(...)` in the write path
  under a lock. Availability is a display; **the booking check is the
  guarantee.** Without it two concurrent bookings both pass the display check and
  bust the pool.

---

## 9. Decisions (all settled)

| # | Decision | Outcome |
|---|---|---|
| **D1** | What does `0` mean? | Explicit `mode`. `rooms=0` = sell nothing, on purpose. Default `unlimited`. No magic zero. |
| **D2** | May channel allotments oversubscribe? | **Yes — for display only.** 8/10 + 8/10 is legitimate free-sell. The invariants make the pool bind absolutely; the booking-time re-check is the hard guarantee. |
| **D3** | Static or per-date? | **Level 1 carries the date variation; level 2 is static.** See below. |
| **D4** | BE's `connection_id` (FK, NOT NULL) | **Synthetic `internal` connection per property.** Keeps the FK and every existing "list channels" query unchanged; a nullable FK would force changes across `integration`. |
| **D5** | `percent` of what? | **Of `cm_capacity`** — the count of CM-synced rooms in the roomType. Static, so it does not erode as rooms sell. *(Corrected twice: "% of current availability" self-erodes; "% of all rooms in the roomType" ignores that CM only distributes its carve-out.)* |
| **D6** | Room assignment + preferences | **CM is roomType-only. No temp ID.** See below. |
| **D7** | Hard carve-out or soft cap? | **Hard carve-out.** The PMS commits `syncToChannelManager` rooms to CM and does not sell them at the front desk — until release (D10). |
| **D8** | `syncToChannelManager` default + backfill | **Default `false`; backfill existing rows to `false`.** Selection is an explicit act per property. ⚠️ See READ FIRST. |
| **D9** | Canary for the cutover? | **No canary.** All BE bookings route from CM. Phase 1 is a hard cut and the route/percent dial is deleted outright. |
| **D10** | Release rules | **Build it — not deferred.** Unsold CM inventory returns to the shared pool N days before arrival. See below. |

### D3 — settled by the two-level model

The per-date lever lives at level 1, not level 2.

- **Level 1 varies by date naturally.** CM's pool for a night is *"CM-synced rooms
  that are free that night"* — it already moves with real occupancy, with no
  per-date configuration at all.
- **Level 2 therefore needs no dates.** The channel split is a ratio applied to
  whatever CM holds that day. Because `base` is per-date, a static split varies by
  date automatically: 50% is 5 rooms on a day CM has 10, and 1 on a day CM has 2.

So: no date columns in `channel_allotments`, no calendar grid, no 365× rows, no
second UI build. If the hotel later wants *scheduled* pool changes ("only 2 rooms
to CM on Dec 24"), that belongs in the PMS's Room Sync as a dated override, and
`inventory_days` is already keyed by `stay_date` to transport it — **zero CM
changes**.

### D6 — roomType-only, and why not a temp ID

The proposal was: return `roomType.id`, and for preference bookings assign a
**temp ID** plus a PMS note (the calendar already renders a per-booking message
icon).

**Adopt the roomType.id half and the note half. Drop the temp ID.**

The temp ID exists only to satisfy CM's `"room_id or hold_token is required"`
check — but that check *is* the bug (G1). Minting a fake room that flows through
two systems to satisfy a validation we control is backwards. Delete the
requirement instead.

- **CM's `create_booking` takes `room_type_id`, never `room_id`.** Not a special
  case for preferences — it's how every channel already works. Booking.com does
  not book room 302. This is what makes BE genuinely "just another channel."
- **Room assignment is 100% a PMS concern.** The PMS owns rooms, already supports
  unassigned bookings, and already renders the note icon.
- **Preferences travel as structured fields + a note**, driving that existing icon.

**Flagged — this reverses an earlier rule.** The established rule was *"preference
→ booking created UNASSIGNED, front desk assigns at check-in."* "Assign a room
always" reverses it. Risk: auto-assigning a smoking room to a guest who requested
non-smoking, where the only signal is a note icon nobody opens, is a service
failure that reports itself as a success.

**Recommended reconciliation** (PMS-side policy, invisible to CM):

> The PMS auto-assigns when it can satisfy the preference, and leaves the booking
> unassigned when it cannot.

"Always assigned" for the common case, front desk involved exactly when it
matters, and no CM change — CM never knew about rooms.

### D8 — default `false`, and backfill `false`

`@default(false)` governs only *new* rooms. Every existing row is `true` from the
old default, so without a backfill the hard carve-out would lock the front desk
out of **every existing room** the moment Phase 3 lands.

**Backfill existing rows to `false`.** Failing closed for CM is the safe
direction: a hotel notices "my OTA has no inventory" and fixes it in the Room Sync
UI in seconds; a hotel discovers "the front desk can't sell room 302" with a
walk-in guest standing at the counter. The front desk is the core operation and a
migration must never silently disable it.

### D10 — release rules (build, do not defer)

**Problem:** hard carve-out with no release means an unsold CM room sits empty
while a walk-in is turned away. Real channel managers release unsold allotment
before arrival. This also backstops inventory stranded by a restrictive level-2
split (e.g. BE 8 + BKG 2, BKG sells nothing, 2 rooms nobody can sell).

**Semantics — release ends the _guarantee_, not the _access_.** Inside the
window the room returns to the **shared pool**: the front desk *and* CM may both
sell it, first come first served. That is the revenue-maximising rule — OTAs and
the direct channel drive last-minute demand, so removing CM would cost bookings,
not save them.

**Config:** `Property.cmReleaseDays Int @default(0)` — `0` = never release, i.e.
exactly today's hard carve-out. Release is opt-in. Surfaced in the Room Sync
settings UI. Extends to per-roomType later without touching the rule.

**Rule — evaluated per night**, consistent with the rest of the model:

```
front_desk_can_sell(room, night) =
      NOT room.syncToChannelManager
   OR (night - today) <= property.cmReleaseDays
```

**CM requires no change whatsoever.** Its `base` is live PMS availability of
CM-synced rooms, so a front-desk sale inside the window shrinks CM automatically
via invariant 3. The entire feature is one predicate on the PMS front-desk
availability path (Phase 3, item 2).

**No oversell risk.** Both sides read live from the PMS, so the window only
creates a *race*, and the booking-time re-check (§8) already resolves races.

**Level 2 needs no release of its own** — level-1 release is the backstop for
anything a restrictive channel split strands.

---

## 10. Risks

- **G1 is live.** See READ FIRST. Preference bookings are being lost now.
- **Backfill blanks distribution.** See READ FIRST. Coordinate the opt-in.
- **Cutover before Phase 0 = lost bookings.** Not optional, not reorderable.
- **The sync clobbers accounting.** `sold = EXCLUDED.sold` with a zero-valued
  writer resets `sold` every sync. Avoided by never relying on `sold`.
- **`inventory_days` means "live mirror" today.** If it is ever repurposed as the
  pool, every reader must be found and updated in the same change.
- **Room-level → roomType-level changes the BE contract and the hold model.**
  `stay.roomId`, the BE pricing path, and `heldRooms` all assume a room id.
- **Cancellations must be excluded from `sold_via_channel`**, or channels starve
  silently.
- **Derived counts are queries, not reads.** Availability costs a reservation
  count per roomType per night. Index first; measure before optimising.

## 11. Sequencing

**Phase 0 → 1 → 2 → 3 → 4. Strictly ordered.**

The channel split (4) is meaningless without a real pool to split (3); the pool is
meaningless while BE can bypass CM entirely (1) or isn't a channel (2); and
deleting the PMS-direct path before parity (0) drops real bookings.

**Next action: Phase 0, G1 + G2.**

## 12. Out of scope (recorded, not planned)

- **No channel-specific rates.** CM owns promos/pricing, but rate parity and
  per-channel rate plans are not modelled.
- **`inventory_days.sold` / `blocked` remain dead columns.** Left in place rather
  than dropped; nothing reads them and the design deliberately does not revive
  them.

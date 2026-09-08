# Storefront combined search API

Guest availability for the voice agent and booking engine goes through Channel Manager storefront:

`POST /api/storefront/v1/{propertyId}` with `action: search_availability`.

The Agent tool `POST /v1/tools/availability/search` forwards that action after validating a discriminated `stay`. Provider adapters (PMS webhook exact/flexible) stay internal. Callers always receive concrete `stay_options`.

## Request

Unified search is selected when the body contains `stay`. Without `stay`, the legacy exact search (`checkin` / `checkout` / `available_rooms`) still runs and **does not mint `offer_id`**.

```json
{
  "action": "search_availability",
  "stay": { },
  "guests": { "adults": 2, "children": 0 },
  "rooms": 1,
  "room_type": "optional filter",
  "sort_by": "soonest | price_low_to_high",
  "limit": 3
}
```

`guests.adults` ≥ 1, `guests.children` ≥ 0, `rooms` ≥ 1. `sort_by` and `limit` apply to flexible search. `limit` max is 10.

### Exact stay

Use when both check-in and check-out are known. Do not send `nights` or `check_in_window`.

```json
{
  "type": "exact",
  "check_in": "2026-09-09",
  "check_out": "2026-09-12"
}
```

### Flexible stay

Use when the guest knows nights but not exact dates. Do not send `check_in` or `check_out`. `check_in_window` is inclusive candidate **arrival** dates (max 31). `nights` is 1–30.

Internally this is mapped to the legacy flexible provider: `earliest_checkin = from`, `latest_checkout = to + nights`.

```json
{
  "type": "flexible",
  "nights": 3,
  "check_in_window": { "from": "2026-09-07", "to": "2026-09-13" }
}
```

A date window is not an exact stay. `search_flexible_availability` remains as a legacy action; new callers should use one `search_availability` with `stay.type`.

## Response

```json
{
  "search_id": "srch_<uuid>",
  "property_id": "<public property id>",
  "meta": {
    "candidate_stays": 1,
    "available_stays": 1,
    "returned_stays": 1,
    "truncated": false
  },
  "stay_options": [
    {
      "stay_id": "stay_<uuid>",
      "check_in": "2026-09-09",
      "check_out": "2026-09-12",
      "nights": 3,
      "offers": [ ]
    }
  ]
}
```

The Agent wraps this object in its envelope (`schema_version`, `correlation_id`, tenant/org/property ids, `data`).

An empty `stay_options` array is a successful no-availability result, not a tool failure.

| `meta` field | Meaning |
|---|---|
| `candidate_stays` | Arrival dates considered (exact = 1; flexible = inclusive window size) |
| `available_stays` | Windows that produced at least one offer |
| `returned_stays` | Windows in this payload |
| `truncated` | `true` when `limit` cut flexible results |

Availability is PMS inventory minus in-flight storefront holds. Disabled booking-engine properties return no offers.

### Offer object

Each offer is one bookable room combination for that stay. Legacy `total_price` / top-level `currency` are rewritten to `total`. Extra PMS attributes (names, types, amenities, capacity, `available_units`, `price_per_night`, …) are passed through.

```json
{
  "offer_id": "off_<uuid>",
  "expires_at": "2026-08-18T02:08:25Z",
  "room_ids": ["<room id>"],
  "room_count": 1,
  "room_names": ["Room 104"],
  "room_type": "Standard Room",
  "room_type_id": "Standard Room",
  "room_types": ["Standard Room"],
  "description": "Comfortable standard room with city view",
  "amenities": ["Free Wi-Fi"],
  "capacity": 2,
  "max_adults": 2,
  "max_children": 1,
  "available_units": 1,
  "price_per_night": 100,
  "total": { "amount": 300, "currency": "USD" }
}
```

`amenities` may be `null`. Currency is per offer. `room_count` must equal `len(room_ids)`.

## `offer_id` behaviour

Yes: unified search mints a server-side offer for every returned row.

1. **Mint** — `offer_id` is `off_` plus a UUID. It is unique per search row, not derived from room ids.
2. **Snapshot** — Redis key `storefront:offer:{offer_id}` stores dates, occupancy, requested rooms, `room_ids`, total amount, currency, `search_id`, `stay_id`, and property id. TTL is **5 minutes** (`DefaultOfferTTL`). Selecting an offer does **not** hold inventory.
3. **Return** — the same id is on the public offer as `offer_id` plus `expires_at` (RFC3339 UTC).
4. **Resolve** — `get_quote` with `offer_id` loads that snapshot, fills `checkin` / `checkout` / `adults` / `room_ids`, and rejects any supplied field that disagrees. It then re-quotes the PMS. If the live total or currency differs, quote fails (`offer price changed; search again`). If still available, a **hold** is placed (`hold_token`).
5. **Book** — `create_booking` still commits with `room_ids`, dates, and (when used) `hold_token`. It does not consume `offer_id` today. Pass `room_ids` exactly as returned; do not reconstruct them.

The Agent Channel Manager client rejects a unified search response if any offer is missing `offer_id`, `room_count` < 1, or `len(room_ids) != room_count`.

### What `offer_id` is not

- Not a hold. Inventory is reserved only at `get_quote`.
- Not minted on legacy storefront search (no `stay` object) or on the PMS webhook `available_rooms` payload. See `docs/PMS_API_REFERENCE.md` for that provider shape.
- Not durable. After TTL, `get_quote` returns offer-not-found; search again.
- Not a cross-property token. Quote rejects an id that belongs to another property.

The Agent PMS-direct adapter may attach a **synthetic** `off_<hash>` when normalizing old PMS `available_rooms` into `stay_options`. That id is not stored in the Channel Manager offer store and cannot be resolved by storefront `get_quote`. Production voice traffic should use Channel Manager unified search.

## Call sequence

```text
search_availability (stay.type=exact|flexible)
  -> stay_options[].offers[].offer_id   // snapshot only
get_quote ({ offer_id })
  -> revalidate price, place hold_token
create_booking ({ room_ids, dates, hold_token, … })
  -> commit reservation
```

After a flexible result, quote/book only from a follow-up **exact** search for the dates the guest picked (same occupancy and room count). Flexible rows are comparable stays, not the booking signature.

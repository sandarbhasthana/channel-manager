# PMS API Reference — Channel Manager Integration

> **Base URL:** `https://<your-pms-domain>` (e.g. `http://pms.local` in dev)
>
> **Auth (internal APIs):** Session cookie via NextAuth — requires `x-property-id` header (or cookie) for property-scoped routes.
>
> **Auth (webhook APIs):** Bearer token via `Authorization: Bearer <token>` header. The server-side `PMS_WEBHOOK_SECRET` env var is a JSON object mapping org IDs → secrets: `{ "org_abc": "secret1" }`.

---

## Table of Contents

1. [Webhook / Booking Engine APIs](#1-webhook--booking-engine-apis) ← **Primary for channel manager**
2. [Properties](#2-properties)
3. [Room Types](#3-room-types)
4. [Rooms](#4-rooms)
5. [Room Availability](#5-room-availability)
6. [Reservations](#6-reservations)
7. [Rates & Pricing](#7-rates--pricing)
8. [Seasonal Rates](#8-seasonal-rates)
9. [Room Blocks](#9-room-blocks)
10. [Payments](#10-payments)
11. [Customer Search](#11-customer-search)
12. [Enums & Constants](#12-enums--constants)

---

## 1. Webhook / Booking Engine APIs

These are the **recommended endpoints** for a channel manager / external booking engine. They are authenticated via `Authorization: Bearer <token>` and do not require a browser session.

### 1.1 Health Check (Organization)

|              |                                                                            |
| ------------ | -------------------------------------------------------------------------- |
| **Endpoint** | `GET /api/webhooks/bookings`                                               |
| **Auth**     | Bearer token                                                               |
| **Response** | `{ status, service, available_actions, organization_id, booking_actions }` |

### 1.2 Search Properties

|              |                               |
| ------------ | ----------------------------- |
| **Endpoint** | `POST /api/webhooks/bookings` |
| **Auth**     | Bearer token                  |

**Request Body:**

```json
{
  "action": "search_properties",
  "city": "string (optional)",
  "country": "string (optional)",
  "name": "string (optional)"
}
```

**Response:** List of properties belonging to the authenticated organization.

### 1.3 Health Check (Property)

|              |                                                                                                    |
| ------------ | -------------------------------------------------------------------------------------------------- |
| **Endpoint** | `GET /api/webhooks/bookings/{propertyId}`                                                          |
| **Auth**     | Bearer token                                                                                       |
| **Response** | `{ status, service, property: { property_id, name, city, country, currency }, available_actions }` |

### 1.4 Search Availability

|              |                                            |
| ------------ | ------------------------------------------ |
| **Endpoint** | `POST /api/webhooks/bookings/{propertyId}` |
| **Auth**     | Bearer token                               |

**Request Body:**

```json
{
  "action": "search_availability",
  "checkin": "YYYY-MM-DD",
  "checkout": "YYYY-MM-DD",
  "adults": 1,
  "children": 0,
  "rooms": 1,
  "room_type": "string (optional, filter by room type name)"
}
```

**Response:** Complete offers with `room_ids: string[]`, `room_count`, combined pricing/currency, aggregate capacity, room names/types, and availability attributes. Scalar/comma-joined offer IDs are not returned.

### 1.5 Get Room Details

|              |                                            |
| ------------ | ------------------------------------------ |
| **Endpoint** | `POST /api/webhooks/bookings/{propertyId}` |
| **Auth**     | Bearer token                               |

**Request Body:**

```json
{
  "action": "get_room_details",
  "room_ids": ["string (optional)"],
  "room_type_id": "string (optional)"
}
```

**Response:** If neither `room_ids` nor `room_type_id` is provided, returns all room types for the property. Otherwise returns details for the specified rooms or room type including amenities, pricing, images, and occupancy limits.

### 1.6 Get Quote

|              |                                            |
| ------------ | ------------------------------------------ |
| **Endpoint** | `POST /api/webhooks/bookings/{propertyId}` |
| **Auth**     | Bearer token                               |

**Request Body:**

```json
{
  "action": "get_quote",
  "room_ids": ["string (required)", "string (optional additional room)"],
  "checkin": "YYYY-MM-DD",
  "checkout": "YYYY-MM-DD",
  "adults": 1
}
```

**Response:** `{ room_ids, room_count, room_name, room_type, checkin, checkout, nights, adults, capacity, price_per_night, total_price, currency, is_available }`

### 1.7 Create Booking

|              |                                            |
| ------------ | ------------------------------------------ |
| **Endpoint** | `POST /api/webhooks/bookings/{propertyId}` |
| **Auth**     | API key                                    |
| **Status**   | `201 Created`                              |

**Request Body:**

```json
{
  "action": "create_booking",
  "room_ids": ["string (required)"],
  "checkin": "YYYY-MM-DD",
  "checkout": "YYYY-MM-DD",
  "guest_name": "string (required)",
  "email": "string (optional)",
  "phone": "string (optional)",
  "adults": 1,
  "children": 0,
  "notes": "string (optional)",
  "total_amount": 480,
  "currency": "USD",
  "idempotency_key": "call-id"
}
```

**Response:**

```json
{
  "data": {
    "booking_ids": ["string", "string"],
    "room_ids": ["string", "string"],
    "group_status": "CONFIRMATION_PENDING",
    "guest_name": "string",
    "room_names": ["string", "string"],
    "room_types": ["string", "string"],
    "property_name": "string",
    "checkin": "ISO datetime",
    "checkout": "ISO datetime",
    "adults": 1,
    "children": 0,
    "total_amount": 480,
    "currency": "USD",
    "payment_status": "UNPAID",
    "message": "Booking confirmed for ..."
  }
}
```

### 1.8 Get Booking

|              |                                            |
| ------------ | ------------------------------------------ |
| **Endpoint** | `POST /api/webhooks/bookings/{propertyId}` |
| **Auth**     | Bearer token                               |

**Request Body:**

```json
{
  "action": "get_booking",
  "booking_id": "string (required)"
}
```

**Response:** `{ booking_id, status, guest_name, email, phone, room_ids, room_name, room_type, property_name, checkin, checkout, adults, children, notes, payment_status, source }`

### 1.9 Update Booking

|              |                                            |
| ------------ | ------------------------------------------ |
| **Endpoint** | `POST /api/webhooks/bookings/{propertyId}` |
| **Auth**     | Bearer token                               |

**Request Body:**

```json
{
  "action": "update_booking",
  "booking_id": "string (required)",
  "checkin": "YYYY-MM-DD (optional)",
  "checkout": "YYYY-MM-DD (optional)",
  "guest_name": "string (optional)",
  "email": "string (optional)",
  "phone": "string (optional)",
  "adults": "number (optional)",
  "children": "number (optional)",
  "notes": "string (optional)",
  "room_ids": ["string (optional, exactly one id to change room)"]
}
```

> **Note:** Only bookings with status `CONFIRMATION_PENDING` can be updated via this endpoint.

**Response:** Updated booking details (same shape as create).

### 1.10 Cancel Booking

|              |                                            |
| ------------ | ------------------------------------------ |
| **Endpoint** | `POST /api/webhooks/bookings/{propertyId}` |
| **Auth**     | Bearer token                               |

**Request Body:**

```json
{
  "action": "cancel_booking",
  "booking_id": "string (required)",
  "reason": "string (optional)"
}
```

> **Note:** Only bookings with status `CONFIRMATION_PENDING` can be cancelled via this endpoint.

**Response:** `{ booking_id, status: "CANCELLED", message }`

---

## 2. Properties

### 2.1 List Properties

|              |                                            |
| ------------ | ------------------------------------------ |
| **Endpoint** | `GET /api/properties`                      |
| **Auth**     | Session (any authenticated user)           |
| **Response** | Array of properties the user has access to |

### 2.2 Get Property Details

|              |                                                                                                    |
| ------------ | -------------------------------------------------------------------------------------------------- |
| **Endpoint** | `GET /api/properties/{id}`                                                                         |
| **Auth**     | Session (user must have property access)                                                           |
| **Response** | Property object with organization info and counts (roomTypes, rooms, reservations, userProperties) |

### 2.3 Create Property

|              |                                       |
| ------------ | ------------------------------------- |
| **Endpoint** | `POST /api/properties`                |
| **Auth**     | Session (ORG_ADMIN only)              |
| **Header**   | `x-organization-id` or `orgId` cookie |

**Request Body:**

```json
{
  "name": "string (required)",
  "phone": "string",
  "email": "string",
  "timezone": "string (default: UTC)",
  "currency": "string (default: USD)",
  "isActive": true,
  "suite": "string",
  "street": "string",
  "city": "string",
  "state": "string",
  "zipCode": "string",
  "country": "string"
}
```

### 2.4 Update Property

|              |                            |
| ------------ | -------------------------- |
| **Endpoint** | `PUT /api/properties/{id}` |
| **Auth**     | Session (ORG_ADMIN only)   |
| **Body**     | Same fields as Create      |

### 2.5 Partial Update Property

|              |                                       |
| ------------ | ------------------------------------- |
| **Endpoint** | `PATCH /api/properties/{id}`          |
| **Auth**     | Session                               |
| **Body**     | `{ "businessRulesEnabled": boolean }` |

### 2.6 Delete Property

|                 |                                                                                        |
| --------------- | -------------------------------------------------------------------------------------- |
| **Endpoint**    | `DELETE /api/properties/{id}`                                                          |
| **Auth**        | Session (ORG_ADMIN only)                                                               |
| **Status**      | `204 No Content`                                                                       |
| **Constraints** | Cannot delete default property or property with existing room types/rooms/reservations |

---

## 3. Room Types

### 3.1 List Room Types

|              |                                                               |
| ------------ | ------------------------------------------------------------- |
| **Endpoint** | `GET /api/room-types`                                         |
| **Auth**     | Session + `x-property-id` header                              |
| **Response** | Array of room types with rooms, property info, and room count |

### 3.2 Get Room Type

|              |                                                     |
| ------------ | --------------------------------------------------- |
| **Endpoint** | `GET /api/room-types/{id}`                          |
| **Auth**     | Session + `x-property-id` header                    |
| **Response** | Room type with rooms, property info, and room count |

### 3.3 Create Room Type

|              |                                                  |
| ------------ | ------------------------------------------------ |
| **Endpoint** | `POST /api/room-types`                           |
| **Auth**     | Session (PROPERTY_MGR+) + `x-property-id` header |

**Request Body:**

```json
{
  "name": "string (required)",
  "abbreviation": "string",
  "privateOrDorm": "private | dorm",
  "physicalOrVirtual": "physical | virtual",
  "maxOccupancy": 1,
  "maxAdults": 1,
  "maxChildren": 0,
  "adultsIncluded": 1,
  "childrenIncluded": 0,
  "description": "string",
  "amenities": ["string"],
  "customAmenities": ["string"],
  "featuredImageUrl": "string",
  "additionalImageUrls": ["string"],
  "basePrice": 0,
  "weekdayPrice": 0,
  "weekendPrice": 0,
  "currency": "USD",
  "availability": 1,
  "minLOS": null,
  "maxLOS": null,
  "closedToArrival": false,
  "closedToDeparture": false
}
```

### 3.4 Update Room Type

|              |                            |
| ------------ | -------------------------- |
| **Endpoint** | `PUT /api/room-types/{id}` |
| **Auth**     | Session (PROPERTY_MGR+)    |
| **Body**     | Same fields as Create      |

### 3.5 Delete Room Type

|                 |                                       |
| --------------- | ------------------------------------- |
| **Endpoint**    | `DELETE /api/room-types/{id}`         |
| **Auth**        | Session (PROPERTY_MGR+)               |
| **Status**      | `204 No Content`                      |
| **Constraints** | Cannot delete if rooms are associated |

---

## 4. Rooms

### 4.1 List Rooms

|              |                                   |
| ------------ | --------------------------------- |
| **Endpoint** | `GET /api/rooms`                  |
| **Auth**     | Session + `x-property-id` header  |
| **Response** | Array of rooms with roomType info |

### 4.2 Get Room

|              |                                                |
| ------------ | ---------------------------------------------- |
| **Endpoint** | `GET /api/rooms/{id}`                          |
| **Auth**     | Session + `x-property-id` header               |
| **Response** | Room with roomType, pricing, and property info |

### 4.3 Create Room

|              |                                                  |
| ------------ | ------------------------------------------------ |
| **Endpoint** | `POST /api/rooms`                                |
| **Auth**     | Session (PROPERTY_MGR+) + `x-property-id` header |
| **Status**   | `201 Created`                                    |

**Request Body:**

```json
{
  "name": "string (required)",
  "type": "string (required)",
  "capacity": "number (required)",
  "imageUrl": "string",
  "description": "string",
  "doorlockId": "string",
  "roomTypeId": "string",
  "basePrice": 0,
  "weekdayPrice": 0,
  "weekendPrice": 0,
  "availability": 1,
  "minLOS": null,
  "maxLOS": null,
  "closedToArrival": false,
  "closedToDeparture": false
}
```

### 4.4 Update Room

|              |                                                               |
| ------------ | ------------------------------------------------------------- |
| **Endpoint** | `PUT /api/rooms/{id}`                                         |
| **Auth**     | Session (PROPERTY_MGR+)                                       |
| **Body**     | `{ name, type, capacity, imageUrl, description, doorlockId }` |

### 4.5 Delete Room

|                 |                                                 |
| --------------- | ----------------------------------------------- |
| **Endpoint**    | `DELETE /api/rooms/{id}`                        |
| **Auth**        | Session (PROPERTY_MGR+)                         |
| **Status**      | `204 No Content`                                |
| **Constraints** | Cannot delete if room has existing reservations |

### 4.6 Bulk Update Rooms

|              |                                                  |
| ------------ | ------------------------------------------------ |
| **Endpoint** | `PUT /api/rooms/bulk-update`                     |
| **Auth**     | Session (PROPERTY_MGR+) + `x-property-id` header |

**Request Body:** Array of room objects:

```json
[
  {
    "id": "string (required)",
    "name": "string (required)",
    "description": "string",
    "doorlockId": "string",
    "imageUrl": "string"
  }
]
```

---

## 5. Room Availability

### 5.1 Check Room Availability (Session-based)

|              |                                  |
| ------------ | -------------------------------- |
| **Endpoint** | `GET /api/rooms/availability`    |
| **Auth**     | Session + `x-property-id` header |

**Query Parameters:**

| Param                | Type     | Required | Description                                   |
| -------------------- | -------- | -------- | --------------------------------------------- |
| `checkIn`            | ISO date | Yes      | Start date                                    |
| `checkOut`           | ISO date | Yes      | End date                                      |
| `excludeReservation` | string   | No       | Reservation ID to exclude from conflict check |

**Response:** Array of rooms with `available: boolean` and `conflictingReservations` array.

### 5.2 Available Rooms (Public-style)

|              |                            |
| ------------ | -------------------------- |
| **Endpoint** | `GET /api/rooms/available` |
| **Auth**     | Session                    |

**Query Parameters:**

| Param             | Type     | Required | Description                                               |
| ----------------- | -------- | -------- | --------------------------------------------------------- |
| `propertyId`      | string   | Yes      | Property to check                                         |
| `startDate`       | ISO date | Yes      | Start date                                                |
| `endDate`         | ISO date | Yes      | End date                                                  |
| `includeOccupied` | boolean  | No       | If `true`, returns ALL rooms with `available` status flag |

**Response:** Array of rooms (filtered to available only, unless `includeOccupied=true`). Also checks room blocks.

---

## 6. Reservations

### 6.1 List Reservations

|              |                                  |
| ------------ | -------------------------------- |
| **Endpoint** | `GET /api/reservations`          |
| **Auth**     | Session + `x-property-id` header |

**Query Parameters:**

| Param                 | Type                   | Required | Description      |
| --------------------- | ---------------------- | -------- | ---------------- |
| `status`              | ReservationStatus enum | No       | Filter by status |
| `start` / `startDate` | ISO date               | No       | Date range start |
| `end` / `endDate`     | ISO date               | No       | Date range end   |
| `roomId`              | string                 | No       | Filter by room   |

**Response:** `{ count, reservations: [...] }` — Each reservation includes room, property, payments, addons, and computed `paymentStatus`.

### 6.2 Get Reservation

|              |                                                                           |
| ------------ | ------------------------------------------------------------------------- |
| **Endpoint** | `GET /api/reservations/{id}`                                              |
| **Auth**     | Session + `x-property-id` header                                          |
| **Response** | Full reservation with room, property, payments, addons, and paymentStatus |

### 6.3 Create Reservation

|              |                                                |
| ------------ | ---------------------------------------------- |
| **Endpoint** | `POST /api/reservations`                       |
| **Auth**     | Session (FRONT_DESK+) + `x-property-id` header |
| **Status**   | `201 Created`                                  |

**Request Body:**

````json
{
  "roomId": "string (required)",
  "guestName": "string (required)",
  "checkIn": "ISO datetime (required)",
  "checkOut": "ISO datetime (required)",
  "adults": "number (required)",
  "children": 0,
  "notes": "string",
  "phone": "string",
  "email": "string",
  "idType": "string",
  "idNumber": "string",
  "issuingCountry": "string",
  "guestImageUrl": "string",
  "idDocumentUrl": "string",
  "idExpiryDate": "ISO date",
  "idDocumentExpired": false,
  "source": "WALK_IN | WEBSITE | PHONE | OTA | AGENT | OTHER",
  "payment": {
    "paymentMethod": "card | cash | bank_transfer | pay_at_checkin",
    "totalAmount": 0,
    "creditCard": {
      "paymentMethodId": "string (Stripe PM ID)",
      "brand": "string",
      "last4": "string",
      "expiryMonth": 12,
      "expiryYear": 2027
    }
  },
  "addons": {
    "extraBed": false,
    "breakfast": false,
    "customAddons": [{ "name": "string", "price": 0, "selected": true }]
  }

### 6.4 Update Reservation

|              |                                \]
|
|--------------|--------------------------------|
| **Endpoint** | `PATCH /api/reservations/{id}` |
| **Auth**     | Session (FRONT_DESK+)          |

**Request Body (all fields optional):**
```json
{
  "guestName": "string",
  "checkIn": "ISO datetime",
  "checkOut": "ISO datetime",
  "adults": 2,
  "children": 0,
  "notes": "string",
  "phone": "string",
  "email": "string",
  "idType": "string",
  "idNumber": "string",
  "issuingCountry": "string",
  "guestImageUrl": "string",
  "idDocumentUrl": "string",
  "idExpiryDate": "ISO date",
  "idDocumentExpired": false,
  "status": "ReservationStatus enum",
  "roomId": "string (to reassign room)"
}
````

**Error `409`:** Returned if the new dates/room have conflicting reservations.

### 6.5 Delete Reservation

|              |                                   |
| ------------ | --------------------------------- |
| **Endpoint** | `DELETE /api/reservations/{id}`   |
| **Auth**     | Session (FRONT_DESK+)             |
| **Status**   | `204 No Content`                  |
| **Note**     | Deletes associated payments first |

### 6.6 Bulk Status Update

|              |                                                  |
| ------------ | ------------------------------------------------ |
| **Endpoint** | `POST /api/reservations/bulk-status`             |
| **Auth**     | Session (PROPERTY_MGR+) + `x-property-id` header |

**Request Body:**

```json
{
  "reservationIds": ["string"],
  "newStatus": "ReservationStatus enum (required)",
  "reason": "string",
  "updatedBy": "string (user ID)"
}
```

**Constraints:** Max 100 reservations per request. All status transitions are validated.

**Response:** `{ success, message, results: [...], summary: { totalRequested, totalUpdated, newStatus } }`

---

## 7. Rates & Pricing

### 7.1 Get Rates Matrix

|              |                                  |
| ------------ | -------------------------------- |
| **Endpoint** | `GET /api/rates`                 |
| **Auth**     | Session + `x-property-id` header |

**Query Parameters:**

| Param        | Type     | Required | Default | Description                    |
| ------------ | -------- | -------- | ------- | ------------------------------ |
| `startDate`  | ISO date | No       | today   | Start of date range            |
| `days`       | number   | No       | 7       | Number of days                 |
| `ratePlan`   | string   | No       | "base"  | "base" or "promo"              |
| `applyRules` | boolean  | No       | false   | Apply business rules to prices |

**Response:** `{ success, data: [{ roomTypeId, roomTypeName, totalRooms, dates: { [date]: { basePrice, finalPrice, availability, isOverride, isSeasonal, restrictions, appliedRules } } }], dateRange, businessRulesEnabled }`

### 7.2 Bulk Update Rates

|              |                                                  |
| ------------ | ------------------------------------------------ |
| **Endpoint** | `POST /api/rates`                                |
| **Auth**     | Session (PROPERTY_MGR+) + `x-property-id` header |

**Request Body:**

```json
{
  "updates": [
    {
      "roomTypeId": "string (required)",
      "date": "ISO date (required)",
      "price": "number (required)",
      "availability": "number",
      "restrictions": {
        "minLOS": null,
        "maxLOS": null,
        "closedToArrival": false,
        "closedToDeparture": false
      }
    }
  ]
}
```

### 7.3 Update Single Room Type Rate

|              |                                 |
| ------------ | ------------------------------- |
| **Endpoint** | `PATCH /api/rates/{roomTypeId}` |
| **Auth**     | Session (PROPERTY_MGR+)         |

**Request Body:**

```json
{
  "date": "ISO date (optional — if omitted, updates base rate)",
  "price": "number (required, >= 0)",
  "availability": "number",
  "restrictions": {
    "minLOS": null,
    "maxLOS": null,
    "closedToArrival": false,
    "closedToDeparture": false
  },
  "ratePlan": "base | weekday | weekend (default: base)"
}
```

### 7.4 Delete Daily Rate Override

|              |                                                                  |
| ------------ | ---------------------------------------------------------------- |
| **Endpoint** | `DELETE /api/rates/{roomTypeId}?date=YYYY-MM-DD`                 |
| **Auth**     | Session (PROPERTY_MGR+)                                          |
| **Response** | `{ success, message, data: { roomTypeId, date, deletedPrice } }` |

---

## 8. Seasonal Rates

### 8.1 List Seasonal Rates

|              |                                  |
| ------------ | -------------------------------- |
| **Endpoint** | `GET /api/rates/seasonal`        |
| **Auth**     | Session + `x-property-id` header |

**Query Parameters:**

| Param        | Type    | Required | Default | Description             |
| ------------ | ------- | -------- | ------- | ----------------------- |
| `roomTypeId` | string  | No       | —       | Filter by room type     |
| `active`     | boolean | No       | true    | Filter by active status |

### 8.2 Create Seasonal Rate

|              |                            |
| ------------ | -------------------------- |
| **Endpoint** | `POST /api/rates/seasonal` |
| **Auth**     | Session (PROPERTY_MGR+)    |

**Request Body:**

```json
{
  "name": "string (required)",
  "startDate": "ISO date (required)",
  "endDate": "ISO date (required)",
  "multiplier": "number > 0 (required, e.g. 1.5 = 50% increase)",
  "roomTypeId": "string (optional — null for property-wide)",
  "isActive": true
}
```

### 8.3 Update Seasonal Rate

|              |                           |
| ------------ | ------------------------- |
| **Endpoint** | `PUT /api/rates/seasonal` |
| **Auth**     | Session (PROPERTY_MGR+)   |

**Request Body:**

```json
{
  "id": "string (required)",
  "name": "string",
  "startDate": "ISO date",
  "endDate": "ISO date",
  "multiplier": "number > 0",
  "isActive": true
}
```

### 8.4 Delete Seasonal Rate

|              |                                      |
| ------------ | ------------------------------------ |
| **Endpoint** | `DELETE /api/rates/seasonal?id={id}` |
| **Auth**     | Session (PROPERTY_MGR+)              |

---

## 9. Room Blocks

### 9.1 List Room Blocks

|              |                                         |
| ------------ | --------------------------------------- |
| **Endpoint** | `GET /api/room-blocks?propertyId={id}`  |
| **Auth**     | Session                                 |
| **Response** | Array of blocks with room and user info |

### 9.2 Get Room Block

|              |                             |
| ------------ | --------------------------- |
| **Endpoint** | `GET /api/room-blocks/{id}` |
| **Auth**     | Session                     |

### 9.3 Create Room Block

|              |                         |
| ------------ | ----------------------- |
| **Endpoint** | `POST /api/room-blocks` |
| **Auth**     | Session                 |

**Request Body:**

```json
{
  "organizationId": "string (required)",
  "propertyId": "string (required)",
  "roomId": "string (required)",
  "startDate": "ISO date (required)",
  "endDate": "ISO date (required)",
  "blockType": "MAINTENANCE | OUT_OF_ORDER | RESERVED | OTHER (required)",
  "reason": "string"
}
```

**Error `409`:** Returned if overlapping blocks or active reservations exist.

### 9.4 Update Room Block

|              |                                             |
| ------------ | ------------------------------------------- |
| **Endpoint** | `PUT /api/room-blocks/{id}`                 |
| **Auth**     | Session                                     |
| **Body**     | `{ startDate, endDate, blockType, reason }` |

### 9.5 Delete Room Block

|              |                                |
| ------------ | ------------------------------ |
| **Endpoint** | `DELETE /api/room-blocks/{id}` |
| **Auth**     | Session                        |

---

## 10. Payments

### 10.1 Authorize Payment

|              |                                |
| ------------ | ------------------------------ |
| **Endpoint** | `POST /api/payments/authorize` |
| **Auth**     | Session                        |

**Request Body:**

```json
{
  "reservationId": "string (required)",
  "amount": "number (in cents, required)",
  "currency": "string (e.g. 'usd', required)"
}
```

**Response:** `{ clientSecret, paymentIntentId }` — Uses Stripe Connect on the organization's connected account.

### 10.2 Capture Payment

|              |                              |
| ------------ | ---------------------------- |
| **Endpoint** | `POST /api/payments/capture` |
| **Auth**     | Session                      |

**Request Body:**

```json
{
  "reservationId": "string (required)"
}
```

**Response:** `{ status, amount }` — Captures a previously authorized payment.

### 10.3 Refund Payment

|              |                             |
| ------------ | --------------------------- |
| **Endpoint** | `POST /api/payments/refund` |
| **Auth**     | Session                     |

**Request Body:**

```json
{
  "reservationId": "string (required)",
  "amount": "number (in cents, optional — omit for full refund)",
  "reason": "duplicate | fraudulent | requested_by_customer"
}
```

**Response:** `{ status, amount, refundId }`

---

## 11. Customer Search

|                      |                                       |
| -------------------- | ------------------------------------- |
| **Endpoint**         | `GET /api/customers/search?q={query}` |
| **Auth**             | None (public)                         |
| **Min query length** | 2 characters                          |

Searches reservations by `guestName`, `email`, `phone`, or `idNumber` (case-insensitive). Returns up to 5 results, deduplicated by email.

---

## 12. Enums & Constants

### ReservationStatus

```
CONFIRMATION_PENDING | CONFIRMED | IN_HOUSE | CHECKED_OUT | CANCELLED | NO_SHOW
```

### ReservationSource

```
WALK_IN | WEBSITE | PHONE | OTA | AGENT | OTHER
```

### BlockType

```
MAINTENANCE | OUT_OF_ORDER | RESERVED | OTHER
```

### Role Hierarchy (highest → lowest)

```
SUPER_ADMIN → ORG_ADMIN → PROPERTY_MGR → FRONT_DESK → HOUSEKEEPING → MAINTENANCE
```

### Payment Status (computed)

```
UNPAID | PARTIALLY_PAID | PAID | refunded | partially_refunded
```

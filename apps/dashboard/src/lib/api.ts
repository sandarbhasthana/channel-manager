import { cookies } from "next/headers";
import { redirect, unstable_rethrow } from "next/navigation";

const API_BASE = process.env.API_URL ?? "http://localhost:8080";

// ── proto-mapped types ─────────────────────────────────────────────────────

export type ChannelKind =
  | "CHANNEL_KIND_UNSPECIFIED"
  | "CHANNEL_KIND_BOOKING_COM"
  | "CHANNEL_KIND_AIRBNB"
  | "CHANNEL_KIND_EXPEDIA"
  | "CHANNEL_KIND_AGODA"
  | "CHANNEL_KIND_DIRECT";

export type ConnectionStatus =
  | "CONNECTION_STATUS_UNSPECIFIED"
  | "CONNECTION_STATUS_INACTIVE"
  | "CONNECTION_STATUS_ACTIVE"
  | "CONNECTION_STATUS_ERROR"
  | "CONNECTION_STATUS_DISABLED";

export interface Connection {
  id: string;
  kind: ChannelKind;
  name: string;
  status: ConnectionStatus;
  createdAt: string;
  updatedAt: string;
}

// ── auth-forwarding fetch ──────────────────────────────────────────────────

async function rpc<T>(procedure: string, body: unknown): Promise<T> {
  const cookieStore = await cookies();
  const token = cookieStore.get("access_token")?.value;
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    Cookie: cookieStore.toString(),
  };
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}${procedure}`, {
    method: "POST",
    headers,
    body: JSON.stringify(body),
    cache: "no-store",
  });

  if (!res.ok) {
    if (res.status === 401) {
      redirect("/login");
    }
    const text = await res.text().catch(() => "");
    throw new Error(`RPC ${procedure} failed [${res.status}]: ${text}`);
  }
  return res.json() as Promise<T>;
}

// ── ConnectionService ──────────────────────────────────────────────────────

export async function listConnections(): Promise<Connection[]> {
  try {
    const data = await rpc<{ connections?: Connection[] }>(
      "/channel.v1.ConnectionService/ListConnections",
      {}
    );
    return data.connections ?? [];
  } catch (err) {
    unstable_rethrow(err);
    return [];
  }
}

export async function createConnection(input: {
  kind: ChannelKind;
  name: string;
  credentials: Record<string, string>;
}): Promise<Connection> {
  const data = await rpc<{ connection: Connection }>(
    "/channel.v1.ConnectionService/CreateConnection",
    input
  );
  return data.connection;
}

export async function updateConnectionStatus(
  id: string,
  status: ConnectionStatus
): Promise<Connection> {
  const data = await rpc<{ connection: Connection }>(
    "/channel.v1.ConnectionService/UpdateConnection",
    { id, status }
  );
  return data.connection;
}

export async function deleteConnection(id: string): Promise<void> {
  await rpc("/channel.v1.ConnectionService/DeleteConnection", { id });
}

// ── Integration Keys (REST) ────────────────────────────────────────────────

export interface IntegrationKey {
  id: string;
  org_id: string;
  name: string;
  key_prefix: string;
  scopes: string[];
  created_at: string;
  expires_at?: string;
  revoked_at?: string;
}

export interface CreateKeyResponse {
  record: IntegrationKey;
  secret_key: string;
}

async function rest<T>(path: string, method: string, body?: unknown): Promise<T> {
  const cookieStore = await cookies();
  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers: {
      "Content-Type": "application/json",
      Cookie: cookieStore.toString(),
    },
    body: body ? JSON.stringify(body) : undefined,
    cache: "no-store",
  });

  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`REST ${path} failed [${res.status}]: ${text}`);
  }
  if (res.status === 204) return {} as T;
  return res.json() as Promise<T>;
}

export async function listIntegrationKeys(): Promise<IntegrationKey[]> {
  try {
    const data = await rest<{ keys: IntegrationKey[] }>("/admin/integration-keys", "GET");
    return data.keys || [];
  } catch (err) {
    unstable_rethrow(err);
    return [];
  }
}

export async function createIntegrationKey(name: string): Promise<CreateKeyResponse> {
  return await rest<CreateKeyResponse>("/admin/integration-keys", "POST", { name });
}

export async function revokeIntegrationKey(id: string): Promise<void> {
  await rest(`/admin/integration-keys/${id}`, "DELETE");
}

export interface MeResponse {
  user_id: string;
  org_id: string;
  role: string;
  email: string;
  full_name: string;
  preferences: Record<string, any>;
}

export async function updatePreferences(preferences: Record<string, any>): Promise<void> {
  await rest("/me/preferences", "PATCH", { preferences });
}

export interface Property {
  id: string;
  externalId?: string;
  external_id?: string;
  name: string;
  timezone: string;
  currency: string;
  status: string;
  createdAt?: string;
  created_at?: string;
}

export async function listProperties(): Promise<Property[]> {
  try {
    const data = await rpc<{ properties: Property[] }>("/pms.v1.PmsService/ListProperties", {});
    return data.properties || [];
  } catch (err) {
    unstable_rethrow(err);
    console.error("Error listing properties:", err);
    return [];
  }
}

export interface Room {
  id: string;
  name: string;
  external_id?: string;
  externalId?: string;
  is_active?: boolean;
  isActive?: boolean;
}

export interface RoomType {
  id: string;
  name: string;
  code: string;
  external_id?: string;
  externalId?: string;
  max_occupancy?: number;
  maxOccupancy?: number;
  base_occupancy?: number;
  baseOccupancy?: number;
  rooms?: Room[];
}

export async function listRoomTypes(propertyId: string): Promise<RoomType[]> {
  try {
    const data = await rpc<{ roomTypes: RoomType[] }>("/pms.v1.PmsService/ListRoomTypes", { property_id: propertyId });
    return data.roomTypes || [];
  } catch (err) {
    unstable_rethrow(err);
    console.error("Error listing room types:", err);
    return [];
  }
}

export async function getMe(): Promise<MeResponse | null> {
  try {
    return await rest<MeResponse>("/me", "GET");
  } catch (err) {
    unstable_rethrow(err);
    return null;
  }
}

export interface PmsBooking {
  bookingId: string;
  booking_id?: string;
  status: string;
  guestName: string;
  guest_name?: string;
  email: string;
  phone: string;
  roomId: string;
  room_id?: string;
  roomName: string;
  room_name?: string;
  roomType: string;
  room_type?: string;
  propertyName: string;
  property_name?: string;
  checkin: string;
  checkout: string;
  adults: number;
  children: number;
  notes: string;
  paymentStatus: string;
  payment_status?: string;
  source: string;
  message: string;
}

export async function listBookings(propertyId: string, status?: string): Promise<PmsBooking[]> {
  try {
    const data = await rpc<{ bookings: PmsBooking[] }>("/pms.v1.PmsService/ListBookings", {
      property_id: propertyId,
      status: status || ""
    });
    return data.bookings || [];
  } catch (err) {
    unstable_rethrow(err);
    console.error("Error listing bookings:", err);
    return [];
  }
}

export async function updateBooking(propertyId: string, bookingId: string, data: any): Promise<PmsBooking> {
  try {
    const res = await rpc<{ booking: PmsBooking }>("/pms.v1.PmsService/UpdateBooking", {
      property_id: propertyId,
      booking_id: bookingId,
      ...data
    });
    return res.booking;
  } catch (err) {
    unstable_rethrow(err);
    console.error("Error updating booking:", err);
    throw err;
  }
}

export async function deleteBooking(propertyId: string, bookingId: string): Promise<boolean> {
  try {
    await rpc("/pms.v1.PmsService/DeleteBooking", {
      property_id: propertyId,
      booking_id: bookingId
    });
    return true;
  } catch (err) {
    unstable_rethrow(err);
    console.error("Error deleting booking:", err);
    throw err;
  }
}

// ── Pricing Service (Rates) ──────────────────────────────────────────────────

export interface RatePoint {
  property_id?: string;
  propertyId?: string;
  room_type_id?: string;
  roomTypeId?: string;
  rate_plan_id?: string;
  ratePlanId?: string;
  date: {
    year: number;
    month: number;
    day: number;
  };
  amount: {
    currency_code: string;
    units: number;
    nanos: number;
  };
}

export async function getRates(propertyId: string, startDate: string, endDate: string): Promise<RatePoint[]> {
  try {
    const startParts = startDate.split("-").map(Number);
    const endParts = endDate.split("-").map(Number);
    const data = await rpc<{ points: RatePoint[] }>("/pricing.v1.PricingService/GetRates", {
      property_id: propertyId,
      room_type_id: "",
      rate_plan_id: "",
      range: {
        start: { year: startParts[0], month: startParts[1], day: startParts[2] },
        end: { year: endParts[0], month: endParts[1], day: endParts[2] }
      }
    });
    return data.points || [];
  } catch (err) {
    unstable_rethrow(err);
    console.error("Error fetching rates:", err);
    return [];
  }
}

export async function bulkUpsertRates(points: RatePoint[]): Promise<boolean> {
  try {
    await rpc("/pricing.v1.PricingService/BulkUpsertRates", {
      points,
      idempotency_key: { key: `upsert-rates-${Date.now()}` },
      audit: { actor_id: "system", action: "bulk_upsert_rates", reason: "calendar update" }
    });
    return true;
  } catch (err) {
    unstable_rethrow(err);
    console.error("Error upserting rates:", err);
    throw err;
  }
}

// ── BookingEngineService ────────────────────────────────────────────────────
//
// The direct (own-website) sales channel. Field casing follows Connect's JSON
// codec: responses are camelCase, and int64 (Money.amountMinor) arrives as a
// string. CalendarDate is { year, month, day }; timestamps are RFC3339 strings.

export interface DirectReservation {
  id: string;
  propertyId: string;
  confirmationCode: string;
  guestName: string;
  checkIn?: { year: number; month: number; day: number };
  checkOut?: { year: number; month: number; day: number };
  status: string;
  total?: { amountMinor: string; currency: string };
  bookedAt: string;
}

export interface BookingEngineSettings {
  propertyId: string;
  directChannelEnabled: boolean;
  // "pms" | "cm" — where the booking engine routes stay actions (Phase 4).
  bookingRoute?: string;
  // 0–100 canary ramp for the "cm" route.
  bookingRoutePercent?: number;
}

export async function listDirectReservations(
  propertyId: string,
  pageToken = ""
): Promise<{ reservations: DirectReservation[]; nextPageToken: string }> {
  try {
    const data = await rpc<{ reservations?: DirectReservation[]; page?: { nextPageToken?: string } }>(
      "/bookingengine.v1.BookingEngineService/ListDirectReservations",
      { property_id: propertyId, page: { page_size: 50, page_token: pageToken } }
    );
    return {
      reservations: data.reservations ?? [],
      nextPageToken: data.page?.nextPageToken ?? "",
    };
  } catch (err) {
    unstable_rethrow(err);
    console.error("Error listing direct reservations:", err);
    return { reservations: [], nextPageToken: "" };
  }
}

export async function getBookingEngineSettings(propertyId: string): Promise<BookingEngineSettings | null> {
  try {
    const data = await rpc<{ settings: BookingEngineSettings }>(
      "/bookingengine.v1.BookingEngineService/GetSettings",
      { property_id: propertyId }
    );
    return data.settings ?? null;
  } catch (err) {
    unstable_rethrow(err);
    console.error("Error fetching booking-engine settings:", err);
    return null;
  }
}

export async function updateBookingEngineSettings(input: {
  propertyId: string;
  directChannelEnabled: boolean;
  bookingRoute: string;
  bookingRoutePercent: number;
}): Promise<BookingEngineSettings> {
  const data = await rpc<{ settings: BookingEngineSettings }>(
    "/bookingengine.v1.BookingEngineService/UpdateSettings",
    {
      property_id: input.propertyId,
      direct_channel_enabled: input.directChannelEnabled,
      booking_route: input.bookingRoute,
      booking_route_percent: input.bookingRoutePercent,
    }
  );
  return data.settings;
}

// ── Promo codes (coupons) ───────────────────────────────────────────────────
// CM owns promo definitions and the redemption counter. Responses are camelCase
// (protojson default); max_uses 0 means unlimited.

export interface PromoCode {
  id: string;
  propertyId?: string;
  code: string;
  description?: string;
  discountPct: number;
  maxUses?: number; // 0 / absent = unlimited
  uses?: number;
  validFrom?: string;
  validUntil?: string;
  isActive: boolean;
  createdAt?: string;
  updatedAt?: string;
}

export interface PromoCodeInput {
  id?: string;
  propertyId?: string;
  code: string;
  description?: string;
  discountPct: number;
  maxUses?: number;
  validFrom?: string;
  validUntil?: string;
  isActive: boolean;
}

function toPromoWire(input: PromoCodeInput): Record<string, unknown> {
  return {
    id: input.id,
    property_id: input.propertyId ?? "",
    code: input.code,
    description: input.description ?? "",
    discount_pct: input.discountPct,
    max_uses: input.maxUses ?? 0,
    valid_from: input.validFrom,
    valid_until: input.validUntil,
    is_active: input.isActive,
  };
}

export async function listPromoCodes(): Promise<PromoCode[]> {
  try {
    const data = await rpc<{ promoCodes?: PromoCode[] }>(
      "/pricing.v1.PricingService/ListPromoCodes",
      {}
    );
    return data.promoCodes ?? [];
  } catch (err) {
    unstable_rethrow(err);
    console.error("Error listing promo codes:", err);
    return [];
  }
}

export async function createPromoCode(input: PromoCodeInput): Promise<PromoCode> {
  const data = await rpc<{ promoCode: PromoCode }>(
    "/pricing.v1.PricingService/CreatePromoCode",
    { promo_code: toPromoWire(input) }
  );
  return data.promoCode;
}

export async function updatePromoCode(input: PromoCodeInput): Promise<PromoCode> {
  const data = await rpc<{ promoCode: PromoCode }>(
    "/pricing.v1.PricingService/UpdatePromoCode",
    { promo_code: toPromoWire(input) }
  );
  return data.promoCode;
}

export async function deletePromoCode(id: string): Promise<void> {
  await rpc("/pricing.v1.PricingService/DeletePromoCode", { id });
}

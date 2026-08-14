"use server";

import { listProperties, listBookings, getDefaultPropertyId, setDefaultPropertyId, Property, PmsBooking, deleteBooking, updateBooking, listRoomTypes, RoomType, getRates, RatePoint, bulkUpsertRates } from "@/lib/api";
import { unstable_rethrow } from "next/navigation";

// Sets the org's default property. Formerly a per-user preference; now shared
// org-wide because the booking engine reads the same value off the storefront
// listing to decide which property to sell when a guest names none.
export async function setUserDefaultProperty(propertyId: string): Promise<void> {
  try {
    await setDefaultPropertyId(propertyId);
  } catch (error) {
    unstable_rethrow(error);
    console.error("Failed to set default property:", error);
    throw new Error(`Failed to set default property: ${(error as Error).message}`);
  }
}

export async function fetchDashboardData(): Promise<{ properties: Property[], bookings: PmsBooking[], defaultPropertyId: string | null }> {
  try {
    const properties = await listProperties();
    if (!properties || properties.length === 0) {
      return { properties: [], bookings: [], defaultPropertyId: null };
    }

    // The starred property, org-wide. Null when no property has been starred yet.
    const starred = await getDefaultPropertyId();
    const defaultPropertyId =
      starred && properties.some(p => p.id === starred) ? starred : null;

    // The picker still has to open on something, so fall back to the first
    // property for the initial selection — but report defaultPropertyId
    // honestly, so the star renders only on a property that is actually starred.
    const selectedPropertyId = defaultPropertyId ?? properties[0].id;

    const bookings = await listBookings(selectedPropertyId);

    return { properties, bookings, defaultPropertyId };
  } catch (error) {
    unstable_rethrow(error);
    console.error("Failed to fetch dashboard data:", error);
    return { properties: [], bookings: [], defaultPropertyId: null };
  }
}

export async function fetchBookingsForProperty(propertyId: string): Promise<PmsBooking[]> {
  try {
    const bookings = await listBookings(propertyId);
    return bookings || [];
  } catch (error) {
    unstable_rethrow(error);
    console.error("Failed to fetch bookings for property:", error);
    return [];
  }
}

export async function updateBookingAction(propertyId: string, bookingId: string, data: any): Promise<PmsBooking> {
  try {
    return await updateBooking(propertyId, bookingId, data);
  } catch (error) {
    unstable_rethrow(error);
    console.error("Failed to update booking:", error);
    throw new Error(`Failed to update booking: ${(error as Error).message}`);
  }
}

export async function deleteBookingAction(propertyId: string, bookingId: string): Promise<void> {
  try {
    await deleteBooking(propertyId, bookingId);
  } catch (error) {
    unstable_rethrow(error);
    console.error("Failed to delete booking:", error);
    throw new Error(`Failed to delete booking: ${(error as Error).message}`);
  }
}

export async function fetchRoomTypesForProperty(propertyId: string): Promise<RoomType[]> {
  try {
    return await listRoomTypes(propertyId);
  } catch (error) {
    unstable_rethrow(error);
    console.error("Failed to fetch room types:", error);
    return [];
  }
}

export async function fetchRatesForProperty(propertyId: string, startDate: string, endDate: string): Promise<RatePoint[]> {
  try {
    return await getRates(propertyId, startDate, endDate);
  } catch (error) {
    unstable_rethrow(error);
    console.error("Failed to fetch rates:", error);
    return [];
  }
}

export async function bulkUpsertRatesAction(points: RatePoint[]): Promise<boolean> {
  try {
    return await bulkUpsertRates(points);
  } catch (error) {
    unstable_rethrow(error);
    console.error("Failed to upsert rates:", error);
    throw new Error(`Failed to upsert rates: ${(error as Error).message}`);
  }
}

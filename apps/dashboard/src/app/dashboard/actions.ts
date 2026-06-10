"use server";

import { listProperties, listBookings, getMe, updatePreferences, Property, PmsBooking, deleteBooking } from "@/lib/api";
import { unstable_rethrow } from "next/navigation";

export async function setUserDefaultProperty(propertyId: string): Promise<void> {
  try {
    await updatePreferences({ default_property_id: propertyId });
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
    
    // Get user preferences
    const me = await getMe();
    let selectedPropertyId = properties[0].id;
    const defaultPropertyId = me?.preferences?.default_property_id || null;
    
    if (defaultPropertyId && properties.some(p => p.id === defaultPropertyId)) {
      selectedPropertyId = defaultPropertyId;
    }

    const bookings = await listBookings(selectedPropertyId);
    
    return { properties, bookings, defaultPropertyId: selectedPropertyId };
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

export async function deleteBookingAction(propertyId: string, bookingId: string): Promise<void> {
  try {
    await deleteBooking(propertyId, bookingId);
  } catch (error) {
    unstable_rethrow(error);
    console.error("Failed to delete booking:", error);
    throw new Error(`Failed to delete booking: ${(error as Error).message}`);
  }
}

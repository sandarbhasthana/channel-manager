"use server";

import { listProperties, listBookings, Property, PmsBooking } from "@/lib/api";

export async function fetchDashboardData(): Promise<{ properties: Property[], bookings: PmsBooking[] }> {
  try {
    const properties = await listProperties();
    if (!properties || properties.length === 0) {
      return { properties: [], bookings: [] };
    }
    
    // Default to the first property for the overview dashboard
    const firstPropertyId = properties[0].id;
    const bookings = await listBookings(firstPropertyId);
    
    return { properties, bookings };
  } catch (error) {
    console.error("Failed to fetch dashboard data:", error);
    return { properties: [], bookings: [] };
  }
}

export async function fetchBookingsForProperty(propertyId: string): Promise<PmsBooking[]> {
  try {
    const bookings = await listBookings(propertyId);
    return bookings || [];
  } catch (error) {
    console.error(`Failed to fetch bookings for property ${propertyId}:`, error);
    return [];
  }
}

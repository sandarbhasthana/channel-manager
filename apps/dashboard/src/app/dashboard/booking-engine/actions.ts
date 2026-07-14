"use server";

import { revalidatePath } from "next/cache";
import { unstable_rethrow } from "next/navigation";
import {
  listDirectReservations,
  getBookingEngineSettings,
  updateBookingEngineSettings,
  listPromoCodes,
  createPromoCode,
  updatePromoCode,
  deletePromoCode,
  type DirectReservation,
  type BookingEngineSettings,
  type PromoCode,
  type PromoCodeInput,
} from "@/lib/api";

export async function fetchDirectReservations(propertyId: string): Promise<DirectReservation[]> {
  try {
    const { reservations } = await listDirectReservations(propertyId);
    return reservations;
  } catch (err) {
    unstable_rethrow(err);
    console.error("Failed to fetch direct reservations:", err);
    return [];
  }
}

export async function fetchBookingEngineSettings(propertyId: string): Promise<BookingEngineSettings | null> {
  try {
    return await getBookingEngineSettings(propertyId);
  } catch (err) {
    unstable_rethrow(err);
    console.error("Failed to fetch booking-engine settings:", err);
    return null;
  }
}

export async function saveBookingEngineSettingsAction(input: {
  propertyId: string;
  directChannelEnabled: boolean;
  bookingRoute: string;
  bookingRoutePercent: number;
}): Promise<BookingEngineSettings> {
  try {
    const settings = await updateBookingEngineSettings(input);
    revalidatePath("/dashboard/booking-engine");
    return settings;
  } catch (err) {
    unstable_rethrow(err);
    throw new Error((err as Error).message);
  }
}

export async function fetchPromoCodes(): Promise<PromoCode[]> {
  return listPromoCodes();
}

export async function savePromoCodeAction(input: PromoCodeInput): Promise<PromoCode> {
  try {
    const saved = input.id ? await updatePromoCode(input) : await createPromoCode(input);
    revalidatePath("/dashboard/booking-engine");
    return saved;
  } catch (err) {
    unstable_rethrow(err);
    throw new Error((err as Error).message);
  }
}

export async function deletePromoCodeAction(id: string): Promise<void> {
  try {
    await deletePromoCode(id);
    revalidatePath("/dashboard/booking-engine");
  } catch (err) {
    unstable_rethrow(err);
    throw new Error((err as Error).message);
  }
}

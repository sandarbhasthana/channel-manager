"use server";

import { revalidatePath } from "next/cache";
import { unstable_rethrow } from "next/navigation";
import {
  listRoomTypes,
  listConnections,
  listChannelRateRules,
  saveChannelRateRules,
  getBaseQuote,
  listRoomBaseRates,
  saveRoomBaseRates,
  type RoomType,
  type Connection,
  type ChannelRateRule,
  type StoredBaseRate,
} from "@/lib/api";

export interface RoomBaseRate {
  roomTypeId: string;
  pricePerNight: number | null;
  currency: string | null;
}

export async function fetchRoomRatesData(
  propertyId: string
): Promise<{
  roomTypes: RoomType[];
  channels: Connection[];
  rules: ChannelRateRule[];
  baseRates: RoomBaseRate[];
}> {
  const [roomTypes, channels, rules, storedBases] = await Promise.all([
    listRoomTypes(propertyId),
    listConnections(),
    listChannelRateRules(propertyId),
    listRoomBaseRates(propertyId),
  ]);
  const storedByRt = new Map(storedBases.map((b) => [b.roomTypeId, b]));

  // Base rate per room type: prefer the CM-stored base; otherwise quote a
  // representative active room live from the PMS (external_id, as the booking
  // flow does). The per-channel adjustment is applied on top of this base.
  const baseRates = await Promise.all(
    roomTypes.map(async (rt): Promise<RoomBaseRate> => {
      const stored = storedByRt.get(rt.id);
      if (stored) return { roomTypeId: rt.id, pricePerNight: stored.amount, currency: stored.currency };
      const rooms = rt.rooms || [];
      // Only quote a room that has a real PMS external id. Falling back to the CM
      // room id (a zombie room with no external id) sends an id the PMS can't
      // match, producing a 404 — so prefer a room with an external id and skip
      // id-less (zombie) rooms entirely.
      const room =
        rooms.find((r) => (r.is_active ?? r.isActive ?? true) && (r.externalId ?? r.external_id)) ||
        rooms.find((r) => r.externalId ?? r.external_id) ||
        null;
      const pmsRoomId = room ? (room.externalId ?? room.external_id ?? null) : null;
      if (!pmsRoomId) return { roomTypeId: rt.id, pricePerNight: null, currency: null };
      const adults = rt.base_occupancy ?? rt.baseOccupancy ?? 2;
      const q = await getBaseQuote(propertyId, pmsRoomId, adults && adults > 0 ? adults : 2);
      const price = q && q.pricePerNight > 0 ? q.pricePerNight : null;
      return { roomTypeId: rt.id, pricePerNight: price, currency: q?.currency ?? null };
    })
  );

  return { roomTypes, channels, rules, baseRates };
}

export async function saveChannelRateRulesAction(
  propertyId: string,
  rules: ChannelRateRule[]
): Promise<void> {
  try {
    await saveChannelRateRules(propertyId, rules);
    revalidatePath("/dashboard/room-rates");
  } catch (err) {
    unstable_rethrow(err);
    throw new Error((err as Error).message);
  }
}

// Saves the CM-stored base rates and the per-channel adjustments together.
export async function saveRoomRatesAction(
  propertyId: string,
  baseRates: StoredBaseRate[],
  rules: ChannelRateRule[]
): Promise<void> {
  try {
    await Promise.all([
      saveRoomBaseRates(propertyId, baseRates),
      saveChannelRateRules(propertyId, rules),
    ]);
    revalidatePath("/dashboard/room-rates");
  } catch (err) {
    unstable_rethrow(err);
    throw new Error((err as Error).message);
  }
}

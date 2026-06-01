"use server";

import { listRoomTypes as apiListRoomTypes, RoomType } from "@/lib/api";

export async function fetchRoomTypesAction(propertyId: string): Promise<RoomType[]> {
  return await apiListRoomTypes(propertyId);
}

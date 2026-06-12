"use client";

import { useMemo } from "react";

import { getEventColor, getBlockColor } from "../utils/eventColors";
import type { Reservation, RoomBlock } from "../utils/types";
import type { ProcessedEvent } from "../utils/types";

interface UseProcessedEventsProps {
  reservations: Reservation[];
  blocks: RoomBlock[];
  isDarkMode: boolean;
}

/**
 * Converts raw reservations and blocks into ProcessedEvent[] —
 * a plain color-mapped array usable by the custom calendar grid.
 * This replaces the FullCalendar eventSources callback protocol.
 */
export function useProcessedEvents({
  reservations,
  blocks,
  isDarkMode
}: UseProcessedEventsProps): ProcessedEvent[] {
  return useMemo(() => {
    const reservationEvents: ProcessedEvent[] = reservations.map((r) => {
      const colors = getEventColor(r.status, isDarkMode);
      return {
        id: r.id,
        roomId: r.roomId,
        title: r.guestName,
        start: r.checkIn,
        end: r.checkOut,
        backgroundColor: colors.backgroundColor,
        textColor: colors.textColor,
        status: r.status,
        paymentStatus: r.paymentStatus,
        depositAmount: r.depositAmount,
        rateLocked: r.rateLocked,
        paidAmount: r.paidAmount,
        unreadMessageCount: r.unreadMessageCount,
        guestName: r.guestName,
        adults: r.adults,
        children: r.children,
        notes: r.notes,
        source: r.source,
        isBlock: false,
        isPartialDay: true
      };
    });

    const blockEvents: ProcessedEvent[] = blocks.map((b) => {
      const colors = getBlockColor(b.blockType, isDarkMode);
      return {
        id: `block-${b.id}`,
        roomId: b.roomId,
        title: `🔒 ${b.blockType.replace(/_/g, " ")}`,
        // Normalize to YYYY-MM-DD: Prisma @db.Date serializes as a full ISO datetime
        // ("2025-05-10T00:00:00.000Z"). Slicing via UTC avoids any local-midnight
        // offset shifting the date by one day in non-UTC browsers.
        start: new Date(b.startDate).toISOString().slice(0, 10),
        end: new Date(b.endDate).toISOString().slice(0, 10),
        backgroundColor: colors.backgroundColor,
        textColor: colors.textColor,
        isBlock: true,
        blockId: b.id,
        blockType: b.blockType,
        reason: b.reason,
        isPartialDay: true
      };
    });

    return [...reservationEvents, ...blockEvents];
  }, [reservations, blocks, isDarkMode]);
}

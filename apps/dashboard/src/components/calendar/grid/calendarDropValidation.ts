import { diffDays } from "@/components/calendar/utils/calendarHelpers";
import type { ProcessedEvent } from "@/components/calendar/utils/types";
import { CALENDAR_COLORS } from "../calendarTheme";

export const DROP_BLOCKING_STATUSES = new Set([
  "CONFIRMED",
  "IN_HOUSE",
  "CONFIRMATION_PENDING",
  "CHECKIN_DUE",
  "CHECKOUT_DUE",
]);

export interface MoveTargetSnapshot {
  eventId?: string;
  event?: { id: string };
  targetRoomId?: string;
  currentTargetRoomId?: string;
  currentSnapDayOffset: number;
  currentRawDayOffset: number;
  durationDays: number;
  todayOff: number;
}

export function eventEndOffsetForDropConflict(event: ProcessedEvent, rangeStart: string): number {
  const endOff = diffDays(rangeStart, event.end);
  // Reservation check-out dates are exclusive. Room block end dates are inclusive.
  return event.isBlock ? endOff + 1 : endOff;
}

export function hasDropConflict(
  events: ProcessedEvent[],
  targetRoomId: string,
  draggingId: string,
  snapOff: number,
  durationDays: number,
  rangeStart: string,
): boolean {
  // Use integer day-offsets so Date comparisons are timezone-safe.
  // Both snapOff and diffDays() derive from the same UTC-midnight baseline.
  const newStartOff = snapOff;
  const newEndOff = snapOff + durationDays;
  for (const e of events) {
    if (e.id === draggingId || e.roomId !== targetRoomId) continue;
    if (!e.isBlock && (!e.status || !DROP_BLOCKING_STATUSES.has(e.status))) continue;
    const eStartOff = diffDays(rangeStart, e.start);
    const eEndOff = eventEndOffsetForDropConflict(e, rangeStart);
    if (newStartOff < eEndOff && newEndOff > eStartOff) return true;
  }
  return false;
}

export function getGhostDropValidityStyles(isInvalid: boolean, origBg: string) {
  return {
    background: isInvalid ? CALENDAR_COLORS.invalidDrop.bg : origBg,
    outline: isInvalid ? `2px solid ${CALENDAR_COLORS.invalidDrop.outline}` : "none",
  };
}

export function getInvalidMoveTarget(
  events: ProcessedEvent[],
  drag: MoveTargetSnapshot,
  rangeStart: string,
): { invalid: boolean; reason: "past" | "conflict" | null } {
  if (drag.currentRawDayOffset < drag.todayOff) return { invalid: true, reason: "past" };
  const eventId = drag.eventId ?? drag.event?.id;
  const targetRoomId = drag.targetRoomId ?? drag.currentTargetRoomId;
  if (!eventId || !targetRoomId) return { invalid: false, reason: null };
  if (
    hasDropConflict(
      events,
      targetRoomId,
      eventId,
      drag.currentSnapDayOffset,
      drag.durationDays,
      rangeStart,
    )
  ) {
    return { invalid: true, reason: "conflict" };
  }
  return { invalid: false, reason: null };
}

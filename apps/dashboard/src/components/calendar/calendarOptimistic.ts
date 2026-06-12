import type { ProcessedEvent } from "@/components/calendar/utils/types";

export interface PendingOptimistic {
  eventId: string;
  newRoomId: string;
  newStart: string;
  newEnd: string;
}

export function applyPendingOptimisticEvents(
  events: ProcessedEvent[],
  pendingDrop: PendingOptimistic | null,
): ProcessedEvent[] {
  if (!pendingDrop) return events;
  return events.map((event) =>
    event.id === pendingDrop.eventId
      ? {
          ...event,
          roomId: pendingDrop.newRoomId,
          start: pendingDrop.newStart,
          end: pendingDrop.newEnd,
        }
      : event,
  );
}

import type { ProcessedEvent } from "@/components/calendar/utils/types";
import { applyPendingOptimisticEvents } from "./calendarOptimistic";

const events: ProcessedEvent[] = [
  {
    id: "reservation-1",
    roomId: "room-1",
    title: "Guest A",
    start: "2026-06-10",
    end: "2026-06-12",
    backgroundColor: "#2563eb",
    textColor: "#fff",
    isBlock: false,
    status: "CONFIRMED",
  },
  {
    id: "reservation-2",
    roomId: "room-3",
    title: "Guest B",
    start: "2026-06-12",
    end: "2026-06-14",
    backgroundColor: "#16a34a",
    textColor: "#fff",
    isBlock: false,
    status: "CONFIRMED",
  },
];

describe("calendar optimistic preview verification", () => {
  it("CAL-046 previews the pending server-validated move without mutating source events", () => {
    const preview = applyPendingOptimisticEvents(events, {
      eventId: "reservation-1",
      newRoomId: "room-2",
      newStart: "2026-06-15",
      newEnd: "2026-06-17",
    });

    expect(preview[0]).toMatchObject({
      id: "reservation-1",
      roomId: "room-2",
      start: "2026-06-15",
      end: "2026-06-17",
    });
    expect(events[0]).toMatchObject({
      id: "reservation-1",
      roomId: "room-1",
      start: "2026-06-10",
      end: "2026-06-12",
    });
    expect(preview[1]).toBe(events[1]);
  });

  it("CAL-046 returns server-authoritative events after the pending preview is cleared", () => {
    expect(applyPendingOptimisticEvents(events, null)).toBe(events);
  });
});

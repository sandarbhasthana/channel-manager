import type { ProcessedEvent } from "@/components/calendar/utils/types";
import { CALENDAR_COLORS } from "../calendarTheme";
import {
  getGhostDropValidityStyles,
  getInvalidMoveTarget,
} from "./calendarDropValidation";

const baseEvent = (overrides: Partial<ProcessedEvent>): ProcessedEvent => ({
  id: "event-1",
  roomId: "room-1",
  title: "Guest",
  start: "2026-06-10",
  end: "2026-06-12",
  backgroundColor: "#2563eb",
  textColor: "#fff",
  isBlock: false,
  status: "CONFIRMED",
  ...overrides,
});

describe("calendar drop validation verification", () => {
  it("CAL-040 returns invalid ghost styling for blocked move targets", () => {
    expect(getGhostDropValidityStyles(true, "#2563eb")).toEqual({
      background: CALENDAR_COLORS.invalidDrop.bg,
      outline: `2px solid ${CALENDAR_COLORS.invalidDrop.outline}`,
    });

    expect(getGhostDropValidityStyles(false, "#2563eb")).toEqual({
      background: "#2563eb",
      outline: "none",
    });
  });

  it("CAL-040 marks past and occupied drop targets invalid before commit", () => {
    const occupied = baseEvent({
      id: "occupied",
      roomId: "room-2",
      start: "2026-06-12",
      end: "2026-06-14",
    });

    expect(
      getInvalidMoveTarget(
        [occupied],
        {
          eventId: "dragging",
          targetRoomId: "room-2",
          currentSnapDayOffset: 2,
          currentRawDayOffset: 2,
          durationDays: 2,
          todayOff: 0,
        },
        "2026-06-10",
      ),
    ).toEqual({ invalid: true, reason: "conflict" });

    expect(
      getInvalidMoveTarget(
        [],
        {
          eventId: "dragging",
          targetRoomId: "room-2",
          currentSnapDayOffset: 0,
          currentRawDayOffset: -1,
          durationDays: 2,
          todayOff: 0,
        },
        "2026-06-10",
      ),
    ).toEqual({ invalid: true, reason: "past" });
  });
});

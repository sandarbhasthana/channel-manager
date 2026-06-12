import { calculateNights } from "@/components/calendar/utils/calendarHelpers";
import { calculateResizedEventRange } from "./calendarEventResize";

describe("calendar event resize verification", () => {
  it("CAL-002 extends the right edge and recalculates stay nights from the resized range", () => {
    const range = calculateResizedEventRange({
      start: "2026-06-10",
      end: "2026-06-13",
      daysDelta: 2,
      type: "resize-right",
    });

    expect(range).toEqual({ newStart: "2026-06-10", newEnd: "2026-06-15" });
    expect(calculateNights(range!.newStart, range!.newEnd)).toBe(5);
  });

  it("CAL-002 shortens from the left edge and preserves the checkout date", () => {
    const range = calculateResizedEventRange({
      start: "2026-06-10",
      end: "2026-06-15",
      daysDelta: 2,
      type: "resize-left",
    });

    expect(range).toEqual({ newStart: "2026-06-12", newEnd: "2026-06-15" });
    expect(calculateNights(range!.newStart, range!.newEnd)).toBe(3);
  });

  it("CAL-002 keeps a resized reservation at least one night", () => {
    const range = calculateResizedEventRange({
      start: "2026-06-10",
      end: "2026-06-13",
      daysDelta: -10,
      type: "resize-right",
    });

    expect(range).toEqual({ newStart: "2026-06-10", newEnd: "2026-06-11" });
    expect(calculateNights(range!.newStart, range!.newEnd)).toBe(1);
  });
});

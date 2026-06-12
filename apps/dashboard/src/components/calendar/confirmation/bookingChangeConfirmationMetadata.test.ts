import {
  buildBookingChangeFinalRows,
  buildBookingChangeStaticRows,
  formatBookingChangeInr,
  getBookingChangeEffectivePriceMode,
} from "./bookingChangeConfirmationMetadata";

describe("booking change confirmation metadata verification", () => {
  const change = {
    oldStart: "2026-03-07T20:00:00.000Z",
    oldEnd: "2026-03-09T16:00:00.000Z",
    newStart: "2026-03-08T07:30:00.000Z",
    newEnd: "2026-03-10T16:00:00.000Z",
    oldRoomName: "101",
    newRoomName: "202",
    oldRoomCategory: "Standard",
    newRoomCategory: "Suite",
  };

  it("CAL-041 exposes old/new room values and formatted price impact values for review", () => {
    expect(buildBookingChangeStaticRows(change)).toEqual([
      { label: "Room type", oldVal: "Standard", newVal: "Suite", changed: true },
      { label: "Room", oldVal: "101", newVal: "202", changed: true },
    ]);

    expect(formatBookingChangeInr(200)).toBe("\u20b9200");
    expect(formatBookingChangeInr(325)).toBe("\u20b9325");
  });

  it("CAL-042 separates final confirmation rows from the initial review and honors locked pricing", () => {
    expect(buildBookingChangeFinalRows(change, "Asia/Calcutta").map((row) => row.label)).toEqual([
      "Room type",
      "Room",
      "Check-in",
      "Check-out",
    ]);

    expect(getBookingChangeEffectivePriceMode(false, "use_new")).toBe("use_new");
    expect(getBookingChangeEffectivePriceMode(true, "use_new")).toBe("keep_current");
  });
});

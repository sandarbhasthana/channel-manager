export interface BookingChangeRowsInput {
  oldStart: string;
  oldEnd: string;
  newStart: string;
  newEnd: string;
  oldRoomName?: string;
  newRoomName?: string;
  oldRoomCategory?: string;
  newRoomCategory?: string;
}

export interface BookingChangeRowDef {
  label: string;
  oldVal: string;
  newVal: string;
  changed: boolean;
}

export function formatBookingChangeTzLocal(iso: string, timezone: string): string {
  const date = new Date(iso);
  if (isNaN(date.getTime())) return "\u2014";
  return new Intl.DateTimeFormat("en-US", {
    timeZone: timezone,
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
  }).format(date);
}

export function formatBookingChangeInr(amount: number): string {
  return new Intl.NumberFormat("en-IN", {
    style: "currency",
    currency: "INR",
    maximumFractionDigits: 0,
  }).format(amount);
}

export function buildBookingChangeStaticRows({
  oldRoomName,
  newRoomName,
  oldRoomCategory,
  newRoomCategory,
}: BookingChangeRowsInput): BookingChangeRowDef[] {
  const staticRows: BookingChangeRowDef[] = [];

  if (oldRoomCategory !== undefined || newRoomCategory !== undefined) {
    staticRows.push({
      label: "Room type",
      oldVal: oldRoomCategory ?? "\u2014",
      newVal: newRoomCategory ?? "\u2014",
      changed: oldRoomCategory !== newRoomCategory,
    });
  }

  if (oldRoomName !== undefined || newRoomName !== undefined) {
    staticRows.push({
      label: "Room",
      oldVal: oldRoomName ?? "\u2014",
      newVal: newRoomName ?? "\u2014",
      changed: oldRoomName !== newRoomName,
    });
  }

  return staticRows;
}

export function buildBookingChangeFinalRows(
  input: BookingChangeRowsInput,
  timezone: string,
): { label: string; newVal: string }[] {
  const staticRows = buildBookingChangeStaticRows(input);
  const startChanged = input.oldStart !== input.newStart;
  const endChanged = input.oldEnd !== input.newEnd;

  return [
    ...staticRows.filter((row) => row.changed).map((row) => ({ label: row.label, newVal: row.newVal })),
    ...(startChanged ? [{ label: "Check-in", newVal: formatBookingChangeTzLocal(input.newStart, timezone) }] : []),
    ...(endChanged ? [{ label: "Check-out", newVal: formatBookingChangeTzLocal(input.newEnd, timezone) }] : []),
  ];
}

export function getBookingChangeEffectivePriceMode(
  rateLocked: boolean,
  priceMode: "keep_current" | "use_new",
): "keep_current" | "use_new" {
  return rateLocked ? "keep_current" : priceMode;
}

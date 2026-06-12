const CHECKED_IN_MOVE_STATUSES = new Set(["IN_HOUSE", "CHECKOUT_DUE"]);
const CALENDAR_EDIT_LOCKED_STATUSES = new Set(["CHECKED_OUT", "CANCELLED"]);

export const CALENDAR_EDIT_LOCKED_ERROR =
  "Checked-out and cancelled reservations cannot be moved or resized from the calendar";
export const NO_SHOW_CALENDAR_EDIT_BLOCKED_ERROR =
  "No-show reservations cannot be moved or resized from the calendar unless no-show calendar edits are enabled for this property";
export const OTA_LOCAL_CALENDAR_EDIT_BLOCKED_ERROR =
  "OTA reservations cannot be moved or resized locally because local OTA calendar edits are disabled for this property";

function toDateOnlyString(date: Date | string): string {
  if (typeof date === "string") {
    return date.slice(0, 10);
  }

  return date.toISOString().slice(0, 10);
}

interface CheckedInMoveValidationInput {
  status: string;
  newCheckIn: Date | string;
  businessDate: string;
  datesChanged: boolean;
}

export function shouldBlockCheckedInMoveBeforeBusinessDate({
  status,
  newCheckIn,
  businessDate,
  datesChanged
}: CheckedInMoveValidationInput): boolean {
  if (!datesChanged || !CHECKED_IN_MOVE_STATUSES.has(status)) {
    return false;
  }

  return toDateOnlyString(newCheckIn) < businessDate.slice(0, 10);
}

export function isCalendarEditLockedStatus(status: string | null | undefined): boolean {
  return !!status && CALENDAR_EDIT_LOCKED_STATUSES.has(status);
}

function getNestedObject(
  value: Record<string, unknown>,
  key: string
): Record<string, unknown> | null {
  const nested = value[key];
  return typeof nested === "object" && nested !== null && !Array.isArray(nested)
    ? (nested as Record<string, unknown>)
    : null;
}

function readsAsEnabled(value: unknown): boolean {
  if (value === true || value === "true" || value === "allow" || value === "allowed") {
    return true;
  }

  if (typeof value === "object" && value !== null && !Array.isArray(value)) {
    const config = value as Record<string, unknown>;
    return (
      config.enabled === true ||
      config.allow === true ||
      config.allowed === true ||
      config.allowCalendarEdits === true ||
      config.calendarEditsAllowed === true
    );
  }

  return false;
}

function readsAsDisabled(value: unknown): boolean {
  if (value === false || value === "false" || value === "deny" || value === "denied") {
    return true;
  }

  if (typeof value === "object" && value !== null && !Array.isArray(value)) {
    const config = value as Record<string, unknown>;
    return (
      config.enabled === false ||
      config.allow === false ||
      config.allowed === false ||
      config.allowCalendarEdits === false ||
      config.calendarEditsAllowed === false ||
      config.allowLocalCalendarEdits === false ||
      config.localCalendarEditsAllowed === false
    );
  }

  return false;
}

export function areNoShowCalendarEditsAllowed(
  config: unknown
): boolean {
  if (!config || typeof config !== "object" || Array.isArray(config)) {
    return false;
  }

  const value = config as Record<string, unknown>;
  const calendar = getNestedObject(value, "calendar");
  const noShow = getNestedObject(value, "noShow");
  const noShowEdits = getNestedObject(value, "noShowEdits");
  const calendarNoShow = calendar ? getNestedObject(calendar, "noShow") : null;
  const calendarNoShowEdits = calendar ? getNestedObject(calendar, "noShowEdits") : null;

  return (
    readsAsEnabled(value.allowNoShowCalendarEdits) ||
    readsAsEnabled(value.noShowCalendarEditsAllowed) ||
    readsAsEnabled(noShowEdits) ||
    readsAsEnabled(calendarNoShowEdits) ||
    readsAsEnabled(calendarNoShow?.calendarEdits) ||
    readsAsEnabled(noShow?.calendarEdits)
  );
}

export function getNoShowCalendarEditBlockReason(
  status: string | null | undefined,
  config: unknown
): string | null {
  if (status !== "NO_SHOW") {
    return null;
  }

  return areNoShowCalendarEditsAllowed(config)
    ? null
    : NO_SHOW_CALENDAR_EDIT_BLOCKED_ERROR;
}

export function areOtaLocalCalendarEditsAllowed(config: unknown): boolean {
  if (!config || typeof config !== "object" || Array.isArray(config)) {
    return true;
  }

  const value = config as Record<string, unknown>;
  const calendar = getNestedObject(value, "calendar");
  const ota = getNestedObject(value, "ota");
  const channel = getNestedObject(value, "channel");
  const channels = getNestedObject(value, "channels");
  const channelManager = getNestedObject(value, "channelManager");
  const calendarOta = calendar ? getNestedObject(calendar, "ota") : null;
  const calendarChannel = calendar ? getNestedObject(calendar, "channel") : null;
  const localEdits = getNestedObject(value, "localEdits");
  const otaLocalEdits = ota ? getNestedObject(ota, "localEdits") : null;
  const calendarOtaLocalEdits = calendarOta
    ? getNestedObject(calendarOta, "localEdits")
    : null;

  return !(
    readsAsDisabled(value.allowOtaLocalCalendarEdits) ||
    readsAsDisabled(value.otaLocalCalendarEditsAllowed) ||
    readsAsDisabled(value.allowChannelLocalCalendarEdits) ||
    readsAsDisabled(value.channelLocalCalendarEditsAllowed) ||
    readsAsEnabled(value.disableOtaLocalCalendarEdits) ||
    readsAsDisabled(localEdits?.ota) ||
    readsAsDisabled(otaLocalEdits) ||
    readsAsDisabled(calendarOtaLocalEdits) ||
    readsAsDisabled(ota?.localCalendarEdits) ||
    readsAsDisabled(ota?.calendarEdits) ||
    readsAsDisabled(channel?.localCalendarEdits) ||
    readsAsDisabled(channels?.localCalendarEdits) ||
    readsAsDisabled(channelManager?.localCalendarEdits) ||
    readsAsDisabled(calendarOta?.localCalendarEdits) ||
    readsAsDisabled(calendarOta?.calendarEdits) ||
    readsAsDisabled(calendarChannel?.localCalendarEdits)
  );
}

export function getOtaLocalCalendarEditBlockReason(
  isOtaReservation: boolean,
  config: unknown
): string | null {
  if (!isOtaReservation) {
    return null;
  }

  return areOtaLocalCalendarEditsAllowed(config)
    ? null
    : OTA_LOCAL_CALENDAR_EDIT_BLOCKED_ERROR;
}

interface CheckedInRoomMoveWindowInput {
  status: string;
  newCheckIn: Date | string;
  newCheckOut: Date | string;
  businessDate: string;
  roomChanged: boolean;
}

type CheckedInRoomMoveWindow =
  | {
      allowed: true;
      targetCheckIn: string;
    }
  | {
      allowed: false;
      error: string;
    };

export function getCheckedInRoomMoveWindow({
  status,
  newCheckIn,
  newCheckOut,
  businessDate,
  roomChanged
}: CheckedInRoomMoveWindowInput): CheckedInRoomMoveWindow {
  if (!roomChanged || !CHECKED_IN_MOVE_STATUSES.has(status)) {
    return {
      allowed: true,
      targetCheckIn: toDateOnlyString(newCheckIn)
    };
  }

  const businessDateOnly = businessDate.slice(0, 10);
  const checkInDate = toDateOnlyString(newCheckIn);
  const checkOutDate = toDateOnlyString(newCheckOut);
  const targetCheckIn =
    checkInDate < businessDateOnly ? businessDateOnly : checkInDate;

  if (targetCheckIn >= checkOutDate) {
    return {
      allowed: false,
      error:
        "Checked-in room moves require at least one remaining occupied night from the property business date"
    };
  }

  return {
    allowed: true,
    targetCheckIn
  };
}

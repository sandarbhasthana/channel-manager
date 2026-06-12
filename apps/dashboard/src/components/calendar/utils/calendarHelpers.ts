// File: src/app/dashboard/bookings/utils/calendarHelpers.ts
/**
 * Calendar helper utilities
 * Provides date manipulation and calendar-specific helper functions
 */

/**
 * Check if a date is today
 * @param date - Date to check
 * @returns True if the date is today
 */
export function isToday(date: Date): boolean {
  const now = new Date();
  return (
    date.getFullYear() === now.getFullYear() &&
    date.getMonth() === now.getMonth() &&
    date.getDate() === now.getDate()
  );
}

/**
 * Check if a date is a weekend (Friday or Saturday)
 * @param date - Date to check
 * @returns True if the date is Friday or Saturday
 */
export function isWeekend(date: Date): boolean {
  const dow = date.getDay();
  return dow === 5 || dow === 6; // Friday (5) or Saturday (6)
}

/**
 * Add days to a date
 * @param date - Starting date
 * @param days - Number of days to add (can be negative)
 * @returns New date with days added
 */
export function addDays(date: Date, days: number): Date {
  const result = new Date(date);
  result.setDate(result.getDate() + days);
  return result;
}

/**
 * Format date to ISO string (YYYY-MM-DD)
 * @param date - Date to format
 * @returns ISO date string
 */
export function toISODateString(date: Date): string {
  return date.toISOString().slice(0, 10);
}

/**
 * Parse ISO date string to Date object
 * @param dateString - ISO date string (YYYY-MM-DD)
 * @returns Date object
 */
export function fromISODateString(dateString: string): Date {
  return new Date(dateString);
}

/**
 * Get date range for calendar view
 * @param startDate - Start date
 * @param days - Number of days to include
 * @returns Array of dates
 */
export function getDateRange(startDate: Date, days: number): Date[] {
  const dates: Date[] = [];
  for (let i = 0; i < days; i++) {
    dates.push(addDays(startDate, i));
  }
  return dates;
}

/**
 * Generate weekend highlight events for calendar
 * @param start - Start date
 * @param end - End date
 * @returns Array of weekend highlight events
 */
export function generateWeekendHighlights(
  start: Date,
  end: Date
): Array<{
  id: string;
  start: string;
  end: string;
  display: string;
  classNames: string[];
  allDay: boolean;
}> {
  const weekends: Array<{
    id: string;
    start: string;
    end: string;
    display: string;
    classNames: string[];
    allDay: boolean;
  }> = [];

  for (let d = new Date(start); d < end; d.setDate(d.getDate() + 1)) {
    if (isWeekend(d)) {
      const s = new Date(d);
      const e = new Date(d);
      e.setDate(e.getDate() + 1);

      weekends.push({
        id: `wknd-${s.toISOString()}`,
        start: s.toISOString(),
        end: e.toISOString(),
        display: "background",
        classNames: ["weekend-highlight"],
        allDay: true
      });
    }
  }

  return weekends;
}

/**
 * Parse a YYYY-MM-DD string as a LOCAL-midnight Date, avoiding the UTC-parse
 * off-by-one that `new Date("YYYY-MM-DD")` causes in negative-UTC-offset timezones.
 */
export function parseLocalDate(dateStr: string): Date {
  const [y, m, d] = dateStr.split("-").map(Number);
  return new Date(y, m - 1, d);
}

/**
 * Return today's date as a YYYY-MM-DD string in the browser's local timezone.
 * `new Date().toISOString()` returns UTC, which can be a different calendar day
 * than the local date (e.g. before 05:30 in IST the UTC day is still yesterday).
 */
export function localDateString(d: Date = new Date()): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

/**
 * Number of whole days from dateA to dateB (positive if B is after A).
 * Operates on YYYY-MM-DD strings — no time zone issues.
 */
export function diffDays(dateA: string, dateB: string): number {
  const msA = new Date(dateA).getTime();
  const msB = new Date(dateB).getTime();
  return Math.round((msB - msA) / 86_400_000);
}

/**
 * Calculate number of nights between two dates
 * @param checkIn - Check-in date
 * @param checkOut - Check-out date
 * @returns Number of nights
 */
export function calculateNights(checkIn: string, checkOut: string): number {
  const start = new Date(checkIn);
  const end = new Date(checkOut);
  const diffTime = Math.abs(end.getTime() - start.getTime());
  const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));
  return diffDays;
}

/**
 * Check if two date ranges overlap
 * @param start1 - Start of first range
 * @param end1 - End of first range
 * @param start2 - Start of second range
 * @param end2 - End of second range
 * @returns True if ranges overlap
 */
export function dateRangesOverlap(
  start1: Date,
  end1: Date,
  start2: Date,
  end2: Date
): boolean {
  return start1 < end2 && end1 > start2;
}

/**
 * Get organization ID from cookies
 * @returns Organization ID or undefined
 */
export function getOrgIdFromCookies(): string | undefined {
  if (typeof document === "undefined") return undefined;
  return document.cookie
    .split("; ")
    .find((row) => row.startsWith("orgId="))
    ?.split("=")[1];
}

/**
 * Get property ID from cookies
 * @returns Property ID or undefined
 */
export function getPropertyIdFromCookies(): string | undefined {
  if (typeof document === "undefined") return undefined;
  return document.cookie
    .split("; ")
    .find((row) => row.startsWith("propertyId="))
    ?.split("=")[1];
}


export type CalendarResizeType = "resize-left" | "resize-right";

export interface CalendarResizeRange {
  newStart: string;
  newEnd: string;
}

function addDaysToDateString(dateString: string, days: number): string {
  const d = new Date(dateString);
  d.setDate(d.getDate() + days);
  return d.toISOString().slice(0, 10);
}

export function calculateResizedEventRange({
  start,
  end,
  daysDelta,
  type,
  todayStr,
}: {
  start: string;
  end: string;
  daysDelta: number;
  type: CalendarResizeType;
  todayStr?: string;
}): CalendarResizeRange | null {
  if (daysDelta === 0) return null;

  if (type === "resize-right") {
    let newEnd = addDaysToDateString(end, daysDelta);
    if (newEnd <= start) {
      newEnd = addDaysToDateString(start, 1);
    }
    return { newStart: start, newEnd };
  }

  let newStart = addDaysToDateString(start, daysDelta);
  if (todayStr && newStart < todayStr) newStart = todayStr;

  if (newStart >= end) {
    newStart = addDaysToDateString(end, -1);
    if (todayStr && newStart < todayStr) return null;
  }

  return { newStart, newEnd: end };
}

"use client";

import { useMemo } from "react";
import type { ProcessedEvent } from "@/components/calendar/utils/types";

const GAP_PX = 4; // gap between event bar edge and cell border

export interface PositionedEvent extends ProcessedEvent {
  leftPx: number;
  widthPx: number;
  clampedLeft: boolean;   // true when bar starts before visible window
  clampedRight: boolean;  // true when bar ends after visible window
}

/**
 * Maps ProcessedEvent[] to PositionedEvent[] — adds pixel positioning
 * relative to the grid's scroll container.
 *
 * @param events - Color-resolved events from useProcessedEvents
 * @param windowStart - First visible date (YYYY-MM-DD)
 * @param visibleDays - Number of day columns
 * @param cellWidth - Width of one day column in px
 * @param leftColumnWidth - Width of the sticky left column in px
 */
export function useEventPositioning(
  events: ProcessedEvent[],
  windowStart: string,
  visibleDays: number,
  cellWidth: number,
  leftColumnWidth: number
): PositionedEvent[] {
  return useMemo(() => {
    if (!windowStart || cellWidth === 0) return [];

    const windowStartDate = new Date(windowStart);

    return events.map((event) => {
      const checkIn = new Date(event.start);
      const checkOut = new Date(event.end);

      // Days from windowStart to checkIn (can be negative = starts before window)
      const startOffset = Math.round(
        (checkIn.getTime() - windowStartDate.getTime()) / 86_400_000
      );
      // Total duration in days
      const durationDays = Math.max(
        1,
        Math.round((checkOut.getTime() - checkIn.getTime()) / 86_400_000)
      );

      const clampedLeft = startOffset < 0;
      const clampedRight = startOffset + durationDays > visibleDays;

      const clampedStart = Math.max(0, startOffset);
      const clampedEnd = Math.min(visibleDays, startOffset + durationDays);
      const clampedSpan = Math.max(0, clampedEnd - clampedStart);

      const leftPx = leftColumnWidth + clampedStart * cellWidth + GAP_PX;
      const widthPx = Math.max(0, clampedSpan * cellWidth - GAP_PX * 2);

      return {
        ...event,
        leftPx,
        widthPx,
        clampedLeft,
        clampedRight
      };
    });
  }, [events, windowStart, visibleDays, cellWidth, leftColumnWidth]);
}

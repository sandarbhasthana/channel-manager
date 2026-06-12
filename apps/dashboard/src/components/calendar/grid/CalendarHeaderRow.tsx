"use client";

import React from "react";
import { isToday, isWeekend, parseLocalDate } from "@/components/calendar/utils/calendarHelpers";
import { HolidayTooltip } from "@/components/calendar/utils/HolidayTooltip";
import type { VirtualItem } from "@tanstack/react-virtual";

import { HEADER_ROW_HEIGHT } from "../calendarConfig";
import { CALENDAR_THEME, CALENDAR_COLORS } from "../calendarTheme";
export { HEADER_ROW_HEIGHT };

interface CalendarHeaderRowProps {
  virtualCols: VirtualItem[];
  colIndexToDate: (i: number) => string;
  totalDateWidth: number;
  holidays: Record<string, string>;
  /** Actual visible start index from colVirtualizer.range (excludes overscan) */
  visibleStartIndex: number;
  /** Actual visible end index from colVirtualizer.range (excludes overscan) */
  visibleEndIndex: number;
  /** Ref to an absolutely-positioned highlight bar at the top of the hovered date column. */
  dateColHighlightRef?: React.RefObject<HTMLDivElement | null>;
  /** Ref to a muted source-date indicator that can persist through review. */
  dateColGhostHighlightRef?: React.RefObject<HTMLDivElement | null>;
}

/**
 * Sticky top date header — renders only the visible virtual date columns.
 * Height is fixed at HEADER_ROW_HEIGHT. Sticky positioning is handled by the parent.
 */
export const CalendarHeaderRow = React.memo(function CalendarHeaderRow({
  virtualCols,
  colIndexToDate,
  totalDateWidth,
  holidays,
  visibleStartIndex,
  visibleEndIndex,
  dateColHighlightRef,
  dateColGhostHighlightRef,
}: CalendarHeaderRowProps) {
  const firstVisibleDate = visibleStartIndex >= 0 ? parseLocalDate(colIndexToDate(visibleStartIndex)) : null;
  const lastVisibleDate = visibleEndIndex >= 0 ? parseLocalDate(colIndexToDate(visibleEndIndex)) : null;
  const lastVisibleIsNewMonth =
    firstVisibleDate &&
    lastVisibleDate &&
    (lastVisibleDate.getMonth() !== firstVisibleDate.getMonth() ||
      lastVisibleDate.getFullYear() !== firstVisibleDate.getFullYear());

  return (
    <div
      style={{
        position: "relative",
        width: totalDateWidth,
        minWidth: totalDateWidth,
        height: HEADER_ROW_HEIGHT,
        minHeight: HEADER_ROW_HEIGHT,
        flexShrink: 0,
      }}
    >
      {/* Drag-move date column highlight — bright bar on top edge (driven by ref from CalendarDateGrid) */}
      {dateColHighlightRef && (
        <div
          ref={dateColHighlightRef}
          aria-hidden="true"
          style={{
            display: "none",
            position: "absolute",
            top: 0,
            left: 0,
            width: 0, // overridden via ref during drag
            height: 4,
            borderRadius: "0 0 2px 2px",
            pointerEvents: "none",
            zIndex: 21,
            transition: "left 0.08s ease, width 0.08s ease, background-color 0.08s ease",
          }}
        />
      )}
      {/* Original date indicator — muted bar kept visible while the review modal is open */}
      {dateColGhostHighlightRef && (
        <div
          ref={dateColGhostHighlightRef}
          aria-hidden="true"
          style={{
            display: "none",
            position: "absolute",
            top: 6,
            left: 0,
            width: 0, // overridden via ref during drag
            height: 4,
            borderRadius: 2,
            backgroundColor: CALENDAR_COLORS.dragGhost,
            pointerEvents: "none",
            zIndex: 20,
            transition: "left 0.08s ease, width 0.08s ease",
          }}
        />
      )}
      {virtualCols.map((vCol) => {
        const dateStr = colIndexToDate(vCol.index);
        const date = parseLocalDate(dateStr); // local midnight — avoids UTC-parse off-by-one
        const todayDate = isToday(date);
        const weekend = isWeekend(date);
        const holiday = holidays[dateStr];
        const weekday = date.toLocaleDateString("en-US", { weekday: "short" });
        const dayNum = date.getDate();
        const monthAbbr = date.toLocaleDateString("en-US", { month: "short" });

        const isFirstVisible = vCol.index === visibleStartIndex;
        const isLastVisible = vCol.index === visibleEndIndex;
        const isMonthBoundary = dayNum === 1;
        const daysInMonth = new Date(date.getFullYear(), date.getMonth() + 1, 0).getDate();
        const isLastOfMonth = dayNum === daysInMonth;
        const showMonthLabel =
          isFirstVisible ||
          isMonthBoundary ||
          isLastOfMonth ||
          (isLastVisible && !!lastVisibleIsNewMonth);

        return (
          <div
            key={vCol.index}
            className={[
              `absolute flex flex-col items-center justify-center ${CALENDAR_THEME.header.base} select-none`,
              todayDate ? CALENDAR_THEME.header.today : "",
              weekend && !todayDate ? CALENDAR_THEME.header.weekend : "",
            ]
              .filter(Boolean)
              .join(" ")}
            style={{
              left: vCol.start,
              width: vCol.size,
              top: 0,
              height: "100%",
            }}
          >
            <span
              className={`text-xs font-medium ${
                todayDate
                  ? CALENDAR_THEME.header.weekdayToday
                  : weekend
                  ? CALENDAR_THEME.header.weekdayWeekend
                  : CALENDAR_THEME.header.weekdayNormal
              }`}
            >
              {weekday}
            </span>

            <span
              className={`text-sm font-bold leading-tight ${
                todayDate
                  ? CALENDAR_THEME.header.dayNumberToday
                  : CALENDAR_THEME.header.dayNumberNormal
              }`}
            >
              {dayNum}
            </span>

            {showMonthLabel && (
              <span className={`absolute bottom-1.5 text-xs ${CALENDAR_THEME.header.monthLabel} leading-none`}>
                {monthAbbr}
              </span>
            )}

            {holiday && (
              <div className="absolute top-1 right-1">
                <HolidayTooltip holidayName={holiday}>
                  <span className="text-xs leading-none select-none">🌸</span>
                </HolidayTooltip>
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
});

"use client";

import React from "react";
import { isToday, isWeekend } from "@/components/calendar/utils/calendarHelpers";
import { CALENDAR_THEME } from "../calendarTheme";

interface CalendarCellProps {
  roomId: string;
  date: string;           // YYYY-MM-DD
  columnIndex: number;
  cellWidth: number;
  /** Optional extra className based on room+date (per-cell styling API) */
  cellClassName?: (roomId: string, date: string) => string | undefined;
  /** Pointer handlers provided by the parent row (drag selection + click) */
  onPointerDown?: (e: React.PointerEvent, roomId: string, date: string, colIndex: number) => void;
  onPointerMove?: (e: React.PointerEvent, roomId: string, date: string, colIndex: number) => void;
  onPointerUp?: (e: React.PointerEvent, roomId: string, date: string, colIndex: number) => void;
  /** Whether this cell is currently highlighted by a drag selection */
  isSelecting?: boolean;
}

/**
 * A single room × date intersection cell.
 *
 * Styling layers (applied in order):
 *  1. base — border, height
 *  2. today — teal tint
 *  3. weekend — slate tint
 *  4. selecting — purple highlight (drag selection in progress)
 *  5. cellClassName — arbitrary caller-provided class
 */
export const CalendarCell = React.memo(function CalendarCell({
  roomId,
  date,
  columnIndex,
  cellWidth,
  cellClassName,
  onPointerDown,
  onPointerMove,
  onPointerUp,
  isSelecting = false
}: CalendarCellProps) {
  const dateObj = new Date(date);
  const todayCell = isToday(dateObj);
  const weekendCell = isWeekend(dateObj);
  const extra = cellClassName?.(roomId, date) ?? "";

  return (
    <div
      data-room-id={roomId}
      data-date={date}
      data-col-index={columnIndex}
      data-selecting={isSelecting || undefined}
      className={[
        `border-b border-r ${CALENDAR_THEME.border} h-full`,
        "shrink-0",
        todayCell ? CALENDAR_THEME.cell.today : "",
        weekendCell && !todayCell ? CALENDAR_THEME.cell.weekend : "",
        isSelecting ? CALENDAR_THEME.cell.selecting : "",
        extra
      ]
        .filter(Boolean)
        .join(" ")}
      style={{ width: cellWidth, minWidth: cellWidth }}
      onPointerDown={onPointerDown ? (e) => onPointerDown(e, roomId, date, columnIndex) : undefined}
      onPointerMove={onPointerMove ? (e) => onPointerMove(e, roomId, date, columnIndex) : undefined}
      onPointerUp={onPointerUp ? (e) => onPointerUp(e, roomId, date, columnIndex) : undefined}
    />
  );
});

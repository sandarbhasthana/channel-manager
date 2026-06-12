"use client";

import React, { useRef, useState, useMemo, useCallback } from "react";
import { CalendarRoomLabel } from "./CalendarRoomLabel";
import { CalendarEventBar } from "./CalendarEventBar";
import { addDays, isToday, isWeekend } from "@/components/calendar/utils/calendarHelpers";
import type { PositionedEvent } from "../layout/useEventPositioning";

import { ROW_HEIGHT } from "../calendarConfig";
import { CALENDAR_THEME, CALENDAR_COLORS } from "../calendarTheme";
export { ROW_HEIGHT };

interface CalendarRoomRowProps {
  roomId: string;
  roomTitle: string;
  windowStart: string;
  visibleDays: number;
  cellWidth: number;
  leftColumnWidth: number;
  events: PositionedEvent[];
  isSelected: boolean;
  onSelectRoom: (roomId: string) => void;
  onCellClick?: (roomId: string, roomName: string, date: string, x: number, y: number) => void;
  onEventClick?: (event: PositionedEvent, x: number, y: number) => void;
  onBlockClick?: (event: PositionedEvent, x: number, y: number) => void;
  /** Called after a multi-cell drag-select. endDate is checkout date (day after last selected). */
  onRangeSelect?: (roomId: string, startDate: string, endDate: string) => void;
  cellClassName?: (roomId: string, date: string) => string | undefined;
  markers?: React.ReactNode;
  /** When true, skip rendering the left label cell (rendered separately in CalendarRoomsColumn) */
  hideLeftColumn?: boolean;
}

/**
 * One room row: sticky label + N cells + absolutely-positioned event bars.
 *
 * Drag-to-select (mouse only — touch stays for scrolling):
 *   pointerDown → setPointerCapture → track columns → pointerUp
 *   Single-cell release  → onCellClick (cell flyout)
 *   Multi-cell release   → onRangeSelect (BlockRoomSheet)
 */
export const CalendarRoomRow = React.memo(function CalendarRoomRow({
  roomId,
  roomTitle,
  windowStart,
  visibleDays,
  cellWidth,
  leftColumnWidth,
  events,
  isSelected,
  onSelectRoom,
  onCellClick,
  onEventClick,
  onBlockClick,
  onRangeSelect,
  cellClassName,
  markers,
  hideLeftColumn = false,
}: CalendarRoomRowProps) {
  const startDate = new Date(windowStart);
  const rowRef = useRef<HTMLDivElement>(null);

  // Drag selection state
  const [selStart, setSelStart] = useState<number | null>(null);
  const [selEnd, setSelEnd]   = useState<number | null>(null);

  // Pre-compute dates array so we don't recompute per-event
  const dates = useMemo(
    () =>
      Array.from({ length: visibleDays }, (_, i) =>
        addDays(startDate, i).toISOString().slice(0, 10)
      ),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [windowStart, visibleDays]
  );

  /** Column index from clientX, or null if over the label area */
  const colFromX = useCallback(
    (clientX: number): number | null => {
      if (!rowRef.current) return null;
      const rect = rowRef.current.getBoundingClientRect();
      // When hideLeftColumn is true the row starts directly with date cells
      const offset = hideLeftColumn ? 0 : leftColumnWidth;
      const x = clientX - rect.left - offset;
      if (x < 0) return null;
      return Math.max(0, Math.min(Math.floor(x / cellWidth), visibleDays - 1));
    },
    [hideLeftColumn, leftColumnWidth, cellWidth, visibleDays]
  );

  const handlePointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    // Only drag with mouse; touch should scroll the container
    if (e.pointerType !== "mouse") return;
    const col = colFromX(e.clientX);
    if (col === null) return;
    rowRef.current?.setPointerCapture(e.pointerId);
    setSelStart(col);
    setSelEnd(col);
  };

  const handlePointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    if (selStart === null) return;
    const col = colFromX(e.clientX);
    if (col !== null && col !== selEnd) setSelEnd(col);
  };

  const handlePointerUp = (e: React.PointerEvent<HTMLDivElement>) => {
    if (selStart === null) return;
    const col = colFromX(e.clientX) ?? selEnd ?? selStart;
    const minCol = Math.min(selStart, col);
    const maxCol = Math.max(selStart, col);

    // Reset before callbacks to avoid stale highlight on re-render
    setSelStart(null);
    setSelEnd(null);

    if (minCol === maxCol) {
      // Single-cell tap/click → cell flyout
      onCellClick?.(roomId, roomTitle, dates[minCol], e.clientX, e.clientY);
    } else {
      // Multi-cell drag → block range
      if (onRangeSelect) {
        const endDateStr = addDays(new Date(dates[maxCol]), 1).toISOString().slice(0, 10);
        onRangeSelect(roomId, dates[minCol], endDateStr);
      }
    }
  };

  const selMin = selStart !== null && selEnd !== null ? Math.min(selStart, selEnd) : -1;
  const selMax = selStart !== null && selEnd !== null ? Math.max(selStart, selEnd) : -2;
  const dateStripWidth = cellWidth * visibleDays;
  const highlightedColumns = useMemo(
    () =>
      dates.flatMap<{ index: number; tone: "today" | "weekend" }>((date, index) => {
        const dateObj = new Date(date);
        if (isToday(dateObj)) return [{ index, tone: "today" }];
        if (isWeekend(dateObj)) return [{ index, tone: "weekend" }];
        return [];
      }),
    [dates],
  );
  const hasSelection = selMin >= 0 && selMax >= selMin;

  return (
    <div
      ref={rowRef}
      className="flex relative select-none"
      style={{ height: ROW_HEIGHT, minHeight: ROW_HEIGHT, touchAction: "pan-x pan-y" }}
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
    >
      {/* Left label — omitted when rendered by CalendarRoomsColumn */}
      {!hideLeftColumn && (
        <CalendarRoomLabel
          title={roomTitle}
          isSelected={isSelected}
          onClick={() => onSelectRoom(roomId)}
          markers={markers}
          leftColumnWidth={leftColumnWidth}
        />
      )}

      {/* Date grid — default path avoids rendering one empty node per day cell. */}
      {cellClassName ? (
        dates.map((date, i) => {
          const dateObj = new Date(date);
          const todayCell = isToday(dateObj);
          const weekendCell = isWeekend(dateObj);
          const extra = cellClassName(roomId, date) ?? "";

          return (
            <div
              key={date}
              data-room-id={roomId}
              data-date={date}
              data-col-index={i}
              data-selecting={i >= selMin && i <= selMax || undefined}
              className={[
                `border-b border-r ${CALENDAR_THEME.border} h-full shrink-0`,
                todayCell ? CALENDAR_THEME.cell.today : "",
                weekendCell && !todayCell ? CALENDAR_THEME.cell.weekend : "",
                i >= selMin && i <= selMax ? CALENDAR_THEME.cell.selecting : "",
                extra,
              ]
                .filter(Boolean)
                .join(" ")}
              style={{ width: cellWidth, minWidth: cellWidth }}
            />
          );
        })
      ) : (
        <div
          aria-hidden="true"
          className={`relative shrink-0 border-b ${CALENDAR_THEME.border}`}
          style={{
            width: dateStripWidth,
            minWidth: dateStripWidth,
            height: "100%",
            overflow: "hidden",
            contain: "layout paint style",
            backgroundImage: `repeating-linear-gradient(to right, transparent 0, transparent calc(100% - 1px), ${CALENDAR_COLORS.gridLine} calc(100% - 1px), ${CALENDAR_COLORS.gridLine} 100%)`,
            backgroundSize: `${cellWidth}px 100%`,
            backgroundRepeat: "repeat-x",
          }}
        >
          {highlightedColumns.map(({ index, tone }) => (
            <div
              key={`${tone}-${index}`}
              className={[
                `absolute top-0 bottom-0 border-r ${CALENDAR_THEME.border}`,
                tone === "today"
                  ? CALENDAR_THEME.cell.today
                  : CALENDAR_THEME.cell.weekend,
              ].join(" ")}
              style={{
                left: index * cellWidth,
                width: cellWidth,
              }}
            />
          ))}
          {hasSelection && (
            <div
              className={`absolute top-0 bottom-0 ${CALENDAR_THEME.dateGridRoomRow.dragSelect}`}
              style={{
                left: selMin * cellWidth,
                width: (selMax - selMin + 1) * cellWidth,
              }}
            />
          )}
        </div>
      )}

      {/* Event bars — absolutely positioned over the cells */}
      {events.map((event) => (
        <CalendarEventBar
          key={event.id}
          event={event}
          cellWidth={cellWidth}
          onEventClick={onEventClick}
          onBlockClick={onBlockClick}
        />
      ))}
    </div>
  );
});

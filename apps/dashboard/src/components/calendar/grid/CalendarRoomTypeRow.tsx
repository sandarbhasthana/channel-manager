"use client";

import React, { useCallback, useMemo } from "react";
import { CalendarRoomTypeLabel } from "./CalendarRoomTypeLabel";
import { addDays, isToday, isWeekend } from "@/components/calendar/utils/calendarHelpers";
import type { RoomTypeRates } from "@/lib/hooks/useRatesData";
import type { Reservation } from "@/components/calendar/utils/types";

import { TYPE_ROW_HEIGHT, RATE_LABEL_HEIGHT, OCCUPANCY_LABEL_HEIGHT } from "../calendarConfig";
import { CALENDAR_THEME, CALENDAR_COLORS } from "../calendarTheme";
export { TYPE_ROW_HEIGHT };

interface CalendarRoomTypeRowProps {
  groupId: string;
  title: string;
  roomCount: number;
  isExpanded: boolean;
  onToggleGroup: (groupId: string) => void;
  windowStart: string;
  visibleDays: number;
  cellWidth: number;
  leftColumnWidth: number;
  ratesData?: RoomTypeRates | null;
  reservations: Reservation[];
  roomIds: string[];
  showRates?: boolean;
  showOccupancy?: boolean;
  onRateClick?: (roomTypeId: string, roomTypeName: string, dateStr: string, currentPrice: number) => void;
  /** When true, skip rendering the left label cell (rendered separately in CalendarRoomsColumn) */
  hideLeftColumn?: boolean;
}

/**
 * Room type group header row.
 * Shows a sticky label on the left and an occupancy/rate overlay across the date columns.
 */
export const CalendarRoomTypeRow = React.memo(function CalendarRoomTypeRow({
  groupId,
  title,
  roomCount,
  isExpanded,
  onToggleGroup,
  windowStart,
  visibleDays,
  cellWidth,
  leftColumnWidth,
  ratesData,
  reservations,
  roomIds,
  showRates = true,
  showOccupancy = true,
  onRateClick,
  hideLeftColumn = false,
}: CalendarRoomTypeRowProps) {
  const handleToggle = useCallback(() => onToggleGroup(groupId), [onToggleGroup, groupId]);

  // Pre-compute a Set for O(1) room membership checks inside the filter.
  const roomIdSet = useMemo(() => new Set(roomIds), [roomIds]);

  // Compute occupancy counts per date once — not inline on every render.
  // Recomputes only when reservations, roomIds, windowStart, or visibleDays change.
  const occupancyByDate = useMemo<Record<string, number>>(() => {
    if (!showOccupancy) return {};
    const relevant = reservations.filter(
      (r) =>
        roomIdSet.has(r.roomId) &&
        (r.status === "CONFIRMED" ||
          r.status === "IN_HOUSE" ||
          r.status === "CHECKIN_DUE" ||
          r.status === "CHECKOUT_DUE"),
    );
    const start = new Date(windowStart);
    const result: Record<string, number> = {};
    for (let i = 0; i < visibleDays; i++) {
      const date = addDays(start, i);
      const dateStr = date.toISOString().slice(0, 10);
      const nextDate = addDays(date, 1);
      result[dateStr] = relevant.filter((r) => {
        const ci = new Date(r.checkIn);
        const co = new Date(r.checkOut);
        return ci < nextDate && co > date;
      }).length;
    }
    return result;
  }, [reservations, roomIdSet, windowStart, visibleDays, showOccupancy]);

  const roomTypeId = ratesData?.roomTypeId;
  const dateStripWidth = cellWidth * visibleDays;
  const columnMeta = useMemo(
    () => {
      const startDate = new Date(windowStart);
      return Array.from({ length: visibleDays }, (_, i) => {
        const date = addDays(startDate, i);
        const dateStr = date.toISOString().slice(0, 10);
        return {
          index: i,
          dateStr,
          todayCol: isToday(date),
          weekendCol: isWeekend(date),
          occupied: occupancyByDate[dateStr] ?? 0,
          rate: showRates && ratesData ? ratesData.dates[dateStr]?.finalPrice ?? null : null,
        };
      });
    },
    [windowStart, visibleDays, occupancyByDate, showRates, ratesData],
  );

  return (
    <div
      className={`flex relative ${CALENDAR_THEME.roomTypeRow.bg}`}
      style={{ height: TYPE_ROW_HEIGHT, minHeight: TYPE_ROW_HEIGHT }}
    >
      {/* Left label — omitted when rendered by CalendarRoomsColumn */}
      {!hideLeftColumn && (
        <CalendarRoomTypeLabel
          title={title}
          roomCount={roomCount}
          isExpanded={isExpanded}
          onToggle={handleToggle}
          leftColumnWidth={leftColumnWidth}
        />
      )}

      {/* Date columns — one painted strip plus positioned overlays */}
      <div
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
        {columnMeta.map(({ index, todayCol, weekendCol }) => {
          if (!todayCol && !weekendCol) return null;
          return (
            <div
              key={`tone-${index}`}
              className={[
                `absolute top-0 bottom-0 border-r ${CALENDAR_THEME.border}`,
                todayCol
                  ? CALENDAR_THEME.roomTypeRow.today
                  : CALENDAR_THEME.roomTypeRow.weekend,
              ].join(" ")}
              style={{
                left: index * cellWidth,
                width: cellWidth,
              }}
            />
          );
        })}
        {columnMeta.map(({ index, dateStr, occupied, rate }) => (
          <div
            key={dateStr}
            className="absolute top-0 bottom-0 flex flex-col items-center justify-center"
            style={{
              left: index * cellWidth,
              width: cellWidth,
            }}
          >
            <div
              className="flex items-center justify-center"
              style={{ minHeight: OCCUPANCY_LABEL_HEIGHT, height: OCCUPANCY_LABEL_HEIGHT }}
            >
              {showOccupancy && (
                <span className={`block max-w-full truncate text-xs font-semibold leading-tight ${CALENDAR_THEME.roomTypeRow.occupancyText}`}>
                  {occupied}/{roomCount}
                </span>
              )}
            </div>
            <div
              className="flex items-center justify-center"
              style={{ minHeight: RATE_LABEL_HEIGHT, height: RATE_LABEL_HEIGHT }}
            >
              {showRates && rate != null ? (
                <button
                  className={`block max-w-full truncate text-xs font-bold leading-tight ${CALENDAR_THEME.roomTypeRow.rateBtn}`}
                  onClick={(e) => {
                    e.stopPropagation();
                    if (!roomTypeId) return;
                    onRateClick?.(roomTypeId, title, dateStr, rate);
                  }}
                >
                  ₹{rate.toLocaleString("en-IN")}
                </button>
              ) : null}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
});

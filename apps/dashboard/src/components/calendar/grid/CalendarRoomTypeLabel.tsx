"use client";

import React from "react";
import { CALENDAR_THEME, CALENDAR_CARET_CLASS } from "../calendarTheme";

interface CalendarRoomTypeLabelProps {
  title: string;
  roomCount: number;
  isExpanded: boolean;
  onToggle: () => void;
  onAddRoom?: () => void;
  markers?: React.ReactNode;
  leftColumnWidth: number;
}

/**
 * Sticky left cell for a room-type group header row.
 * Shows room type name, room count badge, expand/collapse toggle,
 * and optional Add Room button.
 */
export const CalendarRoomTypeLabel = React.memo(function CalendarRoomTypeLabel({
  title,
  roomCount,
  isExpanded,
  onToggle,
  onAddRoom,
  markers,
  leftColumnWidth
}: CalendarRoomTypeLabelProps) {
  return (
    <div
      className={`flex items-center justify-between px-2 sm:px-3 ${CALENDAR_THEME.roomTypeLabel.base} ${CALENDAR_THEME.roomTypeLabel.hover} select-none cursor-pointer transition-colors rounded-0`}
      style={{
        width: leftColumnWidth,
        minWidth: leftColumnWidth,
        flexShrink: 0,
        position: "sticky",
        left: 0,
        zIndex: 10,
        height: "100%"
      }}
      onClick={onToggle}
      role="button"
      aria-expanded={isExpanded}
      aria-label={`${title} — ${isExpanded ? "collapse" : "expand"}`}
    >
      <div className="flex items-center gap-1 min-w-0">
        {/* Expand/collapse chevron — same caret affordance as the "Rooms" header */}
        <span className={CALENDAR_CARET_CLASS} aria-hidden="true">
          <span
            className="text-xs transition-transform"
            style={{ transform: isExpanded ? "rotate(90deg)" : "rotate(0deg)" }}
          >
            ▶
          </span>
        </span>

        {/* Room type name */}
        <span className={`${CALENDAR_THEME.roomTypeLabel.title} truncate`}>
          {title}
        </span>

        {/* Optional markers (asterisks, badges, etc.) */}
        {markers && <span className="shrink-0">{markers}</span>}
      </div>

      {/* Right side: room count + optional Add Room button */}
      <div className="flex items-center shrink-0 ml-1">
        {/* Add Room button */}
        {onAddRoom && (
          <button
            type="button"
            className={`text-xs ${CALENDAR_THEME.roomTypeLabel.addRoomBtn} px-2 py-0.5 rounded transition-colors shrink-0 mr-1`}
            onClick={(e) => {
              e.stopPropagation();
              onAddRoom();
            }}
            aria-label={`Add room to ${title}`}
          >
            + Room
          </button>
        )}

        {/* Room count — right-aligned, no right border-radius (flush with cell edge) */}
        <span
          className={`inline-flex h-5 w-9 shrink-0 items-center justify-center rounded text-xs font-semibold tabular-nums ${CALENDAR_THEME.roomTypeLabel.roomCountBadge}`}
        >
          {roomCount}
        </span>
      </div>
    </div>
  );
});

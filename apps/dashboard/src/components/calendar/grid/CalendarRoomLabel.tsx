"use client";

import React from "react";
import { CALENDAR_THEME } from "../calendarTheme";

interface CalendarRoomLabelProps {
  title: string;
  isSelected: boolean;
  onClick: () => void;
  markers?: React.ReactNode;  // asterisks, status icons, badges, etc.
  leftColumnWidth: number;
}

/**
 * Sticky left cell for an individual room row.
 * Supports arbitrary markers (asterisks, icons, badge counts) via the markers prop.
 */
export const CalendarRoomLabel = React.memo(function CalendarRoomLabel({
  title,
  isSelected,
  onClick,
  markers,
  leftColumnWidth
}: CalendarRoomLabelProps) {
  return (
    <div
      className={[
        `flex items-center justify-between px-2 sm:px-4 ${CALENDAR_THEME.roomLabel.base} select-none cursor-pointer transition-colors rounded-0`,
        isSelected
          ? CALENDAR_THEME.roomLabel.selected
          : `${CALENDAR_THEME.roomLabel.default} ${CALENDAR_THEME.roomLabel.hover}`,
      ].join(" ")}
      style={{
        width: leftColumnWidth,
        minWidth: leftColumnWidth,
        flexShrink: 0,
        position: "sticky",
        left: 0,
        zIndex: 10,
        height: "100%"
      }}
      onClick={onClick}
      role="button"
      aria-label={`Select room ${title}`}
      aria-pressed={isSelected}
    >
      {/* Room name */}
      <span
        className={`text-sm truncate ${
          isSelected
            ? CALENDAR_THEME.roomLabel.selectedText
            : CALENDAR_THEME.roomLabel.defaultText
        }`}
      >
        {title}
      </span>

      {/* Markers (asterisks, icons, badges) */}
      {markers && (
        <span className="flex items-center gap-1 shrink-0 ml-2">{markers}</span>
      )}
    </div>
  );
});

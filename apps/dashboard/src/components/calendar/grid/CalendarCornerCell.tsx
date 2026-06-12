"use client";

import React from "react";
import { CALENDAR_CARET_CLASS } from "../calendarTheme";

interface CalendarCornerCellProps {
  leftColumnWidth: number;
  isMobileViewport: boolean;
  isRoomsColumnSelected: boolean;
  /** Whether any room groups are visible (controls expand/collapse-all affordance). */
  hasGroups: boolean;
  areAllExpanded: boolean;
  showResizeHandle: boolean;
  onHeaderClick: () => void;
  onToggleAll: (e: React.MouseEvent) => void;
  onResizePointerDown: (e: React.PointerEvent<HTMLDivElement>) => void;
}

/**
 * Top-left corner of the calendar grid (the "Rooms" header cell).
 *
 * Sticky on both axes so it pins to the top-left while the grid scrolls.
 * Memoized because none of its props change during scroll — this keeps it out
 * of the parent's per-scroll-frame re-render and avoids needless reconciliation.
 */
export const CalendarCornerCell = React.memo(function CalendarCornerCell({
  leftColumnWidth,
  isMobileViewport,
  isRoomsColumnSelected,
  hasGroups,
  areAllExpanded,
  showResizeHandle,
  onHeaderClick,
  onToggleAll,
  onResizePointerDown,
}: CalendarCornerCellProps) {
  return (
    <div
      className={[
        "relative flex items-center font-semibold text-xs uppercase tracking-wide border-0 border-solid border-b border-r select-none transition-colors",
        isMobileViewport && isRoomsColumnSelected
          ? "text-blue-700 dark:text-blue-300 bg-blue-50 dark:bg-blue-950/40 border-blue-200 dark:border-blue-800"
          : "text-slate-500 dark:text-slate-400 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-700",
      ].join(" ")}
      style={{
        width: leftColumnWidth,
        minWidth: leftColumnWidth,
        flexShrink: 0,
        position: "sticky",
        left: 0,
        zIndex: 30,
        height: "100%",
        overflow: "visible",
        willChange: "transform",
      }}
      onClick={onHeaderClick}
      role={isMobileViewport ? "button" : undefined}
      aria-pressed={isMobileViewport ? isRoomsColumnSelected : undefined}
      aria-label={isMobileViewport ? "Select rooms column to resize" : undefined}
    >
      <div className="absolute inset-0 z-10 flex items-center justify-start gap-1 px-2 sm:px-3 pointer-events-none">
        {hasGroups && (
          <button
            type="button"
            className={`pointer-events-auto ${CALENDAR_CARET_CLASS}`}
            onClick={onToggleAll}
            aria-label={areAllExpanded ? "Collapse all room groups" : "Expand all room groups"}
            title={areAllExpanded ? "Collapse all" : "Expand all"}
          >
            <span
              className="text-xs transition-transform"
              style={{ transform: areAllExpanded ? "rotate(90deg)" : "rotate(0deg)" }}
            >
              ▶
            </span>
          </button>
        )}
        <span
          className="truncate pointer-events-auto cursor-pointer select-none"
          onClick={onToggleAll}
          role="button"
          aria-label={areAllExpanded ? "Collapse all room groups" : "Expand all room groups"}
        >
          Rooms
        </span>
      </div>

      {/* Drag handle on right edge */}
      {showResizeHandle && (
        <div
          onPointerDown={onResizePointerDown}
          className={[
            "group absolute top-0 flex items-center justify-center cursor-col-resize touch-none h-full",
          ].join(" ")}
          style={{ 
            right: isMobileViewport ? -14 : -4, 
            width: isMobileViewport ? 28 : 8, 
            zIndex: 31 
          }}
          title="Drag to resize"
        >
          <div className="w-0.5 h-4 rounded-full bg-blue-500 dark:bg-blue-400 group-hover:bg-blue-600 dark:group-hover:bg-blue-300 transition-colors" />
        </div>
      )}
    </div>
  );
});

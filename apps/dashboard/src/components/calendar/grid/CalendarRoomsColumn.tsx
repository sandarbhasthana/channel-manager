"use client";

import React, { useMemo } from "react";
import { CalendarRoomTypeLabel } from "./CalendarRoomTypeLabel";
import { CalendarRoomLabel } from "./CalendarRoomLabel";
import type { Room } from "@/components/calendar/utils/types";
import type { VirtualRowItem } from "../layout/virtualTypes";

import { HEADER_ROW_HEIGHT, TYPE_ROW_HEIGHT as LABEL_TYPE_ROW_HEIGHT, ROW_HEIGHT as LABEL_ROW_HEIGHT } from "../calendarConfig";
import { CALENDAR_THEME, CALENDAR_COLORS } from "../calendarTheme";
export { HEADER_ROW_HEIGHT };

interface CalendarRoomsColumnProps {
  resources: Room[];
  leftColumnWidth: number;
  selectedResource: string | null;
  onSelectRoom: (roomId: string) => void;
  expandedGroups: Record<string, boolean>;
  onToggleGroup: (groupId: string) => void;
  virtualItems: VirtualRowItem[];
  scrollElementRef: React.RefObject<HTMLDivElement | null>;
  /** Ref to an absolutely-positioned highlight div inside the sticky container. */
  dragHighlightRef?: React.RefObject<HTMLDivElement | null>;
  /** Ref to a muted source-position indicator that can persist through review. */
  dragGhostHighlightRef?: React.RefObject<HTMLDivElement | null>;
}

/**
 * Static rooms label column — sticky left so it never scrolls horizontally.
 * Renders all rows in a plain flex column (heights match CalendarDateGrid's
 * virtual row sizes) so it stays pixel-aligned with the date grid body.
 */
export const CalendarRoomsColumn = React.memo(function CalendarRoomsColumn({
  resources,
  leftColumnWidth,
  selectedResource,
  onSelectRoom,
  expandedGroups,
  onToggleGroup,
  virtualItems,
  dragHighlightRef,
  dragGhostHighlightRef,
}: CalendarRoomsColumnProps) {
  // Pre-build stable toggle callbacks keyed by groupId.
  const toggleCallbacks = useMemo(() => {
    const cbs: Record<string, () => void> = {};
    for (const group of resources) {
      const id = group.id;
      cbs[id] = () => onToggleGroup(id);
    }
    return cbs;
  }, [resources, onToggleGroup]);

  // Pre-build stable select callbacks keyed by roomId.
  const selectCallbacks = useMemo(() => {
    const cbs: Record<string, () => void> = {};
    for (const group of resources) {
      for (const room of group.children ?? []) {
        const id = room.id;
        cbs[id] = () => onSelectRoom(id);
      }
    }
    return cbs;
  }, [resources, onSelectRoom]);

  // Lookup maps to avoid O(N) search per row.
  const groupById = useMemo(() => {
    const m: Record<string, Room> = {};
    for (const g of resources) m[g.id] = g;
    return m;
  }, [resources]);

  const roomById = useMemo(() => {
    const m: Record<string, { room: NonNullable<Room["children"]>[number]; groupId: string }> = {};
    for (const g of resources) {
      for (const r of g.children ?? []) m[r.id] = { room: r, groupId: g.id };
    }
    return m;
  }, [resources]);

  return (
    <div
      className={CALENDAR_THEME.roomsColumn.bg}
      style={{
        position: "sticky",
        left: 0,
        zIndex: 10,
        width: leftColumnWidth,
        minWidth: leftColumnWidth,
        flexShrink: 0,
        alignSelf: "flex-start",
        willChange: "transform",
      }}
    >
      {/* Drag-move room cell highlight — bright bar on left edge (driven by ref from CalendarDateGrid) */}
      {dragHighlightRef && (
        <div
          ref={dragHighlightRef}
          aria-hidden="true"
          style={{
            display: "none",
            position: "absolute",
            left: 0,
            width: 4,
            height: LABEL_ROW_HEIGHT, // overridden via ref during drag
            top: 0,
            borderRadius: "0 2px 2px 0",
            pointerEvents: "none",
            zIndex: 20,
            transition: "top 0.08s ease, height 0.08s ease, background-color 0.08s ease",
          }}
        />
      )}
      {/* Original room indicator — muted bar kept visible while the review modal is open */}
      {dragGhostHighlightRef && (
        <div
          ref={dragGhostHighlightRef}
          aria-hidden="true"
          style={{
            display: "none",
            position: "absolute",
            left: 6,
            width: 4,
            height: LABEL_ROW_HEIGHT, // overridden via ref during drag
            top: 0,
            borderRadius: 2,
            backgroundColor: CALENDAR_COLORS.dragGhost,
            pointerEvents: "none",
            zIndex: 19,
            transition: "top 0.08s ease, height 0.08s ease",
          }}
        />
      )}
      {virtualItems.map((item) => {
        if (item.type === "group") {
          const group = groupById[item.roomTypeId];
          if (!group) return null;
          const isExpanded = expandedGroups[group.id] ?? true;
          return (
            <div
              key={`group-${item.roomTypeId}`}
              style={{ height: LABEL_TYPE_ROW_HEIGHT }}
            >
              <CalendarRoomTypeLabel
                title={group.title}
                roomCount={(group.children ?? []).length}
                isExpanded={isExpanded}
                onToggle={toggleCallbacks[group.id]}
                leftColumnWidth={leftColumnWidth}
              />
            </div>
          );
        }

        const entry = roomById[item.roomId];
        if (!entry) return null;
        return (
          <div
            key={`room-${item.roomId}`}
            style={{ height: LABEL_ROW_HEIGHT }}
          >
            <CalendarRoomLabel
              title={entry.room.title}
              isSelected={selectedResource === item.roomId}
              onClick={selectCallbacks[item.roomId]}
              leftColumnWidth={leftColumnWidth}
            />
          </div>
        );
      })}
    </div>
  );
});

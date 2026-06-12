"use client";

import React, { useMemo } from "react";
import { CalendarRoomTypeRow } from "./CalendarRoomTypeRow";
import { CalendarRoomRow } from "./CalendarRoomRow";
import type { Room, Reservation } from "@/components/calendar/utils/types";
import type { PositionedEvent } from "../layout/useEventPositioning";
import type { RoomTypeRates } from "@/lib/hooks/useRatesData";

// Stable empty array — prevents CalendarRoomRow from seeing a new reference
// when a room has no events.
const EMPTY_EVENTS: PositionedEvent[] = [];
// Stable empty roomIds for groups that somehow have no children.
const EMPTY_ROOM_IDS: string[] = [];

interface CalendarBodyRowsProps {
  resources: Room[];
  windowStart: string;
  visibleDays: number;
  cellWidth: number;
  leftColumnWidth: number;
  positionedEvents: PositionedEvent[];
  reservations: Reservation[];
  ratesLookup: Record<string, RoomTypeRates | null>;
  selectedResource: string | null;
  onSelectRoom: (roomId: string) => void;
  onCellClick?: (roomId: string, roomName: string, date: string, x: number, y: number) => void;
  onEventClick?: (event: PositionedEvent, x: number, y: number) => void;
  onBlockClick?: (event: PositionedEvent, x: number, y: number) => void;
  onRangeSelect?: (roomId: string, startDate: string, endDate: string) => void;
  cellClassName?: (roomId: string, date: string) => string | undefined;
  showRates?: boolean;
  showOccupancy?: boolean;
  onRateClick?: (roomTypeId: string, roomTypeName: string, dateStr: string, currentPrice: number) => void;
  /** Expand state owned by the parent (CustomCalendarGrid) so rooms column stays in sync */
  expandedGroups: Record<string, boolean>;
  onToggleGroup: (groupId: string) => void;
  /** When true, row label cells are omitted (rendered by CalendarRoomsColumn instead) */
  hideLeftColumn?: boolean;
}

/**
 * Renders all room-type groups and their room rows.
 * Memoized so a rates revalidation that returns identical data (stable ratesLookup
 * reference from CalendarPanel) does not re-render the entire body.
 */
export const CalendarBodyRows = React.memo(function CalendarBodyRows({
  resources,
  windowStart,
  visibleDays,
  cellWidth,
  leftColumnWidth,
  positionedEvents,
  reservations,
  ratesLookup,
  selectedResource,
  onSelectRoom,
  onCellClick,
  onEventClick,
  onBlockClick,
  onRangeSelect,
  cellClassName,
  showRates = true,
  showOccupancy = true,
  onRateClick,
  expandedGroups,
  onToggleGroup,
  hideLeftColumn = false,
}: CalendarBodyRowsProps) {

  // Pre-compute roomIds per group so CalendarRoomTypeRow receives a stable
  // array reference across renders (avoids new [] on every render defeating memo).
  const roomIdsByGroup = useMemo(() => {
    const result: Record<string, string[]> = {};
    for (const group of resources) {
      result[group.id] = (group.children ?? []).map((c) => c.id);
    }
    return result;
  }, [resources]);

  // Pre-group events by roomId so each CalendarRoomRow gets a stable slice
  // without a per-row filter() call on every render. Recomputes only when
  // positionedEvents reference changes (i.e. when data actually changes).
  const eventsByRoom = useMemo(() => {
    const map: Record<string, PositionedEvent[]> = {};
    for (const e of positionedEvents) {
      if (!map[e.roomId]) map[e.roomId] = [];
      map[e.roomId].push(e);
    }
    return map;
  }, [positionedEvents]);

  return (
    <div className="flex flex-col">
      {resources.map((group) => {
        const isExpanded = expandedGroups[group.id] ?? true;
        const roomIds = roomIdsByGroup[group.id] ?? EMPTY_ROOM_IDS;
        const ratesForGroup = ratesLookup[group.title] ?? null;

        return (
          <React.Fragment key={group.id}>
            {/* Room type header row */}
            <CalendarRoomTypeRow
              groupId={group.id}
              title={group.title}
              roomCount={roomIds.length}
              isExpanded={isExpanded}
              onToggleGroup={onToggleGroup}
              hideLeftColumn={hideLeftColumn}
              windowStart={windowStart}
              visibleDays={visibleDays}
              cellWidth={cellWidth}
              leftColumnWidth={leftColumnWidth}
              ratesData={ratesForGroup}
              reservations={reservations}
              roomIds={roomIds}
              showRates={showRates}
              showOccupancy={showOccupancy}
              onRateClick={onRateClick}
            />

            {/* Individual room rows (hidden when collapsed) */}
            {isExpanded &&
              (group.children ?? []).map((room) => (
                <CalendarRoomRow
                  key={room.id}
                  roomId={room.id}
                  roomTitle={room.title}
                  windowStart={windowStart}
                  visibleDays={visibleDays}
                  cellWidth={cellWidth}
                  leftColumnWidth={leftColumnWidth}
                  events={eventsByRoom[room.id] ?? EMPTY_EVENTS}
                  isSelected={selectedResource === room.id}
                  onSelectRoom={onSelectRoom}
                  onCellClick={onCellClick}
                  onEventClick={onEventClick}
                  onBlockClick={onBlockClick}
                  onRangeSelect={onRangeSelect}
                  cellClassName={cellClassName}
                  hideLeftColumn={hideLeftColumn}
                />
              ))}
          </React.Fragment>
        );
      })}
    </div>
  );
});

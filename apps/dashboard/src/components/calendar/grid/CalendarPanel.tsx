"use client";

import React, { useMemo, useRef } from "react";
import { CalendarBodyRows } from "./CalendarBodyRows";
import { useEventPositioning, type PositionedEvent } from "../layout/useEventPositioning";
import { useRatesData } from "@/lib/hooks/useRatesData";
import type { Room, Reservation, ProcessedEvent, CalendarFilterCriteria } from "@/components/calendar/utils/types";
import type { RoomTypeRates } from "@/lib/hooks/useRatesData";

// Stable empty object — returned from ratesLookup when rates haven't loaded yet.
// Using a module-level constant ensures the same reference is returned across
// renders, so CalendarBodyRows.React.memo can bail out correctly.
const EMPTY_RATES_LOOKUP: Record<string, RoomTypeRates | null> = {};

/**
 * Deep-equal check for a single RoomTypeRates entry.
 * Compares every RateCell field so React.memo on CalendarRoomTypeRow can bail out
 * when a revalidation returns identical data.
 */
function roomTypeRatesEqual(a: RoomTypeRates, b: RoomTypeRates): boolean {
  if (a.roomTypeId !== b.roomTypeId || a.totalRooms !== b.totalRooms) return false;
  const aEntries = Object.entries(a.dates);
  if (aEntries.length !== Object.keys(b.dates).length) return false;
  for (const [date, ac] of aEntries) {
    const bc = b.dates[date];
    if (!bc) return false;
    if (
      ac.finalPrice   !== bc.finalPrice   ||
      ac.basePrice    !== bc.basePrice    ||
      ac.availability !== bc.availability ||
      ac.isOverride   !== bc.isOverride   ||
      ac.isSeasonal   !== bc.isSeasonal
    ) return false;
  }
  return true;
}

interface CalendarPanelProps {
  windowStart: string;         // YYYY-MM-DD — first visible date for this panel
  panelWidth: number;          // total panel width in px (left col + date cols)
  visibleDays: number;
  cellWidth: number;
  leftColumnWidth: number;
  events: ProcessedEvent[];    // ALL processed events; positioning is per-window
  reservations: Reservation[];
  resources: Room[];
  filters: CalendarFilterCriteria;
  selectedResource: string | null;
  setSelectedResource: (id: string) => void;
  onCellClick?: (roomId: string, roomName: string, date: string, x: number, y: number) => void;
  onEventClick?: (event: PositionedEvent, x: number, y: number) => void;
  onBlockClick?: (event: PositionedEvent, x: number, y: number) => void;
  onRangeSelect?: (roomId: string, startDate: string, endDate: string) => void;
  onRateClick?: (roomTypeId: string, roomTypeName: string, dateStr: string, currentPrice: number) => void;
  cellClassName?: (roomId: string, date: string) => string | undefined;
  /** When true, label cells are omitted — CalendarRoomsColumn renders them as a static layer */
  hideLeftColumn?: boolean;
  expandedGroups: Record<string, boolean>;
  onToggleGroup: (groupId: string) => void;
}

/**
 * One panel in the carousel.
 * Independently positions events and pre-fetches rates for `windowStart`.
 * SWR caches per URL key — so prev/next panels pre-warm the cache at no
 * extra cost: when they become the center panel the data is already there.
 */
export const CalendarPanel = React.memo(function CalendarPanel({
  windowStart,
  panelWidth,
  visibleDays,
  cellWidth,
  leftColumnWidth,
  events,
  reservations,
  resources,
  filters,
  selectedResource,
  setSelectedResource,
  onCellClick,
  onEventClick,
  onBlockClick,
  onRangeSelect,
  onRateClick,
  cellClassName,
  hideLeftColumn = false,
  expandedGroups,
  onToggleGroup,
}: CalendarPanelProps) {
  // ── Event positioning ────────────────────────────────────────────────────
  const positionedEvents = useEventPositioning(
    events,
    windowStart,
    visibleDays,
    cellWidth,
    leftColumnWidth,
  );

  // ── Rates (pre-fetched via SWR — adjacent panels warm the cache) ─────────
  const windowStartDate = useMemo(() => new Date(windowStart), [windowStart]);
  const { data: ratesData } = useRatesData(windowStartDate, visibleDays, "base");

  // Stable-reference lookup: reuse previous RoomTypeRates objects when their values
  // haven't changed so React.memo on CalendarRoomTypeRow (and CalendarBodyRows) can
  // bail out on revalidation fetches that return identical data.
  const prevRatesLookupRef = useRef<Record<string, RoomTypeRates | null>>({});

  const ratesLookup = useMemo<Record<string, RoomTypeRates | null>>(() => {
    if (!ratesData?.length) {
      prevRatesLookupRef.current = EMPTY_RATES_LOOKUP;
      return EMPTY_RATES_LOOKUP;
    }

    const prev = prevRatesLookupRef.current;
    const next: Record<string, RoomTypeRates | null> = {};
    let changed = false;

    for (const rt of ratesData as RoomTypeRates[]) {
      const existing = prev[rt.roomTypeName];
      if (existing && roomTypeRatesEqual(existing, rt)) {
        next[rt.roomTypeName] = existing; // stable ref → CalendarRoomTypeRow bails out
      } else {
        next[rt.roomTypeName] = rt;
        changed = true;
      }
    }

    // Detect removed room types
    for (const key of Object.keys(prev)) {
      if (!(key in next)) { changed = true; break; }
    }

    if (!changed) return prev; // identical content → same object ref → no downstream re-renders

    prevRatesLookupRef.current = next;
    return next;
  }, [ratesData]);

  // ── Apply filters ─────────────────────────────────────────────────────────
  const filteredEvents = useMemo(() => {
    return positionedEvents.filter((e) => {
      if (e.isBlock && !filters.showBlocks) return false;
      if (!e.isBlock) {
        if (filters.statuses.length > 0 && !filters.statuses.includes(e.status ?? "")) return false;
        if (
          filters.paymentStatuses.length > 0 &&
          !filters.paymentStatuses.includes(e.paymentStatus as "PAID" | "PARTIALLY_PAID" | "UNPAID" | "REFUND_DUE")
        ) return false;
        if (
          filters.guestSearch.trim() &&
          !e.title.toLowerCase().includes(filters.guestSearch.toLowerCase())
        ) return false;
      }
      return true;
    });
  }, [positionedEvents, filters]);

  // ── Filtered resources ────────────────────────────────────────────────────
  const filteredResources = useMemo(() => {
    if (filters.roomTypes.length === 0 && filters.roomIds.length === 0) return resources;
    return resources
      .filter((g) => filters.roomTypes.length === 0 || filters.roomTypes.includes(g.id))
      .map((g) => ({
        ...g,
        children:
          filters.roomIds.length === 0
            ? g.children
            : (g.children ?? []).filter((r) => filters.roomIds.includes(r.id)),
      }))
      .filter((g) => (g.children ?? []).length > 0);
  }, [resources, filters.roomTypes, filters.roomIds]);

  return (
    <div
      style={{
        width: panelWidth,
        flexShrink: 0,
        minWidth: panelWidth,
        contain: "layout paint style",
        contentVisibility: "auto",
        containIntrinsicSize: "1200px",
        isolation: "isolate",
      }}
      // Pointer events disabled on non-visible panels to prevent ghost interactions
      aria-hidden={undefined /* set by parent if needed */}
    >
      {/* Room rows */}
      <div style={{ minWidth: panelWidth }}>
        <CalendarBodyRows
          resources={filteredResources}
          windowStart={windowStart}
          visibleDays={visibleDays}
          cellWidth={cellWidth}
          leftColumnWidth={leftColumnWidth}
          positionedEvents={filteredEvents}
          reservations={reservations}
          ratesLookup={ratesLookup}
          selectedResource={selectedResource}
          onSelectRoom={setSelectedResource}
          onCellClick={onCellClick}
          onEventClick={onEventClick}
          onBlockClick={onBlockClick}
          onRangeSelect={onRangeSelect}
          cellClassName={cellClassName}
          showRates={filters.showRates}
          showOccupancy={filters.showOccupancy}
          onRateClick={onRateClick}
          hideLeftColumn={hideLeftColumn}
          expandedGroups={expandedGroups}
          onToggleGroup={onToggleGroup}
        />
      </div>
    </div>
  );
});

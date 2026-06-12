"use client";

import React, { useMemo, useRef, useState, useCallback, useLayoutEffect } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import type { VirtualItem } from "@tanstack/react-virtual";
import { toast } from "sonner";
import { CalendarEventBar } from "./CalendarEventBar";
import { GroupRateCell, RoomRateLoaderCell } from "./RateMatrixCells";
import { addDays, diffDays, isToday, isWeekend, parseLocalDate, localDateString } from "@/components/calendar/utils/calendarHelpers";
import { isCalendarEditLockedStatus } from "@/lib/reservations/calendar-move-validation";
import type { Room, Reservation, ProcessedEvent, CalendarFilterCriteria } from "@/components/calendar/utils/types";
import type { PositionedEvent } from "../layout/useEventPositioning";
import type { VirtualRowItem } from "../layout/virtualTypes";
import { ROW_HEIGHT, TYPE_ROW_HEIGHT, BAR_TOP, BAR_HEIGHT } from "../calendarConfig";
import { CALENDAR_THEME, CALENDAR_COLORS } from "../calendarTheme";
import { getGhostDropValidityStyles, getInvalidMoveTarget, hasDropConflict } from "./calendarDropValidation";

export { TYPE_ROW_HEIGHT, ROW_HEIGHT };

const BAR_START_FRAC = 0.45;  // bar left edge starts 45% into check-in cell
const BAR_END_FRAC = 0.35;    // bar right edge ends at 35% into checkout cell
const BAR_LIGHT_ALPHA = "80";
const BLOCK_CELL_GAP = 4;

const EMPTY_EVENTS: ProcessedEvent[] = [];
const EMPTY_ROOM_IDS: string[] = [];

// ── Types ─────────────────────────────────────────────────────────────────────

interface DragState {
  roomId: string;
  roomTitle: string;
  selStartOffset: number;
  selEndOffset: number;
}

interface MoveDrag {
  event: PositionedEvent;
  sourceRoomId: string;
  pointerId: number;
  /** Pixels from bar's left visible edge to where pointer pressed. */
  originOffsetX: number;
  /** Day offset of event.start from rangeStart at drag start. */
  startDayOffset: number;
  /** Duration of event in days (constant during drag). */
  durationDays: number;
  /** Today offset (min allowed start day). */
  todayOff: number;
  /** Cached grid rect to avoid per-frame layout flush. Updated on scroll. */
  dateGridRect: DOMRect;
  /** Target room for drop. */
  currentTargetRoomId: string;
  /** Current snapped day offset for new start. */
  currentSnapDayOffset: number;
  /** Current raw attempted start day offset before enforcing the today boundary. */
  currentRawDayOffset: number;
  /** Last known pointer position — used by edge-scroll loop. */
  lastClientX: number;
  lastClientY: number;
}

interface DragContext {
  cellWidth: number;
  rangeStart: string;
  virtualItems: VirtualRowItem[];
}

interface CalendarDateGridProps {
  resources: Room[];
  virtualItems: VirtualRowItem[];
  virtualCols: VirtualItem[];
  colIndexToDate: (i: number) => string;
  totalDateWidth: number;
  scrollElementRef: React.RefObject<HTMLDivElement | null>;
  events: ProcessedEvent[];
  reservations: Reservation[];
  rangeStart: string;
  expandedGroups: Record<string, boolean>;
  onToggleGroup: (groupId: string) => void;
  cellWidth: number;
  leftColumnWidth: number;
  filters: CalendarFilterCriteria;
  selectedResource: string | null;
  setSelectedResource: (id: string) => void;
  onCellClick?: (roomId: string, roomName: string, date: string, x: number, y: number) => void;
  onEventClick?: (event: PositionedEvent, x: number, y: number) => void;
  onBlockClick?: (event: PositionedEvent, x: number, y: number) => void;
  onRangeSelect?: (roomId: string, startDate: string, endDate: string, x: number, y: number) => void;
  onEventResized?: (event: PositionedEvent, newStart: string, newEnd: string) => void;
  onEventMoved?: (event: PositionedEvent, newRoomId: string, newStart: string, newEnd: string) => void;
  onRateClick?: (roomTypeId: string, roomTypeName: string, dateStr: string, currentPrice: number) => void;
  /** Ref to the highlight div inside CalendarRoomsColumn — updated during drag. */
  roomCellHighlightRef?: React.RefObject<HTMLDivElement | null>;
  /** Ref to the highlight div inside CalendarHeaderRow — updated during drag. */
  dateColHighlightRef?: React.RefObject<HTMLDivElement | null>;
  /** Ref to the muted source-room indicator inside CalendarRoomsColumn. */
  roomCellGhostHighlightRef?: React.RefObject<HTMLDivElement | null>;
  /** Ref to the muted source-date indicator inside CalendarHeaderRow. */
  dateColGhostHighlightRef?: React.RefObject<HTMLDivElement | null>;
  /** Called to expand a collapsed group row (auto-expand on 3s hover). */
  onExpandGroup?: (groupId: string) => void;
  /** Event IDs currently being committed to the server — render with shimmer. */
  pendingEventIds?: ReadonlySet<string>;
  /**
   * Rate cell values are no longer passed as a prop — individual cells subscribe
   * to the streaming per-cell store (see RateMatrixCells / useRateCell). These
   * flags drive the per-cell loading skeletons while a fetch is in flight.
   */
  ratesIsLoading: boolean;
  ratesIsValidating: boolean;
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function positionEvent(event: ProcessedEvent, rangeStart: string, cellWidth: number): PositionedEvent {
  const startOffset = diffDays(rangeStart, event.start);
  const endOffset = diffDays(rangeStart, event.end);
  const leftPx = event.isBlock
    ? startOffset * cellWidth + BLOCK_CELL_GAP
    : startOffset * cellWidth + BAR_START_FRAC * cellWidth;
  const rightPx = event.isBlock
    ? (Math.max(startOffset, endOffset) + 1) * cellWidth - BLOCK_CELL_GAP
    : endOffset * cellWidth + BAR_END_FRAC * cellWidth;
  return {
    ...event,
    leftPx,
    widthPx:     Math.max(0, rightPx - leftPx),
    clampedLeft:  false,
    clampedRight: false,
  };
}

/** Returns the room row that visually contains absY, or the nearest room row. */
function findTargetRow(
  absY: number,
  vItems: VirtualItem[],
  virtualRowItems: VirtualRowItem[],
): { roomId: string; rowStart: number } | null {
  type RowInfo = { roomId: string; rowStart: number; rowEnd: number };
  const roomRows: RowInfo[] = [];

  for (const vRow of vItems) {
    const item = virtualRowItems[vRow.index];
    if (item?.type === "room") {
      roomRows.push({ roomId: item.roomId, rowStart: vRow.start, rowEnd: vRow.start + vRow.size });
    }
  }
  if (roomRows.length === 0) return null;

  // Exact hit
  const exact = roomRows.find((r) => r.rowStart <= absY && absY < r.rowEnd);
  if (exact) return { roomId: exact.roomId, rowStart: exact.rowStart };

  // Nearest by centre distance
  let nearest = roomRows[0];
  let minDist = Math.abs(absY - (nearest.rowStart + nearest.rowEnd) / 2);
  for (const r of roomRows) {
    const dist = Math.abs(absY - (r.rowStart + r.rowEnd) / 2);
    if (dist < minDist) { minDist = dist; nearest = r; }
  }
  return { roomId: nearest.roomId, rowStart: nearest.rowStart };
}

/** Returns the group row that the pointer is exactly over, or null. */
function findGroupRow(
  absY: number,
  vItems: VirtualItem[],
  virtualRowItems: VirtualRowItem[],
): { groupId: string; rowStart: number } | null {
  for (const vRow of vItems) {
    const item = virtualRowItems[vRow.index];
    if (item?.type === "group") {
      const rowEnd = vRow.start + vRow.size;
      if (vRow.start <= absY && absY < rowEnd) {
        return { groupId: item.roomTypeId, rowStart: vRow.start };
      }
    }
  }
  return null;
}

// ── Drag highlight helpers ─────────────────────────────────────────────────────

/** Updates the rooms-column left-edge bar indicator during booking drag. */
function applyRoomHighlight(
  el: HTMLDivElement,
  type: "room" | "group",
  rowStart: number,
  height: number,
) {
  el.style.top             = `${rowStart}px`;
  el.style.height          = `${height}px`;
  el.style.backgroundColor = type === "room"
    ? CALENDAR_COLORS.dragIndicator.room
    : CALENDAR_COLORS.dragIndicator.group;
  el.style.display         = "block";
}

/** Updates the date-header top-edge bar indicator during booking drag. */
function applyDateHighlight(
  el: HTMLDivElement,
  snapDayOffset: number,
  cellWidth: number,
  type: "room" | "group",
) {
  el.style.left            = `${snapDayOffset * cellWidth}px`;
  el.style.width           = `${cellWidth}px`;
  el.style.backgroundColor = type === "room"
    ? CALENDAR_COLORS.dragIndicator.room
    : CALENDAR_COLORS.dragIndicator.group;
  el.style.display         = "block";
}

function applyRoomGhostHighlight(el: HTMLDivElement, rowStart: number) {
  el.style.top     = `${rowStart}px`;
  el.style.height  = `${ROW_HEIGHT}px`;
  el.style.display = "block";
}

function applyDateGhostHighlight(el: HTMLDivElement, dayOffset: number, cellWidth: number) {
  el.style.left    = `${dayOffset * cellWidth}px`;
  el.style.width   = `${cellWidth}px`;
  el.style.display = "block";
}

function hideGhostHighlights(
  roomCellGhostHighlightRef?: React.RefObject<HTMLDivElement | null>,
  dateColGhostHighlightRef?: React.RefObject<HTMLDivElement | null>,
) {
  if (roomCellGhostHighlightRef?.current) roomCellGhostHighlightRef.current.style.display = "none";
  if (dateColGhostHighlightRef?.current) dateColGhostHighlightRef.current.style.display = "none";
}

function hideDragHighlights(
  roomCellHighlightRef?: React.RefObject<HTMLDivElement | null>,
  dateColHighlightRef?: React.RefObject<HTMLDivElement | null>,
) {
  if (roomCellHighlightRef?.current) roomCellHighlightRef.current.style.display = "none";
  if (dateColHighlightRef?.current) dateColHighlightRef.current.style.display = "none";
}

function findRoomRowStart(roomId: string, virtualRowItems: VirtualRowItem[]) {
  let rowStart = 0;
  for (const item of virtualRowItems) {
    if (item.type === "room" && item.roomId === roomId) return rowStart;
    rowStart += item.type === "group" ? TYPE_ROW_HEIGHT : ROW_HEIGHT;
  }
  return null;
}

/** Returns the exact visual row (room or group) at absY, or null. */
function findVisualRow(
  absY: number,
  vItems: VirtualItem[],
  virtualRowItems: VirtualRowItem[],
): { type: "room"; roomId: string; rowStart: number } | { type: "group"; groupId: string; rowStart: number } | null {
  for (const vRow of vItems) {
    const item = virtualRowItems[vRow.index];
    if (!item) continue;
    const rowEnd = vRow.start + vRow.size;
    if (vRow.start <= absY && absY < rowEnd) {
      if (item.type === "room") return { type: "room", roomId: item.roomId, rowStart: vRow.start };
      if (item.type === "group") return { type: "group", groupId: item.roomTypeId, rowStart: vRow.start };
    }
  }
  return null;
}

function applyGhostDropValidity(el: HTMLDivElement, isInvalid: boolean, origBg: string) {
  const styles = getGhostDropValidityStyles(isInvalid, origBg);
  el.style.background = styles.background;
  el.style.outline = styles.outline;
}

// ── Component ─────────────────────────────────────────────────────────────────

export const CalendarDateGrid = React.memo(function CalendarDateGrid({
  resources,
  virtualItems,
  virtualCols,
  colIndexToDate,
  totalDateWidth,
  scrollElementRef,
  events,
  reservations,
  rangeStart,
  expandedGroups,
  cellWidth,
  leftColumnWidth,
  filters,
  onCellClick,
  onEventClick,
  onBlockClick,
  onRangeSelect,
  onEventResized,
  onEventMoved,
  onRateClick,
  roomCellHighlightRef,
  dateColHighlightRef,
  roomCellGhostHighlightRef,
  dateColGhostHighlightRef,
  onExpandGroup,
  pendingEventIds,
  ratesIsLoading,
  ratesIsValidating,
}: CalendarDateGridProps) {
  // ── Row virtualizer ───────────────────────────────────────────────────────
  const rowVirtualizer = useVirtualizer({
    count: virtualItems.length,
    getScrollElement: () => scrollElementRef.current,
    estimateSize: (i) =>
      virtualItems[i]?.type === "group" ? TYPE_ROW_HEIGHT : ROW_HEIGHT,
    overscan: 5,
  });

  // Rate cell values stream in per-cell via the store (see RateMatrixCells);
  // this flag only gates the loading skeletons while a fetch is in flight.
  const isFetchingRates = ratesIsLoading || ratesIsValidating;

  // ── Lookup maps ───────────────────────────────────────────────────────────
  const groupById = useMemo(() => {
    const m: Record<string, Room> = {};
    for (const g of resources) m[g.id] = g;
    return m;
  }, [resources]);

  const roomById = useMemo(() => {
    const m: Record<string, { room: NonNullable<Room["children"]>[number]; group: Room }> = {};
    for (const g of resources) {
      for (const r of g.children ?? []) m[r.id] = { room: r, group: g };
    }
    return m;
  }, [resources]);

  const roomIdsByGroup = useMemo(() => {
    const m: Record<string, string[]> = {};
    for (const g of resources) m[g.id] = (g.children ?? []).map((r) => r.id);
    return m;
  }, [resources]);

  // ── Filter events ─────────────────────────────────────────────────────────
  const filteredEvents = useMemo(() => {
    return events.filter((e) => {
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
  }, [events, filters]);

  const eventsByRoom = useMemo(() => {
    const m: Record<string, ProcessedEvent[]> = {};
    for (const e of filteredEvents) {
      if (!m[e.roomId]) m[e.roomId] = [];
      m[e.roomId].push(e);
    }
    return m;
  }, [filteredEvents]);

  // ── Occupancy ─────────────────────────────────────────────────────────────
  const occupancyByDateByGroup = useMemo(() => {
    const result: Record<string, Record<string, number>> = {};
    const validStatuses = new Set(["CONFIRMED", "IN_HOUSE", "CHECKIN_DUE", "CHECKOUT_DUE"]);
    for (const group of resources) {
      const roomIdSet = new Set((group.children ?? []).map((r) => r.id));
      const dateMap: Record<string, number> = {};
      for (const r of reservations) {
        if (!roomIdSet.has(r.roomId) || !r.status || !validStatuses.has(r.status)) continue;
        let d = new Date(r.checkIn);
        const end = new Date(r.checkOut);
        while (d < end) {
          const ds = d.toISOString().slice(0, 10);
          dateMap[ds] = (dateMap[ds] ?? 0) + 1;
          d = addDays(d, 1);
        }
      }
      result[group.id] = dateMap;
    }
    return result;
  }, [resources, reservations]);

  const todayStr = useMemo(() => localDateString(), []);

  // ── Drag-to-select state ──────────────────────────────────────────────────
  const [drag, setDrag] = useState<DragState | null>(null);
  const dragRef = useRef<DragState | null>(null);

  const getOffsetFromClientX = useCallback(
    (clientX: number, rowEl: HTMLElement): number => {
      const rect = rowEl.getBoundingClientRect();
      return Math.max(0, Math.floor((clientX - rect.left) / cellWidth));
    },
    [cellWidth],
  );

  const handleRoomPointerDown = useCallback(
    (e: React.PointerEvent<HTMLDivElement>, roomId: string, roomTitle: string) => {
      if (e.pointerType !== "mouse") return;
      if (moveDragRef.current) return; // move drag in progress
      const offset = getOffsetFromClientX(e.clientX, e.currentTarget);
      const state: DragState = { roomId, roomTitle, selStartOffset: offset, selEndOffset: offset };
      dragRef.current = state;
      setDrag(state);
      e.currentTarget.setPointerCapture(e.pointerId);
    },
    [getOffsetFromClientX],
  );

  const handleRoomPointerMove = useCallback(
    (e: React.PointerEvent<HTMLDivElement>) => {
      if (!dragRef.current) return;
      const offset = getOffsetFromClientX(e.clientX, e.currentTarget);
      const updated = { ...dragRef.current, selEndOffset: offset };
      dragRef.current = updated;
      setDrag(updated);
    },
    [getOffsetFromClientX],
  );

  const handleRoomPointerUp = useCallback(
    (e: React.PointerEvent<HTMLDivElement>, roomId: string, roomTitle: string) => {
      if (!dragRef.current || dragRef.current.roomId !== roomId) return;
      const endOffset = getOffsetFromClientX(e.clientX, e.currentTarget);
      const { selStartOffset } = dragRef.current;
      const minOff = Math.min(selStartOffset, endOffset);
      const maxOff = Math.max(selStartOffset, endOffset);
      dragRef.current = null;
      setDrag(null);

      const startDate = colIndexToDate(minOff);
      if (minOff === maxOff) {
        onCellClick?.(roomId, roomTitle, startDate, e.clientX, e.clientY);
      } else {
        const endDate = addDays(new Date(colIndexToDate(maxOff)), 1).toISOString().slice(0, 10);
        onRangeSelect?.(roomId, startDate, endDate, e.clientX, e.clientY);
      }
    },
    [getOffsetFromClientX, colIndexToDate, onCellClick, onRangeSelect],
  );

  // ── Move drag ─────────────────────────────────────────────────────────────
  const [draggingEventId, setDraggingEventId] = useState<string | null>(null);
  const moveDragRef = useRef<MoveDrag | null>(null);
  const stopDragRef = useRef<(() => void) | null>(null);
  const dateGridRef = useRef<HTMLDivElement>(null);
  const ghostRef = useRef<HTMLDivElement>(null);
  const ghostBarRef = useRef<HTMLDivElement>(null);
  const ghostLabelRef = useRef<HTMLSpanElement>(null);
  const edgeScrollFrameRef = useRef<number | null>(null);
  const groupAutoExpandTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const groupAutoExpandTargetRef = useRef<string | null>(null);
  const eventsRef = useRef<ProcessedEvent[]>(events);
  const ghostOrigBgRef = useRef<string>("");

  // Keep mutable context refs fresh each render (safe ref mutation outside effect)
  const rowVirtualizerRef = useRef(rowVirtualizer);
  rowVirtualizerRef.current = rowVirtualizer;

  const dragContextRef = useRef<DragContext>({ cellWidth, rangeStart, virtualItems });
  useLayoutEffect(() => {
    dragContextRef.current = { cellWidth, rangeStart, virtualItems };
  }, [cellWidth, rangeStart, virtualItems]);

  const onEventMovedRef = useRef(onEventMoved);
  useLayoutEffect(() => { onEventMovedRef.current = onEventMoved; }, [onEventMoved]);

  const onExpandGroupRef = useRef(onExpandGroup);
  useLayoutEffect(() => { onExpandGroupRef.current = onExpandGroup; }, [onExpandGroup]);

  const expandedGroupsRef = useRef(expandedGroups);
  useLayoutEffect(() => { expandedGroupsRef.current = expandedGroups; }, [expandedGroups]);

  useLayoutEffect(() => { eventsRef.current = events; }, [events]);

  // Cleanup on unmount
  useLayoutEffect(() => {
    return () => { stopDragRef.current?.(); };
  }, []);

  const handleEventMoveStart = useCallback((
    event: PositionedEvent,
    pointerId: number,
    clientX: number,
    clientY: number,
    barClientLeft: number,
  ) => {
    if (isCalendarEditLockedStatus(event.status)) return;
    if (!dateGridRef.current || !scrollElementRef.current) return;

    const dateGridRect = dateGridRef.current.getBoundingClientRect();
    const ctx = dragContextRef.current;
    const originOffsetX = clientX - barClientLeft;
    const startDayOffset = diffDays(ctx.rangeStart, event.start);
    const durationDays = diffDays(event.start, event.end);
    const todayOff = diffDays(ctx.rangeStart, todayStr);
    const snapOff = Math.max(todayOff, startDayOffset);
    const sourceRowStart = findRoomRowStart(event.roomId, ctx.virtualItems);

    if (sourceRowStart !== null && roomCellGhostHighlightRef?.current) {
      applyRoomGhostHighlight(roomCellGhostHighlightRef.current, sourceRowStart);
    }
    if (dateColGhostHighlightRef?.current) {
      applyDateGhostHighlight(dateColGhostHighlightRef.current, startDayOffset, ctx.cellWidth);
    }

    // ── Prepare ghost appearance ──────────────────────────────────────────
    if (ghostBarRef.current && ghostLabelRef.current) {
      const barWidthPx = (durationDays + BAR_END_FRAC - BAR_START_FRAC) * ctx.cellWidth;
      const barBg = event.isBlock
        ? CALENDAR_COLORS.block.gradient
        : `linear-gradient(to bottom, ${event.backgroundColor} 50%, ${event.backgroundColor}${BAR_LIGHT_ALPHA} 50%)`;
      ghostBarRef.current.style.width = `${barWidthPx}px`;
      ghostBarRef.current.style.background = barBg;
      ghostBarRef.current.style.borderRadius = event.clampedLeft
        ? (event.clampedRight ? "0" : "0 4px 4px 0")
        : (event.clampedRight ? "4px 0 0 4px" : "4px");
      ghostLabelRef.current.style.color = event.textColor;
      ghostLabelRef.current.textContent = event.guestName ?? event.title;
      ghostOrigBgRef.current = barBg;  // store directly, not via el.style readback
    }

    // ── Find initial target row ────────────────────────────────────────────
    const absY = clientY - dateGridRect.top;
    const vItems = rowVirtualizerRef.current.getVirtualItems();
    const target = findTargetRow(absY, vItems, ctx.virtualItems);
    const visualRow = findVisualRow(absY, vItems, ctx.virtualItems);
    const ghostRowStart = visualRow?.rowStart ?? target?.rowStart ?? 0;
    const visibleDateGridLeft = scrollElementRef.current.getBoundingClientRect().left + leftColumnWidth;
    const fixedLeft = Math.max(
      visibleDateGridLeft,
      dateGridRect.left + snapOff * ctx.cellWidth + BAR_START_FRAC * ctx.cellWidth,
    );
    const fixedTop  = dateGridRect.top + ghostRowStart + BAR_TOP;

    // Position ghost before showing to avoid flash
    const isOverCategoryInit = visualRow?.type === "group";
    if (ghostRef.current) {
      ghostRef.current.style.transform = `translate3d(${fixedLeft}px, ${fixedTop}px, 0)`;
      ghostRef.current.style.opacity = isOverCategoryInit ? "0.35" : "1";
      ghostRef.current.style.display = "block";
    }
    if (ghostBarRef.current) {
      const initTargetRoom = target?.roomId ?? event.roomId;
      applyGhostDropValidity(ghostBarRef.current,
        hasDropConflict(eventsRef.current, initTargetRoom, event.id, snapOff, durationDays, ctx.rangeStart),
        ghostOrigBgRef.current);
    }
    {
      const hlType: "room" | "group" | null =
        visualRow?.type === "room" ? "room"
        : (isOverCategoryInit && !(expandedGroupsRef.current[(visualRow as { groupId: string }).groupId] ?? true)) ? "group"
        : null;
      if (roomCellHighlightRef?.current) {
        if (hlType === "room") applyRoomHighlight(roomCellHighlightRef.current, "room", (visualRow as { rowStart: number }).rowStart, ROW_HEIGHT);
        else if (hlType === "group") applyRoomHighlight(roomCellHighlightRef.current, "group", (visualRow as { rowStart: number }).rowStart, TYPE_ROW_HEIGHT);
        else roomCellHighlightRef.current.style.display = "none";
      }
      if (dateColHighlightRef?.current) {
        if (hlType !== null) applyDateHighlight(dateColHighlightRef.current, snapOff, ctx.cellWidth, hlType);
        else dateColHighlightRef.current.style.display = "none";
      }
    }

    moveDragRef.current = {
      event,
      sourceRoomId: event.roomId,
      pointerId,
      originOffsetX,
      startDayOffset: snapOff,
      durationDays,
      todayOff,
      dateGridRect,
      currentTargetRoomId: target?.roomId ?? event.roomId,
      currentSnapDayOffset: snapOff,
      currentRawDayOffset: startDayOffset,
      lastClientX: clientX,
      lastClientY: clientY,
    };

    setDraggingEventId(event.id);

    // Prevent native touch-scroll while dragging; keyboard/mouse scroll still works
    scrollElementRef.current.style.touchAction = "none";

    function handleMovePointer(ev: PointerEvent) {
      const drag = moveDragRef.current;
      if (!drag || ev.pointerId !== drag.pointerId) return;

      drag.lastClientX = ev.clientX;
      drag.lastClientY = ev.clientY;

      const c = dragContextRef.current;
      const rect = drag.dateGridRect;

      // Snap day offset (clamped to today) — subtract start-fraction so ghost
      // aligns to the cell boundary at the check-in edge of the bar.
      const barStartX = ev.clientX - drag.originOffsetX;
      const rawOff = Math.round((barStartX - rect.left - BAR_START_FRAC * c.cellWidth) / c.cellWidth);
      const snapOff = Math.max(drag.todayOff, rawOff);
      drag.currentSnapDayOffset = snapOff;
      drag.currentRawDayOffset = rawOff;

      // Target row (for drop — always a room row)
      const absY = ev.clientY - rect.top;
      const vItems = rowVirtualizerRef.current.getVirtualItems();
      const target = findTargetRow(absY, vItems, c.virtualItems);
      if (target) drag.currentTargetRoomId = target.roomId;

      // Visual row (for ghost positioning — can be room or group)
      const visualRow = findVisualRow(absY, vItems, c.virtualItems);
      const ghostRowStart = visualRow?.rowStart ?? target?.rowStart ?? 0;

      // Update ghost — direct DOM, no React re-render
      const isOverCategory = visualRow?.type === "group";
      const visualOff = rawOff < drag.todayOff ? rawOff : snapOff;
      const visibleDateGridLeft = scrollElementRef.current
        ? scrollElementRef.current.getBoundingClientRect().left + leftColumnWidth
        : rect.left;
      const fixedLeft = Math.max(
        visibleDateGridLeft,
        rect.left + visualOff * c.cellWidth + BAR_START_FRAC * c.cellWidth,
      );
      const fixedTop  = rect.top + ghostRowStart + BAR_TOP;
      if (ghostRef.current) {
        ghostRef.current.style.transform = `translate3d(${fixedLeft}px, ${fixedTop}px, 0)`;
        ghostRef.current.style.opacity = isOverCategory ? "0.35" : "1";
      }
      if (ghostBarRef.current) {
        const validity = getInvalidMoveTarget(eventsRef.current, drag, c.rangeStart);
        applyGhostDropValidity(ghostBarRef.current,
          validity.invalid,
          ghostOrigBgRef.current);
      }
      {
        const hlType: "room" | "group" | null =
          visualRow?.type === "room" ? "room"
          : (isOverCategory && !(expandedGroupsRef.current[(visualRow as { groupId: string }).groupId] ?? true)) ? "group"
          : null;
        if (roomCellHighlightRef?.current) {
          if (hlType === "room") applyRoomHighlight(roomCellHighlightRef.current, "room", (visualRow as { rowStart: number }).rowStart, ROW_HEIGHT);
          else if (hlType === "group") applyRoomHighlight(roomCellHighlightRef.current, "group", (visualRow as { rowStart: number }).rowStart, TYPE_ROW_HEIGHT);
          else roomCellHighlightRef.current.style.display = "none";
        }
        if (dateColHighlightRef?.current) {
          if (hlType !== null) applyDateHighlight(dateColHighlightRef.current, visualOff, c.cellWidth, hlType);
          else dateColHighlightRef.current.style.display = "none";
        }
      }

      // Auto-expand: if pointer is over a collapsed group row for >3s, expand it
      const groupRow = findGroupRow(absY, vItems, c.virtualItems);
      if (groupRow && !(expandedGroupsRef.current[groupRow.groupId] ?? true)) {
        if (groupAutoExpandTargetRef.current !== groupRow.groupId) {
          if (groupAutoExpandTimerRef.current !== null) clearTimeout(groupAutoExpandTimerRef.current);
          groupAutoExpandTargetRef.current = groupRow.groupId;
          groupAutoExpandTimerRef.current = setTimeout(() => {
            onExpandGroupRef.current?.(groupRow.groupId);
            groupAutoExpandTimerRef.current = null;
            groupAutoExpandTargetRef.current = null;
          }, 3000);
        }
      } else {
        if (groupAutoExpandTimerRef.current !== null) {
          clearTimeout(groupAutoExpandTimerRef.current);
          groupAutoExpandTimerRef.current = null;
          groupAutoExpandTargetRef.current = null;
        }
      }
    }

    function handleUpOrCancel(ev: PointerEvent) {
      const drag = moveDragRef.current;
      if (!drag || ev.pointerId !== drag.pointerId) return;
      const isCancel = ev.type === "pointercancel";
      const c = dragContextRef.current;
      const invalidTarget = isCancel
        ? { invalid: false, reason: null as "past" | "conflict" | null }
        : getInvalidMoveTarget(eventsRef.current, drag, c.rangeStart);

      cleanup({
        hideTargetIndicators: isCancel || invalidTarget.invalid,
        hideSourceIndicators: isCancel || invalidTarget.invalid,
      });

      if (invalidTarget.invalid) {
        toast.error(
          invalidTarget.reason === "past"
            ? "Cannot move reservation before today."
            : "Cannot move reservation onto another reservation or blocked room.",
        );
        return;
      }

      if (!isCancel && drag.currentTargetRoomId) {
        const newStartDate = addDays(new Date(c.rangeStart), drag.currentSnapDayOffset);
        const newStart = newStartDate.toISOString().slice(0, 10);
        const newEnd   = addDays(newStartDate, drag.durationDays).toISOString().slice(0, 10);
        onEventMovedRef.current?.(drag.event, drag.currentTargetRoomId, newStart, newEnd);
      }
    }

    function handleKeyDown(ev: KeyboardEvent) {
      if (ev.key === "Escape") cleanup({ hideSourceIndicators: true });
    }

    function handleScroll() {
      // Refresh cached rect when user scrolls during drag
      if (moveDragRef.current && dateGridRef.current) {
        moveDragRef.current.dateGridRect = dateGridRef.current.getBoundingClientRect();
      }
    }

    function cleanup({ hideTargetIndicators = true, hideSourceIndicators = true } = {}) {
      document.removeEventListener("pointermove", handleMovePointer);
      document.removeEventListener("pointerup", handleUpOrCancel);
      document.removeEventListener("pointercancel", handleUpOrCancel);
      document.removeEventListener("keydown", handleKeyDown);
      scrollElementRef.current?.removeEventListener("scroll", handleScroll);

      if (edgeScrollFrameRef.current !== null) {
        cancelAnimationFrame(edgeScrollFrameRef.current);
        edgeScrollFrameRef.current = null;
      }

      if (groupAutoExpandTimerRef.current !== null) {
        clearTimeout(groupAutoExpandTimerRef.current);
        groupAutoExpandTimerRef.current = null;
        groupAutoExpandTargetRef.current = null;
      }

      if (ghostRef.current) { ghostRef.current.style.display = "none"; ghostRef.current.style.opacity = "1"; }
      if (hideTargetIndicators) hideDragHighlights(roomCellHighlightRef, dateColHighlightRef);
      if (hideSourceIndicators) hideGhostHighlights(roomCellGhostHighlightRef, dateColGhostHighlightRef);
      if (scrollElementRef.current) scrollElementRef.current.style.touchAction = "";

      moveDragRef.current = null;
      stopDragRef.current = null;
      setDraggingEventId(null);
    }

    stopDragRef.current = cleanup;

    // ── Edge-scroll loop ──────────────────────────────────────────────────
    const EDGE_ZONE  = 80;  // px from a scroll-container edge triggers scroll
    const MAX_SPEED  = 10;  // px per animation frame at the edge

    function edgeDelta(distanceToStartEdge: number, distanceToEndEdge: number): number {
      if (distanceToEndEdge < EDGE_ZONE) {
        const t = Math.max(0, distanceToEndEdge);
        return MAX_SPEED * (1 - t / EDGE_ZONE);
      }
      if (distanceToStartEdge < EDGE_ZONE) {
        const t = Math.max(0, distanceToStartEdge);
        return -MAX_SPEED * (1 - t / EDGE_ZONE);
      }
      return 0;
    }

    function edgeScrollTick() {
      const drag = moveDragRef.current;
      const scrollEl = scrollElementRef.current;
      if (!drag || !scrollEl) { edgeScrollFrameRef.current = null; return; }

      const scrollRect = scrollEl.getBoundingClientRect();
      const visibleDateGridLeft = scrollRect.left + leftColumnWidth;
      const deltaX = edgeDelta(
        drag.lastClientX - visibleDateGridLeft,
        scrollRect.right - drag.lastClientX,
      );
      const deltaY = edgeDelta(
        drag.lastClientY - scrollRect.top,
        scrollRect.bottom - drag.lastClientY,
      );

      if (deltaX !== 0 || deltaY !== 0) {
        const startScrollLeft = scrollEl.scrollLeft;
        const startScrollTop = scrollEl.scrollTop;
        const maxScrollLeft = Math.max(0, scrollEl.scrollWidth - scrollEl.clientWidth);
        const maxScrollTop = Math.max(0, scrollEl.scrollHeight - scrollEl.clientHeight);
        scrollEl.scrollLeft = Math.max(0, Math.min(maxScrollLeft, startScrollLeft + deltaX));
        scrollEl.scrollTop = Math.max(0, Math.min(maxScrollTop, startScrollTop + deltaY));

        if (scrollEl.scrollLeft === startScrollLeft && scrollEl.scrollTop === startScrollTop) {
          edgeScrollFrameRef.current = requestAnimationFrame(edgeScrollTick);
          return;
        }

        // Update snap offset to reflect new scroll position
        const c = dragContextRef.current;
        const barStartX = drag.lastClientX - drag.originOffsetX;
        if (dateGridRef.current) {
          drag.dateGridRect = dateGridRef.current.getBoundingClientRect();
        }
        const rect = drag.dateGridRect;
        const rawOff = Math.round((barStartX - rect.left - BAR_START_FRAC * c.cellWidth) / c.cellWidth);
        drag.currentSnapDayOffset = Math.max(drag.todayOff, rawOff);
        drag.currentRawDayOffset = rawOff;

        // Re-position ghost after scroll
        const vItems = rowVirtualizerRef.current.getVirtualItems();
        const absY = drag.lastClientY - rect.top;
        const target = findTargetRow(absY, vItems, c.virtualItems);
        if (target) drag.currentTargetRoomId = target.roomId;
        const visualRow = findVisualRow(absY, vItems, c.virtualItems);
        const ghostRowStart = visualRow?.rowStart ?? target?.rowStart ?? 0;
        const visualOff = rawOff < drag.todayOff ? rawOff : drag.currentSnapDayOffset;
        const visibleDateGridLeft = scrollRect.left + leftColumnWidth;
        const fixedLeft = Math.max(
          visibleDateGridLeft,
          rect.left + visualOff * c.cellWidth + BAR_START_FRAC * c.cellWidth,
        );
        const fixedTop  = rect.top + ghostRowStart + BAR_TOP;
        const isOverCategoryEdge = visualRow?.type === "group";
        if (ghostRef.current) {
          ghostRef.current.style.transform = `translate3d(${fixedLeft}px, ${fixedTop}px, 0)`;
          ghostRef.current.style.opacity = isOverCategoryEdge ? "0.35" : "1";
        }
        if (ghostBarRef.current) {
          const validity = getInvalidMoveTarget(eventsRef.current, drag, c.rangeStart);
          applyGhostDropValidity(ghostBarRef.current,
            validity.invalid,
            ghostOrigBgRef.current);
        }
        {
          const hlType: "room" | "group" | null =
            visualRow?.type === "room" ? "room"
            : (isOverCategoryEdge && !(expandedGroupsRef.current[(visualRow as { groupId: string }).groupId] ?? true)) ? "group"
            : null;
          if (roomCellHighlightRef?.current) {
            if (hlType === "room") applyRoomHighlight(roomCellHighlightRef.current, "room", (visualRow as { rowStart: number }).rowStart, ROW_HEIGHT);
            else if (hlType === "group") applyRoomHighlight(roomCellHighlightRef.current, "group", (visualRow as { rowStart: number }).rowStart, TYPE_ROW_HEIGHT);
            else roomCellHighlightRef.current.style.display = "none";
          }
          if (dateColHighlightRef?.current) {
            if (hlType !== null) applyDateHighlight(dateColHighlightRef.current, visualOff, c.cellWidth, hlType);
            else dateColHighlightRef.current.style.display = "none";
          }
        }
      }

      edgeScrollFrameRef.current = requestAnimationFrame(edgeScrollTick);
    }
    edgeScrollFrameRef.current = requestAnimationFrame(edgeScrollTick);

    document.addEventListener("pointermove", handleMovePointer);
    document.addEventListener("pointerup", handleUpOrCancel);
    document.addEventListener("pointercancel", handleUpOrCancel);
    document.addEventListener("keydown", handleKeyDown);
    scrollElementRef.current.addEventListener("scroll", handleScroll, { passive: true });
  }, [
    todayStr,
    scrollElementRef,
    leftColumnWidth,
    roomCellHighlightRef,
    dateColHighlightRef,
    roomCellGhostHighlightRef,
    dateColGhostHighlightRef,
  ]);

  const totalHeight = rowVirtualizer.getTotalSize();

  return (
    <div
      ref={dateGridRef}
      style={{
        position: "relative",
        width: totalDateWidth,
        height: totalHeight,
        flexShrink: 0,
      }}
    >
      {/* ── Column separator lines ── */}
      <div
        aria-hidden="true"
        className="absolute inset-0 pointer-events-none"
        style={{
          backgroundImage:
            `repeating-linear-gradient(to right, transparent 0, transparent calc(100% - 1px), ${CALENDAR_COLORS.gridLineSoft} calc(100% - 1px), ${CALENDAR_COLORS.gridLineSoft} 100%)`,
          backgroundSize: `${cellWidth}px 100%`,
          backgroundRepeat: "repeat-x",
          zIndex: 0,
        }}
      />

      {/* ── Column tone overlays (today + weekend) ── */}
      {virtualCols.map((vCol) => {
        const dateStr = colIndexToDate(vCol.index);
        const date = parseLocalDate(dateStr);
        const todayCol = isToday(date);
        const weekendCol = !todayCol && isWeekend(date);
        if (!todayCol && !weekendCol) return null;
        return (
          <div
            key={`col-tone-${vCol.index}`}
            className={`absolute pointer-events-none ${
              todayCol
                ? CALENDAR_THEME.dateGridColTone.today
                : CALENDAR_THEME.dateGridColTone.weekend
            }`}
            style={{
              left: vCol.start,
              width: vCol.size,
              top: 0,
              height: totalHeight,
              zIndex: 0,
            }}
          />
        );
      })}

      {/* ── Virtual rows ── */}
      {rowVirtualizer.getVirtualItems().map((vRow) => {
        const item = virtualItems[vRow.index];
        if (!item) return null;

        const rowStyle: React.CSSProperties = {
          position: "absolute",
          top: vRow.start,
          height: vRow.size,
          width: totalDateWidth,
          zIndex: 1,
        };

        // ── Group header row ──────────────────────────────────────────────
        if (item.type === "group") {
          const group = groupById[item.roomTypeId];
          if (!group) return null;
          const occupancyForGroup = occupancyByDateByGroup[group.id] ?? {};
          const roomIds = roomIdsByGroup[group.id] ?? EMPTY_ROOM_IDS;
          const roomCount = roomIds.length;
          const showRates = filters.showRates;
          const showOccupancy = filters.showOccupancy;

          return (
            <div
              key={`group-${item.roomTypeId}`}
              className={`relative ${CALENDAR_THEME.dateGridGroupRow.bg}`}
              style={{ ...rowStyle, contain: "layout paint" }}
            >
              {virtualCols.map((vCol) => {
                const dateStr = colIndexToDate(vCol.index);
                const occupied = occupancyForGroup[dateStr] ?? 0;
                return (
                  <div
                    key={`rate-${vCol.index}`}
                    className={`absolute top-0 bottom-0 flex flex-col items-center justify-center border-r ${CALENDAR_THEME.border}`}
                    style={{ left: vCol.start, width: vCol.size }}
                  >
                    <GroupRateCell
                      roomTypeName={group.title}
                      dateStr={dateStr}
                      showRates={showRates}
                      showOccupancy={showOccupancy}
                      occupied={occupied}
                      roomCount={roomCount}
                      isFetching={isFetchingRates}
                      onRateClick={onRateClick}
                    />
                  </div>
                );
              })}
            </div>
          );
        }

        // ── Individual room row ───────────────────────────────────────────
        const entry = roomById[item.roomId];
        if (!entry) return null;
        const { room, group } = entry;
        const roomEvents = eventsByRoom[room.id] ?? EMPTY_EVENTS;
        const showRoomCellLoaders = filters.showRates && isFetchingRates;

        const activeDrag = drag?.roomId === room.id ? drag : null;
        const selMin = activeDrag ? Math.min(activeDrag.selStartOffset, activeDrag.selEndOffset) : -1;
        const selMax = activeDrag ? Math.max(activeDrag.selStartOffset, activeDrag.selEndOffset) : -2;
        const isDraggingThisRoom = !!activeDrag;

        return (
          <div
            key={`room-${item.roomId}`}
            className="relative select-none"
            style={{ ...rowStyle, touchAction: "pan-x pan-y", contain: "layout paint" }}
            onPointerDown={(e) => handleRoomPointerDown(e, room.id, room.title)}
            onPointerMove={handleRoomPointerMove}
            onPointerUp={(e) => handleRoomPointerUp(e, room.id, room.title)}
          >
            {/* Row border */}
            <div
              aria-hidden="true"
              className={`absolute inset-0 ${CALENDAR_THEME.dateGridRoomRow.border}`}
            />

            {/* Today highlight */}
            {(() => {
              const todayOffset = diffDays(rangeStart, todayStr);
              if (todayOffset < 0) return null;
              return (
                <div
                  className={`absolute top-0 bottom-0 pointer-events-none ${CALENDAR_THEME.dateGridRoomRow.todayHighlight}`}
                  style={{ left: todayOffset * cellWidth, width: cellWidth }}
                />
              );
            })()}

            {/* Drag-to-select highlight */}
            {isDraggingThisRoom && selMax >= selMin && (
              <div
                className={`absolute top-0 bottom-0 pointer-events-none ${CALENDAR_THEME.dateGridRoomRow.dragSelect}`}
                style={{
                  left: selMin * cellWidth,
                  width: (selMax - selMin + 1) * cellWidth,
                }}
              />
            )}

            {/* Cell loading skeletons — each clears itself when its room-type
                rate streams in (subscribes to the per-cell store). */}
            {showRoomCellLoaders && virtualCols.map((vCol) => (
              <RoomRateLoaderCell
                key={`cell-load-${vCol.index}`}
                roomTypeName={group.title}
                dateStr={colIndexToDate(vCol.index)}
                left={vCol.start}
                width={vCol.size}
                isFetching={showRoomCellLoaders}
              />
            ))}

            {/* Event bars */}
            {roomEvents.map((event) => (
              <CalendarEventBar
                key={event.id}
                event={positionEvent(event, rangeStart, cellWidth)}
                cellWidth={cellWidth}
                onEventClick={onEventClick}
                onBlockClick={onBlockClick}
                onEventResized={onEventResized}
                onEventMoveStart={handleEventMoveStart}
                isDraggingAsMove={draggingEventId === event.id}
                isPending={pendingEventIds?.has(event.id) ?? false}
                todayStr={todayStr}
              />
            ))}
          </div>
        );
      })}

      {/* ── Move drag ghost (position: fixed, updated via ref — no React re-renders during drag) ── */}
      <div
        ref={ghostRef}
        aria-hidden="true"
        style={{
          display: "none",
          position: "fixed",
          top: 0,
          left: 0,
          zIndex: 9999,
          pointerEvents: "none",
          willChange: "transform",
        }}
      >
        <div
          ref={ghostBarRef}
          style={{
            position: "absolute",
            top: 0,
            left: 0,
            height: BAR_HEIGHT,
            // width / background / borderRadius set dynamically on drag start
            boxShadow: CALENDAR_COLORS.bar.ghostShadow,
            transform: "scale(1.10)",
            transformOrigin: "center center",
            display: "flex",
            alignItems: "center",
            overflow: "hidden",
          }}
        >
          <span
            ref={ghostLabelRef}
            style={{
              display: "block",
              padding: "0 12px",
              fontSize: 12,
              fontWeight: 600,
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
              lineHeight: `${BAR_HEIGHT}px`,
            }}
          />
        </div>
      </div>
    </div>
  );
});

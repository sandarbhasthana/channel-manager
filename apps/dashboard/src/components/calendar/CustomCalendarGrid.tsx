"use client";

import React, { useMemo, useState, useCallback, useRef, useEffect } from "react";
import { CalendarScrollContainer } from "./grid/CalendarScrollContainer";
import { CalendarDateGrid } from "./grid/CalendarDateGrid";
import { CalendarRoomsColumn, HEADER_ROW_HEIGHT } from "./grid/CalendarRoomsColumn";
import { CalendarHeaderRow } from "./grid/CalendarHeaderRow";
import { CalendarCornerCell } from "./grid/CalendarCornerCell";
import { CalendarRatePriceModal } from "./CalendarRatePriceModal";
import { BookingChangeConfirmationModal } from "./confirmation/BookingChangeConfirmationModal";
import { useGridLayout } from "./layout/useGridLayout";
import { useVirtualRows } from "./layout/useVirtualRows";
import { useDateRange } from "./layout/useDateRange";
import { useVerticalMomentum } from "./layout/useVerticalMomentum";
import { useHorizontalScroll } from "./layout/useHorizontalScroll";
import { useSWRConfig } from "swr";
import {
  useRateUpdates,
  useRatesData,
  useMergedRates,
  rateSwrKeyCoversDate,
} from "@/lib/hooks/useRatesData";
import { isCalendarEditLockedStatus } from "@/lib/reservations/calendar-move-validation";
import { addDays, diffDays } from "@/components/calendar/utils/calendarHelpers";
import type { Room, Reservation, ProcessedEvent, CalendarFilterCriteria } from "@/components/calendar/utils/types";
import type { PositionedEvent } from "./layout/useEventPositioning";
import { calculateCalendarPriceImpact } from "./calendarPriceImpact";
import { applyPendingOptimisticEvents, type PendingOptimistic } from "./calendarOptimistic";

const EXPANDED_STORAGE_KEY = "calendar-room-type-expanded";
// Extra days of rates fetched on each side of the viewport for the *initial*
// window. Scrolling within this buffer reuses already-fetched data.
const RATES_FETCH_BUFFER_DAYS = 28;
// Prefetch the next tile once the viewport scrolls within this many days of an
// edge of the fetched coverage.
const RATES_REFETCH_MARGIN_DAYS = 7;
// When the viewport jumps further than this beyond the fetched coverage (e.g.
// date-picker / "today"), reset to a fresh centered window instead of bridging
// the whole gap with one oversized fetch.
const RATES_GAP_RESET_DAYS = 28;
// How often (ms) the coverage check may run while scrolling. Throttled rather
// than debounced so prefetch follows the date reached *during* a fast scroll
// instead of only firing once the scroll settles — a trailing-only debounce
// lets a fast flick outrun the fetched coverage and exposes blank cells.
const RATES_PREFETCH_THROTTLE_MS = 200;
// Lookahead depth (configurable): while the user keeps scrolling in one
// direction, keep this many fetch windows (RATES_FETCH_BUFFER_DAYS each)
// prefetched *ahead* of the viewport so a sustained fast scroll doesn't outrun
// the fetched coverage. The opposite / idle direction keeps a single-window
// buffer, so a stationary or slow viewport never over-fetches.
const RATES_PREFETCH_LOOKAHEAD_WINDOWS = 3;

interface CustomCalendarGridProps {
  resources: Room[];
  events: ProcessedEvent[];
  reservations: Reservation[];
  selectedDate: string;              // YYYY-MM-DD — anchor of visible window
  visibleDays: number;
  selectedResource: string | null;
  holidays: Record<string, string>;
  filters: CalendarFilterCriteria;
  propertyTimezone?: string;
  /** Event ids to render with shimmer while their server write is pending. */
  pendingEventIds?: string[];

  /** Called when the visible date range changes. */
  onDatesSet?: (start: Date, end: Date) => void;

  /**
   * Ref to receive the imperative `navigate(dir)` function.
   * Parent (page.tsx) wires toolbar prev/next through this.
   */
  navigateRef?: React.RefObject<((dir: -1 | 1) => void) | null>;

  /**
   * Ref to receive the imperative `goToDate(dateStr)` function.
   * Parent uses this for animated jumps (e.g. "today" button).
   */
  goToDateRef?: React.RefObject<((dateStr: string) => void) | null>;

  // Callbacks
  onCellClick?: (roomId: string, roomName: string, date: string, x: number, y: number) => void;
  onEventClick?: (event: PositionedEvent, x: number, y: number) => void;
  onBlockClick?: (event: PositionedEvent, x: number, y: number) => void;
  onEventDrop?: (event: PositionedEvent, newRoomId: string, newStart: string, newEnd: string, priceMode?: "keep_current" | "use_new") => Promise<void> | void;
  onEventResize?: (event: PositionedEvent, newStart: string, newEnd: string, priceMode?: "keep_current" | "use_new") => Promise<void> | void;
  onRangeSelect?: (roomId: string, startDate: string, endDate: string, x: number, y: number) => void;
  setSelectedResource: (id: string) => void;
  cellClassName?: (roomId: string, date: string) => string | undefined;
}

interface PendingChange {
  type: "move" | "resize";
  event: PositionedEvent;
  oldStart: string;
  oldEnd: string;
  newStart: string;
  newEnd: string;
  /** Target room for move operations (may differ from event.roomId). */
  newRoomId?: string;
  newRoomName?: string;
  oldRoomName?: string;
  oldRoomCategoryId?: string;
  oldRoomCategory?: string;
  newRoomCategoryId?: string;
  newRoomCategory?: string;
  priceChange?: {
    oldTotal: number;
    newTotal: number;
  } | null;
}

interface RateModalState {
  open: boolean;
  roomTypeId: string;
  roomTypeName: string;
  dateStr: string;
  currentPrice: number;
}

export default function CustomCalendarGrid({
  resources,
  events,
  reservations,
  selectedDate,
  visibleDays,
  selectedResource,
  holidays,
  filters,
  propertyTimezone = "UTC",
  pendingEventIds = [],
  onDatesSet,
  navigateRef,
  goToDateRef,
  onCellClick,
  onEventClick,
  onBlockClick,
  onEventResize,
  onEventDrop,
  onRangeSelect,
  setSelectedResource,
}: CustomCalendarGridProps) {
  // Confirmation modal for resize/move
  const [pendingChange, setPendingChange] = useState<PendingChange | null>(null);

  // Optimistic overlay — applied immediately after the user confirms, cleared when
  // the parent's events prop reflects the new state (API round-trip complete).
  const [pendingDrop, setPendingDrop] = useState<PendingOptimistic | null>(null);

  // Merge optimistic override into events before passing to the grid
  const eventsForGrid = useMemo<ProcessedEvent[]>(() => {
    return applyPendingOptimisticEvents(events, pendingDrop);
  }, [events, pendingDrop]);

  const pendingEventIdSet = useMemo(() => {
    const ids = new Set(pendingEventIds);
    if (pendingDrop?.eventId) ids.add(pendingDrop.eventId);
    return ids;
  }, [pendingEventIds, pendingDrop?.eventId]);

  // ── Room-info lookup helper ─────────────────────────────────────────────────
  const findRoomInfo = useCallback(
    (roomId: string): { roomName?: string; categoryId?: string; categoryName?: string } => {
      for (const group of resources) {
        for (const room of group.children ?? []) {
          if (room.id === roomId) return { roomName: room.title, categoryId: group.id, categoryName: group.title };
        }
      }
      return {};
    },
    [resources],
  );

  // ── Drag highlight refs ─────────────────────────────────────────────────────
  const roomCellHighlightRef = useRef<HTMLDivElement>(null);
  const dateColHighlightRef = useRef<HTMLDivElement>(null);
  const roomCellGhostHighlightRef = useRef<HTMLDivElement>(null);
  const dateColGhostHighlightRef = useRef<HTMLDivElement>(null);

  const hideDragAxisIndicators = useCallback(() => {
    if (roomCellHighlightRef.current) roomCellHighlightRef.current.style.display = "none";
    if (dateColHighlightRef.current) dateColHighlightRef.current.style.display = "none";
    if (roomCellGhostHighlightRef.current) roomCellGhostHighlightRef.current.style.display = "none";
    if (dateColGhostHighlightRef.current) dateColGhostHighlightRef.current.style.display = "none";
  }, []);

  const cancelChange = useCallback(() => {
    setPendingChange(null);
    setPendingDrop(null);  // revert optimistic position
    hideDragAxisIndicators();
  }, [hideDragAxisIndicators]);

  // ── Group expand callback ───────────────────────────────────────────────────
  const handleExpandGroup = useCallback((groupId: string) => {
    setExpandedGroups((prev) => {
      if (prev[groupId] ?? true) return prev; // already expanded
      return { ...prev, [groupId]: true };
    });
  }, []);

  // ── Grid layout ────────────────────────────────────────────────────────────
  const { cellWidth, leftColumnWidth, setLeftColumnWidth, scrollContainerRef } = useGridLayout(visibleDays);

  const [isMobileViewport, setIsMobileViewport] = useState(false);
  const [isRoomsColumnSelected, setIsRoomsColumnSelected] = useState(false);

  // ── Rooms-column drag-resize ───────────────────────────────────────────────
  const isDraggingRef = useRef(false);
  const dragStartXRef = useRef(0);
  const dragStartWidthRef = useRef(0);
  const dragPointerIdRef = useRef<number | null>(null);

  useEffect(() => {
    if (typeof window === "undefined") return;

    const syncViewport = () => {
      const mobile = window.innerWidth < 640;
      setIsMobileViewport(mobile);
      if (!mobile) setIsRoomsColumnSelected(false);
    };

    syncViewport();
    window.addEventListener("resize", syncViewport);
    return () => window.removeEventListener("resize", syncViewport);
  }, []);

  const handleDragHandlePointerDown = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
    e.preventDefault();
    e.stopPropagation();

    isDraggingRef.current = true;
    dragPointerIdRef.current = e.pointerId;
    dragStartXRef.current = e.clientX;
    dragStartWidthRef.current = leftColumnWidth;
    e.currentTarget.setPointerCapture(e.pointerId);

    const onPointerMove = (ev: PointerEvent) => {
      if (!isDraggingRef.current || dragPointerIdRef.current !== ev.pointerId) return;
      const delta = ev.clientX - dragStartXRef.current;
      setLeftColumnWidth(dragStartWidthRef.current + delta);
    };

    const stopDragging = () => {
      isDraggingRef.current = false;
      dragPointerIdRef.current = null;
      document.removeEventListener("pointermove", onPointerMove);
      document.removeEventListener("pointerup", onPointerUp);
      document.removeEventListener("pointercancel", onPointerUp);
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    };

    const onPointerUp = (ev: PointerEvent) => {
      if (dragPointerIdRef.current !== ev.pointerId) return;
      stopDragging();
    };

    document.addEventListener("pointermove", onPointerMove);
    document.addEventListener("pointerup", onPointerUp);
    document.addEventListener("pointercancel", onPointerUp);
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
  }, [leftColumnWidth, setLeftColumnWidth]);

  useEffect(() => {
    return () => {
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    };
  }, []);

  const handleRoomsHeaderClick = useCallback(() => {
    if (!isMobileViewport) return;
    setIsRoomsColumnSelected((prev) => !prev);
  }, [isMobileViewport]);

  const handleSelectResource = useCallback((id: string) => {
    setSelectedResource(id);
    setIsRoomsColumnSelected(false);
  }, [setSelectedResource]);

  const showResizeHandle = !isMobileViewport || isRoomsColumnSelected;

  // ── Expand/collapse state ──────────────────────────────────────────────────
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>(() => {
    if (typeof window === "undefined") return {};
    try {
      const stored = localStorage.getItem(EXPANDED_STORAGE_KEY);
      return stored ? JSON.parse(stored) : {};
    } catch {
      return {};
    }
  });

  useEffect(() => {
    try {
      localStorage.setItem(EXPANDED_STORAGE_KEY, JSON.stringify(expandedGroups));
    } catch {
      // localStorage not available
    }
  }, [expandedGroups]);

  const handleToggleGroup = useCallback((groupId: string) => {
    setExpandedGroups((prev) => ({ ...prev, [groupId]: !(prev[groupId] ?? true) }));
  }, []);

  // ── Filtered resources ─────────────────────────────────────────────────────
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

  const visibleGroupIds = useMemo(() => filteredResources.map((group) => group.id), [filteredResources]);
  const areAllVisibleGroupsExpanded = useMemo(
    () => visibleGroupIds.length > 0 && visibleGroupIds.every((groupId) => expandedGroups[groupId] ?? true),
    [visibleGroupIds, expandedGroups],
  );

  const handleToggleAllGroups = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    setExpandedGroups((prev) => {
      const nextValue = !visibleGroupIds.every((groupId) => prev[groupId] ?? true);
      const next = { ...prev };
      for (const groupId of visibleGroupIds) {
        next[groupId] = nextValue;
      }
      return next;
    });
  }, [visibleGroupIds]);

  // ── Virtual row list ───────────────────────────────────────────────────────
  const virtualItems = useVirtualRows(filteredResources, expandedGroups);

  // ── Horizontal virtualizer + date range ───────────────────────────────────
  const {
    rangeStart,
    totalDateWidth,
    colVirtualizer,
    visibleStartDate,
    colIndexToDate,
  } = useDateRange({
    selectedDate,
    cellWidth,
    scrollElementRef: scrollContainerRef,
    visibleDays,
    onDatesSet,
    navigateRef,
    goToDateRef,
  });

  // ── Scroll momentum hooks ──────────────────────────────────────────────────
  useVerticalMomentum(scrollContainerRef);
  useHorizontalScroll({ scrollElementRef: scrollContainerRef, cellWidth, disabled: !!pendingChange });

  const virtualCols = colVirtualizer.getVirtualItems();

  // Derive the actual visible start/end indices from the DOM scroll position and virtual item
  // positions rather than colVirtualizer.range, which includes overscan and can be stale during
  // batched scroll updates.
  const scrollLeft = scrollContainerRef.current?.scrollLeft ?? 0;
  const viewportWidth = Math.max(0, (scrollContainerRef.current?.clientWidth ?? 0) - leftColumnWidth);
  const visibleStartIndex = virtualCols.find((vc) => vc.start + vc.size > scrollLeft)?.index ?? -1;
  const visibleEndIndex =
    virtualCols.slice().reverse().find((vc) => vc.start < scrollLeft + viewportWidth)?.index ?? -1;
  const actualVisibleStartDate = visibleStartIndex >= 0 ? colIndexToDate(visibleStartIndex) : visibleStartDate;

  // ── Rate modal ─────────────────────────────────────────────────────────────
  const [rateModal, setRateModal] = useState<RateModalState>({
    open: false,
    roomTypeId: "",
    roomTypeName: "",
    dateStr: "",
    currentPrice: 0,
  });
  const { bulkUpdateRates, isUpdating } = useRateUpdates();
  const { mutate: globalMutate } = useSWRConfig();

  // ── Rates fetch coverage (non-overlapping tiles) ───────────────────────────
  // The grid paints every cell straight from the per-cell SSE store (useRateCell),
  // so the fetch window does NOT need to stay centered on the viewport. We track
  // the contiguous [coverStart, coverEnd) range already fetched and, when the
  // viewport nears an edge, prefetch a fresh tile that begins exactly where
  // coverage ends — so each date is requested at most once (no overlapping
  // re-fetches). A jump far outside coverage resets to a fresh centered window.
  const initialFetch = useMemo(
    () => {
      const start = addDays(new Date(actualVisibleStartDate), -RATES_FETCH_BUFFER_DAYS)
        .toISOString()
        .slice(0, 10);
      return { start, days: visibleDays + RATES_FETCH_BUFFER_DAYS * 2 };
    },
    // Mount-time anchor only; coverage advances via the scroll effect below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );

  const coverStartRef = useRef(initialFetch.start);
  const coverEndRef = useRef(
    addDays(new Date(initialFetch.start), initialFetch.days).toISOString().slice(0, 10),
  );

  // The tile currently handed to useRatesData (streams into the shared cell store).
  const [fetchChunk, setFetchChunk] = useState<{
    start: string;
    days: number;
    direction: "forward" | "backward";
  }>(() => ({ start: initialFetch.start, days: initialFetch.days, direction: "forward" }));

  // Latest viewport values fed to the throttled coverage check (read via refs so
  // a trailing throttle fire always uses the most recent scroll position, not the
  // value captured when the timer was scheduled).
  const visibleDaysRef = useRef(visibleDays);
  visibleDaysRef.current = visibleDays;
  const visibleStartDateRef = useRef(actualVisibleStartDate);
  visibleStartDateRef.current = actualVisibleStartDate;
  // Previous sampled viewport start — lets each coverage check tell which way the
  // user is scrolling, so deep lookahead only extends *ahead* of travel.
  const lastSampledStartRef = useRef(actualVisibleStartDate);

  // Extend the fetched coverage based on the date reached so far, keeping a deep
  // lookahead (RATES_PREFETCH_LOOKAHEAD_WINDOWS tiles) ahead of the scroll
  // direction and a single-window buffer on the idle side. Idempotent: a tile
  // only fires once the remaining buffer drops below its direction's threshold,
  // so repeated calls while scrolling self-limit (coverStart/coverEndRef advance
  // synchronously). Multiple tiles may stream concurrently — each fetcher writes
  // into the shared per-cell store independently of the current SWR key.
  const extendRatesCoverage = useCallback(() => {
    const W = RATES_FETCH_BUFFER_DAYS;
    const vStart = visibleStartDateRef.current;
    const visible = visibleDaysRef.current;
    const coverStart = coverStartRef.current;
    const coverEnd = coverEndRef.current;
    const coverDays = diffDays(coverStart, coverEnd);
    const startOffset = diffDays(coverStart, vStart);
    const endOffset = startOffset + visible;

    // Scroll direction since the previous check (one throttle tick ago).
    const movedDays = diffDays(lastSampledStartRef.current, vStart);
    lastSampledStartRef.current = vStart;

    // Big jump well outside coverage (date-picker / "today") — reset to a fresh
    // window centered on the new viewport rather than bridging the whole gap.
    if (
      startOffset < -RATES_GAP_RESET_DAYS ||
      startOffset > coverDays + RATES_GAP_RESET_DAYS
    ) {
      const freshStart = addDays(new Date(vStart), -W).toISOString().slice(0, 10);
      const freshDays = visible + W * 2;
      coverStartRef.current = freshStart;
      coverEndRef.current = addDays(new Date(freshStart), freshDays).toISOString().slice(0, 10);
      setFetchChunk({ start: freshStart, days: freshDays, direction: "forward" });
      return;
    }

    // Deep lookahead in the direction of travel; single-window buffer otherwise.
    const forwardWindows = movedDays > 0 ? RATES_PREFETCH_LOOKAHEAD_WINDOWS : 1;
    const backwardWindows = movedDays < 0 ? RATES_PREFETCH_LOOKAHEAD_WINDOWS : 1;
    // Refill once the remaining buffer in a direction falls below all-but-one of
    // its target windows (plus the edge margin) — keeps ≥ (windows-1) windows of
    // runway while topping back up to the full depth in small tiles.
    const forwardThreshold = (forwardWindows - 1) * W + RATES_REFETCH_MARGIN_DAYS;
    const backwardThreshold = (backwardWindows - 1) * W + RATES_REFETCH_MARGIN_DAYS;

    const remainingForward = coverDays - endOffset;  // fetched days past the viewport end
    const remainingBackward = startOffset;           // fetched days before the viewport start

    // Extend coverage to a buffer past the viewport END in a single tile that
    // begins exactly at coverEnd (no overlap). Streams earliest-first (toward
    // the viewport).
    const tryForward = () => {
      if (remainingForward >= forwardThreshold) return false;
      const newCoverEnd = addDays(new Date(vStart), visible + forwardWindows * W)
        .toISOString()
        .slice(0, 10);
      const tileDays = diffDays(coverEnd, newCoverEnd);
      if (tileDays <= 0) return false;
      coverEndRef.current = newCoverEnd;
      setFetchChunk({ start: coverEnd, days: tileDays, direction: "forward" });
      return true;
    };

    // Extend coverage to a buffer before the viewport START in a single tile that
    // ends exactly at coverStart (no overlap). Streams latest-first (toward the
    // viewport).
    const tryBackward = () => {
      if (remainingBackward >= backwardThreshold) return false;
      const newCoverStart = addDays(new Date(vStart), -backwardWindows * W)
        .toISOString()
        .slice(0, 10);
      const tileDays = diffDays(newCoverStart, coverStart);
      if (tileDays <= 0) return false;
      coverStartRef.current = newCoverStart;
      setFetchChunk({ start: newCoverStart, days: tileDays, direction: "backward" });
      return true;
    };

    // Prefer the direction of travel; fall back to the other edge when idle.
    if (movedDays < 0) {
      if (!tryBackward()) tryForward();
    } else {
      if (!tryForward()) tryBackward();
    }
  }, []);

  // Leading + trailing throttle keyed off the date reached so far. A trailing-only
  // debounce never fired mid-flick — its timer was cleared on every scroll re-render
  // — so a fast scroll outran the fetched coverage and showed blank cells. Throttling
  // extends coverage *during* the scroll while capping how often the check runs.
  const lastPrefetchRunRef = useRef(0);
  const prefetchTimerRef = useRef<number | null>(null);

  useEffect(() => {
    const sinceLast = Date.now() - lastPrefetchRunRef.current;
    if (sinceLast >= RATES_PREFETCH_THROTTLE_MS) {
      // Leading edge — run immediately.
      lastPrefetchRunRef.current = Date.now();
      extendRatesCoverage();
    } else if (prefetchTimerRef.current === null) {
      // Within the throttle window — schedule a single trailing run (don't reset
      // an already-pending one, so continuous scrolling still fires every window).
      prefetchTimerRef.current = window.setTimeout(() => {
        prefetchTimerRef.current = null;
        lastPrefetchRunRef.current = Date.now();
        extendRatesCoverage();
      }, RATES_PREFETCH_THROTTLE_MS - sinceLast);
    }
  }, [actualVisibleStartDate, visibleDays, extendRatesCoverage]);

  // Clear any pending throttle timer on unmount.
  useEffect(
    () => () => {
      if (prefetchTimerRef.current !== null) window.clearTimeout(prefetchTimerRef.current);
    },
    [],
  );

  const fetchChunkStartObj = useMemo(() => new Date(fetchChunk.start), [fetchChunk.start]);
  const {
    isLoading: ratesIsLoading,
    isValidating: ratesIsValidating,
  } = useRatesData(fetchChunkStartObj, fetchChunk.days, "base", true, fetchChunk.direction);
  const visibleRateEndExclusive = useMemo(
    () => addDays(new Date(actualVisibleStartDate), visibleDays).toISOString().slice(0, 10),
    [actualVisibleStartDate, visibleDays],
  );
  const isVisibleRateWindowOutsideCoverage =
    actualVisibleStartDate < coverStartRef.current ||
    visibleRateEndExclusive > coverEndRef.current;

  // Viewport-centered merged view (decoupled from the fetch tile) for the rate
  // modal and drag price-impact, which need rates for the *visible* dates.
  const mergeRangeStart = useMemo(
    () => addDays(new Date(actualVisibleStartDate), -RATES_REFETCH_MARGIN_DAYS),
    [actualVisibleStartDate],
  );
  const currentRatesData = useMergedRates(
    mergeRangeStart,
    visibleDays + RATES_REFETCH_MARGIN_DAYS * 2,
    "base",
    true,
  );

  const calculateChangePrice = useCallback(
    (
      event: PositionedEvent,
      roomCategoryId: string | undefined,
      roomCategoryName: string | undefined,
      newStart: string,
      newEnd: string,
    ) => {
      if (event.rateLocked) return null;
      return calculateCalendarPriceImpact({
        depositAmount: event.depositAmount,
        ratesData: currentRatesData,
        roomCategoryId,
        roomCategoryName,
        newStart,
        newEnd,
      });
    },
    [currentRatesData],
  );

  const handleEventResized = useCallback(
    (event: PositionedEvent, newStart: string, newEnd: string) => {
      if (isCalendarEditLockedStatus(event.status)) return;
      if (newStart === event.start && newEnd === event.end) return;
      // Show bar at new size immediately before user confirms
      setPendingDrop({ eventId: event.id, newRoomId: event.roomId, newStart, newEnd });
      const roomInfo = findRoomInfo(event.roomId);
      setPendingChange({
        type: "resize", event, oldStart: event.start, oldEnd: event.end, newStart, newEnd,
        oldRoomName: roomInfo.roomName, oldRoomCategory: roomInfo.categoryName,
        newRoomName: roomInfo.roomName, newRoomCategory: roomInfo.categoryName,
        oldRoomCategoryId: roomInfo.categoryId,
        newRoomCategoryId: roomInfo.categoryId,
        priceChange: calculateChangePrice(event, roomInfo.categoryId, roomInfo.categoryName, newStart, newEnd),
      });
    },
    [calculateChangePrice, findRoomInfo],
  );

  const handleEventMoved = useCallback(
    (event: PositionedEvent, newRoomId: string, newStart: string, newEnd: string) => {
      if (isCalendarEditLockedStatus(event.status)) {
        hideDragAxisIndicators();
        return;
      }
      if (newRoomId === event.roomId && newStart === event.start && newEnd === event.end) {
        hideDragAxisIndicators();
        return;
      }
      // Show bar at new position immediately before user confirms
      setPendingDrop({ eventId: event.id, newRoomId, newStart, newEnd });
      const oldInfo = findRoomInfo(event.roomId);
      const newInfo = findRoomInfo(newRoomId);
      setPendingChange({
        type: "move", event, oldStart: event.start, oldEnd: event.end, newStart, newEnd,
        newRoomId, newRoomName: newInfo.roomName,
        oldRoomName: oldInfo.roomName, oldRoomCategory: oldInfo.categoryName,
        newRoomCategory: newInfo.categoryName,
        oldRoomCategoryId: oldInfo.categoryId,
        newRoomCategoryId: newInfo.categoryId,
        priceChange: calculateChangePrice(event, newInfo.categoryId, newInfo.categoryName, newStart, newEnd),
      });
    },
    [calculateChangePrice, findRoomInfo, hideDragAxisIndicators],
  );

  const confirmChange = useCallback(async (edited: { newStart: string; newEnd: string; priceMode: "keep_current" | "use_new" }) => {
    if (!pendingChange) return;
    const change = pendingChange;
    setPendingChange(null);
    hideDragAxisIndicators();
    // pendingDrop is already set from handleEventMoved/handleEventResized; keep it alive
    // until the API call completes, then clear.
    try {
      if (change.type === "resize") {
        await onEventResize?.(change.event, edited.newStart, edited.newEnd, edited.priceMode);
      } else {
        await onEventDrop?.(
          change.event,
          change.newRoomId ?? change.event.roomId,
          edited.newStart,
          edited.newEnd,
          edited.priceMode,
        );
      }
    } finally {
      setPendingDrop(null);
    }
  }, [pendingChange, onEventResize, onEventDrop, hideDragAxisIndicators]);

  const handleRateClick = useCallback(
    (roomTypeId: string, roomTypeName: string, dateStr: string, currentPrice: number) => {
      setRateModal({ open: true, roomTypeId, roomTypeName, dateStr, currentPrice });
    },
    [],
  );

  const handleRateModalClose = useCallback(() => {
    setRateModal((prev) => ({ ...prev, open: false }));
  }, []);

  const handleRateSave = useCallback(
    async (updates: Parameters<typeof bulkUpdateRates>[0]) => {
      await bulkUpdateRates(updates);
      // With tiled fetches the edited date may live in a tile other than the one
      // currently fetching, so revalidate every cached tile that covers it — the
      // re-stream repaints the affected cells via the per-cell store.
      const editedDates = updates.map((update) => update.date);
      await globalMutate(
        (key) =>
          typeof key === "string" &&
          editedDates.some((date) => rateSwrKeyCoversDate(key, date)),
      );
      handleRateModalClose();
    },
    [bulkUpdateRates, globalMutate, handleRateModalClose],
  );

  return (
    <CalendarScrollContainer scrollContainerRef={scrollContainerRef}>
      {/* ── Sticky header row: corner cell (sticky-left) + scrolling date header ── */}
      <div
        style={{
          position: "sticky",
          top: 0,
          zIndex: 20,
          display: "flex",
          height: HEADER_ROW_HEIGHT,
          minHeight: HEADER_ROW_HEIGHT,
          minWidth: leftColumnWidth + totalDateWidth,
          willChange: "transform",
        }}
      >
        <CalendarCornerCell
          leftColumnWidth={leftColumnWidth}
          isMobileViewport={isMobileViewport}
          isRoomsColumnSelected={isRoomsColumnSelected}
          hasGroups={visibleGroupIds.length > 0}
          areAllExpanded={areAllVisibleGroupsExpanded}
          showResizeHandle={showResizeHandle}
          onHeaderClick={handleRoomsHeaderClick}
          onToggleAll={handleToggleAllGroups}
          onResizePointerDown={handleDragHandlePointerDown}
        />

        <CalendarHeaderRow
          virtualCols={virtualCols}
          colIndexToDate={colIndexToDate}
          totalDateWidth={totalDateWidth}
          holidays={holidays}
          visibleStartIndex={visibleStartIndex}
          visibleEndIndex={visibleEndIndex}
          dateColHighlightRef={dateColHighlightRef}
          dateColGhostHighlightRef={dateColGhostHighlightRef}
        />
      </div>

      {/* ── Body row: sticky-left rooms column + scrolling date grid ── */}
      <div style={{ display: "flex", minWidth: leftColumnWidth + totalDateWidth }}>
        <CalendarRoomsColumn
          resources={filteredResources}
          leftColumnWidth={leftColumnWidth}
          selectedResource={selectedResource}
          onSelectRoom={handleSelectResource}
          expandedGroups={expandedGroups}
          onToggleGroup={handleToggleGroup}
          virtualItems={virtualItems}
          scrollElementRef={scrollContainerRef}
          dragHighlightRef={roomCellHighlightRef}
          dragGhostHighlightRef={roomCellGhostHighlightRef}
        />

        <CalendarDateGrid
          resources={filteredResources}
          virtualItems={virtualItems}
          virtualCols={virtualCols}
          colIndexToDate={colIndexToDate}
          totalDateWidth={totalDateWidth}
          scrollElementRef={scrollContainerRef}
          events={eventsForGrid}
          pendingEventIds={pendingEventIdSet}
          reservations={reservations}
          rangeStart={rangeStart}
          expandedGroups={expandedGroups}
          onToggleGroup={handleToggleGroup}
          cellWidth={cellWidth}
          leftColumnWidth={leftColumnWidth}
          filters={filters}
          selectedResource={selectedResource}
          setSelectedResource={handleSelectResource}
          onCellClick={onCellClick}
          onEventClick={onEventClick}
          onBlockClick={onBlockClick}
          onRangeSelect={onRangeSelect}
          onEventResized={handleEventResized}
          onEventMoved={handleEventMoved}
          onRateClick={handleRateClick}
          roomCellHighlightRef={roomCellHighlightRef}
          dateColHighlightRef={dateColHighlightRef}
          roomCellGhostHighlightRef={roomCellGhostHighlightRef}
          dateColGhostHighlightRef={dateColGhostHighlightRef}
          onExpandGroup={handleExpandGroup}
          ratesIsLoading={ratesIsLoading || isVisibleRateWindowOutsideCoverage}
          ratesIsValidating={ratesIsValidating}
        />
      </div>

      {/* Resize / move confirmation modal */}
      {pendingChange && (
        <BookingChangeConfirmationModal
          type={pendingChange.type}
          label={pendingChange.event.guestName ?? pendingChange.event.title}
          oldStart={pendingChange.oldStart}
          oldEnd={pendingChange.oldEnd}
          newStart={pendingChange.newStart}
          newEnd={pendingChange.newEnd}
          oldRoomName={pendingChange.oldRoomName}
          newRoomName={pendingChange.newRoomName}
          oldRoomCategory={pendingChange.oldRoomCategory}
          newRoomCategory={pendingChange.newRoomCategory}
          priceChange={pendingChange.priceChange}
          rateLocked={pendingChange.event.rateLocked}
          propertyTimezone={propertyTimezone}
          onCancel={cancelChange}
          onConfirm={confirmChange}
        />
      )}

      {/* Rate price modal */}
      <CalendarRatePriceModal
        open={rateModal.open}
        onClose={handleRateModalClose}
        roomTypes={currentRatesData}
        initialRoomTypeId={rateModal.roomTypeId}
        roomTypeName={rateModal.roomTypeName}
        dateStr={rateModal.dateStr}
        currentPrice={rateModal.currentPrice}
        onSave={handleRateSave}
        isUpdating={isUpdating}
      />
    </CalendarScrollContainer>
  );
}

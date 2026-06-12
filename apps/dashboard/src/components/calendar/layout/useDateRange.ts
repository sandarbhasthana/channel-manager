"use client";

import { startTransition, useCallback, useEffect, useLayoutEffect, useMemo, useRef } from "react";
import { useVirtualizer, type Virtualizer } from "@tanstack/react-virtual";
import { animate } from "motion/react";
import { addDays, diffDays } from "@/components/calendar/utils/calendarHelpers";

// ── Range constants ───────────────────────────────────────────────────────────
// Total days rendered in the virtual column list (365 before + 365 after center date).
// Extend in RANGE_EXTEND_DAYS chunks when the user scrolls within RANGE_EDGE_GUARD columns
// of either end.
const HALF_RANGE   = 365;
const TOTAL_RANGE  = HALF_RANGE * 2;  // 730 columns
const RANGE_EDGE_GUARD = 30;          // columns from edge that trigger a range extension
export const COL_OVERSCAN = 5;        // extra columns rendered beyond the visible viewport on each side

export interface DateRangeResult {
  rangeStart: string;
  rangeSize: number;
  totalDateWidth: number;
  colVirtualizer: Virtualizer<HTMLDivElement, Element>;
  visibleStartDate: string;
  visibleEndDate: string;
  colIndexToDate: (i: number) => string;
  scrollToDate: (date: string, animated?: boolean) => void;
  navigateDays: (daysOffset: number) => void;
}

interface Options {
  selectedDate: string;
  cellWidth: number;
  scrollElementRef: React.RefObject<HTMLDivElement | null>;
  visibleDays: number;
  onDatesSet?: (start: Date, end: Date) => void;
  navigateRef?: React.RefObject<((dir: -1 | 1) => void) | null>;
  goToDateRef?: React.RefObject<((date: string) => void) | null>;
}

export function useDateRange({
  selectedDate,
  cellWidth,
  scrollElementRef,
  visibleDays,
  onDatesSet,
  navigateRef,
  goToDateRef,
}: Options): DateRangeResult {
  // ── Range state (ref-based to avoid re-renders on extension) ─────────────
  const rangeStartRef = useRef(
    addDays(new Date(selectedDate), -HALF_RANGE).toISOString().slice(0, 10),
  );
  const rangeSizeRef = useRef(TOTAL_RANGE);

  // ── Stable getter — used by virtualizer and event positioning ─────────────
  const colIndexToDate = useCallback((i: number): string => {
    return addDays(new Date(rangeStartRef.current), i).toISOString().slice(0, 10);
  }, []); // stable — rangeStartRef is a mutable ref

  const cellWidthRef = useRef(cellWidth);
  useEffect(() => { cellWidthRef.current = cellWidth; }, [cellWidth]);

  // ── Re-anchor scroll when cellWidth changes (e.g. after ResizeObserver fires) ──
  // The initial scroll position is set with cellWidth=MIN_CELL_WIDTH (containerWidth=0
  // at mount). Once the container is measured, cellWidth updates and we must rescale
  // scrollLeft so the same calendar day stays visible.
  const prevCellWidthRef = useRef<number>(0);
  useLayoutEffect(() => {
    const prev = prevCellWidthRef.current;
    prevCellWidthRef.current = cellWidth;
    if (prev === 0 || prev === cellWidth) return;
    const el = scrollElementRef.current;
    if (!el) return;
    el.scrollLeft = Math.round(el.scrollLeft / prev) * cellWidth;
  }, [cellWidth, scrollElementRef]);

  // ── Column virtualizer ────────────────────────────────────────────────────
  // estimateSize uses cellWidth from the closure (not a ref) so the virtualizer
  // recomputes column starts whenever cellWidth changes.
  const estimateSize = useCallback(() => cellWidth, [cellWidth]);
  const colVirtualizer = useVirtualizer({
    count: rangeSizeRef.current,
    horizontal: true,
    getScrollElement: () => scrollElementRef.current,
    estimateSize,
    overscan: COL_OVERSCAN,
  });

  // TanStack Virtual v3 does not auto-invalidate its size cache when estimateSize
  // changes. Call measure() explicitly so column starts are recomputed with the
  // new cellWidth (e.g. after the ResizeObserver fires on mount).
  useEffect(() => {
    colVirtualizer.measure();
  }, [colVirtualizer, cellWidth]);

  // ── Scroll-to-date helpers ────────────────────────────────────────────────
  const activeAnimRef = useRef<{ stop: () => void } | null>(null);
  // Tracks the logical scroll target of the in-flight animation so rapid
  // navigate calls chain from the intended destination rather than the
  // mid-animation (fractional) scrollLeft, which would misalign columns.
  const pendingTargetRef = useRef<number | null>(null);

  const scrollToDate = useCallback((date: string, animated = true) => {
    const el = scrollElementRef.current;
    if (!el) return;
    const dayOffset = diffDays(rangeStartRef.current, date);
    const target = Math.max(0, dayOffset * cellWidthRef.current);
    if (!animated) {
      el.scrollLeft = target;
      pendingTargetRef.current = null;
      return;
    }
    activeAnimRef.current?.stop();
    pendingTargetRef.current = target;
    const controls = animate(el.scrollLeft, target, {
      type: "spring",
      stiffness: 280,
      damping: 35,
      onUpdate: (v: number) => { el.scrollLeft = v; },
      onComplete: () => { activeAnimRef.current = null; pendingTargetRef.current = null; },
    });
    activeAnimRef.current = controls;
  }, [scrollElementRef]);

  const navigateDays = useCallback((daysOffset: number) => {
    const el = scrollElementRef.current;
    if (!el) return;
    // Chain from the pending target when an animation is already in flight so
    // rapid button presses accumulate correctly and always land on a cell boundary.
    const from = pendingTargetRef.current ?? el.scrollLeft;
    const target = Math.max(
      0,
      Math.min(
        (rangeSizeRef.current - 1) * cellWidthRef.current,
        from + daysOffset * cellWidthRef.current,
      ),
    );
    pendingTargetRef.current = target;
    activeAnimRef.current?.stop();
    const controls = animate(el.scrollLeft, target, {
      type: "spring",
      stiffness: 280,
      damping: 35,
      onUpdate: (v: number) => { el.scrollLeft = v; },
      onComplete: () => { activeAnimRef.current = null; pendingTargetRef.current = null; },
    });
    activeAnimRef.current = controls;
  }, [scrollElementRef]);

  // ── Expose imperative refs ────────────────────────────────────────────────
  useEffect(() => {
    if (navigateRef) navigateRef.current = (dir) => navigateDays(dir * visibleDays);
    return () => { if (navigateRef) navigateRef.current = null; };
  }, [navigateRef, navigateDays, visibleDays]);

  useEffect(() => {
    if (goToDateRef) goToDateRef.current = (date) => scrollToDate(date, true);
    return () => { if (goToDateRef) goToDateRef.current = null; };
  }, [goToDateRef, scrollToDate]);

  // ── Initial scroll position — jump to selectedDate column ─────────────────
  useLayoutEffect(() => {
    const el = scrollElementRef.current;
    if (!el) return;
    const dayOffset = diffDays(rangeStartRef.current, selectedDate);
    el.scrollLeft = Math.max(0, dayOffset * cellWidthRef.current);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []); // only on mount

  // ── Notify parent of visible date range on scroll ─────────────────────────
  const onDatesSetRef = useRef(onDatesSet);
  useEffect(() => { onDatesSetRef.current = onDatesSet; }, [onDatesSet]);

  const lastNotifiedRangeRef = useRef<string>("");
  const notifyDatesSet = useCallback(() => {
    const el = scrollElementRef.current;
    if (!el || !onDatesSetRef.current) return;
    const startOffset = Math.floor(el.scrollLeft / cellWidthRef.current);
    const start = addDays(new Date(rangeStartRef.current), startOffset);
    const end = addDays(start, visibleDays);
    const rangeKey = `${start.toISOString().slice(0, 10)}:${end.toISOString().slice(0, 10)}`;
    if (lastNotifiedRangeRef.current === rangeKey) return;
    lastNotifiedRangeRef.current = rangeKey;
    onDatesSetRef.current(start, end);
  }, [scrollElementRef, visibleDays]);

  useEffect(() => {
    const el = scrollElementRef.current;
    if (!el) return;
    let notifyTimer: ReturnType<typeof setTimeout> | null = null;

    const scheduleNotifyDatesSet = () => {
      if (notifyTimer !== null) clearTimeout(notifyTimer);
      notifyTimer = setTimeout(() => {
        notifyTimer = null;
        notifyDatesSet();
      }, 180);
    };

    // Notify once on mount
    notifyDatesSet();
    el.addEventListener("scroll", scheduleNotifyDatesSet, { passive: true });
    return () => {
      if (notifyTimer !== null) clearTimeout(notifyTimer);
      el.removeEventListener("scroll", scheduleNotifyDatesSet);
    };
  }, [scrollElementRef, notifyDatesSet]);

  // ── Extend range when approaching edges ───────────────────────────────────
  useEffect(() => {
    const el = scrollElementRef.current;
    if (!el) return;

    const checkEdge = () => {
      const scrollLeft = el.scrollLeft;
      const colsFromLeft  = scrollLeft / cellWidthRef.current;
      const totalWidth = rangeSizeRef.current * cellWidthRef.current;
      const colsFromRight = (totalWidth - scrollLeft - el.clientWidth) / cellWidthRef.current;

      if (colsFromLeft < RANGE_EDGE_GUARD) {
        // Prepend HALF_RANGE days — shift rangeStart backward, adjust scrollLeft
        startTransition(() => {
          rangeStartRef.current = addDays(new Date(rangeStartRef.current), -HALF_RANGE)
            .toISOString().slice(0, 10);
          rangeSizeRef.current += HALF_RANGE;
          // Compensate scroll position so content doesn't jump
          el.scrollLeft = scrollLeft + HALF_RANGE * cellWidthRef.current;
        });
      } else if (colsFromRight < RANGE_EDGE_GUARD) {
        // Append HALF_RANGE days — just increase count
        startTransition(() => {
          rangeSizeRef.current += HALF_RANGE;
        });
      }
    };

    el.addEventListener("scroll", checkEdge, { passive: true });
    return () => el.removeEventListener("scroll", checkEdge);
  }, [scrollElementRef]);

  // ── Derived visible dates ─────────────────────────────────────────────────
  const range = colVirtualizer.range;
  const visibleStartDate = useMemo(
    () => colIndexToDate(range?.startIndex ?? 0),
    [range?.startIndex, colIndexToDate],
  );
  const visibleEndDate = useMemo(
    () => colIndexToDate((range?.endIndex ?? visibleDays) - 1),
    [range?.endIndex, visibleDays, colIndexToDate],
  );

  return {
    rangeStart: rangeStartRef.current,
    rangeSize: rangeSizeRef.current,
    totalDateWidth: colVirtualizer.getTotalSize(),
    colVirtualizer,
    visibleStartDate,
    visibleEndDate,
    colIndexToDate,
    scrollToDate,
    navigateDays,
  };
}

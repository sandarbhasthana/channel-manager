"use client";

import { useState, useEffect, useLayoutEffect, useRef, useCallback } from "react";

/**
 * Responsive left column width.
 * Returns the appropriate width based on current viewport.
 * Called once per mount; ResizeObserver handles updates.
 */
export function getLeftColumnWidth(): number {
  if (typeof window === "undefined") return 200;
  if (window.innerWidth < 640) return 80;    // mobile
  if (window.innerWidth < 1024) return 180;  // tablet
  return 280;                                 // desktop
}

import { LEFT_COLUMN_WIDTH, MIN_CELL_WIDTH, MIN_ROOMS_COLUMN_WIDTH, MAX_ROOMS_COLUMN_WIDTH } from "../calendarConfig";
export { LEFT_COLUMN_WIDTH, MIN_ROOMS_COLUMN_WIDTH, MAX_ROOMS_COLUMN_WIDTH };

const ROOMS_WIDTH_STORAGE_KEY = "calendar-rooms-column-width";

interface GridLayout {
  cellWidth: number;
  leftColumnWidth: number;
  setLeftColumnWidth: (width: number) => void;
  containerWidth: number;
  totalGridWidth: number;
  scrollContainerRef: React.RefObject<HTMLDivElement | null>;
}

/**
 * Tracks the scroll container's width via ResizeObserver and computes:
 *   leftColumnWidth — responsive default, overridable by the user via drag
 *   cellWidth — (containerWidth - leftColumnWidth) / visibleDays, min 40px
 *   totalGridWidth — leftColumnWidth + cellWidth * visibleDays
 *
 * The user-chosen column width is persisted to localStorage.
 */
export function useGridLayout(visibleDays: number): GridLayout {
  const scrollContainerRef = useRef<HTMLDivElement | null>(null);
  const [containerWidth, setContainerWidth] = useState(0);

  // Initialise from localStorage if available, otherwise from viewport
  const [leftColumnWidth, setLeftColumnWidthState] = useState<number>(() => {
    if (typeof window === "undefined") return LEFT_COLUMN_WIDTH;
    try {
      const stored = localStorage.getItem(ROOMS_WIDTH_STORAGE_KEY);
      if (stored) {
        const parsed = parseInt(stored, 10);
        if (!isNaN(parsed)) return parsed;
      }
    } catch {
      // ignore
    }
    return getLeftColumnWidth();
  });

  const setLeftColumnWidth = useCallback((width: number) => {
    const clamped = Math.min(MAX_ROOMS_COLUMN_WIDTH, Math.max(MIN_ROOMS_COLUMN_WIDTH, width));
    setLeftColumnWidthState(clamped);
    try {
      localStorage.setItem(ROOMS_WIDTH_STORAGE_KEY, String(clamped));
    } catch {
      // ignore
    }
  }, []);

  const updateContainerWidth = useCallback(() => {
    if (scrollContainerRef.current) {
      setContainerWidth(scrollContainerRef.current.clientWidth);
    }
  }, []);

  // Measure synchronously before first paint so cellWidth is correct on initial render.
  useLayoutEffect(() => {
    updateContainerWidth();
  }, [updateContainerWidth]);

  useEffect(() => {
    const el = scrollContainerRef.current;
    if (!el) return;
    const observer = new ResizeObserver(updateContainerWidth);
    observer.observe(el);
    return () => observer.disconnect();
  }, [updateContainerWidth]);

  const availableWidth = Math.max(0, containerWidth - leftColumnWidth);
  const rawCellWidth = visibleDays > 0 ? availableWidth / visibleDays : 80;
  // Round to integer so virtualizer positions (index * cellWidth) and the CSS
  // background-size grid lines (backgroundSize: cellWidth px) stay on the same
  // pixel boundaries — float cellWidth causes sub-pixel drift between header and body.
  const cellWidth = Math.max(MIN_CELL_WIDTH, Math.round(rawCellWidth));
  const totalGridWidth = leftColumnWidth + cellWidth * visibleDays;

  return { cellWidth, leftColumnWidth, setLeftColumnWidth, containerWidth, totalGridWidth, scrollContainerRef };
}

"use client";

import React from "react";
import { CALENDAR_THEME } from "../calendarTheme";

interface CalendarScrollContainerProps {
  scrollContainerRef: React.RefObject<HTMLDivElement | null>;
  children: React.ReactNode;
  className?: string;
}

/**
 * The ONLY scrollable element in the custom calendar grid.
 * Both axes scroll here. position:sticky works inside this container
 * for headers and the left column.
 *
 * height is set to fill available space below the toolbar.
 * -webkit-overflow-scrolling: touch enables momentum scroll on iOS.
 */
export const CalendarScrollContainer = React.memo(function CalendarScrollContainer({
  scrollContainerRef,
  children,
  className = ""
}: CalendarScrollContainerProps) {
  return (
    <div
      ref={scrollContainerRef}
      className={`relative rounded-xl border ${CALENDAR_THEME.border}
        h-[calc(100dvh-282px)] sm:h-[calc(100dvh-282px)]
        [&::-webkit-scrollbar]:hidden ${className}`}
      style={{
        // h-[calc(100dvh-256px)] mobile:  64px app-header + 16px main-pt + 8px page-pt
        //   + 76px 2-row booking-header (h1 28 + gap 8 + controls 40) + 12px mb-3
        //   + 8px page-pb + 16px main-pb + 48px app-footer = 248px + 8px buffer
        // sm:h-[calc(100dvh-272px)] sm:   64px app-header + 16px main-pt + 24px page-pt
        //   + 50px 1-row booking-header + 24px mb-6
        //   + 24px page-pb + 16px main-pb + 48px app-footer = 266px + 6px buffer
        minHeight: 280,
        // Both axes scroll natively; scrollbars hidden via CSS; momentum driven by hooks
        overflowX: "scroll" as React.CSSProperties["overflowX"],
        overflowY: "scroll" as React.CSSProperties["overflowY"],
        scrollbarWidth: "none" as React.CSSProperties["scrollbarWidth"], // Firefox
        WebkitOverflowScrolling: "touch" as React.CSSProperties["WebkitOverflowScrolling"],
        overscrollBehavior: "contain",
        // pan-y: allow native vertical scroll; the carousel hook intercepts horizontal swipes
        touchAction: "pan-y"
      }}
    >
      {children}
    </div>
  );
});

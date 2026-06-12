"use client";

import {
  ROW_HEIGHT,
  TYPE_ROW_HEIGHT,
  HEADER_ROW_HEIGHT,
  LEFT_COLUMN_WIDTH,
  BAR_TOP,
  BAR_HEIGHT
} from "./calendarConfig";

/**
 * Placeholder that mirrors the CustomCalendarGrid layout (rooms column + date
 * header + room rows) so first load swaps to the real grid without a jump.
 * Uses the shared `.skeleton` shimmer from globals.css.
 *
 * Shapes only — no data. A couple of room-type groups, each with a few rooms,
 * and a scattering of "event" bars to hint at the real content density.
 */
export default function CalendarGridSkeleton({
  visibleDays = 14
}: {
  visibleDays?: number;
}) {
  const days = Array.from({ length: visibleDays });

  // Two groups of rooms, mimicking room-type groupings.
  const groups = [
    { rooms: 4 },
    { rooms: 3 }
  ];

  // Pseudo-random-but-stable event bars keyed by row index so the placeholder
  // looks lived-in rather than empty. [rowIndex, startDay, span]
  const bars: Array<[number, number, number]> = [
    [0, 1, 3],
    [1, 4, 2],
    [2, 0, 2],
    [3, 6, 4],
    [4, 2, 3],
    [5, 8, 2],
    [6, 3, 5]
  ];

  let rowIndex = -1;

  return (
    <div
      className="w-full overflow-hidden rounded-lg border border-slate-200 dark:border-slate-700"
      aria-hidden="true"
    >
      {/* Header row: rooms-column corner + date cells */}
      <div
        className="flex border-b border-slate-200 dark:border-slate-700"
        style={{ height: HEADER_ROW_HEIGHT }}
      >
        <div
          className="shrink-0 border-r border-slate-200 dark:border-slate-700 flex items-center px-4"
          style={{ width: LEFT_COLUMN_WIDTH }}
        >
          <div className="skeleton h-5 w-32 rounded" />
        </div>
        <div className="flex flex-1">
          {days.map((_, i) => (
            <div
              key={i}
              className="flex flex-1 flex-col items-center justify-center gap-1.5 border-r border-slate-100 dark:border-slate-800 last:border-r-0"
            >
              <div className="skeleton h-3 w-6 rounded" />
              <div className="skeleton h-4 w-5 rounded" />
            </div>
          ))}
        </div>
      </div>

      {/* Room-type groups + room rows */}
      {groups.map((group, gi) => (
        <div key={gi}>
          {/* Group header row */}
          <div
            className="flex border-b border-slate-200 bg-slate-50 dark:border-slate-700 dark:bg-slate-800/40"
            style={{ height: TYPE_ROW_HEIGHT }}
          >
            <div
              className="shrink-0 border-r border-slate-200 dark:border-slate-700 flex items-center px-4"
              style={{ width: LEFT_COLUMN_WIDTH }}
            >
              <div className="skeleton h-4 w-40 rounded" />
            </div>
            <div className="flex flex-1">
              {days.map((_, i) => (
                <div
                  key={i}
                  className="flex flex-1 items-center justify-center border-r border-slate-100 dark:border-slate-800 last:border-r-0"
                >
                  <div className="skeleton h-3 w-4 rounded" />
                </div>
              ))}
            </div>
          </div>

          {/* Room rows */}
          {Array.from({ length: group.rooms }).map((_, ri) => {
            rowIndex += 1;
            const bar = bars.find(([r]) => r === rowIndex);
            return (
              <div
                key={ri}
                className="flex border-b border-slate-100 dark:border-slate-800"
                style={{ height: ROW_HEIGHT }}
              >
                <div
                  className="shrink-0 border-r border-slate-200 dark:border-slate-700 flex items-center gap-3 px-4"
                  style={{ width: LEFT_COLUMN_WIDTH }}
                >
                  <div className="skeleton h-8 w-8 rounded-full" />
                  <div className="skeleton h-4 w-28 rounded" />
                </div>
                <div className="relative flex flex-1">
                  {days.map((_, i) => (
                    <div
                      key={i}
                      className="flex-1 border-r border-slate-100 dark:border-slate-800 last:border-r-0"
                    />
                  ))}
                  {bar && (
                    <div
                      className="skeleton absolute rounded-md"
                      style={{
                        top: BAR_TOP,
                        height: BAR_HEIGHT,
                        left: `${(bar[1] / visibleDays) * 100}%`,
                        width: `${(bar[2] / visibleDays) * 100}%`
                      }}
                    />
                  )}
                </div>
              </div>
            );
          })}
        </div>
      ))}
    </div>
  );
}

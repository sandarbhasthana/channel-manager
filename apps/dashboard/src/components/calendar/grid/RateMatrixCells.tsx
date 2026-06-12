"use client";

import React from "react";
import { useRateCell } from "@/lib/hooks/useRatesData";
import { CALENDAR_THEME } from "../calendarTheme";
import { OCCUPANCY_LABEL_HEIGHT, RATE_LABEL_HEIGHT } from "../calendarConfig";

// Leaf cells that subscribe to a single streamed rate slot via `useRateCell`, so
// each paints the instant its piece arrives over the SSE stream without
// re-rendering the surrounding (memoized) grid rows. The calendar always reads
// the "base" rate plan with business rules applied.
const RATE_PLAN = "base";
const APPLY_BUSINESS_RULES = true;

interface GroupRateCellProps {
  roomTypeName: string;
  dateStr: string;
  showRates: boolean;
  showOccupancy: boolean;
  occupied: number;
  roomCount: number;
  /** A rates fetch is in-flight — show a skeleton for cells not yet streamed. */
  isFetching: boolean;
  onRateClick?: (roomTypeId: string, roomTypeName: string, dateStr: string, currentPrice: number) => void;
}

/** Room-type (group) row cell: occupancy + clickable rate, streamed per cell. */
export const GroupRateCell = React.memo(function GroupRateCell({
  roomTypeName,
  dateStr,
  showRates,
  showOccupancy,
  occupied,
  roomCount,
  isFetching,
  onRateClick,
}: GroupRateCellProps) {
  const entry = useRateCell(RATE_PLAN, APPLY_BUSINESS_RULES, roomTypeName, dateStr);
  const rate = showRates && entry ? entry.cell.finalPrice : null;
  const isRateLoading = showRates && isFetching && rate == null;
  const hasRate = showRates && rate != null;

  if (isRateLoading) {
    return (
      <div
        className="flex flex-col items-center justify-center gap-1"
        style={{ height: OCCUPANCY_LABEL_HEIGHT + RATE_LABEL_HEIGHT }}
      >
        <div className={`h-2 w-7 animate-pulse rounded ${CALENDAR_THEME.dateGridGroupRow.rateSkeleton}`} />
        <div className={`h-2 w-11 animate-pulse rounded ${CALENDAR_THEME.dateGridGroupRow.rateSkeleton}`} />
      </div>
    );
  }

  if (hasRate) {
    return (
      <>
        {showOccupancy && (
          <div
            className="flex items-center justify-center"
            style={{ height: OCCUPANCY_LABEL_HEIGHT, minHeight: OCCUPANCY_LABEL_HEIGHT }}
          >
            <span className={`block max-w-full truncate text-xs font-semibold leading-tight ${CALENDAR_THEME.dateGridGroupRow.occupancyText}`}>
              {occupied}/{roomCount}
            </span>
          </div>
        )}
        <div
          className="flex items-center justify-center"
          style={{ height: RATE_LABEL_HEIGHT, minHeight: RATE_LABEL_HEIGHT }}
        >
          <button
            className={`block max-w-full truncate text-xs font-bold leading-tight ${CALENDAR_THEME.dateGridGroupRow.rateBtn}`}
            onClick={(e) => {
              e.stopPropagation();
              if (!entry) return;
              onRateClick?.(entry.roomTypeId, roomTypeName, dateStr, rate);
            }}
          >
            ₹{rate.toLocaleString("en-IN")}
          </button>
        </div>
      </>
    );
  }

  if (!showRates && showOccupancy) {
    return (
      <div
        className="flex items-center justify-center"
        style={{ height: OCCUPANCY_LABEL_HEIGHT, minHeight: OCCUPANCY_LABEL_HEIGHT }}
      >
        <span className={`block max-w-full truncate text-xs font-semibold leading-tight ${CALENDAR_THEME.dateGridGroupRow.occupancyText}`}>
          {occupied}/{roomCount}
        </span>
      </div>
    );
  }

  return null;
});

interface RoomRateLoaderCellProps {
  roomTypeName: string;
  dateStr: string;
  left: number;
  width: number;
  /** A rates fetch is in-flight — show a skeleton until this cell streams in. */
  isFetching: boolean;
}

/** Individual room row: a per-cell skeleton shown until its room-type rate arrives. */
export const RoomRateLoaderCell = React.memo(function RoomRateLoaderCell({
  roomTypeName,
  dateStr,
  left,
  width,
  isFetching,
}: RoomRateLoaderCellProps) {
  const entry = useRateCell(RATE_PLAN, APPLY_BUSINESS_RULES, roomTypeName, dateStr);
  if (!isFetching || entry) return null;
  return (
    <div
      className="absolute top-0 bottom-0 flex items-center justify-center pointer-events-none"
      style={{ left, width }}
    >
      <div className={`h-2 w-8 animate-pulse rounded opacity-50 ${CALENDAR_THEME.dateGridRoomRow.skeleton}`} />
    </div>
  );
});

"use client";

import useSWR from "swr";
import { useEffect, useState, useCallback, useMemo, useSyncExternalStore } from "react";
import { addDays } from "date-fns";
import { toast } from "sonner";
import { usePropertyContext } from "@/components/property-provider";
import { RatePoint } from "@/lib/api";
import { fetchRatesForProperty, bulkUpsertRatesAction, fetchRoomTypesForProperty } from "@/app/dashboard/actions";

// Types for rates data
export interface RateRestrictions {
  minLOS?: number;
  maxLOS?: number;
  closedToArrival: boolean;
  closedToDeparture: boolean;
}

export interface RateCell {
  basePrice: number;
  finalPrice: number;
  availability: number;
  isOverride: boolean;
  isSeasonal: boolean;
  restrictions: RateRestrictions;
}

export interface RoomTypeRates {
  roomTypeId: string;
  roomTypeName: string;
  totalRooms: number;
  dates: Record<string, RateCell>;
}

export interface RatesResponse {
  success: boolean;
  data: RoomTypeRates[];
  dateRange: {
    startDate: string;
    endDate: string;
    dates: string[];
  };
  businessRulesEnabled?: boolean;
}

const EMPTY_RATES: RoomTypeRates[] = [];
const successfulRatesCache = new Map<string, RatesResponse>();
const ratesCacheListeners = new Set<() => void>();

function emitRatesCacheChange() {
  for (const listener of ratesCacheListeners) listener();
}

function subscribeRatesCache(listener: () => void) {
  ratesCacheListeners.add(listener);
  return () => {
    ratesCacheListeners.delete(listener);
  };
}

export interface RateCellEntry {
  roomTypeId: string;
  roomTypeName: string;
  cell: RateCell;
}

const rateCellStore = new Map<string, RateCellEntry>();
const rateCellListeners = new Map<string, Set<() => void>>();

export function rateCellStoreKey(
  ratePlan: string,
  applyBusinessRules: boolean,
  roomTypeName: string,
  date: string,
): string {
  return `${ratePlan}|${applyBusinessRules}|${roomTypeName}|${date}`;
}

function setRateCell(key: string, entry: RateCellEntry) {
  rateCellStore.set(key, entry);
  const listeners = rateCellListeners.get(key);
  if (listeners) for (const listener of listeners) listener();
}

function subscribeRateCell(key: string, listener: () => void) {
  let listeners = rateCellListeners.get(key);
  if (!listeners) {
    listeners = new Set();
    rateCellListeners.set(key, listeners);
  }
  listeners.add(listener);
  return () => {
    listeners!.delete(listener);
    if (listeners!.size === 0) rateCellListeners.delete(key);
  };
}

export function useRateCell(
  ratePlan: string,
  applyBusinessRules: boolean,
  roomTypeName: string,
  date: string,
): RateCellEntry | undefined {
  const key = rateCellStoreKey(ratePlan, applyBusinessRules, roomTypeName, date);
  const subscribe = useCallback((listener: () => void) => subscribeRateCell(key, listener), [key]);
  const getSnapshot = useCallback(() => rateCellStore.get(key), [key]);
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

// Format number as zero-padded string
const pad = (n: number) => n.toString().padStart(2, '0');

export function useRatesData(
  startDate: Date,
  days: number = 7,
  ratePlan: string = "base",
  applyBusinessRules: boolean = true,
  direction: "forward" | "backward" = "forward"
) {
  const { activeProperty } = usePropertyContext();
  const propertyId = activeProperty?.id;

  const startDateStr = startDate.toISOString().slice(0, 10);
  const endDateStr = addDays(startDate, days - 1).toISOString().slice(0, 10);
  
  const requestedDates = useMemo(
    () => Array.from({ length: days }, (_, i) =>
      addDays(startDate, i).toISOString().slice(0, 10)
    ),
    [startDateStr, days], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const swrKey = propertyId ? `/api/rates/${propertyId}?start=${startDateStr}&end=${endDateStr}` : null;

  const fetcher = async (url: string): Promise<RatesResponse> => {
    if (!propertyId) throw new Error("No property id");
    const [roomTypes, points] = await Promise.all([
      fetchRoomTypesForProperty(propertyId),
      fetchRatesForProperty(propertyId, startDateStr, endDateStr)
    ]);

    const data: RoomTypeRates[] = roomTypes.map((rt) => ({
      roomTypeId: rt.id,
      roomTypeName: rt.name,
      totalRooms: rt.rooms?.length ?? 10,
      dates: {}
    }));

    const rtMap = new Map<string, RoomTypeRates>();
    for (const rt of data) {
      rtMap.set(rt.roomTypeId, rt);
    }

    for (const pt of points) {
      const ptRoomTypeId = pt.room_type_id || pt.roomTypeId;
      if (!ptRoomTypeId) continue;
      const rt = rtMap.get(ptRoomTypeId);
      if (!rt) continue;

      const y = pt.date.year;
      const m = pad(pt.date.month);
      const d = pad(pt.date.day);
      const dateStr = `${y}-${m}-${d}`;

      const price = pt.amount.units + (pt.amount.nanos / 1e9);

      const cell: RateCell = {
        basePrice: price,
        finalPrice: price,
        availability: rt.totalRooms, // Default to total rooms for now
        isOverride: false,
        isSeasonal: false,
        restrictions: {
          closedToArrival: false,
          closedToDeparture: false
        }
      };

      rt.dates[dateStr] = cell;

      // Update per-cell store for fast re-rendering
      setRateCell(rateCellStoreKey(ratePlan, applyBusinessRules, rt.roomTypeName, dateStr), {
        roomTypeId: rt.roomTypeId,
        roomTypeName: rt.roomTypeName,
        cell
      });
    }

    const response: RatesResponse = {
      success: true,
      data,
      dateRange: { startDate: startDateStr, endDate: endDateStr, dates: requestedDates },
      businessRulesEnabled: false
    };

    successfulRatesCache.set(url, response);
    emitRatesCacheChange();
    return response;
  };

  const { data, error, isLoading, isValidating, mutate } = useSWR<RatesResponse>(
    swrKey,
    fetcher,
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
    }
  );

  const dates = useMemo(
    () => Array.from({ length: days }, (_, i) => addDays(startDate, i)),
    [startDateStr, days] // eslint-disable-line react-hooks/exhaustive-deps
  );

  return {
    data: data?.data || EMPTY_RATES,
    dateRange: data?.dateRange,
    dates,
    isLoading,
    isValidating,
    error,
    mutate
  };
}

// Stub implementation since we fetch directly
export function useMergedRates(
  rangeStart: Date,
  rangeDays: number,
  ratePlan: string = "base",
  applyBusinessRules: boolean = true,
): RoomTypeRates[] {
  // Can just rely on useRatesData since our Channel Manager calendar will just use useRatesData
  return EMPTY_RATES;
}

export function rateSwrKeyCoversDate(key: string, date: string): boolean {
  return true; // Simplified for channel manager
}

export function useRateUpdates() {
  const [isUpdating, setIsUpdating] = useState(false);
  const { activeProperty } = usePropertyContext();

  const bulkUpdateRates = useCallback(
    async (
      updates: Array<{
        roomTypeId: string;
        date: string;
        price: number;
        availability?: number;
        restrictions?: Partial<RateRestrictions>;
      }>
    ) => {
      if (!activeProperty?.id) throw new Error("No active property");
      setIsUpdating(true);

      try {
        const points: RatePoint[] = updates.map((u) => {
          const parts = u.date.split("-").map(Number);
          return {
            property_id: activeProperty.id,
            room_type_id: u.roomTypeId,
            rate_plan_id: "base", // Default
            date: { year: parts[0], month: parts[1], day: parts[2] },
            amount: { currency_code: "INR", units: Math.floor(u.price), nanos: Math.round((u.price % 1) * 1e9) }
          };
        });

        await bulkUpsertRatesAction(points);
        toast.success(`Successfully updated ${updates.length} rates`);
        return { success: true, updatedCount: updates.length };
      } catch (error) {
        toast.error(`Failed to bulk update rates`);
        throw error;
      } finally {
        setIsUpdating(false);
      }
    },
    [activeProperty]
  );

  // Simplified stubs for unused methods
  const updateRate = useCallback(async () => {}, []);
  const deleteRate = useCallback(async () => {}, []);

  return {
    updateRate,
    bulkUpdateRates,
    deleteRate,
    isUpdating
  };
}

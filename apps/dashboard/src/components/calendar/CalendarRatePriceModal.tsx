"use client";

import { useState, useEffect, useRef, useMemo } from "react";
import { format, eachDayOfInterval } from "date-fns";
import { IndianRupee, X } from "lucide-react";
import { Modal, Button, Input, Checkbox, Select, Tooltip } from "antd";
import { cn } from "@/lib/utils";
import type { RoomTypeRates, RateRestrictions } from "@/lib/hooks/useRatesData";

type RateUpdate = {
  roomTypeId: string;
  date: string;
  price: number;
  availability?: number;
  restrictions?: Partial<RateRestrictions>;
};

interface CalendarRatePriceModalProps {
  open: boolean;
  onClose: () => void;
  /** All room types available for selection */
  roomTypes: RoomTypeRates[];
  /** The room type that was clicked — pre-selected */
  initialRoomTypeId: string;
  /** Display name of the clicked room type */
  roomTypeName: string;
  /** YYYY-MM-DD of the clicked cell — used as the initial date range */
  dateStr: string;
  /** Current (existing) price of the clicked cell — used as reference */
  currentPrice: number;
  onSave: (updates: RateUpdate[]) => Promise<void>;
  isUpdating?: boolean;
}

const DAY_NAMES = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];
const WEEKDAY_INDICES = [1, 2, 3, 4, 5];
const WEEKEND_INDICES = [0, 6];

function formatInr(amount: number) {
  return new Intl.NumberFormat("en-IN", {
    style: "currency",
    currency: "INR",
    minimumFractionDigits: 0,
    maximumFractionDigits: 0
  }).format(amount);
}

export function CalendarRatePriceModal({
  open,
  onClose,
  roomTypes,
  initialRoomTypeId,
  roomTypeName,
  dateStr,
  currentPrice,
  onSave,
  isUpdating = false
}: CalendarRatePriceModalProps) {
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  // ── form state (mirrors BulkUpdateModal) ──────────────────────────────────
  const [selectedRoomTypes, setSelectedRoomTypes] = useState<string[]>([]);
  const [dateRange, setDateRange] = useState({ start: dateStr, end: dateStr });
  const [updateType, setUpdateType] = useState<"set" | "adjust">("set");
  const [priceValue, setPriceValue] = useState("");
  const [adjustmentType, setAdjustmentType] = useState<"amount" | "percentage">("amount");
  const [availability, setAvailability] = useState("");
  const [selectedDays, setSelectedDays] = useState<number[]>([0, 1, 2, 3, 4, 5, 6]);
  const [applyToWeekdays, setApplyToWeekdays] = useState(true);
  const [applyToWeekends, setApplyToWeekends] = useState(true);
  const [restrictions, setRestrictions] = useState({
    minLOS: "",
    maxLOS: "",
    closedToArrival: false,
    closedToDeparture: false
  });

  // Reset & pre-fill when opening
  useEffect(() => {
    if (!open) return;
    setSelectedRoomTypes(initialRoomTypeId ? [initialRoomTypeId] : []);
    setDateRange({ start: dateStr, end: dateStr });
    setUpdateType("set");
    setPriceValue(currentPrice > 0 ? String(currentPrice) : "");
    setAdjustmentType("amount");
    setAvailability("");
    setSelectedDays([0, 1, 2, 3, 4, 5, 6]);
    setApplyToWeekdays(true);
    setApplyToWeekends(true);
    setRestrictions({ minLOS: "", maxLOS: "", closedToArrival: false, closedToDeparture: false });
    // focus the price input after paint
    setTimeout(() => {
      const el = scrollContainerRef.current?.querySelector<HTMLInputElement>('input[type="number"]');
      el?.focus();
      el?.select();
    }, 80);
  }, [open, initialRoomTypeId, dateStr, currentPrice]);

  // Keep weekday/weekend checkboxes in sync with individual day selections
  useEffect(() => {
    setApplyToWeekdays(WEEKDAY_INDICES.every((d) => selectedDays.includes(d)));
    setApplyToWeekends(WEEKEND_INDICES.every((d) => selectedDays.includes(d)));
  }, [selectedDays]);

  // ── helpers ───────────────────────────────────────────────────────────────
  const toggleRoomType = (id: string) =>
    setSelectedRoomTypes((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
    );

  const selectAllRoomTypes = () =>
    setSelectedRoomTypes(
      selectedRoomTypes.length === roomTypes.length ? [] : roomTypes.map((rt) => rt.roomTypeId)
    );

  const toggleDay = (i: number) =>
    setSelectedDays((prev) =>
      prev.includes(i) ? prev.filter((d) => d !== i) : [...prev, i].sort((a, b) => a - b)
    );

  const handleWeekdaysToggle = (checked: boolean) => {
    setApplyToWeekdays(checked);
    setSelectedDays((prev) =>
      checked
        ? [...new Set([...prev, ...WEEKDAY_INDICES])].sort((a, b) => a - b)
        : prev.filter((d) => !WEEKDAY_INDICES.includes(d))
    );
  };

  const handleWeekendsToggle = (checked: boolean) => {
    setApplyToWeekends(checked);
    setSelectedDays((prev) =>
      checked
        ? [...new Set([...prev, ...WEEKEND_INDICES])].sort((a, b) => a - b)
        : prev.filter((d) => !WEEKEND_INDICES.includes(d))
    );
  };

  const handleSelectAllDays = () => {
    if (selectedDays.length === 7) {
      setSelectedDays([]);
    } else {
      setSelectedDays([0, 1, 2, 3, 4, 5, 6]);
    }
  };

  // ── calculate updates (same logic as BulkUpdateModal) ────────────────────
  const calculateUpdates = useMemo((): RateUpdate[] => {
    const updates: RateUpdate[] = [];
    if (!priceValue || !selectedRoomTypes.length) return updates;

    const start = new Date(dateRange.start + "T00:00:00");
    const end = new Date(dateRange.end + "T00:00:00");
    if (isNaN(start.getTime()) || isNaN(end.getTime()) || start > end) return updates;

    const dates = eachDayOfInterval({ start, end });

    selectedRoomTypes.forEach((roomTypeId) => {
      const roomType = roomTypes.find((rt) => rt.roomTypeId === roomTypeId);
      if (!roomType) return;

      dates.forEach((date) => {
        const dayOfWeek = date.getDay();
        if (!selectedDays.includes(dayOfWeek)) return;

        const dateString = format(date, "yyyy-MM-dd");
        const existingRate = roomType.dates[dateString];
        if (!existingRate) return;

        let newPrice: number;
        if (updateType === "set") {
          newPrice = parseFloat(priceValue);
        } else if (adjustmentType === "amount") {
          newPrice = existingRate.finalPrice + parseFloat(priceValue);
        } else {
          newPrice = existingRate.finalPrice * (1 + parseFloat(priceValue) / 100);
        }
        if (isNaN(newPrice) || newPrice < 0) return;

        const update: RateUpdate = {
          roomTypeId,
          date: dateString,
          price: Math.round(newPrice * 100) / 100
        };

        if (availability) {
          const av = parseInt(availability);
          if (!isNaN(av) && av >= 0) update.availability = av;
        }

        const r: Partial<RateRestrictions> = {};
        if (restrictions.minLOS) { const v = parseInt(restrictions.minLOS); if (!isNaN(v) && v > 0) r.minLOS = v; }
        if (restrictions.maxLOS) { const v = parseInt(restrictions.maxLOS); if (!isNaN(v) && v > 0) r.maxLOS = v; }
        if (restrictions.closedToArrival) r.closedToArrival = true;
        if (restrictions.closedToDeparture) r.closedToDeparture = true;
        if (Object.keys(r).length) update.restrictions = r;

        updates.push(update);
      });
    });

    return updates;
  }, [selectedRoomTypes, dateRange, selectedDays, updateType, priceValue, adjustmentType, availability, restrictions, roomTypes]);

  // ── Price Impact filter tag (null = overall / aggregate) ─────────────────
  const [activeImpactFilter, setActiveImpactFilter] = useState<string | null>(null);

  // Clear the filter if the filtered room type is deselected
  useEffect(() => {
    if (activeImpactFilter && !selectedRoomTypes.includes(activeImpactFilter)) {
      setActiveImpactFilter(null);
    }
  }, [selectedRoomTypes, activeImpactFilter]);

  // ── overall table: one row per selected room type ────────────────────────
  const overallTableRows = useMemo(() => {
    return selectedRoomTypes.map((id) => {
      const rt = roomTypes.find((r) => r.roomTypeId === id);
      const updates = calculateUpdates.filter((u) => u.roomTypeId === id);
      if (updates.length === 0) {
        return { id, name: rt?.roomTypeName ?? id, current: null as number | null, newPrice: null as number | null, diff: null as number | null, count: 0 };
      }
      const avgCurrent = updates.reduce((s, u) => s + (rt?.dates[u.date]?.finalPrice ?? 0), 0) / updates.length;
      const avgNew     = updates.reduce((s, u) => s + u.price, 0) / updates.length;
      return { id, name: rt?.roomTypeName ?? id, current: avgCurrent, newPrice: avgNew, diff: avgNew - avgCurrent, count: updates.length };
    });
  }, [selectedRoomTypes, calculateUpdates, roomTypes]);

  // ── filtered table: one row per date for the active room type ─────────────
  const filteredTableRows = useMemo(() => {
    if (!activeImpactFilter) return [];
    const rt = roomTypes.find((r) => r.roomTypeId === activeImpactFilter);
    return calculateUpdates
      .filter((u) => u.roomTypeId === activeImpactFilter)
      .map((u) => {
        const current = rt?.dates[u.date]?.finalPrice ?? 0;
        return { date: u.date, current, newPrice: u.price, diff: u.price - current };
      });
  }, [activeImpactFilter, calculateUpdates, roomTypes]);

  const isFormValid = selectedRoomTypes.length > 0 && !!priceValue && !isNaN(parseFloat(priceValue));

  const handleSubmit = async () => {
    if (calculateUpdates.length === 0) return;
    try {
      await onSave(calculateUpdates);
      onClose();
    } catch {
      // error toast handled upstream
    }
  };

  // ── render ────────────────────────────────────────────────────────────────
  return (
    <Modal
      open={open}
      onCancel={onClose}
      footer={<div className="flex justify-end gap-2 mt-4">
            <Button variant="outlined" onClick={onClose} disabled={isUpdating}>
              Cancel
            </Button>
            <Button
              onClick={handleSubmit}
              disabled={!isFormValid || isUpdating || calculateUpdates.length === 0}
            >
              {isUpdating ? "Updating…" : `Update ${calculateUpdates.length} Rate${calculateUpdates.length !== 1 ? "s" : ""}`}
            </Button>
          </div>}
      title={<div>
            <div className="text-lg font-semibold">
              <div className="flex items-center gap-2">
                <IndianRupee className="h-4 w-4" />
                Rate Update
              </div>
              <Tooltip title="Close">
                  <Button
                    variant="outlined"
                    size="small"
                    onClick={onClose}
                    className="h-8 w-8 p-0 border-2 border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500 transition-all duration-200 hover:rotate-90"
                  >
                    <X className="h-4 w-4" />
                  </Button>
                </Tooltip>
            </div>
            <div className="text-sm text-gray-500 mt-1">
              Update rates for one or more room types across a date range.
              Pre-selected: <strong>{roomTypeName}</strong> · {dateStr ? format(new Date(dateStr + "T00:00:00"), "EEE, d MMM yyyy") : ""}
            </div>
          </div>}
      width={800}
      centered
      closeIcon={<X className="h-4 w-4" />}
    >

          {/* ── Header ── */}
          

          <div className="space-y-8">

            {/* ── 1. Room Type Selection ── */}
            <div>
              <div className="flex items-center justify-between mb-3">
                <label className="text-base font-medium">Room Types</label>
                <Button type="default" variant="outlined" size="small" onClick={selectAllRoomTypes}>
                  {selectedRoomTypes.length === roomTypes.length ? "Deselect All" : "Select All"}
                </Button>
              </div>
              <div className="grid grid-cols-3 gap-3 max-h-40 overflow-y-auto border rounded p-4 bg-gray-50 dark:bg-gray-800">
                {roomTypes.map((rt) => (
                  <div key={rt.roomTypeId} className="flex items-start space-x-2">
                    <Checkbox
                      id={rt.roomTypeId}
                      checked={selectedRoomTypes.includes(rt.roomTypeId)}
                      onChange={() => toggleRoomType(rt.roomTypeId)}
                      className="mt-0.5"
                    />
                    <label htmlFor={rt.roomTypeId} className="text-sm cursor-pointer font-medium leading-4">
                      {rt.roomTypeName} ({rt.totalRooms})
                    </label>
                  </div>
                ))}
              </div>
            </div>

            {/* ── 2. Date Range ── */}
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label htmlFor="startDate">Start Date</label>
                <Input
                  id="startDate"
                  type="date"
                  value={dateRange.start}
                  onChange={(e) => setDateRange((prev) => ({ ...prev, start: e.target.value }))}
                />
              </div>
              <div>
                <label htmlFor="endDate">End Date</label>
                <Input
                  id="endDate"
                  type="date"
                  value={dateRange.end}
                  onChange={(e) => setDateRange((prev) => ({ ...prev, end: e.target.value }))}
                />
              </div>
            </div>

            {/* ── 3. Apply To Days ── */}
            <div>
              <div className="flex items-center justify-between mb-3">
                <label className="text-base font-medium">Apply To Days</label>
                <Button type="default" variant="outlined" size="small" onClick={handleSelectAllDays}>
                  {selectedDays.length === 7 ? "Deselect All" : "Select All"}
                </Button>
              </div>
              {/* Quick: weekdays / weekends */}
              <div className="flex space-x-4 mb-3">
                <div className="flex items-start space-x-2">
                  <Checkbox
                    id="weekdays"
                    checked={applyToWeekdays}
                    onChange={(e) => handleWeekdaysToggle(e.target.checked)}
                    className="mt-0.5"
                  />
                  <label htmlFor="weekdays" className="text-sm font-medium leading-4 cursor-pointer">
                    Weekdays (Mon–Fri)
                  </label>
                </div>
                <div className="flex items-start space-x-2">
                  <Checkbox
                    id="weekends"
                    checked={applyToWeekends}
                    onChange={(e) => handleWeekendsToggle(e.target.checked)}
                    className="mt-0.5"
                  />
                  <label htmlFor="weekends" className="text-sm font-medium leading-4 cursor-pointer">
                    Weekends (Sat–Sun)
                  </label>
                </div>
              </div>
              {/* Individual days */}
              <div className="grid grid-cols-7 gap-3 p-4 border rounded-lg bg-gray-50 dark:bg-gray-800">
                {DAY_NAMES.map((name, i) => (
                  <div key={i} className="flex items-start space-x-2">
                    <Checkbox
                      id={`day-${i}`}
                      checked={selectedDays.includes(i)}
                      onChange={() => toggleDay(i)}
                      className="mt-0.5"
                    />
                    <label htmlFor={`day-${i}`} className="text-sm font-medium cursor-pointer leading-4">
                      {name.slice(0, 3)}
                    </label>
                  </div>
                ))}
              </div>
              {selectedDays.length > 0 && (
                <p className="text-sm text-gray-600 dark:text-gray-400 mt-2">
                  Selected: {selectedDays.map((d) => DAY_NAMES[d]).join(", ")}
                </p>
              )}
            </div>

            {/* ── 4. Price Update ── */}
            <div>
              <label className="text-base font-medium">Price Update</label>
              <div className="grid grid-cols-3 gap-4 mt-4">
                <div>
                  <label className="text-sm font-medium">Update Type</label>
                  <Select value={updateType} onChange={(v) => { setUpdateType(v as "set" | "adjust"); setPriceValue(v === "set" ? String(currentPrice) : ""); }} className="w-full">
  <Select.Option value="set">Set Fixed Price</Select.Option>
  <Select.Option value="adjust">Adjust Existing Prices</Select.Option>
</Select>
                </div>
                <div>
                  <label className="text-sm font-medium">
                    {updateType === "set" ? "Price Amount" : "Adjustment Value"}
                  </label>
                  <Input
                    type="number"
                    placeholder={updateType === "set" ? "Enter price" : "Enter adjustment"}
                    value={priceValue}
                    onChange={(e) => setPriceValue(e.target.value)}
                    step="1"
                    min={updateType === "set" ? "0" : undefined}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && isFormValid && !isUpdating) handleSubmit();
                      if (e.key === "Escape") onClose();
                    }}
                  />
                </div>
                {updateType === "adjust" && (
                  <div>
                    <label className="text-sm font-medium">Adjustment Type</label>
                    <Select value={adjustmentType} onChange={(v) => setAdjustmentType(v as "amount" | "percentage")} className="w-full">
  <Select.Option value="amount">Amount (₹)</Select.Option>
  <Select.Option value="percentage">Percentage (%)</Select.Option>
</Select>
                  </div>
                )}
              </div>
            </div>

            {/* ── 5. Availability ── */}
            <div>
              <label htmlFor="availability">Availability (optional)</label>
              <Input
                id="availability"
                type="number"
                placeholder="Leave empty to keep current"
                value={availability}
                onChange={(e) => setAvailability(e.target.value)}
                min="0"
              />
            </div>

            {/* ── 6. Restrictions ── */}
            <div>
              <label className="text-base font-medium">Restrictions (optional)</label>
              <div className="grid grid-cols-4 gap-4 mt-4">
                <div>
                  <label htmlFor="minLOS" className="text-sm font-medium">Min LOS (nights)</label>
                  <Input
                    id="minLOS"
                    type="number"
                    placeholder="No minimum"
                    value={restrictions.minLOS}
                    onChange={(e) => setRestrictions((p) => ({ ...p, minLOS: e.target.value }))}
                    min="1"
                  />
                </div>
                <div>
                  <label htmlFor="maxLOS" className="text-sm font-medium">Max LOS (nights)</label>
                  <Input
                    id="maxLOS"
                    type="number"
                    placeholder="No maximum"
                    value={restrictions.maxLOS}
                    onChange={(e) => setRestrictions((p) => ({ ...p, maxLOS: e.target.value }))}
                    min="1"
                  />
                </div>
                <div className="mt-2 ml-8">
                  <label className="text-sm font-medium invisible">Placeholder</label>
                  <div className="flex items-start space-x-2">
                    <Checkbox
                      id="closedToArrival"
                      checked={restrictions.closedToArrival}
                      onChange={(e) => setRestrictions((p) => ({ ...p, closedToArrival: e.target.checked }))}
                      className="mt-0.5"
                    />
                    <label htmlFor="closedToArrival" className="text-sm font-medium leading-4 cursor-pointer">
                      Closed to Arrival
                    </label>
                  </div>
                </div>
                <div className="mt-2">
                  <label className="text-sm font-medium invisible">Placeholder</label>
                  <div className="flex items-start space-x-2">
                    <Checkbox
                      id="closedToDeparture"
                      checked={restrictions.closedToDeparture}
                      onChange={(e) => setRestrictions((p) => ({ ...p, closedToDeparture: e.target.checked }))}
                      className="mt-0.5"
                    />
                    <label htmlFor="closedToDeparture" className="text-sm font-medium leading-4 cursor-pointer">
                      Closed to Departure
                    </label>
                  </div>
                </div>
              </div>
            </div>

            {/* ── 7. Price Impact ── */}
            <div>
              <label className="text-base font-medium">Price Impact</label>

              {/* Room-type filter tags */}
              {selectedRoomTypes.length > 0 && (
                <div className="flex flex-wrap gap-2 mt-3">
                  {selectedRoomTypes.map((id) => {
                    const rt = roomTypes.find((r) => r.roomTypeId === id);
                    const isActive = activeImpactFilter === id;
                    return (
                      <button
                        key={id}
                        type="button"
                        onClick={() => setActiveImpactFilter(isActive ? null : id)}
                        className={cn(
                          "inline-flex items-center gap-1 rounded-full px-3 py-0.5 text-xs font-medium border transition-colors",
                          isActive
                            ? "bg-blue-600 text-white border-blue-600"
                            : "bg-white dark:bg-gray-900 text-muted-foreground border-gray-300 dark:border-gray-600 hover:border-blue-400 hover:text-blue-600 dark:hover:text-blue-400"
                        )}
                      >
                        {rt?.roomTypeName ?? id}
                        {isActive && <X className="h-3 w-3 ml-0.5" />}
                      </button>
                    );
                  })}
                </div>
              )}

              {/* Table */}
              <div className="mt-3 border rounded-lg overflow-hidden">
                {!activeImpactFilter ? (
                  /* Overall: one row per room type */
                  <div className="max-h-52 overflow-y-auto">
                    <table className="w-full text-sm">
                      <thead className="sticky top-0 bg-gray-100 dark:bg-gray-800 z-10">
                        <tr>
                          <th className="text-left px-3 py-2 font-medium text-muted-foreground">Room Type</th>
                          <th className="text-right px-3 py-2 font-medium text-muted-foreground">Current (avg)</th>
                          <th className="text-right px-3 py-2 font-medium text-muted-foreground">New (avg)</th>
                          <th className="text-right px-3 py-2 font-medium text-muted-foreground">Change</th>
                          <th className="text-right px-3 py-2 font-medium text-muted-foreground">Dates</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
                        {overallTableRows.length === 0 ? (
                          <tr>
                            <td colSpan={5} className="px-3 py-4 text-center text-muted-foreground text-xs">
                              Select room types and enter a price to see impact.
                            </td>
                          </tr>
                        ) : overallTableRows.map((row) => (
                          <tr key={row.id} className="bg-white dark:bg-gray-900 hover:bg-gray-50 dark:hover:bg-gray-800/60">
                            <td className="px-3 py-2 font-medium">{row.name}</td>
                            <td className="px-3 py-2 text-right text-muted-foreground">
                              {row.current !== null ? formatInr(Math.round(row.current)) : "—"}
                            </td>
                            <td className="px-3 py-2 text-right font-medium">
                              {row.newPrice !== null ? formatInr(Math.round(row.newPrice)) : "—"}
                            </td>
                            <td className={cn(
                              "px-3 py-2 text-right font-medium",
                              row.diff === null || Math.round(row.diff) === 0
                                ? "text-muted-foreground"
                                : row.diff > 0
                                ? "text-green-600 dark:text-green-400"
                                : "text-red-600 dark:text-red-400"
                            )}>
                              {row.diff === null ? "—"
                                : Math.round(row.diff) === 0 ? "—"
                                : row.diff > 0
                                  ? `+${formatInr(Math.round(row.diff))}`
                                  : formatInr(Math.round(row.diff))}
                            </td>
                            <td className="px-3 py-2 text-right text-muted-foreground">{row.count || "—"}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                ) : (
                  /* Filtered: one row per date */
                  <div className="max-h-52 overflow-y-auto">
                    <table className="w-full text-sm">
                      <thead className="sticky top-0 bg-gray-100 dark:bg-gray-800 z-10">
                        <tr>
                          <th className="text-left px-3 py-2 font-medium text-muted-foreground">Date</th>
                          <th className="text-right px-3 py-2 font-medium text-muted-foreground">Current</th>
                          <th className="text-right px-3 py-2 font-medium text-muted-foreground">New</th>
                          <th className="text-right px-3 py-2 font-medium text-muted-foreground">Change</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
                        {filteredTableRows.length === 0 ? (
                          <tr>
                            <td colSpan={4} className="px-3 py-4 text-center text-muted-foreground text-xs">
                              No matching dates for the selected filters.
                            </td>
                          </tr>
                        ) : filteredTableRows.map((row) => (
                          <tr key={row.date} className="bg-white dark:bg-gray-900 hover:bg-gray-50 dark:hover:bg-gray-800/60">
                            <td className="px-3 py-2 font-medium">
                              {format(new Date(row.date + "T00:00:00"), "EEE, d MMM")}
                            </td>
                            <td className="px-3 py-2 text-right text-muted-foreground">{formatInr(row.current)}</td>
                            <td className="px-3 py-2 text-right font-medium">{formatInr(Math.round(row.newPrice))}</td>
                            <td className={cn(
                              "px-3 py-2 text-right font-medium",
                              Math.round(row.diff) === 0
                                ? "text-muted-foreground"
                                : row.diff > 0
                                ? "text-green-600 dark:text-green-400"
                                : "text-red-600 dark:text-red-400"
                            )}>
                              {Math.round(row.diff) === 0 ? "—"
                                : row.diff > 0
                                  ? `+${formatInr(Math.round(row.diff))}`
                                  : formatInr(Math.round(row.diff))}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
            </div>

            {/* ── 8. Preview — always visible ── */}
            <div className={cn(
              "border rounded-lg p-3 transition-colors",
              isFormValid && calculateUpdates.length > 0
                ? "bg-blue-50 dark:bg-blue-900/20 border-blue-200 dark:border-blue-800"
                : "bg-gray-50 dark:bg-gray-800/40 border-gray-200 dark:border-gray-700"
            )}>
              {isFormValid && calculateUpdates.length > 0 ? (
                <p className="text-sm text-blue-700 dark:text-blue-300">
                  <strong>Preview:</strong> This will update{" "}
                  <strong>{calculateUpdates.length}</strong> rate{calculateUpdates.length !== 1 ? "s" : ""} across{" "}
                  <strong>{selectedRoomTypes.length}</strong> room type{selectedRoomTypes.length !== 1 ? "s" : ""}.
                </p>
              ) : (
                <p className="text-sm text-muted-foreground">
                  {!selectedRoomTypes.length
                    ? "Select at least one room type to preview changes."
                    : !priceValue
                    ? "Enter a price value to preview changes."
                    : calculateUpdates.length === 0
                    ? "No matching dates found for the selected day filters."
                    : "Configure the options above to preview changes."}
                </p>
              )}
            </div>

          </div>

          
        
    </Modal>
  );
}

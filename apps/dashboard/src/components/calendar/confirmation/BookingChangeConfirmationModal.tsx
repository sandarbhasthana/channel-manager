"use client";

import React from "react";
import { DateTimeDiffText } from "./DateTimeDiffText";
import { TimezoneSwitcher } from "./TimezoneSwitcher";
import {
  buildBookingChangeFinalRows,
  buildBookingChangeStaticRows,
  formatBookingChangeInr,
  formatBookingChangeTzLocal,
  getBookingChangeEffectivePriceMode,
} from "./bookingChangeConfirmationMetadata";

interface BookingChangeConfirmationModalProps {
  type: "move" | "resize";
  label: string;
  oldStart: string;
  oldEnd: string;
  newStart: string;
  newEnd: string;
  oldRoomName?: string;
  newRoomName?: string;
  oldRoomCategory?: string;
  newRoomCategory?: string;
  priceChange?: {
    oldTotal: number;
    newTotal: number;
  } | null;
  rateLocked?: boolean;
  propertyTimezone?: string;
  onCancel: () => void;
  onConfirm: (values: { newStart: string; newEnd: string; priceMode: "keep_current" | "use_new" }) => void;
  zIndexClassName?: string;
}

export function BookingChangeConfirmationModal({
  type,
  label,
  oldStart,
  oldEnd,
  newStart,
  newEnd,
  oldRoomName,
  newRoomName,
  oldRoomCategory,
  newRoomCategory,
  priceChange,
  rateLocked = false,
  propertyTimezone = "UTC",
  onCancel,
  onConfirm,
  zIndexClassName = "z-50",
}: BookingChangeConfirmationModalProps) {
  const [timezone, setTimezone] = React.useState(
    () => Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
  );
  const [step, setStep] = React.useState<"review" | "transitioning" | "confirm">("review");
  const [priceMode, setPriceMode] = React.useState<"keep_current" | "use_new">("use_new");
  const effectivePriceMode = getBookingChangeEffectivePriceMode(rateLocked, priceMode);

  React.useEffect(() => {
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => { document.body.style.overflow = prev; };
  }, []);

  React.useEffect(() => {
    if (step !== "transitioning") return;
    const timer = window.setTimeout(() => setStep("confirm"), 1200);
    return () => window.clearTimeout(timer);
  }, [step]);

  const rowInput = {
    oldStart,
    oldEnd,
    newStart,
    newEnd,
    oldRoomName,
    newRoomName,
    oldRoomCategory,
    newRoomCategory,
  };
  const staticRows = buildBookingChangeStaticRows(rowInput);

  const startChanged = oldStart !== newStart;
  const endChanged = oldEnd !== newEnd;
  const changedRows = buildBookingChangeFinalRows(rowInput, timezone);

  return (
    <div
      className={`fixed inset-0 ${zIndexClassName} flex items-center justify-center bg-black/40 backdrop-blur-[1px] transition-opacity duration-200 ${step === "transitioning" ? "pointer-events-none opacity-0" : "opacity-100"}`}
    >
      {step === "review" ? (
        <div className="w-[36rem] max-w-[90vw] rounded-lg bg-white p-6 shadow-2xl dark:bg-slate-900">
          <div className="mb-5 flex items-start justify-between gap-4">
            <div className="min-w-0">
              <h3 className="mb-1 text-lg font-semibold text-slate-900 dark:text-white">
                {type === "move" ? "Move reservation" : "Resize reservation"}
              </h3>
              <p className="truncate text-sm text-slate-500 dark:text-slate-400">{label}</p>
            </div>
            <TimezoneSwitcher
              value={timezone}
              onChange={setTimezone}
              propertyTimezone={propertyTimezone}
              className="mt-0.5"
            />
          </div>

          {/* 3-column table: label | Was | Now */}
          <div className="mb-6 flex justify-center text-sm">
            <div
              style={{ display: "grid", gridTemplateColumns: "auto auto auto", columnGap: 20, rowGap: 7 }}
            >
              {/* Header */}
              <div />
              <div className="pb-1 text-center text-xs font-semibold uppercase tracking-wide text-slate-400">Was</div>
              <div className="pb-1 text-center text-xs font-semibold uppercase tracking-wide text-slate-400">Now</div>

              {staticRows.map((row) => (
                <React.Fragment key={row.label}>
                  <div className="flex items-center pr-4 text-[10px] font-semibold uppercase tracking-widest text-slate-400 dark:text-slate-500">
                    {row.label}
                  </div>
                  <div
                    className={[
                      "flex items-center justify-center min-w-0",
                      row.changed ? "text-slate-500 dark:text-slate-400" : "text-slate-300 dark:text-slate-600",
                    ].join(" ")}
                  >
                    {row.oldVal}
                  </div>
                  <div
                    className={[
                      "flex items-center justify-center min-w-0",
                      row.changed
                        ? "font-semibold text-purple-700 dark:text-purple-400"
                        : "text-slate-300 dark:text-slate-600",
                    ].join(" ")}
                  >
                    {row.newVal}
                  </div>
                </React.Fragment>
              ))}

              {/* Check-in row */}
              <div className="flex items-center pr-4 text-[10px] font-semibold uppercase tracking-widest text-slate-400 dark:text-slate-500">
                Check-in
              </div>
              <div
                className={[
                  "flex items-center justify-center min-w-0",
                  startChanged ? "text-slate-500 dark:text-slate-400" : "text-slate-300 dark:text-slate-600",
                ].join(" ")}
              >
                <DateTimeDiffText value={oldStart} compareTo={newStart} timezone={timezone} />
              </div>
              <div className="flex items-center justify-center min-w-0 font-semibold text-purple-700 dark:text-purple-400">
                {formatBookingChangeTzLocal(newStart, timezone)}
              </div>

              {/* Check-out row */}
              <div className="flex items-center pr-4 text-[10px] font-semibold uppercase tracking-widest text-slate-400 dark:text-slate-500">
                Check-out
              </div>
              <div
                className={[
                  "flex items-center justify-center min-w-0",
                  endChanged ? "text-slate-500 dark:text-slate-400" : "text-slate-300 dark:text-slate-600",
                ].join(" ")}
              >
                <DateTimeDiffText value={oldEnd} compareTo={newEnd} timezone={timezone} />
              </div>
              <div className="flex items-center justify-center min-w-0 font-semibold text-purple-700 dark:text-purple-400">
                {formatBookingChangeTzLocal(newEnd, timezone)}
              </div>
            </div>
          </div>

          <div className="flex justify-end gap-3">
            <button
              type="button"
              onClick={onCancel}
              className="rounded-lg border border-slate-200 px-4 py-2 text-sm text-slate-700 transition-colors hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={() => setStep("transitioning")}
              className="rounded-lg bg-purple-600 px-4 py-2 text-sm text-white transition-colors hover:bg-purple-700"
            >
              Review
            </button>
          </div>
        </div>
      ) : step === "confirm" ? (
        /* ── Step 2: final double-confirmation ── */
        <div className="w-[30rem] max-w-[90vw] rounded-lg bg-white p-6 shadow-2xl dark:bg-slate-900">
          <div className="mb-4">
            <h3 className="mb-1 text-lg font-semibold text-slate-900 dark:text-white">
              Confirm reservation changes?
            </h3>
            <p className="truncate text-sm text-slate-500 dark:text-slate-400">{label}</p>
          </div>

          {changedRows.length === 0 ? (
            <p className="mb-6 text-sm text-slate-400">No values were changed.</p>
          ) : (
            <div className="mb-6 flex justify-center rounded-lg bg-slate-50 p-4 dark:bg-slate-800">
              <div className="space-y-3">
                {changedRows.map((row) => (
                  <div key={row.label} className="flex items-start gap-3 text-sm">
                    <span className="w-20 shrink-0 text-[10px] font-semibold uppercase tracking-widest text-slate-400 dark:text-slate-500 pt-0.5">
                      {row.label}
                    </span>
                    <span className="font-semibold text-purple-700 dark:text-purple-400">
                      {row.newVal}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {rateLocked ? (
            <div className="mb-6 rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm dark:border-amber-800 dark:bg-amber-950/30">
              <div className="font-semibold text-slate-900 dark:text-white">Rate locked</div>
              <div className="mt-1 text-slate-600 dark:text-slate-300">
                Existing pricing will be preserved for this change.
              </div>
            </div>
          ) : priceChange && (
            <div className="mb-6 rounded-lg border border-blue-200 bg-blue-50 p-4 dark:border-blue-800 dark:bg-blue-950/30">
              <div className="mb-3 flex items-center justify-between gap-3 text-sm">
                <span className="font-semibold text-slate-900 dark:text-white">Price changed</span>
                <span className="text-slate-500 dark:text-slate-400">
                  {formatBookingChangeInr(priceChange.oldTotal)} to {formatBookingChangeInr(priceChange.newTotal)}
                </span>
              </div>
              <div className="grid gap-2 sm:grid-cols-2">
                <button
                  type="button"
                  onClick={() => setPriceMode("keep_current")}
                  className={[
                    "rounded-md border px-3 py-2 text-left text-sm transition-colors",
                    priceMode === "keep_current"
                      ? "border-blue-500 bg-white text-blue-700 shadow-sm dark:bg-slate-900 dark:text-blue-300"
                      : "border-slate-200 bg-white/60 text-slate-600 hover:bg-white dark:border-slate-700 dark:bg-slate-900/40 dark:text-slate-300",
                  ].join(" ")}
                >
                  <span className="block font-medium">Carry old price</span>
                  <span className="block text-xs opacity-75">{formatBookingChangeInr(priceChange.oldTotal)}</span>
                </button>
                <button
                  type="button"
                  onClick={() => setPriceMode("use_new")}
                  className={[
                    "rounded-md border px-3 py-2 text-left text-sm transition-colors",
                    priceMode === "use_new"
                      ? "border-blue-500 bg-white text-blue-700 shadow-sm dark:bg-slate-900 dark:text-blue-300"
                      : "border-slate-200 bg-white/60 text-slate-600 hover:bg-white dark:border-slate-700 dark:bg-slate-900/40 dark:text-slate-300",
                  ].join(" ")}
                >
                  <span className="block font-medium">Use new price</span>
                  <span className="block text-xs opacity-75">{formatBookingChangeInr(priceChange.newTotal)}</span>
                </button>
              </div>
            </div>
          )}

          <div className="flex justify-end gap-3">
            <button
              type="button"
              onClick={onCancel}
              className="rounded-lg border border-slate-200 px-4 py-2 text-sm text-slate-700 transition-colors hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={() => onConfirm({ newStart, newEnd, priceMode: effectivePriceMode })}
              className="rounded-lg bg-purple-600 px-4 py-2 text-sm text-white transition-colors hover:bg-purple-700"
            >
              Confirm
            </button>
          </div>
        </div>
      ) : (
        <div aria-hidden="true" />
      )}
    </div>
  );
}

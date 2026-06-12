"use client";

import React, { useCallback, useMemo, useState } from "react";

const MAX_SAMPLES = 120;
const PANEL_KEY = "calendar-carousel-debug-panel-open";
const GRAPH_KEY = "calendar-carousel-debug-graph-visible";
const LOGS_KEY = "calendar-carousel-debug-logs-enabled";
const SCALE_KEY = "calendar-carousel-debug-graph-scale";

export type CalendarCarouselDebugPhase =
  | "idle"
  | "touch-drag"
  | "touch-glide"
  | "wheel-drag"
  | "wheel-glide"
  | "snap"
  | "imperative";

export interface CalendarCarouselDebugSample {
  velocity: number;
  momentum: number;
  carry: number;
  phase: CalendarCarouselDebugPhase;
  timestamp: number;
}

export interface CalendarCarouselDebugTierThresholds {
  gentleMax: number;
  fastMin: number;
}

export interface CalendarCarouselDebugApi {
  enabled: boolean;
  log: (message: string, details?: Record<string, unknown>) => void;
  pushSample: (sample: Omit<CalendarCarouselDebugSample, "timestamp">) => void;
  clear: () => void;
}

interface OverlayProps {
  enabled: boolean;
  panelOpen: boolean;
  graphVisible: boolean;
  logsEnabled: boolean;
  graphScale: number;
  samples: CalendarCarouselDebugSample[];
  thresholds: CalendarCarouselDebugTierThresholds;
  onTogglePanel: () => void;
  onToggleGraph: () => void;
  onToggleLogs: () => void;
  onScaleChange: (value: number) => void;
  onClear: () => void;
}

interface LegendItem {
  label: string;
  value: string;
  color: string;
}

function getStoredBoolean(key: string, fallback: boolean): boolean {
  if (typeof window === "undefined") return fallback;
  try {
    const value = window.sessionStorage.getItem(key);
    if (value === null) return fallback;
    return value === "1";
  } catch {
    return fallback;
  }
}

function getStoredScale(): number {
  if (typeof window === "undefined") return 1;
  try {
    const raw = window.sessionStorage.getItem(SCALE_KEY);
    if (!raw) return 1;
    const parsed = Number(raw);
    if (!Number.isFinite(parsed)) return 1;
    return Math.min(3, Math.max(0.5, parsed));
  } catch {
    return 1;
  }
}

function storeValue(key: string, value: string) {
  try {
    window.sessionStorage.setItem(key, value);
  } catch {
    // Ignore session storage failures for debug-only controls.
  }
}

function formatValue(value: number): string {
  return `${value.toFixed(3)} px/ms`;
}

function phaseColor(phase: CalendarCarouselDebugPhase): string {
  switch (phase) {
    case "touch-drag":
      return "#0f766e";
    case "touch-glide":
      return "#0ea5e9";
    case "wheel-drag":
      return "#7c3aed";
    case "wheel-glide":
      return "#f97316";
    case "snap":
      return "#dc2626";
    case "imperative":
      return "#4f46e5";
    default:
      return "#64748b";
  }
}

function buildGraphPath(samples: CalendarCarouselDebugSample[], maxMomentum: number, width: number, height: number): string {
  return samples
    .map((sample, index) => {
      const x = samples.length === 1 ? 0 : (index / (samples.length - 1)) * width;
      const y = height - (sample.momentum / maxMomentum) * height;
      return `${index === 0 ? "M" : "L"} ${x.toFixed(2)} ${y.toFixed(2)}`;
    })
    .join(" ");
}

function tierLineY(threshold: number, maxMomentum: number, height: number): number {
  const normalized = Math.max(0, Math.min(1, threshold / maxMomentum));
  return height - normalized * height;
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function DebugToggle({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={[
        "rounded border px-2 py-1 text-[11px] font-medium transition-colors",
        active
          ? "border-amber-400 bg-amber-200/70 text-slate-900 dark:border-amber-500 dark:bg-amber-500/20 dark:text-amber-100"
          : "border-slate-300 bg-white/70 text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-900/70 dark:text-slate-300 dark:hover:bg-slate-800",
      ].join(" ")}
    >
      {label}
    </button>
  );
}

function VerticalScaleSlider({
  value,
  onChange,
}: {
  value: number;
  onChange: (value: number) => void;
}) {
  return (
    <div className="flex items-center gap-2">
      <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500 dark:text-slate-400">
        Scale
      </div>
      <div className="flex h-28 w-8 items-center justify-center overflow-visible">
        <input
          type="range"
          min="0.5"
          max="3"
          step="0.1"
          value={value}
          onChange={(event) => onChange(Number(event.target.value))}
          aria-label="Adjust momentum graph scale"
          className="cursor-pointer accent-amber-500"
          style={{
            width: 96,
            transform: "rotate(-90deg)",
            transformOrigin: "center",
          }}
        />
      </div>
      <div className="text-[10px] tabular-nums text-slate-500 dark:text-slate-400">
        {value.toFixed(1)}x
      </div>
    </div>
  );
}

function CalendarCarouselDebugOverlay({
  enabled,
  panelOpen,
  graphVisible,
  logsEnabled,
  graphScale,
  samples,
  thresholds,
  onTogglePanel,
  onToggleGraph,
  onToggleLogs,
  onScaleChange,
  onClear,
}: OverlayProps) {
  const visibleSamples = 36;
  const [dragOffset, setDragOffset] = useState(0);
  const maxPanOffset = Math.max(0, samples.length - visibleSamples);
  const clampedDragOffset = clamp(dragOffset, 0, maxPanOffset);
  const windowStart = Math.max(0, samples.length - visibleSamples - clampedDragOffset);
  const visibleGraphSamples = samples.slice(windowStart, windowStart + visibleSamples);
  const handleGraphDragStart = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
    const startX = event.clientX;
    const startOffset = clampedDragOffset;
    event.currentTarget.setPointerCapture(event.pointerId);

    const handlePointerMove = (moveEvent: PointerEvent) => {
      const deltaX = moveEvent.clientX - startX;
      const sampleDelta = Math.round(deltaX / 18);
      setDragOffset(clamp(startOffset - sampleDelta, 0, maxPanOffset));
    };

    const handlePointerUp = (upEvent: PointerEvent) => {
      if (event.currentTarget.hasPointerCapture(upEvent.pointerId)) {
        event.currentTarget.releasePointerCapture(upEvent.pointerId);
      }
      event.currentTarget.removeEventListener("pointermove", handlePointerMove);
      event.currentTarget.removeEventListener("pointerup", handlePointerUp);
      event.currentTarget.removeEventListener("pointercancel", handlePointerUp);
    };

    event.currentTarget.addEventListener("pointermove", handlePointerMove);
    event.currentTarget.addEventListener("pointerup", handlePointerUp);
    event.currentTarget.addEventListener("pointercancel", handlePointerUp);
  }, [clampedDragOffset, maxPanOffset]);

  if (!enabled) return null;

  if (!panelOpen) {
    return (
      <div className="sticky top-0 z-40 flex justify-end px-3 pt-2">
        <button
          type="button"
          onClick={onTogglePanel}
          className="rounded border border-amber-300 bg-amber-50/95 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.18em] text-amber-800 shadow-sm backdrop-blur transition-colors hover:bg-amber-100 dark:border-amber-800 dark:bg-slate-950/95 dark:text-amber-200 dark:hover:bg-slate-900"
        >
          Show Debug
        </button>
      </div>
    );
  }

  const width = 640;
  const height = 112;
  const latest = samples[samples.length - 1];
  const rawMaxMomentum = Math.max(
    thresholds.fastMin * 1.2,
    0.05,
    ...visibleGraphSamples.map((sample) => sample.momentum),
  );
  const maxMomentum = Math.max(0.05, rawMaxMomentum / graphScale);
  const lowThresholdY = tierLineY(thresholds.gentleMax, maxMomentum, height);
  const highThresholdY = tierLineY(thresholds.fastMin, maxMomentum, height);
  const path = visibleGraphSamples.length > 1
    ? buildGraphPath(visibleGraphSamples, maxMomentum, width, height)
    : "";
  const frictionLegend: LegendItem[] = [
    { label: "FRICTION_HIGH", value: `>= ${formatValue(thresholds.fastMin)}`, color: "#16a34a" },
    { label: "FRICTION_MED", value: `${formatValue(thresholds.gentleMax)} to ${formatValue(thresholds.fastMin)}`, color: "#f59e0b" },
    { label: "FRICTION_LOW", value: `< ${formatValue(thresholds.gentleMax)}`, color: "#dc2626" },
  ];
  const phaseLegend: LegendItem[] = [
    { label: "Touch Drag", value: "", color: phaseColor("touch-drag") },
    { label: "Touch Glide", value: "", color: phaseColor("touch-glide") },
    { label: "Wheel Drag", value: "", color: phaseColor("wheel-drag") },
    { label: "Wheel Glide", value: "", color: phaseColor("wheel-glide") },
    { label: "Snap", value: "", color: phaseColor("snap") },
    { label: "Imperative", value: "", color: phaseColor("imperative") },
  ];

  return (
    <div className="sticky top-0 z-40 border-b border-amber-200 bg-amber-50/95 px-3 py-2 text-[11px] text-slate-800 shadow-sm backdrop-blur dark:border-amber-900 dark:bg-slate-950/95 dark:text-slate-100">
      <div className="mb-2 flex items-start justify-between gap-3">
        <div>
          <div className="font-semibold uppercase tracking-[0.18em] text-amber-700 dark:text-amber-300">
            Carousel Debug
          </div>
          <div className="text-slate-600 dark:text-slate-300">
            {latest
              ? `momentum ${formatValue(latest.momentum)} • velocity ${formatValue(latest.velocity)} • carry ${formatValue(latest.carry)} • ${latest.phase}`
              : "Waiting for momentum samples"}
          </div>
        </div>
        <button
          type="button"
          onClick={onTogglePanel}
          className="rounded border border-amber-300 px-2 py-0.5 text-xs font-medium text-slate-700 transition-colors hover:bg-amber-100 dark:border-amber-800 dark:text-slate-200 dark:hover:bg-slate-900"
          aria-label="Hide carousel debug panel"
        >
          x
        </button>
      </div>

      <div className="mb-2 flex flex-wrap items-center gap-2">
        <DebugToggle label={graphVisible ? "Graph On" : "Graph Off"} active={graphVisible} onClick={onToggleGraph} />
        <DebugToggle label={logsEnabled ? "Logs On" : "Logs Off"} active={logsEnabled} onClick={onToggleLogs} />
        <button
          type="button"
          onClick={onClear}
          className="rounded border border-slate-300 bg-white/70 px-2 py-1 text-[11px] font-medium text-slate-700 transition-colors hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-900/70 dark:text-slate-300 dark:hover:bg-slate-800"
        >
          Clear
        </button>
        <div className="text-[10px] uppercase tracking-[0.18em] text-slate-500 dark:text-slate-400">
          Drag graph horizontally to inspect older samples
        </div>
      </div>

      {graphVisible && (
        <div className="flex items-stretch gap-3 overflow-hidden rounded border border-amber-200 bg-white/80 p-2 dark:border-slate-800 dark:bg-slate-900/80">
          <VerticalScaleSlider value={graphScale} onChange={onScaleChange} />
          <div
            className="min-w-0 flex-1 touch-none cursor-grab active:cursor-grabbing"
            onPointerDown={handleGraphDragStart}
          >
            <svg viewBox={`0 0 ${width} ${height}`} className="block h-28 w-full">
              <line x1="0" y1={height} x2={width} y2={height} stroke="#cbd5e1" strokeWidth="1" />
              <line x1="0" y1={lowThresholdY} x2={width} y2={lowThresholdY} stroke="#dc2626" strokeWidth="1" strokeDasharray="6 4" />
              <line x1="0" y1={highThresholdY} x2={width} y2={highThresholdY} stroke="#16a34a" strokeWidth="1" strokeDasharray="6 4" />
              <text x="8" y={Math.max(12, highThresholdY - 4)} fill="#166534" fontSize="10" fontWeight="600">FRICTION_HIGH</text>
              <text x="8" y={Math.max(lowThresholdY - 4, highThresholdY + 14)} fill="#a16207" fontSize="10" fontWeight="600">FRICTION_MED</text>
              <text x="8" y={Math.min(height - 6, lowThresholdY + 14)} fill="#b91c1c" fontSize="10" fontWeight="600">FRICTION_LOW</text>
              {visibleGraphSamples.slice(1).map((sample, index) => {
                const prev = visibleGraphSamples[index];
                const x1 = (index / Math.max(visibleGraphSamples.length - 1, 1)) * width;
                const x2 = ((index + 1) / Math.max(visibleGraphSamples.length - 1, 1)) * width;
                const y1 = height - (prev.momentum / maxMomentum) * height;
                const y2 = height - (sample.momentum / maxMomentum) * height;
                return (
                  <line
                    key={`${sample.timestamp}-${index}`}
                    x1={x1}
                    y1={y1}
                    x2={x2}
                    y2={y2}
                    stroke={phaseColor(sample.phase)}
                    strokeWidth="2"
                    strokeLinecap="round"
                  />
                );
              })}
              {path && <path d={path} fill="none" stroke="transparent" />}
            </svg>
          </div>
          <div className="w-44 shrink-0 border-l border-amber-200 pl-3 dark:border-slate-800">
            <div className="mb-2 text-[10px] font-semibold uppercase tracking-[0.18em] text-slate-500 dark:text-slate-400">
              Friction Legend
            </div>
            <div className="space-y-2">
              {frictionLegend.map((item) => (
                <div key={item.label} className="flex items-start gap-2">
                  <span className="mt-1 h-2.5 w-2.5 rounded-full" style={{ backgroundColor: item.color }} />
                  <div className="min-w-0">
                    <div className="font-medium text-slate-700 dark:text-slate-200">{item.label}</div>
                    <div className="text-[10px] text-slate-500 dark:text-slate-400">{item.value}</div>
                  </div>
                </div>
              ))}
            </div>
            <div className="mb-2 mt-4 text-[10px] font-semibold uppercase tracking-[0.18em] text-slate-500 dark:text-slate-400">
              Phase Colors
            </div>
            <div className="space-y-2">
              {phaseLegend.map((item) => (
                <div key={item.label} className="flex items-center gap-2">
                  <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: item.color }} />
                  <div className="text-[10px] text-slate-600 dark:text-slate-300">{item.label}</div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export function useCalendarCarouselDebug(thresholds: CalendarCarouselDebugTierThresholds) {
  const [panelOpen, setPanelOpen] = useState(() => getStoredBoolean(PANEL_KEY, true));
  const [graphVisible, setGraphVisible] = useState(() => getStoredBoolean(GRAPH_KEY, true));
  const [logsEnabled, setLogsEnabled] = useState(() => getStoredBoolean(LOGS_KEY, true));
  const [graphScale, setGraphScale] = useState(getStoredScale);
  const [samples, setSamples] = useState<CalendarCarouselDebugSample[]>([]);

  const enabled = process.env.NODE_ENV !== "production";

  const log = useCallback((message: string, details?: Record<string, unknown>) => {
    if (!enabled || !logsEnabled) return;
    if (details) {
      console.debug(`[calendar-carousel] ${message}`, details);
      return;
    }
    console.debug(`[calendar-carousel] ${message}`);
  }, [enabled, logsEnabled]);

  const pushSample = useCallback((sample: Omit<CalendarCarouselDebugSample, "timestamp">) => {
    if (!enabled) return;

    const nextSample: CalendarCarouselDebugSample = {
      velocity: Math.abs(sample.velocity),
      momentum: Math.abs(sample.momentum),
      carry: Math.abs(sample.carry),
      phase: sample.phase,
      timestamp: Date.now(),
    };

    setSamples((prev) => [...prev.slice(-(MAX_SAMPLES - 1)), nextSample]);
  }, [enabled]);

  const clear = useCallback(() => {
    setSamples([]);
  }, []);

  const togglePanel = useCallback(() => {
    setPanelOpen((prev) => {
      const next = !prev;
      storeValue(PANEL_KEY, next ? "1" : "0");
      return next;
    });
  }, []);

  const toggleGraph = useCallback(() => {
    setGraphVisible((prev) => {
      const next = !prev;
      storeValue(GRAPH_KEY, next ? "1" : "0");
      return next;
    });
  }, []);

  const toggleLogs = useCallback(() => {
    setLogsEnabled((prev) => {
      const next = !prev;
      storeValue(LOGS_KEY, next ? "1" : "0");
      return next;
    });
  }, []);

  const handleScaleChange = useCallback((value: number) => {
    const next = Math.min(3, Math.max(0.5, value));
    setGraphScale(next);
    storeValue(SCALE_KEY, next.toString());
  }, []);

  const DebugOverlay = useMemo(() => {
    return function BoundDebugOverlay() {
      return (
        <CalendarCarouselDebugOverlay
          enabled={enabled}
          panelOpen={panelOpen}
          graphVisible={graphVisible}
          logsEnabled={logsEnabled}
          graphScale={graphScale}
          samples={samples}
          thresholds={thresholds}
          onTogglePanel={togglePanel}
          onToggleGraph={toggleGraph}
          onToggleLogs={toggleLogs}
          onScaleChange={handleScaleChange}
          onClear={clear}
        />
      );
    };
  }, [
    clear,
    enabled,
    graphScale,
    graphVisible,
    handleScaleChange,
    logsEnabled,
    panelOpen,
    samples,
    thresholds,
    toggleGraph,
    toggleLogs,
    togglePanel,
  ]);

  const api = useMemo<CalendarCarouselDebugApi>(() => ({
    enabled,
    log,
    pushSample,
    clear,
  }), [clear, enabled, log, pushSample]);

  return { api, DebugOverlay };
}

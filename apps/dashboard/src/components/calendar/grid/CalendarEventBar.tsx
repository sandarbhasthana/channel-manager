"use client";

import React, { useState, useRef, useEffect } from "react";

// Inject shimmer keyframe once into the document head
let _shimmerInjected = false;
function ensureShimmerKeyframe() {
  if (_shimmerInjected || typeof document === "undefined") return;
  _shimmerInjected = true;
  const s = document.createElement("style");
  s.textContent =
    "@keyframes cal-bar-shimmer{0%{transform:translateX(-100%)}100%{transform:translateX(250%)}}";
  document.head.appendChild(s);
}
import {
  Baby,
  Building2,
  Check,
  DollarSign,
  Globe,
  MessageCircle,
  Minus,
  PersonStanding,
  Phone,
  Users,
  X,
} from "lucide-react";
import type { PositionedEvent } from "../layout/useEventPositioning";
import { showTimeTravelToast, ensureShakeKeyframe, triggerBarShake } from "../timeTravelFeedback";

import { ROW_HEIGHT, BAR_TOP, BAR_HEIGHT } from "../calendarConfig";
import { CALENDAR_COLORS } from "../calendarTheme";
import { calculateResizedEventRange } from "./calendarEventResize";
import { isCalendarEditLockedStatus } from "@/lib/reservations/calendar-move-validation";
const HANDLE_WIDTH = 8;
const BAR_GAP = 4;
const LONG_PRESS_MS = 400;

const SOURCE_ICONS: Record<string, React.ElementType> = {
  WEBSITE: Globe,
  PHONE: Phone,
  WALK_IN: PersonStanding,
  CHANNEL: Building2,
};

/** Darken a hex color by `factor` (0–1) to produce a solid softer shade. */
function bottomShade(hex: string, factor = 0.12): string {
  if (!hex.startsWith("#") || hex.length < 7) return hex;
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  return `rgb(${Math.round(r * (1 - factor))},${Math.round(g * (1 - factor))},${Math.round(b * (1 - factor))})`;
}

const PAYMENT_STATUS_COLORS = CALENDAR_COLORS.payment;

const PAYMENT_STATUS_LABELS: Record<string, string> = {
  PAID: "paid",
  PARTIALLY_PAID: "partially paid",
  UNPAID: "unpaid",
  REFUND_DUE: "refund due",
};

const BOTTOM_ICON_SIZE = 12;
const BOTTOM_STATUS_ICON_SIZE = 9;

/** Dollar icon followed by a separate compact status marker. */
function PaymentStatusIcon({
  status,
  textColor,
}: {
  status: string;
  textColor: string;
}) {
  const StatusIcon =
    status === "PAID" ? Check :
    status === "UNPAID" ? X :
    Minus;
  const statusColor = PAYMENT_STATUS_COLORS[status] ?? PAYMENT_STATUS_COLORS.PARTIALLY_PAID;
  const statusLabel = PAYMENT_STATUS_LABELS[status] ?? "payment status";

  return (
    <span
      className="flex items-center gap-0.5 shrink-0"
      aria-label={`Payment ${statusLabel}`}
      title={`Payment ${statusLabel}`}
    >
      <DollarSign size={BOTTOM_ICON_SIZE} strokeWidth={2.6} style={{ color: textColor, flexShrink: 0 }} />
      <span
        className="flex items-center justify-center rounded-full"
        style={{
          width: 10,
          height: 10,
          backgroundColor: statusColor,
          color: "#ffffff",
          flexShrink: 0,
        }}
        aria-hidden="true"
      >
        <StatusIcon size={BOTTOM_STATUS_ICON_SIZE} strokeWidth={3} />
      </span>
    </span>
  );
}

interface CalendarEventBarProps {
  event: PositionedEvent;
  cellWidth: number;
  onEventClick?: (event: PositionedEvent, x: number, y: number) => void;
  onBlockClick?: (event: PositionedEvent, x: number, y: number) => void;
  onEventResized?: (event: PositionedEvent, newStart: string, newEnd: string) => void;
  /** Called when long-press activates a cross-room move drag. */
  onEventMoveStart?: (
    event: PositionedEvent,
    pointerId: number,
    clientX: number,
    clientY: number,
    barClientLeft: number,
  ) => void;
  /** Set to true by CalendarDateGrid while THIS event is the one being moved. */
  isDraggingAsMove?: boolean;
  /** True while the API call for this event's move/resize is in flight. */
  isPending?: boolean;
  /** Today's date string (YYYY-MM-DD) for clamping left-resize to prevent past dates. */
  todayStr?: string;
}

export const CalendarEventBar = React.memo(function CalendarEventBar({
  event,
  cellWidth,
  onEventClick,
  onBlockClick,
  onEventResized,
  onEventMoveStart,
  isDraggingAsMove = false,
  isPending = false,
  todayStr,
}: CalendarEventBarProps) {
  // ── Resize drag state ─────────────────────────────────────────────────────
  const [dragDeltaDays, setDragDeltaDays] = useState(0);
  const [dragType, setDragType] = useState<"resize-left" | "resize-right" | null>(null);
  const dragStartXRef = useRef(0);
  const dragPointerIdRef = useRef<number | null>(null);
  const didDragRef = useRef(false);

  // ── Long-press move detection ─────────────────────────────────────────────
  const [isLongPressing, setIsLongPressing] = useState(false);
  const hitAreaRef = useRef<HTMLDivElement>(null);
  const visibleBarRef = useRef<HTMLDivElement>(null);
  const longPressTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const longPressInfoRef = useRef<{ x: number; y: number; pointerId: number } | null>(null);

  useEffect(() => {
    if (isPending) ensureShimmerKeyframe();
    ensureShakeKeyframe();
    return () => {
      if (longPressTimerRef.current) clearTimeout(longPressTimerRef.current);
    };
  }, [isPending]);

  if (event.widthPx <= 0) return null;

  const isBlock = event.isBlock;
  const isCalendarEditable = !isBlock && !isCalendarEditLockedStatus(event.status);

  const borderRadius = [
    event.clampedLeft ? "0" : "4px",
    event.clampedRight ? "0" : "4px",
  ];
  const borderRadiusCSS = `${borderRadius[0]} ${borderRadius[1]} ${borderRadius[1]} ${borderRadius[0]}`;

  // ── Live display geometry during resize ───────────────────────────────────
  let displayLeftPx = event.leftPx;
  let displayWidthPx = event.widthPx;

  if (dragType && dragDeltaDays !== 0) {
    const delta = dragDeltaDays * cellWidth;
    if (dragType === "resize-right") {
      displayWidthPx = Math.max(cellWidth, event.widthPx + delta);
    } else {
      const maxShrink = event.widthPx - cellWidth;
      let clampedDelta = Math.max(-maxShrink, Math.min(delta, event.widthPx - cellWidth));
      // Prevent the bar from visually extending before today
      if (todayStr) {
        const daysToToday = Math.round(
          (new Date(todayStr).getTime() - new Date(event.start).getTime()) / 86400000
        );
        clampedDelta = Math.max(clampedDelta, daysToToday * cellWidth);
      }
      displayLeftPx = event.leftPx + clampedDelta;
      displayWidthPx = Math.max(cellWidth, event.widthPx - clampedDelta);
    }
  }

  // ── Resize drag handlers ──────────────────────────────────────────────────
  const startResizeDrag = (
    e: React.PointerEvent<HTMLDivElement>,
    type: "resize-left" | "resize-right"
  ) => {
    e.preventDefault();
    e.stopPropagation();
    if (!isCalendarEditable) return;

    const startX = e.clientX;
    const pointerId = e.pointerId;
    dragStartXRef.current = startX;
    dragPointerIdRef.current = pointerId;
    didDragRef.current = false;
    setDragType(type);
    setDragDeltaDays(0);

    let wasAtPastBoundary = false;

    const onMove = (ev: PointerEvent) => {
      if (ev.pointerId !== pointerId) return;
      const pixelDelta = ev.clientX - startX;
      if (Math.abs(pixelDelta) > 3) didDragRef.current = true;
      setDragDeltaDays(Math.round(pixelDelta / cellWidth));

      if (type === "resize-left" && todayStr) {
        const d = new Date(event.start);
        d.setDate(d.getDate() + Math.round(pixelDelta / cellWidth));
        const atPastBoundary = d.toISOString().slice(0, 10) < todayStr;
        if (atPastBoundary && !wasAtPastBoundary) {
          wasAtPastBoundary = true;
          if (visibleBarRef.current) triggerBarShake(visibleBarRef.current);
          showTimeTravelToast();
        } else if (!atPastBoundary) {
          wasAtPastBoundary = false;
        }
      }
    };

    const onUp = (ev: PointerEvent) => {
      if (ev.pointerId !== pointerId) return;
      const daysDelta = Math.round((ev.clientX - startX) / cellWidth);
      dragPointerIdRef.current = null;
      setDragType(null);
      setDragDeltaDays(0);

      document.removeEventListener("pointermove", onMove);
      document.removeEventListener("pointerup", onUp);
      document.removeEventListener("pointercancel", onUp);

      if (!didDragRef.current || daysDelta === 0 || !onEventResized) return;

      const resizedRange = calculateResizedEventRange({
        start: event.start,
        end: event.end,
        daysDelta,
        type,
        todayStr,
      });
      if (!resizedRange) return;

      onEventResized(event, resizedRange.newStart, resizedRange.newEnd);
    };

    document.addEventListener("pointermove", onMove);
    document.addEventListener("pointerup", onUp);
    document.addEventListener("pointercancel", onUp);
  };

  // ── Long-press detection ──────────────────────────────────────────────────
  const cancelLongPress = () => {
    if (longPressTimerRef.current) {
      clearTimeout(longPressTimerRef.current);
      longPressTimerRef.current = null;
    }
    longPressInfoRef.current = null;
    setIsLongPressing(false);
  };

  const handleHitAreaPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    // Always stop mouse events from bubbling to room-row drag-to-select
    if (e.pointerType === "mouse") e.stopPropagation();
    // Don't start long press during resize or while already being moved
    if (!isCalendarEditable) return;
    if (isDraggingAsMove || dragType || !onEventMoveStart) return;
    if (e.button !== 0 && e.pointerType === "mouse") return;

    longPressInfoRef.current = { x: e.clientX, y: e.clientY, pointerId: e.pointerId };
    setIsLongPressing(true);

    longPressTimerRef.current = setTimeout(() => {
      const info = longPressInfoRef.current;
      if (!info || !hitAreaRef.current) {
        setIsLongPressing(false);
        return;
      }
      longPressInfoRef.current = null;
      longPressTimerRef.current = null;
      setIsLongPressing(false);

      const rect = hitAreaRef.current.getBoundingClientRect();
      const barClientLeft = rect.left + BAR_GAP; // left edge of the visible bar
      onEventMoveStart(event, info.pointerId, info.x, info.y, barClientLeft);
    }, LONG_PRESS_MS);
  };

  const handleHitAreaPointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!longPressInfoRef.current) return;
    const dx = e.clientX - longPressInfoRef.current.x;
    const dy = e.clientY - longPressInfoRef.current.y;
    if (Math.abs(dx) > 5 || Math.abs(dy) > 5) cancelLongPress();
  };

  const handleHitAreaPointerUp = () => {
    cancelLongPress();
  };

  const openMenu = (x: number, y: number) => {
    if (didDragRef.current) { didDragRef.current = false; return; }
    if (isDraggingAsMove || isLongPressing) return;
    if (isBlock) {
      onBlockClick?.(event, x, y);
    } else {
      onEventClick?.(event, x, y);
    }
  };

  // ── Click handler ─────────────────────────────────────────────────────────
  const handleClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    openMenu(e.clientX, e.clientY);
  };

  const handleContextMenu = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    openMenu(e.clientX, e.clientY);
  };

  // ── Derived display values ────────────────────────────────────────────────
  const SourceIcon = !isBlock && event.source ? (SOURCE_ICONS[event.source] ?? Building2) : null;
  const topLine = isBlock ? null : (event.guestName ?? event.title);
  const adults = event.adults ?? 0;
  const children = event.children ?? 0;
  const unreadCount = Math.max(0, event.unreadMessageCount ?? 0);
  const hasUnreadMessages = unreadCount > 0;
  const unreadDisplay = unreadCount > 99 ? "99+" : String(unreadCount);
  const isTooNarrow = displayWidthPx < cellWidth * 1.5;

  // Parse guest name: "John Smith" → lastName="Smith", firstInitial="J"
  const nameParts = topLine ? topLine.trim().split(/\s+/) : [];
  const lastName = nameParts[nameParts.length - 1] ?? "";
  const firstInitial = nameParts.length > 1 ? (nameParts[0]?.[0]?.toUpperCase() ?? "") : "";

  const hitLeft = displayLeftPx - BAR_GAP;
  const hitWidth = displayWidthPx + BAR_GAP * 2;

  const isResizeDragging = !!dragType;

  // Bottom-half solid shade (no alpha transparency)
  const barBottomColor = bottomShade(event.backgroundColor);

  // Ghost of original extent during resize — positioned at event.leftPx/widthPx
  // while the live bar follows displayLeftPx/displayWidthPx.
  const ghostLeftInHitArea = event.leftPx - displayLeftPx + BAR_GAP;

  return (
    <div
      ref={hitAreaRef}
      role="button"
      data-calendar-event-bar="true"
      aria-label={`${isBlock ? "Block" : "Reservation"}: ${event.title}`}
      className="absolute select-none"
      style={{
        left: hitLeft,
        width: hitWidth,
        top: 0,
        height: ROW_HEIGHT,
        zIndex: isResizeDragging ? 20 : isDraggingAsMove ? 1 : 5,
        touchAction: "none",
        cursor: isDraggingAsMove ? "grabbing" : isLongPressing ? "grab" : "pointer",
        pointerEvents: isDraggingAsMove ? "none" : "auto",
      }}
      onClick={handleClick}
      onPointerDown={handleHitAreaPointerDown}
      onPointerMove={handleHitAreaPointerMove}
      onPointerUp={handleHitAreaPointerUp}
      onPointerCancel={handleHitAreaPointerUp}
      onContextMenu={handleContextMenu}
    >
      {/* Ghost of original extent — shown while resize-dragging */}
      {isResizeDragging && (
        <div
          aria-hidden="true"
          className="absolute pointer-events-none"
          style={{
            left: ghostLeftInHitArea,
            width: event.widthPx,
            top: BAR_TOP,
            height: BAR_HEIGHT,
            background: isBlock
              ? CALENDAR_COLORS.block.gradient
              : event.widthPx < cellWidth * 1.5
                ? event.backgroundColor
                : `linear-gradient(to bottom, ${event.backgroundColor} 50%, ${barBottomColor} 50%)`,
            borderRadius: borderRadiusCSS,
            opacity: 0.35,
          }}
        />
      )}

      {/* Visible bar — inset from hit area */}
      <div
        ref={visibleBarRef}
        className="absolute overflow-hidden flex flex-col justify-center"
        style={{
          left: BAR_GAP,
          width: displayWidthPx,
          top: BAR_TOP,
          height: BAR_HEIGHT,
          background: isBlock
            ? CALENDAR_COLORS.block.gradient
            : isTooNarrow
              ? event.backgroundColor
              : `linear-gradient(to bottom, ${event.backgroundColor} 50%, ${barBottomColor} 50%)`,
          color: event.textColor,
          borderRadius: borderRadiusCSS,
          boxShadow: isResizeDragging
            ? CALENDAR_COLORS.bar.shadowDrag
            : isLongPressing
              ? CALENDAR_COLORS.bar.shadowLongPress
              : CALENDAR_COLORS.bar.shadowDefault,
          transition: isResizeDragging ? "none" : "opacity 150ms ease, transform 150ms ease, box-shadow 150ms ease",
          opacity: isDraggingAsMove ? 0.35 : isPending ? 0.7 : 1,
          transform: isDraggingAsMove ? "scale(0.97)" : isLongPressing ? "scale(1.015)" : "scale(1)",
          transformOrigin: "center center",
        }}
      >
        {isTooNarrow ? (
          <span
            className="px-2 text-xs font-medium truncate leading-none"
            style={{ color: event.textColor }}
          >
            {event.title}
          </span>
        ) : isBlock ? (
          <span
            className="px-2 text-xs font-medium truncate leading-none"
            style={{ color: event.textColor }}
          >
            {event.title}
          </span>
        ) : (
          <div
            className="flex flex-col h-full"
            style={{ paddingLeft: HANDLE_WIDTH + 4, paddingRight: HANDLE_WIDTH + 4 }}
          >
            {/* Top bar: channel icon · last name · first initial */}
            <div className="flex items-center gap-1 flex-1 min-h-0 overflow-hidden">
              {SourceIcon && (
                <SourceIcon size={9} style={{ color: event.textColor, flexShrink: 0 }} />
              )}
              <span
                className="text-xs font-semibold truncate leading-tight"
                style={{ color: event.textColor }}
              >
                {lastName}
              </span>
              {firstInitial && (
                <span
                  className="text-xs font-semibold leading-tight shrink-0"
                  style={{ color: event.textColor }}
                >
                  {firstInitial}
                </span>
              )}
            </div>

            {/* Bottom bar: adults · children · messages · payment — all left-aligned */}
            <div className="flex items-center flex-1 min-h-0 overflow-hidden gap-1">
              {adults > 0 && (
                <div className="flex items-center gap-0.5 shrink-0">
                  <Users size={BOTTOM_ICON_SIZE} style={{ color: event.textColor, flexShrink: 0 }} />
                  <span className="text-[10px] leading-none" style={{ color: event.textColor }}>
                    {adults}
                  </span>
                </div>
              )}
              {/* Always rendered to reserve consistent space; dimmed when no children */}
              <div className="flex items-center gap-0.5 shrink-0" style={{ opacity: children > 0 ? 1 : 0.35 }}>
                <Baby size={BOTTOM_ICON_SIZE} style={{ color: event.textColor, flexShrink: 0 }} />
                {children > 0 && (
                  <span className="text-[10px] leading-none" style={{ color: event.textColor }}>
                    {children}
                  </span>
                )}
              </div>
              <div className="flex items-center gap-0.5 shrink-0">
                <MessageCircle
                  size={BOTTOM_ICON_SIZE}
                  style={{
                    color: hasUnreadMessages ? CALENDAR_COLORS.unreadDot : event.textColor,
                    opacity: hasUnreadMessages ? 1 : 0.55,
                    flexShrink: 0,
                  }}
                />
                <span
                  className="text-[10px] leading-none font-semibold"
                  style={{
                    color: hasUnreadMessages ? CALENDAR_COLORS.unreadDot : event.textColor,
                    opacity: hasUnreadMessages ? 1 : 0.55,
                  }}
                >
                  {unreadDisplay}
                </span>
              </div>
              {event.paymentStatus && (
                <PaymentStatusIcon
                  status={event.paymentStatus}
                  textColor={event.textColor}
                />
              )}
            </div>
          </div>
        )}

        {/* Shimmer overlay — visible while API call is in flight */}
        {isPending && (
          <div
            aria-hidden="true"
            className="absolute inset-0 overflow-hidden pointer-events-none"
            style={{ borderRadius: borderRadiusCSS }}
          >
            <div
              style={{
                position: "absolute",
                top: 0,
                bottom: 0,
                width: "45%",
                background: CALENDAR_COLORS.bar.shimmer,
                animation: "cal-bar-shimmer 1.4s ease-in-out infinite",
                willChange: "transform",
              }}
            />
          </div>
        )}

        {/* Left resize handle */}
        {isCalendarEditable && !event.clampedLeft && onEventResized && (
          <div
            className="absolute left-0 top-0 bottom-0 cursor-w-resize z-10 flex items-center justify-center hover:bg-white/20 active:bg-white/30 transition-colors"
            style={{ width: HANDLE_WIDTH }}
            onPointerDown={(e) => startResizeDrag(e, "resize-left")}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="w-0.5 h-3 rounded-full bg-current opacity-40" />
          </div>
        )}

        {/* Right resize handle */}
        {isCalendarEditable && !event.clampedRight && onEventResized && (
          <div
            className="absolute right-0 top-0 bottom-0 cursor-e-resize z-10 flex items-center justify-center hover:bg-white/20 active:bg-white/30 transition-colors"
            style={{ width: HANDLE_WIDTH }}
            onPointerDown={(e) => startResizeDrag(e, "resize-right")}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="w-0.5 h-3 rounded-full bg-current opacity-40" />
          </div>
        )}
      </div>

      {/* Original edge indicator — rendered above live bar so it shows when extending */}
      {isResizeDragging && (
        <div
          aria-hidden="true"
          className="absolute pointer-events-none"
          style={{
            left: dragType === "resize-left"
              ? ghostLeftInHitArea
              : ghostLeftInHitArea + event.widthPx - 2,
            width: 2,
            top: BAR_TOP,
            height: BAR_HEIGHT,
            background: CALENDAR_COLORS.bar.resizeEdge,
            borderRadius: 1,
            zIndex: 25,
            boxShadow: CALENDAR_COLORS.bar.resizeEdgeShadow,
          }}
        />
      )}
    </div>
  );
});

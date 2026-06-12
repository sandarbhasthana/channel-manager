"use client";

import { useCallback, useEffect, useRef } from "react";
import { animate } from "motion/react";

// ── Velocity thresholds (px/ms) — same tiers as old carousel ─────────────────
const CAROUSEL_VEL_LOW  = 0.4;
const CAROUSEL_VEL_HIGH = 0.78;

// ── Touch direction lock ──────────────────────────────────────────────────────
const DIR_LOCK_PX = 8;

// ── Ring buffer for velocity sampling ────────────────────────────────────────
const VEL_SAMPLES     = 5;
const FLICK_BOOST_VEL = 0.9;
const FLICK_BOOST_MIN = 1.24;

// ── Wheel debounce ────────────────────────────────────────────────────────────
const WHEEL_END_MS = 80;
const EVENT_BAR_SELECTOR = "[data-calendar-event-bar='true']";

interface Options {
  scrollElementRef: React.RefObject<HTMLDivElement | null>;
  cellWidth: number;
  disabled?: boolean;
}

function springParamsFor(absVel: number): { stiffness: number; damping: number } {
  if (absVel < CAROUSEL_VEL_LOW)   return { stiffness: 400, damping: 40 };
  if (absVel >= CAROUSEL_VEL_HIGH) return { stiffness: 120, damping: 22 };
  return { stiffness: 220, damping: 30 };
}

// Epsilon is only for effectively-exact browser floating point noise. A real
// fractional overshoot should still snap in the user's original direction.
const SNAP_EPSILON = 0.000001;

/**
 * Snap scrollLeft to a cellWidth boundary.
 * direction > 0: snap forward (ceil)  — never retreat when scrolling right
 * direction < 0: snap backward (floor) — never advance when scrolling left
 * direction = 0: snap to nearest (round)
 */
function snapTarget(scrollLeft: number, cellWidth: number, direction: 1 | -1 | 0): number {
  const exact = scrollLeft / cellWidth;
  if (direction > 0) {
    const lower = Math.floor(exact) * cellWidth;
    return Math.max(0, scrollLeft <= lower + SNAP_EPSILON ? lower : lower + cellWidth);
  }
  if (direction < 0) {
    const upper = Math.ceil(exact) * cellWidth;
    return Math.max(0, scrollLeft >= upper - SNAP_EPSILON ? upper : upper - cellWidth);
  }
  return Math.max(0, Math.round(exact) * cellWidth);
}

function clampSnapUpdate(value: number, previous: number, target: number, direction: 1 | -1 | 0): number {
  if (direction > 0) return Math.min(target, Math.max(previous, value));
  if (direction < 0) return Math.max(target, Math.min(previous, value));
  return value;
}

function startedOnEventBar(target: EventTarget | null): boolean {
  return target instanceof Element && !!target.closest(EVENT_BAR_SELECTOR);
}

export function useHorizontalScroll({ scrollElementRef, cellWidth, disabled }: Options): void {
  const cellWidthRef = useRef(cellWidth);
  useEffect(() => { cellWidthRef.current = cellWidth; }, [cellWidth]);

  const disabledRef = useRef(disabled ?? false);
  useEffect(() => { disabledRef.current = disabled ?? false; }, [disabled]);

  const activeAnimRef = useRef<{ stop: () => void } | null>(null);

  const stopAnimation = useCallback(() => {
    activeAnimRef.current?.stop();
    activeAnimRef.current = null;
  }, []);

  // ── Touch handler ─────────────────────────────────────────────────────────
  useEffect(() => {
    if (!scrollElementRef.current) return;
    const el: HTMLDivElement = scrollElementRef.current;

    const touchState = {
      startX: 0,
      startY: 0,
      startScrollLeft: 0,
      swiping: false,
      active: false,
    };

    type Sample = { x: number; t: number };
    const samples: Sample[] = [];

    function pushSample(x: number) {
      samples.push({ x, t: performance.now() });
      if (samples.length > VEL_SAMPLES) samples.shift();
    }

    function getVelocity(): number {
      if (samples.length < 2) return 0;
      const newest = samples[samples.length - 1];
      const cutoff = newest.t - 80;
      let oldest = samples[samples.length - 2];
      for (let i = 0; i < samples.length - 1; i++) {
        if (samples[i].t >= cutoff) { oldest = samples[i]; break; }
      }
      const dt = newest.t - oldest.t;
      const windowVel = dt < 1 ? 0 : (newest.x - oldest.x) / dt;
      const prev = samples[samples.length - 2];
      const tailDt = newest.t - prev.t;
      const tailVel = tailDt < 1 ? 0 : (newest.x - prev.x) / tailDt;
      const chosenVel = Math.abs(tailVel) > Math.abs(windowVel) ? tailVel : windowVel;
      if (Math.abs(chosenVel) < FLICK_BOOST_VEL) return chosenVel;
      return chosenVel * FLICK_BOOST_MIN;
    }

    function onTouchStart(e: TouchEvent) {
      if (disabledRef.current) return;
      stopAnimation();
      if (startedOnEventBar(e.target)) {
        touchState.active = false;
        touchState.swiping = false;
        samples.length = 0;
        return;
      }
      samples.length = 0;
      touchState.startX = e.touches[0].clientX;
      touchState.startY = e.touches[0].clientY;
      touchState.startScrollLeft = el.scrollLeft;
      touchState.swiping = false;
      touchState.active = true;
      pushSample(e.touches[0].clientX);
    }

    function onTouchMove(e: TouchEvent) {
      if (!touchState.active) return;

      const dx = e.touches[0].clientX - touchState.startX;
      const dy = e.touches[0].clientY - touchState.startY;

      if (!touchState.swiping) {
        const adx = Math.abs(dx), ady = Math.abs(dy);
        if      (adx > ady && adx > DIR_LOCK_PX) touchState.swiping = true;
        else if (ady > DIR_LOCK_PX)               { touchState.active = false; return; }
        else return;
      }

      e.preventDefault();
      pushSample(e.touches[0].clientX);
      el.scrollLeft = touchState.startScrollLeft - dx;
    }

    function onTouchEnd(e: TouchEvent) {
      if (!touchState.active) return;
      touchState.active = false;
      if (!touchState.swiping) { touchState.swiping = false; return; }
      touchState.swiping = false;

      const releasedX = e.changedTouches[0]?.clientX;
      if (typeof releasedX === "number" && releasedX !== samples[samples.length - 1]?.x) {
        pushSample(releasedX);
      }

      const vel = getVelocity(); // px/ms; positive = finger moved right = scroll went left
      const absVel = Math.abs(vel);
      const params = springParamsFor(absVel);
      // Finger moved right → scroll should go backward (increase scrollLeft)
      // vel is positive when finger moves right, so scroll velocity = -vel
      const scrollVel = -vel; // px/ms in scroll-coordinate space

      // Velocity direction is the reliable signal: it comes from clientX samples
      // and is immune to the fractional startScrollLeft that occurs when a touch
      // starts during a mid-flight spring animation. Position delta is the fallback
      // for near-zero-velocity gestures (taps).
      const netScrollDelta = el.scrollLeft - touchState.startScrollLeft;
      const velDir: 1 | -1 | 0 = scrollVel > 0 ? 1 : scrollVel < 0 ? -1 : 0;
      const posDir: 1 | -1 | 0 = netScrollDelta > 0 ? 1 : netScrollDelta < 0 ? -1 : 0;
      const direction: 1 | -1 | 0 = velDir !== 0 ? velDir : posDir;
      const snapped = snapTarget(el.scrollLeft, cellWidthRef.current, direction);
      let lastSnapValue = el.scrollLeft;
      const snapVelocity = direction === 0 ? scrollVel : Math.abs(scrollVel) * direction;

      const controls = animate(el.scrollLeft, snapped, {
        type: "spring",
        stiffness: params.stiffness,
        damping: params.damping,
        velocity: snapVelocity * 1000, // px/ms → px/s
        onUpdate: (v: number) => {
          const next = clampSnapUpdate(v, lastSnapValue, snapped, direction);
          lastSnapValue = next;
          el.scrollLeft = next;
        },
        onComplete: () => { activeAnimRef.current = null; },
      });
      activeAnimRef.current = controls;
    }

    el.addEventListener("touchstart",  onTouchStart,  { passive: true  });
    el.addEventListener("touchmove",   onTouchMove,   { passive: false });
    el.addEventListener("touchend",    onTouchEnd,    { passive: true  });
    el.addEventListener("touchcancel", onTouchEnd,    { passive: true  });

    return () => {
      el.removeEventListener("touchstart",  onTouchStart);
      el.removeEventListener("touchmove",   onTouchMove);
      el.removeEventListener("touchend",    onTouchEnd);
      el.removeEventListener("touchcancel", onTouchEnd);
      stopAnimation();
    };
  }, [scrollElementRef, stopAnimation]);

  // ── Wheel handler (horizontal) ────────────────────────────────────────────
  useEffect(() => {
    if (!scrollElementRef.current) return;
    const el: HTMLDivElement = scrollElementRef.current;

    let wheelDelta = 0;
    let lastWheelTime = performance.now();
    let wheelVel = 0;
    let wheelEndTimeout: ReturnType<typeof setTimeout> | null = null;

    function onWheel(e: WheelEvent) {
      if (disabledRef.current) return;
      // Only intercept predominantly-horizontal scroll
      if (Math.abs(e.deltaX) <= Math.abs(e.deltaY)) return;
      e.preventDefault();
      // Kill any in-flight touch spring so it doesn't fight the wheel over scrollLeft.
      stopAnimation();

      const now = performance.now();
      const dt = now - lastWheelTime;
      if (dt > 0 && dt < 150) wheelVel = e.deltaX / dt; // px/ms
      lastWheelTime = now;

      wheelDelta += e.deltaX;
      el.scrollLeft += e.deltaX;

      if (wheelEndTimeout !== null) clearTimeout(wheelEndTimeout);
      wheelEndTimeout = setTimeout(() => {
        wheelEndTimeout = null;

        const absVel = Math.abs(wheelVel);
        const params = springParamsFor(absVel);
        const direction = wheelDelta > 0 ? 1 : wheelDelta < 0 ? -1 : 0;
        const snapped = snapTarget(el.scrollLeft, cellWidthRef.current, direction);
        let lastSnapValue = el.scrollLeft;
        const snapVelocity = direction === 0 ? wheelVel : Math.abs(wheelVel) * direction;

        const controls = animate(el.scrollLeft, snapped, {
          type: "spring",
          stiffness: params.stiffness,
          damping: params.damping,
          velocity: snapVelocity * 1000, // px/ms → px/s
          onUpdate: (v: number) => {
            const next = clampSnapUpdate(v, lastSnapValue, snapped, direction);
            lastSnapValue = next;
            el.scrollLeft = next;
          },
          onComplete: () => { activeAnimRef.current = null; },
        });
        activeAnimRef.current = controls;

        wheelDelta = 0;
        wheelVel   = 0;
      }, WHEEL_END_MS);
    }

    el.addEventListener("wheel", onWheel, { passive: false });
    return () => {
      el.removeEventListener("wheel", onWheel);
      if (wheelEndTimeout !== null) clearTimeout(wheelEndTimeout);
    };
  }, [scrollElementRef, stopAnimation]);
}

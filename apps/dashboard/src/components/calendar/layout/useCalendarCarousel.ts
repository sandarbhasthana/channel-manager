"use client";

import { startTransition, useRef, useCallback, useEffect } from "react";
import type { CalendarCarouselDebugApi, CalendarCarouselDebugPhase } from "../debug/useCalendarCarouselDebug";

// ── Friction per 16 ms frame (applied as Math.pow(friction, dt/16)) ──────────
const FRICTION_LOW  = 0.9;    // Level 1: gentle nudge — still settles quickly
const FRICTION_MED  = 0.972;  // Level 2: normal swipe — longer carry than before
const FRICTION_HIGH = 0.9962; // Level 3: fast flick — noticeably longer coast

// ── Velocity thresholds to select a level (px / ms) ─────────────────────────
export const CAROUSEL_VEL_LOW  = 0.4;  // |v| < 0.4  → Level 1
export const CAROUSEL_VEL_HIGH = 0.78; // |v| ≥ 0.78 → Level 3 (else Level 2)

// ── Stop condition ────────────────────────────────────────────────────────────
const VEL_STOP_LOW  = 0.010; // px/ms
const VEL_STOP_MED  = 0.007;
const VEL_STOP_HIGH = 0.0018;

const SNAP_SETTLE_DAMPING = 0.88;
const SNAP_SETTLE_MIN_VEL = 0.018;
const SNAP_SETTLE_MAX_REMAINING_STEP = 0.22;

// ── Kept for toolbar / jumpToDate imperative navigation ──────────────────────
const SNAP_MS_IMPERATIVE = 280;

// ── Touch direction-lock ──────────────────────────────────────────────────────
const DIR_LOCK_PX = 8;

// ── Wheel debounce ────────────────────────────────────────────────────────────
const WHEEL_END_MS = 80;
const EDGE_SOFT_ZONE_PX = 96;
const GLIDE_CURVE_EXPONENT = 1.7;
const GLIDE_CURVE_DRAG_MAX = 0.2;
const SWIPE_MOMENTUM_CARRY = 0.82;

// ── Touch velocity ring-buffer size ──────────────────────────────────────────
const VEL_SAMPLES = 5;
const FLICK_BOOST_VEL = 0.9; // px/ms; short flicks above this should still coast
const FLICK_BOOST_MIN = 1.24;
const IMPERATIVE_MOMENTUM_MULTIPLIER = 1;

type MomentumProfile = {
  friction: number;
  stopVelocity: number;
  tier: "gentle" | "normal" | "fast";
};

/** Pick momentum tier from |velocity| (px/ms). Module-level — no closures. */
function momentumProfileFor(absVel: number): MomentumProfile {
  if (absVel < CAROUSEL_VEL_LOW) {
    return { friction: FRICTION_LOW, stopVelocity: VEL_STOP_LOW, tier: "gentle" };
  }
  if (absVel >= CAROUSEL_VEL_HIGH) {
    return { friction: FRICTION_HIGH, stopVelocity: VEL_STOP_HIGH, tier: "fast" };
  }
  return { friction: FRICTION_MED, stopVelocity: VEL_STOP_MED, tier: "normal" };
}

export interface CarouselHandle {
  /** imperative navigate — call from toolbar prev/next buttons */
  navigate: (dir: -1 | 1) => void;
  /**
   * Animate in `dir` direction then call `onJump` to set the new date.
   * Used by the "today" button to jump an arbitrary distance with the same
   * slide animation as prev/next.
   */
  jumpToDate: (dir: -1 | 1, onJump: () => void) => void;
}

interface Options {
  panelWidth: number;
  /** Width of a single day column in pixels — used to snap at 1-day granularity. */
  cellWidth: number;
  /** Called after any navigation. Positive = forward in time, negative = backward. */
  onCommit: (daysOffset: number) => void;
  /** the DOM element that receives touch / wheel events */
  touchTargetRef: React.RefObject<HTMLElement | null>;
  /** the strip element whose transform we drive */
  stripRef: React.RefObject<HTMLDivElement | null>;
  /** optional header strip — kept in sync with stripRef during animation */
  headerStripRef?: React.RefObject<HTMLDivElement | null>;
  debug?: CalendarCarouselDebugApi;
}

/**
 * Drives a 3-panel horizontal carousel with physics-based momentum.
 *
 * Strategy:
 *   • During live drag / wheel  → bypass React state; write directly to
 *     `stripRef.current.style.transform` inside rAF. Zero React renders.
 *   • On release / wheel-end   → run a friction-deceleration rAF loop.
 *     When velocity falls below VEL_STOP, snap (CSS transition) to the
 *     nearest panel boundary — not necessarily the "next" one.
 *   • Three momentum levels pick the friction coefficient, producing
 *     short (Level 1), medium (Level 2), and long (Level 3) glides.
 *   • The strip baseline is always `translateX(-panelWidth)` (centre panel).
 *     All `dx` offsets are relative to that baseline.
 */
export function useCalendarCarousel({
  panelWidth,
  cellWidth,
  onCommit,
  touchTargetRef,
  stripRef,
  headerStripRef,
  debug,
}: Options): CarouselHandle {
  const settlingRef  = useRef(false);
  const panelRef     = useRef(panelWidth);
  // Keep a ref to onCommit so commitFromPos never needs to be recreated when
  // the caller's commit function changes (e.g. after internalDate updates).
  const onCommitRef  = useRef(onCommit);
  useEffect(() => { onCommitRef.current = onCommit; }, [onCommit]);
  const cellRef      = useRef(cellWidth);
  const dragXRef     = useRef(0);
  const rafRef       = useRef<number | null>(null);
  const settleTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const wheelEndTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const activeGlideCancelRef = useRef<(() => void) | null>(null);
  const motionVelocityRef = useRef(0);
  const carryVelocityRef = useRef(0);

  const debugLog = useCallback((message: string, details?: Record<string, unknown>) => {
    debug?.log(message, details);
  }, [debug]);

  const debugSample = useCallback((speed: number, phase: CalendarCarouselDebugPhase) => {
    debug?.pushSample({
      velocity: speed,
      momentum: speed,
      carry: 0,
      phase,
    });
  }, [debug]);

  const debugMomentumSample = useCallback((
    velocity: number,
    carry: number,
    phase: CalendarCarouselDebugPhase,
  ) => {
    debug?.pushSample({
      velocity,
      momentum: velocity + carry,
      carry,
      phase,
    });
  }, [debug]);

  useEffect(() => { panelRef.current = panelWidth; }, [panelWidth]);
  useEffect(() => { cellRef.current  = cellWidth;  }, [cellWidth]);

  const setStripWillChange = useCallback((active: boolean) => {
    const value = active ? "transform" : "";

    const el = stripRef.current;
    if (el) el.style.willChange = value;

    const hel = headerStripRef?.current;
    if (hel) hel.style.willChange = value;
  }, [stripRef, headerStripRef]);

  /* ─────────────────── core helpers ─────────────────── */

  /** Write transform directly to the DOM. durationMs=0 → no transition. */
  const setStripTransform = useCallback((dx: number, durationMs: number) => {
    const transition = durationMs > 0
      ? `transform ${durationMs}ms cubic-bezier(0.4,0,0.2,1)`
      : "none";
    const transform = `translate3d(${-panelRef.current + dx}px, 0, 0)`;

    const el = stripRef.current;
    if (el) { el.style.transition = transition; el.style.transform = transform; }

    const hel = headerStripRef?.current;
    if (hel) { hel.style.transition = transition; hel.style.transform = transform; }
  }, [stripRef, headerStripRef]);

  const queueCommit = useCallback((daysOffset: number) => {
    if (daysOffset === 0) return;
    startTransition(() => {
      onCommitRef.current(daysOffset);
    });
  }, []);

  const clearPointerLock = useCallback(() => {
    if (stripRef.current) stripRef.current.style.pointerEvents = "";
  }, [stripRef]);

  const clearPendingMotion = useCallback((
    keepWillChange = false,
    preserveCarryVelocity = false,
  ) => {
    debugLog("clearPendingMotion", { keepWillChange, preserveCarryVelocity });
    if (preserveCarryVelocity) {
      carryVelocityRef.current = motionVelocityRef.current;
    } else {
      carryVelocityRef.current = 0;
    }
    if (rafRef.current !== null) {
      cancelAnimationFrame(rafRef.current);
      rafRef.current = null;
    }
    if (settleTimeoutRef.current !== null) {
      clearTimeout(settleTimeoutRef.current);
      settleTimeoutRef.current = null;
    }
    if (wheelEndTimeoutRef.current !== null) {
      clearTimeout(wheelEndTimeoutRef.current);
      wheelEndTimeoutRef.current = null;
    }
    activeGlideCancelRef.current?.();
    activeGlideCancelRef.current = null;
    motionVelocityRef.current = 0;
    debugSample(0, "idle");
    settlingRef.current = false;
    clearPointerLock();
    if (!keepWillChange) setStripWillChange(false);
  }, [clearPointerLock, debugLog, debugSample, setStripWillChange]);

  /**
   * After the glide exhausts, snap to the nearest 1-day boundary (cellWidth
   * multiple), then commit. Strip moving right (positive dx) = going back in
   * time (negative daysOffset); left (negative dx) = going forward.
   *
   * swipeDir: sign of the release velocity — snap only in this direction so
   * the strip never jumps backward against the user's gesture.
   */
  const settleToSnap = useCallback((
    startPos: number,
    targetPos: number,
    startVel: number,
    onDone: () => void,
  ) => {
    debugLog("settleToSnap:start", { startPos, targetPos, startVel });
    let pos = startPos;
    let vel = startVel;
    let last: number | null = null;
    const targetDir = Math.sign(targetPos - startPos);

    const initialSpeed = Math.max(SNAP_SETTLE_MIN_VEL, Math.abs(vel));
    vel = targetDir === 0 ? 0 : targetDir * initialSpeed;
    motionVelocityRef.current = vel;
    debugSample(vel, "snap");

    const step = (now: number) => {
      if (last === null) {
        last = now;
        rafRef.current = requestAnimationFrame(step);
        return;
      }

      const dt = Math.min(now - last, 32);
      last = now;

      const remaining = targetPos - pos;
      const remainingDir = Math.sign(remaining);
      const nextSpeed = Math.max(
        SNAP_SETTLE_MIN_VEL,
        Math.abs(vel) * Math.pow(SNAP_SETTLE_DAMPING, dt / 16),
      );
      const maxStep = Math.abs(remaining) * SNAP_SETTLE_MAX_REMAINING_STEP;
      const stepPx = Math.min(nextSpeed * dt, maxStep);
      vel = remainingDir === 0 ? 0 : remainingDir * (stepPx / Math.max(dt, 1));
      pos += remainingDir * stepPx;
      motionVelocityRef.current = vel;
      debugSample(vel, "snap");

      if (
        Math.abs(targetPos - pos) < 0.35 ||
        stepPx < 0.08
      ) {
        dragXRef.current = targetPos;
        motionVelocityRef.current = 0;
        debugSample(0, "idle");
        setStripTransform(targetPos, 0);
        rafRef.current = null;
        debugLog("settleToSnap:done", { targetPos });
        onDone();
        return;
      }

      dragXRef.current = pos;
      setStripTransform(pos, 0);
      rafRef.current = requestAnimationFrame(step);
    };

    rafRef.current = requestAnimationFrame(step);
    return () => {
      if (rafRef.current !== null) cancelAnimationFrame(rafRef.current);
      rafRef.current = null;
    };
  }, [debugLog, debugSample, setStripTransform]);

  const commitFromPos = useCallback((finalPos: number, swipeDir: number, finalVel: number) => {
    const pw = panelRef.current;
    const cw = cellRef.current;

    // Snap in the direction of movement — never against it.
    // Math.ceil/floor ensures we don't round back past where the finger was heading.
    const rawDays = finalPos / cw;
    const roundedDays =
      swipeDir > 0 ? Math.ceil(rawDays)
      : swipeDir < 0 ? Math.floor(rawDays)
      : Math.round(rawDays);
    const snapped = Math.max(-pw, Math.min(pw, roundedDays * cw));
    const snapDistance = Math.abs(snapped - finalPos);
    // daysOffset: positive = forward in time, negative = backward
    const daysOffset = -Math.round(snapped / cw);
    debugLog("commitFromPos", {
      finalPos,
      swipeDir,
      finalVel,
      snapped,
      daysOffset,
      snapDistance,
    });

    const finishCommit = () => {
      settleTimeoutRef.current = setTimeout(() => {
        settleTimeoutRef.current = null;
        queueCommit(daysOffset);
        dragXRef.current = daysOffset === 0 ? 0 : snapped;
        settlingRef.current = false;
        debugSample(0, "idle");
        setStripWillChange(false);
        clearPointerLock();
      }, 0);
    };

    if (snapDistance < 0.5) {
      dragXRef.current = snapped;
      setStripTransform(snapped, 0);
      finishCommit();
      return;
    }

    activeGlideCancelRef.current = settleToSnap(
      finalPos,
      snapped,
      finalVel,
      () => {
        activeGlideCancelRef.current = null;
        finishCommit();
      },
    );
  }, [clearPointerLock, debugLog, debugSample, queueCommit, settleToSnap, setStripTransform, setStripWillChange]);

  /**
   * Run a shaped deceleration loop starting from `startPos` / `startVel`.
   * Early frames keep more of the launch velocity, then drag ramps up as the
   * glide approaches its stop threshold so the motion reads as a curve.
   * Calls onFrame every frame (for live strip update) and onDone when stopped.
   */
  const momentumGlide = useCallback((
    startPos: number,
    startVel: number,
    friction: number,
    stopVelocity: number,
    phase: "touch-glide" | "wheel-glide",
    onFrame: (pos: number) => void,
    onDone: (finalPos: number, finalVel: number) => void,
  ) => {
    const pw = panelRef.current;
    let pos = startPos;
    let vel = startVel;
    let last: number | null = null;
    let raf: number;
    const launchSpeed = Math.max(Math.abs(startVel), stopVelocity * 4);
    motionVelocityRef.current = vel;
    debugLog("momentumGlide:start", { startPos, startVel, friction, stopVelocity });

    function step(now: number) {
      if (last === null) { last = now; raf = requestAnimationFrame(step); return; }

      const dt = Math.min(now - last, 64); // cap dt so a tab-switch doesn't jump
      last = now;

      pos += vel * dt;
      const speed = Math.abs(vel);
      const normalizedSpeed =
        launchSpeed <= stopVelocity
          ? 0
          : (speed - stopVelocity) / (launchSpeed - stopVelocity);
      const clampedSpeed = Math.max(0, Math.min(1, normalizedSpeed));
      const curveProgress = 1 - clampedSpeed;
      const curveDrag =
        1 - GLIDE_CURVE_DRAG_MAX * Math.pow(curveProgress, GLIDE_CURVE_EXPONENT);
      vel *= Math.pow(friction * curveDrag, dt / 16);

      const distanceToEdge = pw - Math.abs(pos);
      const movingTowardEdge = pos !== 0 && Math.sign(vel) === Math.sign(pos);
      if (movingTowardEdge && distanceToEdge < EDGE_SOFT_ZONE_PX) {
        const edgeRatio = Math.max(0.12, distanceToEdge / EDGE_SOFT_ZONE_PX);
        vel *= Math.pow(edgeRatio, dt / 16);
      }

      if (pos > pw) {
        pos = pw;
        vel *= 0.08;
      } else if (pos < -pw) {
        pos = -pw;
        vel *= 0.08;
      }

      motionVelocityRef.current = vel;
      debugSample(vel, phase);
      onFrame(pos);

      if (Math.abs(vel) < stopVelocity) {
        motionVelocityRef.current = vel;
        debugLog("momentumGlide:stop", { pos, vel, stopVelocity });
        onDone(pos, vel);
        return;
      }
      raf = requestAnimationFrame(step);
    }

    raf = requestAnimationFrame(step);
    // Return cancel handle so callers can abort if needed
    return () => cancelAnimationFrame(raf);
  }, [debugLog, debugSample]);

  /* ─────────────────── imperative API (toolbar / today) ─────────────────── */

  /** Snap to destination: dir=0 → bounce back, dir=±1 → navigate one full panel. */
  const snapTo = useCallback((dir: 0 | -1 | 1, durationMs = SNAP_MS_IMPERATIVE) => {
    clearPendingMotion();
    settlingRef.current = true;
    setStripWillChange(true);
    if (stripRef.current) stripRef.current.style.pointerEvents = "none";
    debugLog("snapTo", { dir, durationMs });
    debugMomentumSample(
      dir === 0 ? 0 : panelRef.current / Math.max(durationMs, 1),
      0,
      "imperative",
    );

    if (dir === 0) {
      setStripTransform(0, durationMs);
      settleTimeoutRef.current = setTimeout(() => {
        settleTimeoutRef.current = null;
        dragXRef.current = 0;
        settlingRef.current = false;
        debugSample(0, "idle");
        setStripWillChange(false);
        clearPointerLock();
      }, durationMs);
      return;
    }

    const target = -dir * panelRef.current;
    setStripTransform(target, durationMs);

    // Full-panel navigation: advance by visibleDays (= panelWidth / cellWidth)
    const daysPerPanel = Math.round(panelRef.current / cellRef.current);

    settleTimeoutRef.current = setTimeout(() => {
      settleTimeoutRef.current = null;
      queueCommit(dir * daysPerPanel);
      dragXRef.current = target;
      settlingRef.current = false;
      debugSample(0, "idle");
      setStripWillChange(false);
      clearPointerLock();
    }, durationMs);
  }, [clearPendingMotion, clearPointerLock, debugLog, debugMomentumSample, debugSample, queueCommit, setStripTransform, setStripWillChange, stripRef]);

  const navigate = useCallback((dir: -1 | 1) => {
    snapTo(dir);
  }, [snapTo]);

  const jumpToDate = useCallback((dir: -1 | 1, onJump: () => void) => {
    clearPendingMotion();
    settlingRef.current = true;
    setStripWillChange(true);
    if (stripRef.current) stripRef.current.style.pointerEvents = "none";
    debugLog("jumpToDate", { dir });
    debugMomentumSample(
      panelRef.current / SNAP_MS_IMPERATIVE * IMPERATIVE_MOMENTUM_MULTIPLIER,
      0,
      "imperative",
    );

    setStripTransform(-dir * panelRef.current, SNAP_MS_IMPERATIVE);

    settleTimeoutRef.current = setTimeout(() => {
      settleTimeoutRef.current = null;
      startTransition(() => {
        onJump();
      });
      dragXRef.current = -dir * panelRef.current;
      settlingRef.current = false;
      debugSample(0, "idle");
      setStripWillChange(false);
      clearPointerLock();
    }, SNAP_MS_IMPERATIVE);
  }, [clearPendingMotion, clearPointerLock, debugLog, debugMomentumSample, debugSample, setStripTransform, setStripWillChange, stripRef]);

  /* ─────────────────── touch handlers ─────────────────── */

  useEffect(() => {
    const el = touchTargetRef.current;
    if (!el) return;

    const touchState = { startX: 0, startY: 0, swiping: false, active: false };

    // Ring-buffer of recent (x, t) samples for velocity calculation
    type Sample = { x: number; t: number };
    const samples: Sample[] = [];

    function pushSample(x: number) {
      samples.push({ x, t: performance.now() });
      if (samples.length > VEL_SAMPLES) samples.shift();
    }

    function getVelocity(): number {
      if (samples.length < 2) return 0;
      const newest = samples[samples.length - 1];
      // Use only the last 80 ms — avoids capturing the deceleration tail
      // that occurs as the finger slows before leaving the screen.
      const cutoff = newest.t - 80;
      let oldest = samples[samples.length - 2]; // fallback: immediate predecessor
      for (let i = 0; i < samples.length - 1; i++) {
        if (samples[i].t >= cutoff) { oldest = samples[i]; break; }
      }
      const dt = newest.t - oldest.t;
      const windowVel = dt < 1 ? 0 : (newest.x - oldest.x) / dt; // px/ms, signed

      // Use the most recent segment too; a fast flick often accelerates near release,
      // and averaging across the whole swipe can underreport that final launch speed.
      const prev = samples[samples.length - 2];
      const tailDt = newest.t - prev.t;
      const tailVel = tailDt < 1 ? 0 : (newest.x - prev.x) / tailDt;

      const chosenVel =
        Math.abs(tailVel) > Math.abs(windowVel) ? tailVel : windowVel;

      if (Math.abs(chosenVel) < FLICK_BOOST_VEL) return chosenVel;
      return chosenVel * FLICK_BOOST_MIN;
    }

    function onTouchStart(e: TouchEvent) {
      clearPendingMotion(true, true);
      setStripWillChange(true);
      debugLog("touch:start", { x: e.touches[0].clientX, y: e.touches[0].clientY });
      samples.length = 0;
      touchState.startX  = e.touches[0].clientX;
      touchState.startY  = e.touches[0].clientY;
      touchState.swiping = false;
      touchState.active  = true;
      pushSample(e.touches[0].clientX);
    }

    function onTouchMove(e: TouchEvent) {
      if (!touchState.active || settlingRef.current) return;

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

      dragXRef.current = dx;
      debugMomentumSample(
        dx === 0 ? 0 : dx / Math.max(performance.now() - samples[0].t, 1),
        carryVelocityRef.current * SWIPE_MOMENTUM_CARRY,
        "touch-drag",
      );

      if (rafRef.current === null) {
        rafRef.current = requestAnimationFrame(() => {
          setStripTransform(dragXRef.current, 0);
          rafRef.current = null;
        });
      }
    }

    function onTouchEnd(e: TouchEvent) {
      if (rafRef.current !== null) { cancelAnimationFrame(rafRef.current); rafRef.current = null; }
      if (!touchState.active) return;
      touchState.active = false;

      if (!touchState.swiping) { touchState.swiping = false; return; }
      touchState.swiping = false;

      // Capture the final released position if the browser skipped the last move event.
      const releasedX = e.changedTouches[0]?.clientX;
      if (typeof releasedX === "number" && releasedX !== samples[samples.length - 1]?.x) {
        pushSample(releasedX);
        dragXRef.current = releasedX - touchState.startX;
      }

      let vel = getVelocity(); // px/ms, signed (positive = moved right = navigate back)
      const carryVel = carryVelocityRef.current;
      carryVelocityRef.current = 0;
      const carryContribution =
        vel !== 0 && carryVel !== 0 && Math.sign(vel) === Math.sign(carryVel)
          ? carryVel * SWIPE_MOMENTUM_CARRY
          : 0;

      // Same-direction repeated flicks stack on top of the existing glide.
      if (carryContribution !== 0) {
        vel += carryContribution;
      }

      const absVel = Math.abs(vel);
      const { friction, stopVelocity, tier } = momentumProfileFor(absVel);
      // Direction of the swipe: prefer velocity sign; fall back to drag position sign
      const swipeDir = vel !== 0 ? Math.sign(vel) : Math.sign(dragXRef.current || carryVel);
      debugLog("touch:end", {
        releasedX,
        dragX: dragXRef.current,
        vel,
        carryVel,
        carryContribution,
        launchMomentum: vel,
        absVel,
        tier,
        swipeDir,
        friction,
        stopVelocity,
      });
      debugMomentumSample(vel, carryContribution, "touch-glide");

      settlingRef.current = true;
      setStripWillChange(true);
      if (stripRef.current) stripRef.current.style.pointerEvents = "none";

      activeGlideCancelRef.current = momentumGlide(
        dragXRef.current,
        vel,
        friction,
        stopVelocity,
        "touch-glide",
        (pos) => {
          dragXRef.current = pos;
          setStripTransform(pos, 0);
        },
        (finalPos, finalVel) => {
          activeGlideCancelRef.current = null;
          commitFromPos(finalPos, swipeDir, finalVel);
        },
      );
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
      clearPendingMotion();
    };
  }, [touchTargetRef, setStripTransform, momentumGlide, commitFromPos, setStripWillChange, stripRef, clearPendingMotion, debugLog, debugMomentumSample, debugSample]);

  /* ─────────────────── wheel handler ─────────────────── */

  useEffect(() => {
    const el = touchTargetRef.current;
    if (!el) return;

    let wheelDelta = 0;
    let lastWheelTime = performance.now();
    let wheelVel = 0;

    function onWheel(e: WheelEvent) {
      // Only intercept predominantly-horizontal scroll
      if (Math.abs(e.deltaX) <= Math.abs(e.deltaY)) return;

      e.preventDefault();
      clearPendingMotion(true);
      setStripWillChange(true);

      // Velocity estimate (px/ms)
      const now = performance.now();
      const dt = now - lastWheelTime;
      if (dt > 0 && dt < 150) wheelVel = e.deltaX / dt;
      lastWheelTime = now;

      // Accumulate and clamp
      wheelDelta -= e.deltaX;  // negative deltaX = move right = go back
      const pw = panelRef.current;
      wheelDelta = Math.max(-pw, Math.min(pw, wheelDelta));
      dragXRef.current = wheelDelta;
      debugLog("wheel:move", { deltaX: e.deltaX, deltaY: e.deltaY, dt, wheelVel, wheelDelta });
      debugMomentumSample(wheelVel, 0, "wheel-drag");

      if (rafRef.current === null) {
        rafRef.current = requestAnimationFrame(() => {
          setStripTransform(dragXRef.current, 0);
          rafRef.current = null;
        });
      }

      // Debounce "wheel ended"
      if (wheelEndTimeoutRef.current !== null) clearTimeout(wheelEndTimeoutRef.current);
      wheelEndTimeoutRef.current = setTimeout(() => {
        wheelEndTimeoutRef.current = null;

        const absVel = Math.abs(wheelVel);
        const { friction, stopVelocity, tier } = momentumProfileFor(absVel);

        settlingRef.current = true;
        setStripWillChange(true);
        if (stripRef.current) stripRef.current.style.pointerEvents = "none";

        // Use the wheel velocity sign: positive deltaX = scroll left = go forward (dir=1)
        // wheelDelta is negated deltaX, so positive wheelVel means user wants to go back
        const glideVel = -wheelVel; // convert to strip-space: strip moves opposite to delta
        // swipeDir in strip-space: same sign as glideVel (or wheelDelta as fallback)
        const swipeDir = glideVel !== 0 ? Math.sign(glideVel) : Math.sign(wheelDelta);
        debugLog("wheel:end", {
          wheelDelta,
          wheelVel,
          glideVel,
          swipeDir,
          tier,
          friction,
          stopVelocity,
        });
        debugMomentumSample(glideVel, 0, "wheel-glide");

        activeGlideCancelRef.current = momentumGlide(
          wheelDelta,
          glideVel,
          friction,
          stopVelocity,
          "wheel-glide",
          (pos) => {
            dragXRef.current = pos;
            setStripTransform(pos, 0);
          },
          (finalPos, finalVel) => {
            activeGlideCancelRef.current = null;
            commitFromPos(finalPos, swipeDir, finalVel);
          },
        );

        wheelDelta = 0;
        wheelVel   = 0;
      }, WHEEL_END_MS);
    }

    el.addEventListener("wheel", onWheel, { passive: false });

    return () => {
      el.removeEventListener("wheel", onWheel);
      clearPendingMotion();
    };
  }, [touchTargetRef, setStripTransform, momentumGlide, commitFromPos, setStripWillChange, stripRef, clearPendingMotion, debugLog, debugMomentumSample, debugSample]);

  useEffect(() => () => clearPendingMotion(), [clearPendingMotion]);

  return { navigate, jumpToDate };
}

"use client";

import { useEffect, useRef } from "react";
import { animate } from "motion/react";

export function useVerticalMomentum(
  scrollElementRef: React.RefObject<HTMLDivElement | null>,
) {
  const animRef = useRef<{ stop: () => void } | null>(null);

  useEffect(() => {
    const el = scrollElementRef.current;
    if (!el) return;

    const handler = (e: WheelEvent) => {
      // Only handle vertical scroll; horizontal is managed by useHorizontalScroll
      if (Math.abs(e.deltaX) > Math.abs(e.deltaY)) return;

      e.preventDefault();
      animRef.current?.stop();
      animRef.current = null;

      let delta = e.deltaY;
      if (e.deltaMode === 1) delta *= 16;          // line → px
      if (e.deltaMode === 2) delta *= el.clientHeight; // page → px

      const maxScroll = el.scrollHeight - el.clientHeight;
      const from = el.scrollTop;
      const to = Math.max(0, Math.min(maxScroll, from + delta * 2.5));
      if (Math.abs(to - from) < 1) return;

      const controls = animate(from, to, {
        type: "spring",
        stiffness: 240,
        damping: 32,
        velocity: (delta / 16) * 1000,
        onUpdate: (v: number) => { el.scrollTop = v; },
        onComplete: () => { animRef.current = null; },
      });
      animRef.current = controls;
    };

    el.addEventListener("wheel", handler, { passive: false });
    return () => {
      el.removeEventListener("wheel", handler);
      animRef.current?.stop();
    };
  }, [scrollElementRef]);
}

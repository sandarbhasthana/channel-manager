// Shared "time travel not allowed" feedback — toast notification + bar shake animation.

let _toastEl: HTMLDivElement | null = null;
let _toastTimer: ReturnType<typeof setTimeout> | null = null;
let _lastTriggerAt = 0;
const THROTTLE_MS = 1800;

export function showTimeTravelToast(): void {
  if (typeof document === "undefined") return;
  const now = Date.now();
  if (now - _lastTriggerAt < THROTTLE_MS) return;
  _lastTriggerAt = now;

  if (!_toastEl) {
    _toastEl = document.createElement("div");
    Object.assign(_toastEl.style, {
      position: "fixed",
      bottom: "80px",
      left: "50%",
      transform: "translateX(-50%) translateY(8px)",
      background: "rgba(15,23,42,0.90)",
      color: "#f1f5f9",
      padding: "7px 16px",
      borderRadius: "8px",
      fontSize: "12px",
      fontWeight: "600",
      fontFamily: "system-ui, sans-serif",
      letterSpacing: "0.025em",
      boxShadow: "0 4px 16px rgba(0,0,0,0.28)",
      zIndex: "99999",
      opacity: "0",
      transition: "opacity 180ms ease, transform 180ms ease",
      pointerEvents: "none",
      whiteSpace: "nowrap",
      userSelect: "none",
    });
    _toastEl.textContent = "Time travel not allowed";
    document.body.appendChild(_toastEl);
  }

  if (_toastTimer) clearTimeout(_toastTimer);
  void _toastEl.offsetHeight; // reset transition
  _toastEl.style.opacity = "1";
  _toastEl.style.transform = "translateX(-50%) translateY(0)";

  _toastTimer = setTimeout(() => {
    if (_toastEl) {
      _toastEl.style.opacity = "0";
      _toastEl.style.transform = "translateX(-50%) translateY(-6px)";
    }
  }, 1800);
}

let _shakeInjected = false;

export function ensureShakeKeyframe(): void {
  if (_shakeInjected || typeof document === "undefined") return;
  _shakeInjected = true;
  const s = document.createElement("style");
  // Uses translateX only — safe to apply on elements that have a separate transform: scale().
  s.textContent =
    "@keyframes cal-bar-shake{" +
    "0%,100%{transform:translateX(0)}" +
    "12%{transform:translateX(-5px)}" +
    "25%{transform:translateX(5px)}" +
    "37%{transform:translateX(-4px)}" +
    "50%{transform:translateX(4px)}" +
    "62%{transform:translateX(-2px)}" +
    "75%{transform:translateX(2px)}" +
    "87%{transform:translateX(-1px)}" +
    "}";
  document.head.appendChild(s);
}

export function triggerBarShake(el: HTMLElement): void {
  el.style.animation = "none";
  void el.offsetHeight; // force reflow so next animation starts fresh
  el.style.animation = "cal-bar-shake 0.38s ease";
  el.addEventListener(
    "animationend",
    () => {
      el.style.animation = "";
    },
    { once: true },
  );
}

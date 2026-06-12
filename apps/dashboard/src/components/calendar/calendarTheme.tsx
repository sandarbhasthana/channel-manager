/**
 * Centralised colour tokens for the custom calendar grid.
 *
 * TAILWIND TOKENS  — combined light + dark class strings (use in className=).
 * INLINE COLOURS   — hex / rgba / CSS-variable strings (use in style={}).
 *
 * Edit this file to retheme the entire calendar without touching individual
 * components.  Dimensions (row heights, column widths, etc.) live in
 * calendarConfig.ts.
 */

// ─── Tailwind class tokens ────────────────────────────────────────────────────

export const CALENDAR_THEME = {
  /** Structural border shared by every cell, label, and header column */
  border: "border-0 border-solid border-slate-200 dark:border-[#1e2c49]",

  /** Default white surface: rooms column, header row, room labels */
  surface: "bg-white dark:bg-[#101a2e]",

  // ── Individual room × date cells ──────────────────────────────────────────
  cell: {
    today: "bg-blue-50/40 dark:bg-blue-900/20",
    weekend: "bg-slate-50 dark:bg-[#15213a]/50",
    selecting:
      "bg-blue-100 dark:bg-blue-900/50 border-0 border-solid border-blue-300 dark:border-blue-700",
  },

  // ── Room-type group header row ─────────────────────────────────────────────
  roomTypeRow: {
    bg: "bg-white dark:bg-[#101a2e]",
    today: "bg-blue-50/60 dark:bg-[#15213a]/80",
    weekend: "bg-slate-50/80 dark:bg-[#15213a]/80",
    occupancyText:
      "w-[7ch] max-w-full text-center tabular-nums text-slate-500 dark:text-[#8d9ab6]",
    rateBtn:
      "text-blue-600 hover:text-blue-700 hover:underline dark:text-blue-400 dark:hover:text-blue-300",
  },

  // ── Sticky date header row ─────────────────────────────────────────────────
  header: {
    base: "bg-white dark:bg-[#101a2e] border-0 border-solid border-b border-r border-slate-200 dark:border-[#1e2c49]",
    today: "bg-blue-50 dark:bg-[#15213a]",
    weekend: "bg-slate-50 dark:bg-[#0b1322]",
    weekdayToday: "text-blue-600 dark:text-blue-400",
    weekdayWeekend: "text-slate-500 dark:text-[#8d9ab6]",
    weekdayNormal: "text-slate-400 dark:text-[#5b6987]",
    dayNumberToday: "text-blue-700 dark:text-blue-300",
    dayNumberNormal: "text-slate-700 dark:text-[#e8edf7]",
    monthLabel: "text-slate-400 dark:text-[#5b6987]",
    holidayIcon: "text-amber-500 dark:text-amber-300",
  },

  // ── Rooms (left) sticky column ─────────────────────────────────────────────
  roomsColumn: {
    bg: "bg-white dark:bg-[#101a2e]",
  },

  // ── Full-height virtual column tone strips (CalendarDateGrid overlay) ──────
  dateGridColTone: {
    today: "bg-blue-50/30 dark:bg-[#15213a]/40",
    weekend: "bg-slate-50/60 dark:bg-[#0b1322]/40",
  },

  // ── Group header row rendered inline in CalendarDateGrid ──────────────────
  dateGridGroupRow: {
    bg: "bg-white dark:bg-[#101a2e] border-0 border-solid border-b border-slate-200 dark:border-[#1e2c49]",
    occupancyText:
      "w-[7ch] max-w-full text-center tabular-nums text-slate-500 dark:text-[#8d9ab6]",
    rateBtn: "text-blue-600 hover:underline dark:text-blue-400",
    rateSkeleton: "bg-slate-200 dark:bg-[#1e2c49]",
  },

  // ── Room row highlights rendered inline in CalendarDateGrid ───────────────
  dateGridRoomRow: {
    border: "border-0 border-solid border-b border-slate-200 dark:border-[#1e2c49]",
    todayHighlight: "bg-blue-50/40 dark:bg-[#15213a]/60",
    dragSelect:
      "bg-blue-100/80 dark:bg-blue-900/50 border-0 border-solid border-l border-r border-blue-300 dark:border-blue-700",
    skeleton: "bg-slate-200 dark:bg-[#1e2c49]",
  },

  // ── Individual room label ──────────────────────────────────────────────────
  roomLabel: {
    base: "border-0 border-solid border-b border-r border-slate-200 dark:border-[#1e2c49]",
    selected: "bg-blue-100 dark:bg-[#15213a]",
    default: "bg-white dark:bg-[#101a2e]",
    hover: "hover:bg-slate-50 dark:hover:bg-[#15213a]",
    selectedText: "font-semibold text-blue-800 dark:text-blue-200",
    defaultText: "text-slate-700 dark:text-[#e8edf7]",
  },

  // ── Room-type group label ──────────────────────────────────────────────────
  roomTypeLabel: {
    base: "bg-white dark:bg-[#101a2e] border-0 border-solid border-b border-r border-slate-200 dark:border-[#1e2c49]",
    hover: "hover:bg-blue-50 dark:hover:bg-[#15213a]",
    title: "font-bold text-sm uppercase tracking-wide text-blue-800 dark:text-blue-300",
    addRoomBtn:
      "bg-blue-100 hover:bg-blue-200 dark:bg-[#15213a] dark:hover:bg-[#1e2c49] text-blue-700 dark:text-blue-200",
    roomCountBadge: "bg-blue-100 dark:bg-[#1e2c49] text-blue-700 dark:text-blue-200",
  },
} as const;

export const CALENDAR_CARET_CLASS =
  "flex h-6 w-6 shrink-0 items-center justify-center rounded text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600 dark:text-[#8d9ab6] dark:hover:bg-[#1e2c49] dark:hover:text-[#e8edf7]";

// ─── Inline-style colour constants ────────────────────────────────────────────

export const CALENDAR_COLORS = {
  // ── Block / out-of-order event ─────────────────────────────────────────────
  block: {
    /** Diagonal stripe pattern for maintenance / blocked rooms */
    gradient:
      "repeating-linear-gradient(45deg, #ff1744, #ff1744 6px, #ff4081 6px, #ff4081 20px)",
  },

  // ── Payment status indicator ───────────────────────────────────────────────
  payment: {
    PAID: "#22c55e",
    PARTIALLY_PAID: "#f59e0b",
    UNPAID: "#ef4444",
    REFUND_DUE: "#3b82f6",
  } as Record<string, string>,

  /** Unread-message notification colour */
  unreadDot: "#ef4444",

  // ── Drag-move position indicators ─────────────────────────────────────────
  dragGhost: "rgba(100, 116, 139, 0.35)",

  dragIndicator: {
    room: "rgba(100, 116, 139, 0.9)",
    group: "rgba(37, 99, 235, 0.9)", // Changed to blue to match channel-manager
  },

  invalidDrop: {
    bg: "rgba(239, 68, 68, 0.18)",
    outline: "rgba(239, 68, 68, 0.8)",
  },

  gridLineSoft: "var(--cal-grid-line-soft)",

  bar: {
    shimmer:
      "linear-gradient(90deg, transparent 0%, rgba(255,255,255,0.38) 50%, transparent 100%)",
    resizeEdge: "rgba(255,255,255,0.85)",
    resizeEdgeShadow: "0 0 4px rgba(0,0,0,0.25)",
    shadowDefault: "0 1px 3px rgba(0,0,0,0.15)",
    shadowDrag: "0 4px 12px rgba(0,0,0,0.3)",
    shadowLongPress:
      "0 0 0 2px rgba(255,255,255,0.85), 0 4px 12px rgba(0,0,0,0.2)",
    ghostShadow: "0 8px 28px rgba(0,0,0,0.28), 0 2px 8px rgba(0,0,0,0.14)",
  },

  gridLine: "var(--cal-grid-line)",
} as const;

// ── Calendar Grid Dimensions ─────────────────────────────────────────────────
// Edit these values to resize the calendar grid globally.

// Row heights (px)
export const ROW_HEIGHT = 62;           // individual room row
export const TYPE_ROW_HEIGHT = 62;      // room-type group header row
export const HEADER_ROW_HEIGHT = 82;    // sticky date header row

// Event bar sizing — derived from ROW_HEIGHT
export const BAR_TOP = 7;
export const BAR_HEIGHT = ROW_HEIGHT - BAR_TOP * 2; // 44px

// Group-row label heights (px)
export const OCCUPANCY_LABEL_HEIGHT = 20;
export const RATE_LABEL_HEIGHT = 20;

// Column widths (px)
export const LEFT_COLUMN_WIDTH = 280;       // default rooms column (desktop)
export const MIN_CELL_WIDTH = 40;           // minimum date cell width
export const MIN_ROOMS_COLUMN_WIDTH = 60;
export const MAX_ROOMS_COLUMN_WIDTH = 480;

// File: src/app/dashboard/bookings/utils/eventColors.ts
/**
 * Event color utilities for calendar reservations
 * Provides theme-aware color mapping for different reservation statuses
 */

export interface EventColors {
  backgroundColor: string;
  textColor: string;
}

interface ColorConfig {
  bg: string;
  text: string;
}

/**
 * Light mode color mapping for reservation statuses
 */
const lightColorMap: Record<string, ColorConfig> = {
  CONFIRMED: { bg: "#6c956e", text: "#1f2937" }, // Green with gray-900 text
  CONFIRMATION_PENDING: { bg: "#ec4899", text: "#f0f8f9" }, // Pink with alice blue text
  CHECKIN_DUE: { bg: "#0ea5e9", text: "#1f2937" }, // Sky blue with gray-900 text
  IN_HOUSE: { bg: "#22c55e", text: "#f0f8f9" }, // Green with alice blue text
  CHECKOUT_DUE: { bg: "#f59e0b", text: "#1f2937" }, // Amber with gray-900 text
  CANCELLED: { bg: "#6b7280", text: "#f0f8f9" }, // Gray with alice blue text
  CHECKED_OUT: { bg: "#8b5cf6", text: "#f0f8f9" }, // Purple with alice blue text
  NO_SHOW: { bg: "#b91c1c", text: "#f0f8f9" } // Brick red with alice blue text
};

/**
 * Dark mode color mapping for reservation statuses
 */
const darkColorMap: Record<string, ColorConfig> = {
  CONFIRMED: { bg: "#3b513b", text: "#f0f8f9" }, // Sage Green (dark) with alice blue text
  CONFIRMATION_PENDING: { bg: "#db2777", text: "#f0f8f9" }, // Pink-600 with alice blue text
  CHECKIN_DUE: { bg: "#0284c7", text: "#f0f8f9" }, // Sky-600 with alice blue text
  IN_HOUSE: { bg: "#10b981", text: "#f0f8f9" }, // Emerald-500 with alice blue text
  CHECKOUT_DUE: { bg: "#b45309", text: "#f0f8f9" }, // Amber-800 with alice blue text
  CANCELLED: { bg: "#4b5563", text: "#f0f8f9" }, // Gray-700 with alice blue text
  CHECKED_OUT: { bg: "#7c3aed", text: "#f0f8f9" }, // Violet-600 with alice blue text
  NO_SHOW: { bg: "#991b1b", text: "#f0f8f9" } // Brick red (dark) with alice blue text
};

/**
 * Default colors for unknown statuses
 */
const defaultColors = {
  light: { bg: "#6c956e", text: "#1f2937" }, // Green with gray-900 text
  dark: { bg: "#047857", text: "#f0f8f9" } // Emerald-700 with alice blue text
};

/**
 * Get event colors based on reservation status and theme
 * @param status - Reservation status (CONFIRMED, IN_HOUSE, etc.)
 * @param isDarkMode - Whether dark mode is active
 * @returns Object containing backgroundColor and textColor
 */
export function getEventColor(
  status?: string,
  isDarkMode: boolean = false
): EventColors {
  const colorMap = isDarkMode ? darkColorMap : lightColorMap;
  const colors =
    colorMap[status || "CONFIRMED"] ||
    (isDarkMode ? defaultColors.dark : defaultColors.light);

  return {
    backgroundColor: colors.bg,
    textColor: colors.text
  };
}

/**
 * Get all available status colors for legend/documentation
 * @param isDarkMode - Whether dark mode is active
 * @returns Record of status to color mapping
 */
export function getAllStatusColors(
  isDarkMode: boolean = false
): Record<string, EventColors> {
  const colorMap = isDarkMode ? darkColorMap : lightColorMap;
  const result: Record<string, EventColors> = {};

  for (const [status, colors] of Object.entries(colorMap)) {
    result[status] = {
      backgroundColor: colors.bg,
      textColor: colors.text
    };
  }

  return result;
}

/**
 * Get color for block events
 * @param blockType - Type of block (MAINTENANCE, OUT_OF_ORDER, etc.)
 * @param isDarkMode - Whether dark mode is active
 * @returns Object containing backgroundColor and textColor
 */
export function getBlockColor(
  blockType: string,
  isDarkMode: boolean = false
): EventColors {
  const blockColors = {
    light: {
      MAINTENANCE: { bg: "#f59e0b", text: "#1f2937" }, // Amber
      OUT_OF_ORDER: { bg: "#ef4444", text: "#f0f8f9" }, // Red
      RESERVED: { bg: "#8b5cf6", text: "#f0f8f9" }, // Purple
      OTHER: { bg: "#6b7280", text: "#f0f8f9" } // Gray
    },
    dark: {
      MAINTENANCE: { bg: "#b45309", text: "#f0f8f9" }, // Amber-800
      OUT_OF_ORDER: { bg: "#dc2626", text: "#f0f8f9" }, // Red-600
      RESERVED: { bg: "#7c3aed", text: "#f0f8f9" }, // Violet-600
      OTHER: { bg: "#4b5563", text: "#f0f8f9" } // Gray-700
    }
  };

  const colors = isDarkMode ? blockColors.dark : blockColors.light;
  const color = colors[blockType as keyof typeof colors] || colors.OTHER;

  return {
    backgroundColor: color.bg,
    textColor: color.text
  };
}

/**
 * Status labels for display
 */
export const STATUS_LABELS: Record<string, string> = {
  CONFIRMED: "Confirmed",
  CONFIRMATION_PENDING: "Pending Confirmation",
  CHECKIN_DUE: "Check-in Due",
  IN_HOUSE: "In House",
  CHECKOUT_DUE: "Check-out Due",
  CANCELLED: "Cancelled",
  CHECKED_OUT: "Checked Out",
  NO_SHOW: "No Show"
};


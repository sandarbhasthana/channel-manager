import { type ChannelKind } from "@/lib/api";
import { cn } from "@/lib/utils";

interface OtaLogoProps {
  kind: ChannelKind;
  size?: number;
  className?: string;
}

function AirbnbLogo({ size }: { size: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 40 40" fill="none" aria-label="Airbnb">
      <rect width="40" height="40" rx="10" fill="#FF385C" />
      {/* Bélo mark — simplified */}
      <path
        d="M20 8c-2 0-3.5 1.6-3.5 3.5 0 1.2.6 2.3 1.5 3l2 2.5 2-2.5c.9-.7 1.5-1.8 1.5-3C23.5 9.6 22 8 20 8z"
        fill="white"
      />
      <path
        d="M26 20c0-1.5-1-2.8-2.4-3.4l-3.6 4.5-3.6-4.5C14.8 17.2 14 18.5 14 20c0 2.5 2.7 5.2 6 7.5 3.3-2.3 6-5 6-7.5z"
        fill="white"
      />
    </svg>
  );
}

function BookingLogo({ size }: { size: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 40 40" fill="none" aria-label="Booking.com">
      <rect width="40" height="40" rx="10" fill="#003580" />
      <text x="8" y="28" fontSize="22" fontWeight="900" fill="white" fontFamily="Arial, sans-serif">B.</text>
    </svg>
  );
}

function ExpediaLogo({ size }: { size: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 40 40" fill="none" aria-label="Expedia">
      <rect width="40" height="40" rx="10" fill="#FFC72C" />
      {/* Stylised E */}
      <rect x="11" y="11" width="18" height="4" rx="1.5" fill="#00355F" />
      <rect x="11" y="18" width="14" height="4" rx="1.5" fill="#00355F" />
      <rect x="11" y="25" width="18" height="4" rx="1.5" fill="#00355F" />
    </svg>
  );
}

function AgodaLogo({ size }: { size: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 40 40" fill="none" aria-label="Agoda">
      <rect width="40" height="40" rx="10" fill="#D0021B" />
      {/* stylised 'a' */}
      <text x="9" y="29" fontSize="24" fontWeight="900" fill="white" fontFamily="Arial, sans-serif">a</text>
    </svg>
  );
}

function DirectLogo({ size }: { size: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 40 40" fill="none" aria-label="Direct">
      <rect width="40" height="40" rx="10" fill="#2563EB" />
      {/* Globe icon */}
      <circle cx="20" cy="20" r="9" stroke="white" strokeWidth="2" />
      <ellipse cx="20" cy="20" rx="4" ry="9" stroke="white" strokeWidth="2" />
      <line x1="11" y1="20" x2="29" y2="20" stroke="white" strokeWidth="2" />
    </svg>
  );
}

function CustomLogo({ size }: { size: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 40 40" fill="none" aria-label="Custom">
      <rect width="40" height="40" rx="10" fill="#64748B" />
      {/* Plug icon */}
      <rect x="17" y="10" width="3" height="6" rx="1" fill="white" />
      <rect x="20" y="10" width="3" height="6" rx="1" fill="white" />
      <path d="M13 16h14v6a7 7 0 01-14 0v-6z" fill="white" opacity="0.9" />
      <rect x="19" y="22" width="2" height="8" rx="1" fill="white" />
    </svg>
  );
}

export function OtaLogo({ kind, size = 40, className }: OtaLogoProps) {
  return (
    <span className={cn("shrink-0", className)}>
      {kind === "CHANNEL_KIND_AIRBNB" && <AirbnbLogo size={size} />}
      {kind === "CHANNEL_KIND_BOOKING_COM" && <BookingLogo size={size} />}
      {kind === "CHANNEL_KIND_EXPEDIA" && <ExpediaLogo size={size} />}
      {kind === "CHANNEL_KIND_AGODA" && <AgodaLogo size={size} />}
      {kind === "CHANNEL_KIND_DIRECT" && <DirectLogo size={size} />}
      {(kind === "CHANNEL_KIND_UNSPECIFIED" || !kind) && <CustomLogo size={size} />}
    </span>
  );
}

export const OTA_DISPLAY: Record<ChannelKind, string> = {
  CHANNEL_KIND_UNSPECIFIED: "Custom",
  CHANNEL_KIND_AIRBNB: "Airbnb",
  CHANNEL_KIND_BOOKING_COM: "Booking.com",
  CHANNEL_KIND_EXPEDIA: "Expedia",
  CHANNEL_KIND_AGODA: "Agoda",
  CHANNEL_KIND_DIRECT: "Direct Booking",
};

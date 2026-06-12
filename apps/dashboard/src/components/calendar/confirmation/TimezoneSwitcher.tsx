"use client";

import React from "react";

interface TimezoneOption {
  value: string;
  label: string;
}

interface TimezoneSwitcherProps {
  value: string;
  onChange: (timezone: string) => void;
  propertyTimezone?: string;
  className?: string;
}

const COMMON_TIMEZONES = [
  "UTC",
  "Asia/Kolkata",
  "America/New_York",
  "America/Los_Angeles",
  "Europe/London",
  "Europe/Paris",
  "Asia/Dubai",
  "Asia/Singapore",
  "Australia/Sydney",
];

function getLocalTimezone() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
}

function uniqueOptions(propertyTimezone?: string): TimezoneOption[] {
  const localTimezone = getLocalTimezone();
  const values = [
    propertyTimezone || "UTC",
    localTimezone,
    ...COMMON_TIMEZONES,
  ].filter(Boolean);

  return Array.from(new Set(values)).map((timezone) => {
    if (timezone === propertyTimezone) {
      return { value: timezone, label: `Property (${timezone})` };
    }
    if (timezone === localTimezone) {
      return { value: timezone, label: `Local (${timezone})` };
    }
    return { value: timezone, label: timezone };
  });
}

export function TimezoneSwitcher({
  value,
  onChange,
  propertyTimezone,
  className = "",
}: TimezoneSwitcherProps) {
  const options = React.useMemo(
    () => uniqueOptions(propertyTimezone),
    [propertyTimezone],
  );

  return (
    <label className={`flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400 ${className}`}>
      <span className="shrink-0 pr-4">Timezone</span>
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="cursor-pointer rounded-none border-0 border-b border-slate-300 bg-transparent pb-0.5 pl-0 pr-1 text-xs text-slate-600 outline-none transition-colors hover:border-slate-400 focus:border-purple-400 dark:border-slate-600 dark:text-slate-300 dark:hover:border-slate-500 dark:focus:border-purple-400"
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </label>
  );
}

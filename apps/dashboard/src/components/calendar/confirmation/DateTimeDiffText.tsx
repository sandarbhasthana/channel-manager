"use client";

import React from "react";

type DateTimeField = "year" | "month" | "day" | "hour" | "minute";

interface DateTimeParts {
  year: string;
  month: string;
  day: string;
  hour: string;
  minute: string;
}

interface DateTimeDiffTextProps {
  value: string;
  compareTo: string;
  timezone: string;
  showTime?: boolean;
  className?: string;
}

const FIELD_ORDER: DateTimeField[] = ["year", "month", "day", "hour", "minute"];

const DATE_ONLY_RE = /^(\d{4})-(\d{2})-(\d{2})$/;

function getDateTimeParts(value: string, timezone: string): DateTimeParts {
  const m = DATE_ONLY_RE.exec(value);
  if (m) {
    return { year: m[1], month: m[2], day: m[3], hour: "", minute: "" };
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return { year: value, month: "", day: "", hour: "", minute: "" };
  }

  const formatter = new Intl.DateTimeFormat("en-CA", {
    timeZone: timezone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  });

  const parts = formatter.formatToParts(date);
  const lookup = Object.fromEntries(parts.map((part) => [part.type, part.value]));

  return {
    year: lookup.year ?? "",
    month: lookup.month ?? "",
    day: lookup.day ?? "",
    hour: lookup.hour ?? "",
    minute: lookup.minute ?? "",
  };
}

function getMutedFields(value: DateTimeParts, compareTo: DateTimeParts) {
  const muted = new Set<DateTimeField>();

  for (const field of FIELD_ORDER) {
    if (value[field] !== compareTo[field]) break;
    muted.add(field);
  }

  return muted;
}

function Segment({
  children,
  muted,
}: {
  children: React.ReactNode;
  muted: boolean;
}) {
  return (
    <span className={muted ? "text-slate-400 dark:text-slate-500" : undefined}>
      {children}
    </span>
  );
}

export function DateTimeDiffText({
  value,
  compareTo,
  timezone,
  showTime = false,
  className = "",
}: DateTimeDiffTextProps) {
  const valueParts = React.useMemo(
    () => getDateTimeParts(value, timezone),
    [value, timezone],
  );
  const compareParts = React.useMemo(
    () => getDateTimeParts(compareTo, timezone),
    [compareTo, timezone],
  );
  const mutedFields = React.useMemo(
    () => getMutedFields(valueParts, compareParts),
    [valueParts, compareParts],
  );

  const hasRealTime = valueParts.hour !== "";
  const showTimePart = hasRealTime || showTime;
  const displayHour = hasRealTime ? valueParts.hour : "00";
  const displayMinute = hasRealTime ? valueParts.minute : "00";
  // Synthesized time (no real data in source) is always muted
  const hourMuted = !hasRealTime || mutedFields.has("hour");
  const minuteMuted = !hasRealTime || mutedFields.has("minute");

  return (
    <span className={`whitespace-nowrap font-mono tabular-nums ${className}`}>
      <Segment muted={mutedFields.has("year")}>{valueParts.year}</Segment>
      -
      <Segment muted={mutedFields.has("month")}>{valueParts.month}</Segment>
      -
      <Segment muted={mutedFields.has("day")}>{valueParts.day}</Segment>
      {showTimePart && (
        <>
          {" "}
          <Segment muted={hourMuted}>{displayHour}</Segment>
          :
          <Segment muted={minuteMuted}>{displayMinute}</Segment>
        </>
      )}
    </span>
  );
}

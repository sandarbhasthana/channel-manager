import React from "react";
import { Tooltip } from "antd";

interface HolidayTooltipProps {
  holidayName: string;
  children: React.ReactNode;
}

export function HolidayTooltip({ holidayName, children }: HolidayTooltipProps) {
  return (
    <Tooltip title={holidayName} placement="top">
      {children}
    </Tooltip>
  );
}

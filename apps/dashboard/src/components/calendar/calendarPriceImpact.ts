type CalendarRateCell = {
  basePrice: number;
  finalPrice: number;
  availability: number;
  isOverride: boolean;
  isSeasonal: boolean;
  restrictions: {
    closedToArrival: boolean;
    closedToDeparture: boolean;
  };
};

type CalendarRoomTypeRates = {
  roomTypeId: string;
  roomTypeName: string;
  totalRooms: number;
  dates: Record<string, CalendarRateCell>;
};

export type CalendarPriceImpact = {
  oldTotal: number;
  newTotal: number;
};

export function calculateCalendarPriceImpact(params: {
  depositAmount?: number;
  ratesData: CalendarRoomTypeRates[] | undefined;
  roomCategoryId: string | undefined;
  roomCategoryName: string | undefined;
  newStart: string;
  newEnd: string;
}): CalendarPriceImpact | null {
  const oldTotal =
    typeof params.depositAmount === "number" ? params.depositAmount / 100 : null;
  if (oldTotal === null || oldTotal <= 0) return null;

  const rates = params.ratesData?.find(
    (rate) =>
      rate.roomTypeId === params.roomCategoryId ||
      rate.roomTypeName === params.roomCategoryName
  );
  if (!rates) return null;

  let newTotal = 0;
  for (
    let cursor = new Date(`${params.newStart}T00:00:00Z`);
    cursor < new Date(`${params.newEnd}T00:00:00Z`);
    cursor.setUTCDate(cursor.getUTCDate() + 1)
  ) {
    const dateKey = cursor.toISOString().slice(0, 10);
    const price = rates.dates[dateKey]?.finalPrice;
    if (typeof price !== "number") return null;
    newTotal += price;
  }

  newTotal = Math.round(newTotal * 100) / 100;
  return Math.round(oldTotal * 100) === Math.round(newTotal * 100)
    ? null
    : { oldTotal, newTotal };
}

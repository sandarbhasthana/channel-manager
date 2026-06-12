import { calculateCalendarPriceImpact } from "./calendarPriceImpact";

describe("calendar price-impact verification", () => {
  it("CAL-010 calculates the price impact for a valid cross-room-type move with different pricing", () => {
    const priceImpact = calculateCalendarPriceImpact({
      depositAmount: 20000,
      ratesData: [
        {
          roomTypeId: "room-type-2",
          roomTypeName: "Suite",
          totalRooms: 1,
          dates: {
            "2026-06-15": {
              basePrice: 150,
              finalPrice: 150,
              availability: 1,
              isOverride: false,
              isSeasonal: false,
              restrictions: {
                closedToArrival: false,
                closedToDeparture: false
              }
            },
            "2026-06-16": {
              basePrice: 175,
              finalPrice: 175,
              availability: 1,
              isOverride: false,
              isSeasonal: false,
              restrictions: {
                closedToArrival: false,
                closedToDeparture: false
              }
            }
          }
        }
      ],
      roomCategoryId: "room-type-2",
      roomCategoryName: "Suite",
      newStart: "2026-06-15",
      newEnd: "2026-06-17"
    });

    expect(priceImpact).toEqual({
      oldTotal: 200,
      newTotal: 325
    });
  });

  it("CAL-010 suppresses the price review when target rates are incomplete", () => {
    const priceImpact = calculateCalendarPriceImpact({
      depositAmount: 20000,
      ratesData: [
        {
          roomTypeId: "room-type-2",
          roomTypeName: "Suite",
          totalRooms: 1,
          dates: {
            "2026-06-15": {
              basePrice: 150,
              finalPrice: 150,
              availability: 1,
              isOverride: false,
              isSeasonal: false,
              restrictions: {
                closedToArrival: false,
                closedToDeparture: false
              }
            }
          }
        }
      ],
      roomCategoryId: "room-type-2",
      roomCategoryName: "Suite",
      newStart: "2026-06-15",
      newEnd: "2026-06-17"
    });

    expect(priceImpact).toBeNull();
  });

  it("CAL-011 calculates the price impact for a date change that preserves the assigned room type", () => {
    const priceImpact = calculateCalendarPriceImpact({
      depositAmount: 20000,
      ratesData: [
        {
          roomTypeId: "room-type-1",
          roomTypeName: "Standard",
          totalRooms: 2,
          dates: {
            "2026-06-18": {
              basePrice: 120,
              finalPrice: 120,
              availability: 2,
              isOverride: false,
              isSeasonal: false,
              restrictions: {
                closedToArrival: false,
                closedToDeparture: false
              }
            },
            "2026-06-19": {
              basePrice: 180,
              finalPrice: 180,
              availability: 2,
              isOverride: true,
              isSeasonal: false,
              restrictions: {
                closedToArrival: false,
                closedToDeparture: false
              }
            }
          }
        }
      ],
      roomCategoryId: "room-type-1",
      roomCategoryName: "Standard",
      newStart: "2026-06-18",
      newEnd: "2026-06-20"
    });

    expect(priceImpact).toEqual({
      oldTotal: 200,
      newTotal: 300
    });
  });
});

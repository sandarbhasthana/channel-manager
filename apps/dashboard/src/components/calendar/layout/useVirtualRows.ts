"use client";

import { useMemo } from "react";
import type { Room } from "@/components/calendar/utils/types";
import type { VirtualRowItem } from "./virtualTypes";

export function useVirtualRows(
  resources: Room[],
  expandedGroups: Record<string, boolean>,
): VirtualRowItem[] {
  return useMemo(() => {
    const items: VirtualRowItem[] = [];
    for (const group of resources) {
      items.push({ type: "group", roomTypeId: group.id });
      if (expandedGroups[group.id] ?? true) {
        for (const room of group.children ?? []) {
          items.push({ type: "room", roomId: room.id, roomTypeId: group.id });
        }
      }
    }
    return items;
  }, [resources, expandedGroups]);
}

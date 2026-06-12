"use client";

export type VirtualRowItem =
  | { type: "group"; roomTypeId: string }
  | { type: "room";  roomId: string; roomTypeId: string };

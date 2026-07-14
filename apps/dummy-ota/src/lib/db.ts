import fs from "fs/promises";
import path from "path";

const DB_FILE = path.join(process.cwd(), "dummy-db.json");

export interface InventoryUpdate {
  room_type_id: string;
  date: string;
  available: number;
  stop_sell: boolean;
  min_stay: number;
  max_stay: number;
  provider: string; // e.g., "bookingcom" or "expedia"
}

export interface RateUpdate {
  room_type_id: string;
  date: string;
  price: number;
  currency: string;
  provider: string;
}

export interface Reservation {
  id: string;
  provider: string;
  status: "confirmed" | "cancelled" | "modified";
  check_in: string;
  check_out: string;
  guest_name: string;
  room_type_id: string;
  total_price: number;
  currency: string;
  created_at: string;
}

export interface OtaData {
  reservations: Reservation[];
  inventory: InventoryUpdate[];
  rates: RateUpdate[];
}

const defaultData: OtaData = {
  reservations: [],
  inventory: [],
  rates: [],
};

export async function readDb(): Promise<OtaData> {
  try {
    const data = await fs.readFile(DB_FILE, "utf-8");
    return JSON.parse(data) as OtaData;
  } catch (err) {
    return defaultData;
  }
}

export async function writeDb(data: OtaData): Promise<void> {
  await fs.writeFile(DB_FILE, JSON.stringify(data, null, 2), "utf-8");
}

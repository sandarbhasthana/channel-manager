import { cookies } from "next/headers";

const API_BASE = process.env.API_URL ?? "http://localhost:8080";

// ── proto-mapped types ─────────────────────────────────────────────────────

export type ChannelKind =
  | "CHANNEL_KIND_UNSPECIFIED"
  | "CHANNEL_KIND_BOOKING_COM"
  | "CHANNEL_KIND_AIRBNB"
  | "CHANNEL_KIND_EXPEDIA"
  | "CHANNEL_KIND_AGODA"
  | "CHANNEL_KIND_DIRECT";

export type ConnectionStatus =
  | "CONNECTION_STATUS_UNSPECIFIED"
  | "CONNECTION_STATUS_INACTIVE"
  | "CONNECTION_STATUS_ACTIVE"
  | "CONNECTION_STATUS_ERROR"
  | "CONNECTION_STATUS_DISABLED";

export interface Connection {
  id: string;
  kind: ChannelKind;
  name: string;
  status: ConnectionStatus;
  createdAt: string;
  updatedAt: string;
}

// ── auth-forwarding fetch ──────────────────────────────────────────────────

async function rpc<T>(procedure: string, body: unknown): Promise<T> {
  const cookieStore = await cookies();
  const res = await fetch(`${API_BASE}${procedure}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Cookie: cookieStore.toString(),
    },
    body: JSON.stringify(body),
    cache: "no-store",
  });

  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`RPC ${procedure} failed [${res.status}]: ${text}`);
  }
  return res.json() as Promise<T>;
}

// ── ConnectionService ──────────────────────────────────────────────────────

export async function listConnections(): Promise<Connection[]> {
  try {
    const data = await rpc<{ connections?: Connection[] }>(
      "/channel.v1.ConnectionService/ListConnections",
      {}
    );
    return data.connections ?? [];
  } catch {
    return [];
  }
}

export async function createConnection(input: {
  kind: ChannelKind;
  name: string;
  credentials: Record<string, string>;
}): Promise<Connection> {
  const data = await rpc<{ connection: Connection }>(
    "/channel.v1.ConnectionService/CreateConnection",
    input
  );
  return data.connection;
}

export async function updateConnectionStatus(
  id: string,
  status: ConnectionStatus
): Promise<Connection> {
  const data = await rpc<{ connection: Connection }>(
    "/channel.v1.ConnectionService/UpdateConnection",
    { id, status }
  );
  return data.connection;
}

export async function deleteConnection(id: string): Promise<void> {
  await rpc("/channel.v1.ConnectionService/DeleteConnection", { id });
}

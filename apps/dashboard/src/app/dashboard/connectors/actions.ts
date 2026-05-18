"use server";

import { revalidatePath } from "next/cache";
import {
  createConnection,
  updateConnectionStatus,
  deleteConnection,
  type ChannelKind,
  type ConnectionStatus,
} from "@/lib/api";

export async function createConnectionAction(formData: FormData) {
  const kind = formData.get("kind") as ChannelKind;
  const name = formData.get("name") as string;
  const apiKey = formData.get("apiKey") as string;
  const apiSecret = formData.get("apiSecret") as string;

  if (!kind || !name) {
    return { error: "Kind and name are required." };
  }

  const credentials: Record<string, string> = {};
  if (apiKey) credentials.api_key = apiKey;
  if (apiSecret) credentials.api_secret = apiSecret;

  try {
    await createConnection({ kind, name, credentials });
    revalidatePath("/dashboard/connectors");
    return { success: true };
  } catch (err) {
    return { error: (err as Error).message };
  }
}

export async function toggleConnectionAction(id: string, currentStatus: ConnectionStatus) {
  const nextStatus: ConnectionStatus =
    currentStatus === "CONNECTION_STATUS_ACTIVE"
      ? "CONNECTION_STATUS_DISABLED"
      : "CONNECTION_STATUS_ACTIVE";

  try {
    await updateConnectionStatus(id, nextStatus);
    revalidatePath("/dashboard/connectors");
  } catch (err) {
    throw new Error((err as Error).message);
  }
}

export async function deleteConnectionAction(id: string) {
  try {
    await deleteConnection(id);
    revalidatePath("/dashboard/connectors");
  } catch (err) {
    throw new Error((err as Error).message);
  }
}

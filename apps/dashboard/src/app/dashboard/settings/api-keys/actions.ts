"use server";

import { revalidatePath } from "next/cache";
import { unstable_rethrow } from "next/navigation";
import { createIntegrationKey, revokeIntegrationKey } from "@/lib/api";

export async function createKeyAction(formData: FormData) {
  const name = formData.get("name") as string;
  if (!name) return { error: "Name is required" };

  try {
    const result = await createIntegrationKey(name);
    revalidatePath("/dashboard/settings/api-keys");
    return { success: true, secretKey: result.secret_key };
  } catch (err) {
    unstable_rethrow(err);
    return { error: (err as Error).message };
  }
}

export async function revokeKeyAction(id: string) {
  try {
    await revokeIntegrationKey(id);
    revalidatePath("/dashboard/settings/api-keys");
    return { success: true };
  } catch (err) {
    unstable_rethrow(err);
    return { error: (err as Error).message };
  }
}

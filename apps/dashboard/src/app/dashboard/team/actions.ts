"use server";

import { revalidatePath } from "next/cache";
import {
  sendInvitation,
  revokeInvitation,
  removeMember,
} from "@/lib/team-api";

export async function sendInviteAction(email: string, role: string) {
  try {
    await sendInvitation(email, role);
    revalidatePath("/dashboard/team");
    return { success: true };
  } catch (err) {
    return { error: (err as Error).message };
  }
}

export async function revokeInviteAction(id: string) {
  try {
    await revokeInvitation(id);
    revalidatePath("/dashboard/team");
    return { success: true };
  } catch (err) {
    return { error: (err as Error).message };
  }
}

export async function removeMemberAction(membershipId: string) {
  try {
    await removeMember(membershipId);
    revalidatePath("/dashboard/team");
    return { success: true };
  } catch (err) {
    return { error: (err as Error).message };
  }
}

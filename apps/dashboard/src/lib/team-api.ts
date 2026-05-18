import { cookies } from "next/headers";

const API_BASE = process.env.API_URL ?? "http://localhost:8080";

async function authHeaders(): Promise<Record<string, string>> {
  const cookieStore = await cookies();
  return {
    "Content-Type": "application/json",
    Cookie: cookieStore.toString(),
  };
}

// ── Types ──────────────────────────────────────────────────────────────────

export interface TeamMember {
  id: string;
  userId: string;
  email: string;
  fullName: string;
  role: string;
  status: string;
  avatarUrl?: string;
}

export interface TeamInvitation {
  id: string;
  email: string;
  state: string;
  createdAt: string;
  expiresAt: string;
}

// ── Fetchers ───────────────────────────────────────────────────────────────

export async function listTeamMembers(): Promise<TeamMember[]> {
  try {
    const res = await fetch(`${API_BASE}/team/members`, {
      headers: await authHeaders(),
      cache: "no-store",
    });
    if (!res.ok) return [];
    const data = await res.json();
    return data.members ?? [];
  } catch {
    return [];
  }
}

export async function listInvitations(): Promise<TeamInvitation[]> {
  try {
    const res = await fetch(`${API_BASE}/team/invitations`, {
      headers: await authHeaders(),
      cache: "no-store",
    });
    if (!res.ok) return [];
    const data = await res.json();
    return data.invitations ?? [];
  } catch {
    return [];
  }
}

export async function sendInvitation(email: string, role: string): Promise<void> {
  const res = await fetch(`${API_BASE}/team/invite`, {
    method: "POST",
    headers: await authHeaders(),
    body: JSON.stringify({ email, role }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as { error?: string }).error ?? "Failed to send invitation");
  }
}

export async function revokeInvitation(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/team/revoke-invitation`, {
    method: "POST",
    headers: await authHeaders(),
    body: JSON.stringify({ id }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as { error?: string }).error ?? "Failed to revoke invitation");
  }
}

export async function removeMember(membershipId: string): Promise<void> {
  const res = await fetch(`${API_BASE}/team/remove-member`, {
    method: "POST",
    headers: await authHeaders(),
    body: JSON.stringify({ membershipId }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error((body as { error?: string }).error ?? "Failed to remove member");
  }
}

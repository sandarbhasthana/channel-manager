import { listTeamMembers, listInvitations } from "@/lib/team-api";
import { TeamManagement } from "@/components/team/team-management";

export const metadata = { title: "Team — Channel Manager" };
export const dynamic = "force-dynamic";

export default async function TeamPage() {
  const [members, invitations] = await Promise.all([
    listTeamMembers(),
    listInvitations(),
  ]);

  return (
    <TeamManagement
      initialMembers={members}
      initialInvitations={invitations}
    />
  );
}

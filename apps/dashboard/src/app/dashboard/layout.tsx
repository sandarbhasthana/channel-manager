import { DashboardShell } from "@/components/layout/dashboard-shell";
import { getMe } from "@/lib/api";

export default async function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const me = await getMe();
  const userEmail = me?.email || "admin@channel-manager.com";
  const defaultName = me?.email ? me.email.split('@')[0] : "Admin User";
  const userName = me?.full_name || defaultName;
  
  return <DashboardShell userEmail={userEmail} userName={userName}>{children}</DashboardShell>;
}

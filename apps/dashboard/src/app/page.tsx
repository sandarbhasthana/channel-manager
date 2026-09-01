import { cookies } from "next/headers";
import { redirect } from "next/navigation";

/**
 * Session-aware root. The old unconditional redirect("/login") bounced even
 * freshly authenticated users back to the login screen — the OAuth callback
 * and the password form both land here with valid cookies already set.
 *
 * Presence of the access_token cookie is enough to route optimistically: the
 * dashboard's own API calls are what actually validate the token, and an
 * expired one just fails those calls and returns the user to /login.
 */
export default async function Home() {
  const hasSession = (await cookies()).has("access_token");
  redirect(hasSession ? "/dashboard" : "/login");
}

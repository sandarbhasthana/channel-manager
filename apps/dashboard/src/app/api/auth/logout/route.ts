import { cookies } from "next/headers";
import { NextResponse } from "next/server";

export async function GET(request: Request) {
  const cookieStore = await cookies();
  cookieStore.delete("access_token");
  cookieStore.delete("refresh_token");

  // Redirect to the login page after logging out
  return NextResponse.redirect(new URL("/login", request.url));
}

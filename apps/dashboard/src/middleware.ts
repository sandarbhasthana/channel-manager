import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

// Passthrough middleware — reserved for auth guards in future phases.
export function middleware(_request: NextRequest) {
  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next|favicon.ico|api).*)"],
};

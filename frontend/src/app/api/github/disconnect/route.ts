import { NextResponse } from "next/server";
import { GH_COOKIE } from "@/lib/session";

export const dynamic = "force-dynamic";

export async function GET(request: Request) {
  const origin = new URL(request.url).origin;
  const res = NextResponse.redirect(`${origin}/connect`);
  res.cookies.delete(GH_COOKIE);
  return res;
}

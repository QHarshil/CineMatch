import { NextResponse } from "next/server";
import { createSupabaseServerClient } from "@/lib/supabase-server";

/**
 * Magic-link landing route: exchanges Supabase's ?code= for a session, then
 * redirects to ?next= (relative paths only) or /for-you.
 */
export async function GET(request: Request) {
  const { searchParams, origin } = new URL(request.url);
  const code = searchParams.get("code");
  const nextParam = searchParams.get("next") ?? "/for-you";
  // Prevent open redirect: only allow relative paths starting with /
  const next = nextParam.startsWith("/") && !nextParam.startsWith("//") ? nextParam : "/for-you";

  if (code) {
    const supabase = await createSupabaseServerClient();

    const { error } = await supabase.auth.exchangeCodeForSession(code);
    if (!error) {
      return NextResponse.redirect(`${origin}${next}`);
    }
  }

  return NextResponse.redirect(`${origin}/login?error=auth_callback_failed`);
}
